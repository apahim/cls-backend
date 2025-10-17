package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/apahim/cls-backend/internal/models"
	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ReconciliationRepository handles reconciliation-related database operations
type ReconciliationRepository struct {
	client *Client
	logger *utils.Logger
}

// NewReconciliationRepository creates a new reconciliation repository
func NewReconciliationRepository(client *Client) *ReconciliationRepository {
	return &ReconciliationRepository{
		client: client,
		logger: utils.NewLogger("reconciliation_repository"),
	}
}

// FindClustersNeedingReconciliation finds clusters that need reconciliation (fan-out to all controllers)
func (r *ReconciliationRepository) FindClustersNeedingReconciliation(ctx context.Context) ([]*models.ReconciliationTarget, error) {
	query := `SELECT * FROM find_clusters_needing_reconciliation()`

	rows, err := r.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find clusters needing reconciliation: %w", err)
	}
	defer rows.Close()

	var targets []*models.ReconciliationTarget
	for rows.Next() {
		target := &models.ReconciliationTarget{}
		if err := rows.Scan(
			&target.ClusterID,
			&target.Reason,
			&target.LastReconciledAt,
			&target.ClusterGeneration,
		); err != nil {
			return nil, fmt.Errorf("failed to scan reconciliation target: %w", err)
		}
		targets = append(targets, target)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reconciliation targets: %w", err)
	}

	r.logger.Debug("Found clusters needing reconciliation (fan-out)",
		zap.Int("count", len(targets)))

	return targets, nil
}

// UpdateReconciliationSchedule updates the cluster reconciliation schedule (fan-out approach)
func (r *ReconciliationRepository) UpdateReconciliationSchedule(ctx context.Context, clusterID uuid.UUID) error {
	query := `SELECT update_cluster_reconciliation_schedule($1)`

	_, err := r.client.ExecContext(ctx, query, clusterID)
	if err != nil {
		return fmt.Errorf("failed to update cluster reconciliation schedule: %w", err)
	}

	r.logger.Debug("Updated cluster reconciliation schedule (fan-out)",
		zap.String("cluster_id", clusterID.String()))

	return nil
}

// GetReconciliationSchedule gets the cluster reconciliation schedule (fan-out approach)
func (r *ReconciliationRepository) GetReconciliationSchedule(ctx context.Context, clusterID uuid.UUID) (*models.ReconciliationSchedule, error) {
	query := `
		SELECT id, cluster_id, last_reconciled_at, next_reconcile_at,
		       reconcile_interval, enabled, created_at, updated_at,
		       healthy_interval, unhealthy_interval, adaptive_enabled,
		       last_health_check, is_healthy
		FROM reconciliation_schedule
		WHERE cluster_id = $1`

	schedule := &models.ReconciliationSchedule{}
	err := r.client.QueryRowContext(ctx, query, clusterID).Scan(
		&schedule.ID,
		&schedule.ClusterID,
		&schedule.LastReconciledAt,
		&schedule.NextReconcileAt,
		&schedule.ReconcileInterval,
		&schedule.Enabled,
		&schedule.CreatedAt,
		&schedule.UpdatedAt,
		&schedule.HealthyInterval,
		&schedule.UnhealthyInterval,
		&schedule.AdaptiveEnabled,
		&schedule.LastHealthCheck,
		&schedule.IsHealthy,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrReconciliationScheduleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster reconciliation schedule: %w", err)
	}

	return schedule, nil
}

// CreateReconciliationSchedule creates a new cluster reconciliation schedule (fan-out approach)
func (r *ReconciliationRepository) CreateReconciliationSchedule(ctx context.Context, schedule *models.ReconciliationSchedule) error {
	query := `
		INSERT INTO reconciliation_schedule (
			cluster_id, next_reconcile_at, reconcile_interval, enabled,
			healthy_interval, unhealthy_interval, adaptive_enabled
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`

	// Set defaults if not provided
	healthyInterval := schedule.HealthyInterval
	if healthyInterval == "" {
		healthyInterval = "5 minutes"
	}
	unhealthyInterval := schedule.UnhealthyInterval
	if unhealthyInterval == "" {
		unhealthyInterval = "30 seconds"
	}

	err := r.client.QueryRowContext(ctx, query,
		schedule.ClusterID,
		schedule.NextReconcileAt,
		schedule.ReconcileInterval,
		schedule.Enabled,
		healthyInterval,
		unhealthyInterval,
		schedule.AdaptiveEnabled,
	).Scan(&schedule.ID, &schedule.CreatedAt, &schedule.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create cluster reconciliation schedule: %w", err)
	}

	r.logger.Info("Created cluster reconciliation schedule (fan-out)",
		zap.String("cluster_id", schedule.ClusterID.String()),
		zap.Int64("schedule_id", schedule.ID))

	return nil
}

