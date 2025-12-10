package reconciliation

import (
	"context"
	"sync"
	"time"

	"github.com/apahim/cls-backend/internal/config"
	"github.com/apahim/cls-backend/internal/database"
	"github.com/apahim/cls-backend/internal/models"
	"github.com/apahim/cls-backend/internal/pubsub"
	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Scheduler manages periodic reconciliation of clusters
type Scheduler struct {
	repository *database.Repository
	publisher  *pubsub.Publisher
	config     *config.ReconciliationConfig
	logger     *utils.Logger

	// Internal state
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// NewScheduler creates a new reconciliation scheduler
func NewScheduler(repository *database.Repository, publisher *pubsub.Publisher, cfg *config.ReconciliationConfig) *Scheduler {
	if cfg == nil {
		// Provide simplified default values - binary state model handles intervals in database
		defaultConfig := &config.ReconciliationConfig{
			Enabled:                    true,
			CheckInterval:              1 * time.Minute,
			MaxConcurrent:              50,
			DefaultInterval:            2 * time.Minute, // Fallback only, database handles actual intervals
			ReactiveEnabled:            false,
			ReactiveDebounce:           2 * time.Second,
			ReactiveMaxEventsPerMinute: 60,
		}
		cfg = defaultConfig
	}

	return &Scheduler{
		repository: repository,
		publisher:  publisher,
		config:     cfg,
		logger:     utils.NewLogger("reconciliation_scheduler"),
		stopChan:   make(chan struct{}),
	}
}

// Start starts the reconciliation scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil // Already running
	}

	if !s.config.Enabled {
		s.logger.Info("Reconciliation scheduler is disabled")
		return nil
	}

	// Simple validation - only check essential config
	if s.config.CheckInterval <= 0 {
		return utils.NewValidationError("INVALID_CHECK_INTERVAL", "check_interval must be positive", s.config.CheckInterval)
	}
	if s.config.DefaultInterval <= 0 {
		return utils.NewValidationError("INVALID_DEFAULT_INTERVAL", "default_interval must be positive", s.config.DefaultInterval)
	}
	if s.config.MaxConcurrent <= 0 {
		return utils.NewValidationError("INVALID_MAX_CONCURRENT", "max_concurrent must be positive", s.config.MaxConcurrent)
	}

	s.running = true
	s.logger.Info("Starting reconciliation scheduler with simplified binary state model",
		zap.Duration("check_interval", s.config.CheckInterval),
		zap.Duration("default_interval", s.config.DefaultInterval),
		zap.Int("max_concurrent", s.config.MaxConcurrent),
		zap.String("model", "binary_state_30s_5m"))

	s.wg.Add(1)
	go s.reconciliationLoop(ctx)

	return nil
}

// Stop stops the reconciliation scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.logger.Info("Stopping reconciliation scheduler")
	s.running = false
	close(s.stopChan)
	s.wg.Wait()
	s.logger.Info("Reconciliation scheduler stopped")
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// reconciliationLoop runs the main reconciliation loop
func (s *Scheduler) reconciliationLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	// Run initial reconciliation
	s.checkAndScheduleReconciliation(ctx)

	for {
		select {
		case <-ticker.C:
			s.checkAndScheduleReconciliation(ctx)
		case <-s.stopChan:
			s.logger.Info("Reconciliation loop stopping")
			return
		case <-ctx.Done():
			s.logger.Info("Reconciliation loop stopping due to context cancellation")
			return
		}
	}
}

