package database

import (
	"context"
	"fmt"
	"time"

	"github.com/apahim/cls-backend/internal/models"
	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// StatusAggregator handles real-time status aggregation logic
type StatusAggregator struct {
	client *Client
	logger *utils.Logger
}

// NewStatusAggregator creates a new status aggregator
func NewStatusAggregator(client *Client) *StatusAggregator {
	return &StatusAggregator{
		client: client,
		logger: utils.NewLogger("status_aggregator"),
	}
}

// StatusAggregationResult contains the computed status information
type StatusAggregationResult struct {
	Status             *models.ClusterStatusInfo `json:"status"`
	TotalControllers   int                      `json:"total_controllers"`
	ReadyControllers   int                      `json:"ready_controllers"`
	FailedControllers  int                      `json:"failed_controllers"`
	HasErrors          bool                     `json:"has_errors"`
	Generation         int64                    `json:"generation"`
}

// CalculateClusterStatus performs real-time status aggregation for a cluster
// This replaces the PostgreSQL stored procedure with Go logic
func (a *StatusAggregator) CalculateClusterStatus(ctx context.Context, cluster *models.Cluster) (*StatusAggregationResult, error) {
	if cluster == nil {
		return nil, fmt.Errorf("cluster cannot be nil")
	}

	a.logger.Debug("Calculating cluster status",
		zap.String("cluster_id", cluster.ID.String()),
		zap.Int64("generation", cluster.Generation),
	)

	// Get controller status counts for the current generation only
	stats, err := a.getControllerStats(ctx, cluster.ID, cluster.Generation)
	if err != nil {
		return nil, fmt.Errorf("failed to get controller stats: %w", err)
	}

	// Apply aggregation logic (same logic as the PostgreSQL function)
	result := a.applyAggregationRules(stats, cluster.Generation)

	a.logger.Debug("Calculated cluster status",
		zap.String("cluster_id", cluster.ID.String()),
		zap.String("phase", result.Status.Phase),
		zap.String("reason", result.Status.Reason),
		zap.Int("total_controllers", result.TotalControllers),
		zap.Int("ready_controllers", result.ReadyControllers),
	)

	return result, nil
}

// ControllerStats holds the aggregated controller statistics
type ControllerStats struct {
	TotalCount  int
	ReadyCount  int
	ErrorCount  int
	Generation  int64
}

// getControllerStats queries controller status and counts them for the current generation
func (a *StatusAggregator) getControllerStats(ctx context.Context, clusterID uuid.UUID, generation int64) (*ControllerStats, error) {
	query := `
		SELECT
			COUNT(*) AS total,
			COUNT(CASE WHEN
				(
					SELECT COUNT(*)
					FROM jsonb_array_elements(conditions) AS condition
					WHERE condition->>'type' = 'Available' AND condition->>'status' = 'True'
				) > 0
			THEN 1 END) AS ready,
			COUNT(CASE WHEN last_error IS NOT NULL THEN 1 END) AS errors
		FROM controller_status
		WHERE cluster_id = $1 AND observed_generation = $2`

	var stats ControllerStats
	a.logger.Debug("Executing controller stats query",
		zap.String("cluster_id", clusterID.String()),
		zap.Int64("generation", generation),
		zap.String("query", query),
	)

	err := a.client.QueryRowContext(ctx, query, clusterID, generation).Scan(
		&stats.TotalCount,
		&stats.ReadyCount,
		&stats.ErrorCount,
	)

	if err != nil {
		a.logger.Error("Failed to get controller stats",
			zap.String("cluster_id", clusterID.String()),
			zap.Int64("generation", generation),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to query controller stats: %w", err)
	}

	stats.Generation = generation

	a.logger.Debug("Controller stats retrieved",
		zap.String("cluster_id", clusterID.String()),
		zap.Int64("generation", generation),
		zap.Int("total", stats.TotalCount),
		zap.Int("ready", stats.ReadyCount),
		zap.Int("errors", stats.ErrorCount),
	)

	return &stats, nil
}

// applyAggregationRules applies the Kubernetes-like status aggregation logic
func (a *StatusAggregator) applyAggregationRules(stats *ControllerStats, generation int64) *StatusAggregationResult {
	now := time.Now()

	var (
		phase            string
		reason           string
		message          string
		readyCondition   models.Condition
		availableCondition models.Condition
	)

	failedCount := stats.TotalCount - stats.ReadyCount
	hasErrors := stats.ErrorCount > 0

	// Apply Kubernetes-like aggregation logic
	if stats.TotalCount == 0 {
		// No controllers have reported status yet
		phase = "Pending"
		reason = "NoControllers"
		message = "Waiting for controllers to report status"

		readyCondition = models.Condition{
			Type:               "Ready",
			Status:             "False",
			LastTransitionTime: now,
			Reason:             "ControllersNotReady",
			Message:            "No controllers have reported status yet",
		}

		availableCondition = models.Condition{
			Type:               "Available",
			Status:             "False",
			LastTransitionTime: now,
			Reason:             "ControllersNotAvailable",
			Message:            "No controllers are available yet",
		}

	} else if stats.ReadyCount == stats.TotalCount && !hasErrors {
		// All controllers ready and no errors
		phase = "Ready"
		reason = "AllControllersReady"
		message = fmt.Sprintf("Cluster is ready with %d controllers operational", stats.TotalCount)

		readyCondition = models.Condition{
			Type:               "Ready",
			Status:             "True",
			LastTransitionTime: now,
			Reason:             "AllControllersReady",
			Message:            fmt.Sprintf("All %d controllers are ready", stats.TotalCount),
		}

		availableCondition = models.Condition{
			Type:               "Available",
			Status:             "True",
			LastTransitionTime: now,
			Reason:             "AllControllersAvailable",
			Message:            fmt.Sprintf("All %d controllers are available", stats.TotalCount),
		}

	} else if stats.ReadyCount > 0 {
		// Some controllers ready
		phase = "Progressing"

		readyCondition = models.Condition{
			Type:               "Ready",
			Status:             "False",
			LastTransitionTime: now,
			Reason:             "PartiallyReady",
			Message:            fmt.Sprintf("%d of %d controllers are ready", stats.ReadyCount, stats.TotalCount),
		}

		if hasErrors {
			reason = "ControllersWithErrors"
			message = fmt.Sprintf("Cluster is progressing but some controllers have errors (%d/%d ready)", stats.ReadyCount, stats.TotalCount)

			availableCondition = models.Condition{
				Type:               "Available",
				Status:             "False",
				LastTransitionTime: now,
				Reason:             "PartiallyAvailableWithErrors",
				Message:            fmt.Sprintf("Some controllers have errors (%d available of %d)", stats.ReadyCount, stats.TotalCount),
			}
		} else {
			reason = "PartialProgress"
			message = fmt.Sprintf("Cluster is progressing (%d/%d controllers ready)", stats.ReadyCount, stats.TotalCount)

			availableCondition = models.Condition{
				Type:               "Available",
				Status:             "False",
				LastTransitionTime: now,
				Reason:             "PartiallyAvailable",
				Message:            fmt.Sprintf("Controllers are still becoming available (%d available of %d)", stats.ReadyCount, stats.TotalCount),
			}
		}

	} else {
		// No controllers ready
		phase = "Failed"
		reason = "NoControllersReady"
		message = fmt.Sprintf("Cluster failed - no controllers are operational (%d controllers exist)", stats.TotalCount)

		readyCondition = models.Condition{
			Type:               "Ready",
			Status:             "False",
			LastTransitionTime: now,
			Reason:             "ControllersNotReady",
			Message:            fmt.Sprintf("None of %d controllers are ready", stats.TotalCount),
		}

		availableCondition = models.Condition{
			Type:               "Available",
			Status:             "False",
			LastTransitionTime: now,
			Reason:             "ControllersNotAvailable",
			Message:            fmt.Sprintf("None of %d controllers are available", stats.TotalCount),
		}
	}

	// Build the Kubernetes-like status block
	status := &models.ClusterStatusInfo{
		ObservedGeneration: generation,
		Conditions:         []models.Condition{readyCondition, availableCondition},
		Phase:              phase,
		Message:            message,
		Reason:             reason,
		LastUpdateTime:     now,
	}

	return &StatusAggregationResult{
		Status:            status,
		TotalControllers:  stats.TotalCount,
		ReadyControllers:  stats.ReadyCount,
		FailedControllers: failedCount,
		HasErrors:         hasErrors,
		Generation:        generation,
	}
}

// EnrichClusterWithStatus calculates and applies status to a cluster (only if dirty)
// This is the main method that should be called from the repository layer
func (a *StatusAggregator) EnrichClusterWithStatus(ctx context.Context, cluster *models.Cluster) error {
	if cluster == nil {
		return fmt.Errorf("cluster cannot be nil")
	}

	// If status is not dirty, use the cached status from database
	if !cluster.StatusDirty {
		a.logger.Debug("Status is clean, using cached status",
			zap.String("cluster_id", cluster.ID.String()),
		)
		return nil // Status is already current, no need to recalculate
	}

	a.logger.Debug("Status is dirty, recalculating",
		zap.String("cluster_id", cluster.ID.String()),
		zap.Int64("generation", cluster.Generation),
	)

	// Status is dirty, need to recalculate and cache
	result, err := a.CalculateClusterStatus(ctx, cluster)
	if err != nil {
		a.logger.Error("Failed to calculate cluster status",
			zap.String("cluster_id", cluster.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to calculate status for cluster %s: %w", cluster.ID, err)
	}

	// Apply the calculated status to the cluster object
	cluster.Status = result.Status

	// Update the database with cached results and mark as clean
	err = a.updateClusterStatusInDB(ctx, cluster.ID, result)
	if err != nil {
		a.logger.Warn("Failed to cache status in database",
			zap.String("cluster_id", cluster.ID.String()),
			zap.Error(err))
		// Don't fail the request - we have the calculated status in memory
	} else {
		// Mark as clean now that we've cached the results
		cluster.StatusDirty = false
		a.logger.Debug("Successfully cached status and marked cluster as clean",
			zap.String("cluster_id", cluster.ID.String()),
		)
	}

	a.logger.Debug("Enriched cluster with calculated status",
		zap.String("cluster_id", cluster.ID.String()),
		zap.String("phase", result.Status.Phase),
		zap.String("reason", result.Status.Reason),
		zap.Int("conditions", len(result.Status.Conditions)),
	)

	return nil
}

// updateClusterStatusInDB caches the calculated status in the database and marks as clean
func (a *StatusAggregator) updateClusterStatusInDB(ctx context.Context, clusterID uuid.UUID, result *StatusAggregationResult) error {
	// Convert the status to JSON for storage
	statusJSON, err := result.Status.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal status to JSON: %w", err)
	}

	query := `
		UPDATE clusters
		SET
			status = $2,
			status_dirty = FALSE,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`

	result_db, err := a.client.ExecContext(ctx, query,
		clusterID,
		statusJSON,
	)

	if err != nil {
		a.logger.Error("Failed to update cluster status in database",
			zap.String("cluster_id", clusterID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update cluster status: %w", err)
	}

	rowsAffected, err := result_db.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("cluster not found or already deleted")
	}

	a.logger.Debug("Successfully cached cluster status in database",
		zap.String("cluster_id", clusterID.String()),
		zap.String("phase", result.Status.Phase),
		zap.String("reason", result.Status.Reason),
	)

	return nil
}

// EnrichClustersWithStatus calculates and applies real-time status to multiple clusters
func (a *StatusAggregator) EnrichClustersWithStatus(ctx context.Context, clusters []*models.Cluster) error {
	if len(clusters) == 0 {
		return nil
	}

	a.logger.Debug("Enriching multiple clusters with real-time status",
		zap.Int("cluster_count", len(clusters)),
	)

	var enrichmentErrors []error

	for _, cluster := range clusters {
		if err := a.EnrichClusterWithStatus(ctx, cluster); err != nil {
			a.logger.Error("Failed to enrich cluster with status",
				zap.String("cluster_id", cluster.ID.String()),
				zap.Error(err),
			)
			enrichmentErrors = append(enrichmentErrors, err)
			// Continue with other clusters even if one fails
		}
	}

	if len(enrichmentErrors) > 0 {
		a.logger.Warn("Some clusters failed status enrichment",
			zap.Int("failed_count", len(enrichmentErrors)),
			zap.Int("total_count", len(clusters)),
		)
		// Return the first error but log all of them
		return fmt.Errorf("failed to enrich %d out of %d clusters with status: %w", len(enrichmentErrors), len(clusters), enrichmentErrors[0])
	}

	a.logger.Debug("Successfully enriched all clusters with real-time status",
		zap.Int("cluster_count", len(clusters)),
	)

	return nil
}