// UpdateReconciliationScheduleConfig updates cluster reconciliation schedule configuration (fan-out approach)
func (r *ReconciliationRepository) UpdateReconciliationScheduleConfig(ctx context.Context, clusterID uuid.UUID, interval time.Duration, enabled bool) error {
	query := `
		UPDATE reconciliation_schedule
		SET reconcile_interval = $2, enabled = $3, updated_at = NOW()
		WHERE cluster_id = $1`

	result, err := r.client.ExecContext(ctx, query, clusterID, interval, enabled)
	if err != nil {
		return fmt.Errorf("failed to update cluster reconciliation schedule config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrReconciliationScheduleNotFound
	}

	r.logger.Info("Updated cluster reconciliation schedule config (fan-out)",
		zap.String("cluster_id", clusterID.String()),
		zap.Duration("interval", interval),
		zap.Bool("enabled", enabled))

	return nil
}

// DeleteReconciliationSchedules deletes all reconciliation schedules for a cluster
func (r *ReconciliationRepository) DeleteReconciliationSchedules(ctx context.Context, clusterID uuid.UUID) error {
	query := `DELETE FROM reconciliation_schedule WHERE cluster_id = $1`

	result, err := r.client.ExecContext(ctx, query, clusterID)
	if err != nil {
		return fmt.Errorf("failed to delete reconciliation schedules: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	r.logger.Info("Deleted reconciliation schedules",
		zap.String("cluster_id", clusterID.String()),
		zap.Int64("schedules_deleted", rowsAffected))

	return nil
}

// ListReconciliationSchedules lists the cluster reconciliation schedule (fan-out approach - returns single schedule)
func (r *ReconciliationRepository) ListReconciliationSchedules(ctx context.Context, clusterID uuid.UUID) ([]*models.ReconciliationSchedule, error) {
	query := `
		SELECT id, cluster_id, last_reconciled_at, next_reconcile_at,
		       reconcile_interval, enabled, created_at, updated_at,
		       healthy_interval, unhealthy_interval, adaptive_enabled,
		       last_health_check, is_healthy
		FROM reconciliation_schedule
		WHERE cluster_id = $1`

	rows, err := r.client.QueryContext(ctx, query, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster reconciliation schedule: %w", err)
	}
	defer rows.Close()

	var schedules []*models.ReconciliationSchedule
	for rows.Next() {
		schedule := &models.ReconciliationSchedule{}
		if err := rows.Scan(
			&schedule.ID,
			&schedule.ClusterID,
			&schedule.LastReconciledAt,
			&schedule.NextReconcileAt,
			&schedule.ReconcileInterval,
			&schedule.Enabled,
			&schedule.CreatedAt,
			&schedule.UpdatedAt,
			&schedule.HealthyInterval,
			&schedule.UnhealthyInterval,
			&schedule.AdaptiveEnabled,
			&schedule.LastHealthCheck,
			&schedule.IsHealthy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan reconciliation schedule: %w", err)
		}
		schedules = append(schedules, schedule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reconciliation schedules: %w", err)
	}

	return schedules, nil
}

// MarkReconciliationNeeded marks a cluster as needing reconciliation (fan-out approach)
func (r *ReconciliationRepository) MarkReconciliationNeeded(ctx context.Context, clusterID uuid.UUID) error {
	query := `
		UPDATE reconciliation_schedule
		SET next_reconcile_at = NOW(), updated_at = NOW()
		WHERE cluster_id = $1`

	result, err := r.client.ExecContext(ctx, query, clusterID)
	if err != nil {
		return fmt.Errorf("failed to mark cluster reconciliation needed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Schedule doesn't exist, create it with health-aware defaults
		now := time.Now()
		schedule := &models.ReconciliationSchedule{
			ClusterID:         clusterID,
			NextReconcileAt:   &now, // Schedule immediately
			ReconcileInterval: "5 minutes",
			Enabled:           true,
			HealthyInterval:   "5 minutes",
			UnhealthyInterval: "30 seconds",
			AdaptiveEnabled:   true,
		}
		return r.CreateReconciliationSchedule(ctx, schedule)
	}

	r.logger.Info("Marked cluster reconciliation as needed (fan-out)",
		zap.String("cluster_id", clusterID.String()))

	return nil
}