// checkAndScheduleReconciliation finds clusters and nodepools needing reconciliation and publishes events
func (s *Scheduler) checkAndScheduleReconciliation(ctx context.Context) {
	start := time.Now()
	s.logger.Debug("Starting reconciliation check (clusters + nodepools)")

	var clusterPublishedEvents int
	var nodepoolPublishedEvents int
	var errors int

	// No complex health status updates needed with simplified binary model

	// Get all clusters needing reconciliation (fan-out approach)
	allTargets, err := s.repository.Reconciliation.FindClustersNeedingReconciliation(ctx)
	if err != nil {
		s.logger.Error("Failed to find clusters needing reconciliation", zap.Error(err))
	} else {
		// Group targets by cluster ID to avoid duplicate events
		clusterTargets := make(map[uuid.UUID]*models.ReconciliationTarget)
		for _, target := range allTargets {
			// Keep the most recent or highest priority target per cluster
			if existing, exists := clusterTargets[target.ClusterID]; !exists || target.ClusterGeneration > existing.ClusterGeneration {
				clusterTargets[target.ClusterID] = target
			}
		}

		// Apply global concurrency limit for clusters
		processed := 0
		for _, target := range clusterTargets {
			if processed >= s.config.MaxConcurrent {
				s.logger.Debug("Reached max concurrent cluster reconciliations",
					zap.Int("max", s.config.MaxConcurrent),
					zap.Int("remaining", len(clusterTargets)-processed))
				break
			}

			if s.publishReconciliationEvent(ctx, target) {
				clusterPublishedEvents++
			} else {
				errors++
			}
			processed++
		}
	}

	// Get all nodepools needing reconciliation
	nodepoolTargets, err := s.repository.Reconciliation.FindNodePoolsNeedingReconciliation(ctx)
	if err != nil {
		s.logger.Error("Failed to find nodepools needing reconciliation", zap.Error(err))
	} else {
		// Group targets by nodepool ID to avoid duplicate events
		nodepoolMap := make(map[uuid.UUID]*models.NodePoolReconciliationTarget)
		for _, target := range nodepoolTargets {
			// Keep the most recent or highest priority target per nodepool
			if existing, exists := nodepoolMap[target.NodePoolID]; !exists || target.NodePoolGeneration > existing.NodePoolGeneration {
				nodepoolMap[target.NodePoolID] = target
			}
		}

		// Apply global concurrency limit for nodepools
		processed := 0
		for _, target := range nodepoolMap {
			if processed >= s.config.MaxConcurrent {
				s.logger.Debug("Reached max concurrent nodepool reconciliations",
					zap.Int("max", s.config.MaxConcurrent),
					zap.Int("remaining", len(nodepoolMap)-processed))
				break
			}

			if s.publishNodePoolReconciliationEvent(ctx, target) {
				nodepoolPublishedEvents++
			} else {
				errors++
			}
			processed++
		}
	}

	duration := time.Since(start)
	s.logger.Info("Reconciliation check completed",
		zap.Duration("duration", duration),
		zap.Int("cluster_events", clusterPublishedEvents),
		zap.Int("nodepool_events", nodepoolPublishedEvents),
		zap.Int("errors", errors))
}

// publishReconciliationEvent publishes a reconciliation event for a target
func (s *Scheduler) publishReconciliationEvent(ctx context.Context, target *models.ReconciliationTarget) bool {
	event := &models.ReconciliationEvent{
		Type:       "cluster.reconcile",
		ClusterID:  target.ClusterID.String(),
		Reason:     target.Reason,
		Generation: target.ClusterGeneration,
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"scheduled_by":       "reconciliation_scheduler",
			"last_reconciled_at": target.LastReconciledAt,
			"cluster_generation": target.ClusterGeneration,
		},
	}

	if err := s.publisher.PublishReconciliationEvent(ctx, event); err != nil {
		s.logger.Error("Failed to publish reconciliation event",
			zap.String("cluster_id", target.ClusterID.String()),
			zap.String("reason", target.Reason),
			zap.Error(err))
		return false
	}

	// Update reconciliation schedule using simplified logic
	if err := s.repository.Reconciliation.UpdateReconciliationSchedule(ctx, target.ClusterID); err != nil {
		s.logger.Warn("Failed to update reconciliation schedule after publishing event",
			zap.String("cluster_id", target.ClusterID.String()),
			zap.Error(err))
		// Don't return false here - the event was published successfully
	}

	s.logger.Debug("Published reconciliation event",
		zap.String("cluster_id", target.ClusterID.String()),
		zap.String("reason", target.Reason))

	return true
}

// publishNodePoolReconciliationEvent publishes a nodepool reconciliation event
func (s *Scheduler) publishNodePoolReconciliationEvent(ctx context.Context, target *models.NodePoolReconciliationTarget) bool {
	// Get nodepool to fetch cluster_id
	nodepool, err := s.repository.NodePools.GetByIDInternal(ctx, target.NodePoolID)
	if err != nil {
		s.logger.Error("Failed to get nodepool for reconciliation event",
			zap.String("nodepool_id", target.NodePoolID.String()),
			zap.Error(err))
		return false
	}

	event := &models.NodePoolReconciliationEvent{
		Type:       "nodepool.reconcile",
		ClusterID:  nodepool.ClusterID.String(),
		NodePoolID: target.NodePoolID.String(),
		Reason:     target.Reason,
		Generation: target.NodePoolGeneration,
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"scheduled_by":        "reconciliation_scheduler",
			"last_reconciled_at":  target.LastReconciledAt,
			"nodepool_generation": target.NodePoolGeneration,
		},
	}

	if err := s.publisher.PublishNodePoolReconciliationEvent(ctx, event); err != nil {
		s.logger.Error("Failed to publish nodepool reconciliation event",
			zap.String("nodepool_id", target.NodePoolID.String()),
			zap.String("reason", target.Reason),
			zap.Error(err))
		return false
	}

	// Update reconciliation schedule
	if err := s.repository.Reconciliation.UpdateNodePoolReconciliationSchedule(ctx, target.NodePoolID); err != nil {
		s.logger.Warn("Failed to update nodepool reconciliation schedule after publishing event",
			zap.String("nodepool_id", target.NodePoolID.String()),
			zap.Error(err))
		// Don't return false here - the event was published successfully
	}

	s.logger.Debug("Published nodepool reconciliation event",
		zap.String("nodepool_id", target.NodePoolID.String()),
		zap.String("cluster_id", nodepool.ClusterID.String()),
		zap.String("reason", target.Reason))

	return true
}

// TriggerReconciliation manually triggers reconciliation for a specific cluster
func (s *Scheduler) TriggerReconciliation(ctx context.Context, clusterID string) error {
	s.logger.Info("Manual reconciliation triggered",
		zap.String("cluster_id", clusterID))

	// Parse cluster ID to validate it
	_, err := uuid.Parse(clusterID)
	if err != nil {
		return err
	}

	// For manual triggers, we don't need to look up the generation since controllers
	// will handle generation validation. Use generation 0 to indicate "any generation"
	// which allows controllers to process regardless of current cluster generation.

	// Publish immediate reconciliation event (fan-out to all controllers)
	event := &models.ReconciliationEvent{
		Type:       "cluster.reconcile",
		ClusterID:  clusterID,
		Reason:     "manual_trigger",
		Generation: 0, // Use 0 for manual triggers - controllers will validate current generation
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"scheduled_by": "manual_trigger",
			"trigger_type": "immediate",
		},
	}

	if err := s.publisher.PublishReconciliationEvent(ctx, event); err != nil {
		s.logger.Error("Failed to publish manual reconciliation event",
			zap.String("cluster_id", clusterID),
			zap.Error(err))
		return err
	}

	s.logger.Info("Published manual reconciliation event",
		zap.String("cluster_id", clusterID))

	return nil
}

// GetStats returns reconciliation scheduler statistics
func (s *Scheduler) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := map[string]interface{}{
		"running":          s.IsRunning(),
		"check_interval":   s.config.CheckInterval.String(),
		"default_interval": s.config.DefaultInterval.String(),
		"max_concurrent":   s.config.MaxConcurrent,
		"enabled":          s.config.Enabled,
		"approach":         "fan-out", // Indicates we use fan-out to all controllers
		"model":            "binary_state_model",
		"intervals":        "30s_needs_attention_5m_stable",
		"reactive_enabled": s.config.ReactiveEnabled,
	}

	// Get pending cluster reconciliations count
	allTargets, err := s.repository.Reconciliation.FindClustersNeedingReconciliation(ctx)
	if err != nil {
		s.logger.Warn("Failed to get pending cluster reconciliations for stats", zap.Error(err))
		stats["pending_cluster_reconciliations"] = "unknown"
	} else {
		// Count unique clusters
		uniqueClusters := make(map[uuid.UUID]bool)
		for _, target := range allTargets {
			uniqueClusters[target.ClusterID] = true
		}
		stats["pending_cluster_reconciliations"] = len(uniqueClusters)
		stats["total_cluster_reconciliation_targets"] = len(allTargets)
	}

	// Get pending nodepool reconciliations count
	nodepoolTargets, err := s.repository.Reconciliation.FindNodePoolsNeedingReconciliation(ctx)
	if err != nil {
		s.logger.Warn("Failed to get pending nodepool reconciliations for stats", zap.Error(err))
		stats["pending_nodepool_reconciliations"] = "unknown"
	} else {
		// Count unique nodepools
		uniqueNodePools := make(map[uuid.UUID]bool)
		for _, target := range nodepoolTargets {
			uniqueNodePools[target.NodePoolID] = true
		}
		stats["pending_nodepool_reconciliations"] = len(uniqueNodePools)
		stats["total_nodepool_reconciliation_targets"] = len(nodepoolTargets)
	}

	return stats, nil
}

// Note: Complex health evaluation functions removed in favor of simplified binary state model
// The database now handles all interval logic through simplified functions
