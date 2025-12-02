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

// ReconciliationUpdater interface for updating reconciliation schedules (fan-out approach)
type ReconciliationUpdater interface {
	UpdateReconciliationSchedule(ctx context.Context, clusterID uuid.UUID) error
}

// StatusRepository handles database operations for controller status
type StatusRepository struct {
	client                *Client
	logger                *utils.Logger
	reconciliationUpdater ReconciliationUpdater
}

// NewStatusRepository creates a new status repository
func NewStatusRepository(client *Client) *StatusRepository {
	return &StatusRepository{
		client: client,
		logger: utils.NewLogger("status_repo"),
	}
}

// SetReconciliationUpdater sets the reconciliation updater (to avoid circular dependency)
func (r *StatusRepository) SetReconciliationUpdater(updater ReconciliationUpdater) {
	r.reconciliationUpdater = updater
}

// UpsertClusterControllerStatus inserts or updates cluster controller status
func (r *StatusRepository) UpsertClusterControllerStatus(ctx context.Context, status *models.ClusterControllerStatus) error {
	status.LastUpdated = time.Now()

	query := `
		INSERT INTO controller_status (
			cluster_id, controller_name, observed_generation, conditions,
			metadata, last_error, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		ON CONFLICT (cluster_id, controller_name)
		DO UPDATE SET
			observed_generation = EXCLUDED.observed_generation,
			conditions = EXCLUDED.conditions,
			metadata = EXCLUDED.metadata,
			last_error = EXCLUDED.last_error,
			updated_at = EXCLUDED.updated_at`

	_, err := r.client.ExecContext(ctx, query,
		status.ClusterID,
		status.ControllerName,
		status.ObservedGeneration,
		status.Conditions,
		status.Metadata,
		status.LastError,
		status.LastUpdated,
	)

	if err != nil {
		r.logger.Error("Failed to upsert cluster controller status",
			zap.String("cluster_id", status.ClusterID.String()),
			zap.String("controller_name", status.ControllerName),
			zap.Error(err),
		)
		return fmt.Errorf("failed to upsert cluster controller status: %w", err)
	}

	r.logger.Debug("Cluster controller status upserted",
		zap.String("cluster_id", status.ClusterID.String()),
		zap.String("controller_name", status.ControllerName),
		zap.Int64("observed_generation", status.ObservedGeneration),
	)

	// NOTE: Controller status updates should NOT trigger reconciliation schedule updates.
	// Only the reconciliation scheduler should update schedules after actual reconciliation events.
	// Removed the incorrect UpdateReconciliationSchedule call that was marking clusters as "reconciled"
	// when they were only reporting status.

	return nil
}

// GetClusterControllerStatus retrieves status for a specific cluster controller
func (r *StatusRepository) GetClusterControllerStatus(ctx context.Context, clusterID uuid.UUID, controllerName string) (*models.ClusterControllerStatus, error) {
	query := `
		SELECT cluster_id, controller_name, observed_generation, conditions,
			   metadata, last_error, updated_at
		FROM controller_status
		WHERE cluster_id = $1 AND controller_name = $2`

	var status models.ClusterControllerStatus
	err := r.client.QueryRowContext(ctx, query, clusterID, controllerName).Scan(
		&status.ClusterID,
		&status.ControllerName,
		&status.ObservedGeneration,
		&status.Conditions,
		&status.Metadata,
		&status.LastError,
		&status.LastUpdated,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("controller status not found for cluster %s, controller %s", clusterID, controllerName)
	}
	if err != nil {
		r.logger.Error("Failed to get cluster controller status",
			zap.String("cluster_id", clusterID.String()),
			zap.String("controller_name", controllerName),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get cluster controller status: %w", err)
	}

	return &status, nil
}

// ListClusterControllerStatus retrieves all controller statuses for a cluster
func (r *StatusRepository) ListClusterControllerStatus(ctx context.Context, clusterID uuid.UUID) ([]*models.ClusterControllerStatus, error) {
	query := `
		SELECT cluster_id, controller_name, observed_generation, conditions,
			   metadata, last_error, updated_at
		FROM controller_status
		WHERE cluster_id = $1
		ORDER BY controller_name`

	rows, err := r.client.QueryContext(ctx, query, clusterID)
	if err != nil {
		r.logger.Error("Failed to list cluster controller status",
			zap.String("cluster_id", clusterID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to list cluster controller status: %w", err)
	}
	defer rows.Close()

	var statuses []*models.ClusterControllerStatus
	for rows.Next() {
		var status models.ClusterControllerStatus
		err := rows.Scan(
			&status.ClusterID,
			&status.ControllerName,
			&status.ObservedGeneration,
			&status.Conditions,
			&status.Metadata,
			&status.LastError,
			&status.LastUpdated,
		)
		if err != nil {
			r.logger.Error("Failed to scan cluster controller status row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan cluster controller status: %w", err)
		}
		statuses = append(statuses, &status)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating cluster controller status rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating cluster controller status: %w", err)
	}

	return statuses, nil
}

// DeleteClusterControllerStatus deletes status for a specific cluster controller
func (r *StatusRepository) DeleteClusterControllerStatus(ctx context.Context, clusterID uuid.UUID, controllerName string) error {
	query := `DELETE FROM controller_status WHERE cluster_id = $1 AND controller_name = $2`

	result, err := r.client.ExecContext(ctx, query, clusterID, controllerName)
	if err != nil {
		r.logger.Error("Failed to delete cluster controller status",
			zap.String("cluster_id", clusterID.String()),
			zap.String("controller_name", controllerName),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete cluster controller status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	r.logger.Debug("Cluster controller status deleted",
		zap.String("cluster_id", clusterID.String()),
		zap.String("controller_name", controllerName),
		zap.Int64("rows_affected", rowsAffected),
	)

	return nil
}

// DeleteAllClusterControllerStatus deletes all controller status for a cluster
func (r *StatusRepository) DeleteAllClusterControllerStatus(ctx context.Context, clusterID uuid.UUID) error {
	query := `DELETE FROM controller_status WHERE cluster_id = $1`

	result, err := r.client.ExecContext(ctx, query, clusterID)
	if err != nil {
		r.logger.Error("Failed to delete all cluster controller status",
			zap.String("cluster_id", clusterID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete all cluster controller status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	r.logger.Info("All cluster controller status deleted",
		zap.String("cluster_id", clusterID.String()),
		zap.Int64("rows_affected", rowsAffected),
	)

	return nil
}

// UpsertNodePoolControllerStatus inserts or updates nodepool controller status
func (r *StatusRepository) UpsertNodePoolControllerStatus(ctx context.Context, status *models.NodePoolControllerStatus) error {
	status.LastUpdated = time.Now()

	query := `
		INSERT INTO nodepool_controller_status (
			nodepool_id, controller_name, observed_generation, conditions,
			metadata, last_error, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		ON CONFLICT (nodepool_id, controller_name)
		DO UPDATE SET
			observed_generation = EXCLUDED.observed_generation,
			conditions = EXCLUDED.conditions,
			metadata = EXCLUDED.metadata,
			last_error = EXCLUDED.last_error,
			updated_at = EXCLUDED.updated_at`

	_, err := r.client.ExecContext(ctx, query,
		status.NodePoolID,
		status.ControllerName,
		status.ObservedGeneration,
		status.Conditions,
		status.Metadata,
		status.LastError,
		status.LastUpdated,
	)

	if err != nil {
		r.logger.Error("Failed to upsert nodepool controller status",
			zap.String("nodepool_id", status.NodePoolID.String()),
			zap.String("controller_name", status.ControllerName),
			zap.Error(err),
		)
		return fmt.Errorf("failed to upsert nodepool controller status: %w", err)
	}

	r.logger.Debug("NodePool controller status upserted",
		zap.String("nodepool_id", status.NodePoolID.String()),
		zap.String("controller_name", status.ControllerName),
		zap.Int64("observed_generation", status.ObservedGeneration),
	)

	return nil
}

// GetNodePoolControllerStatus retrieves status for a specific nodepool controller
func (r *StatusRepository) GetNodePoolControllerStatus(ctx context.Context, nodepoolID uuid.UUID, controllerName string) (*models.NodePoolControllerStatus, error) {
	query := `
		SELECT nodepool_id, controller_name, observed_generation, conditions,
			   metadata, last_error, updated_at
		FROM nodepool_controller_status
		WHERE nodepool_id = $1 AND controller_name = $2`

	var status models.NodePoolControllerStatus
	err := r.client.QueryRowContext(ctx, query, nodepoolID, controllerName).Scan(
		&status.NodePoolID,
		&status.ControllerName,
		&status.ObservedGeneration,
		&status.Conditions,
		&status.Metadata,
		&status.LastError,
		&status.LastUpdated,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("controller status not found for nodepool %s, controller %s", nodepoolID, controllerName)
	}
	if err != nil {
		r.logger.Error("Failed to get nodepool controller status",
			zap.String("nodepool_id", nodepoolID.String()),
			zap.String("controller_name", controllerName),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get nodepool controller status: %w", err)
	}

	return &status, nil
}

// ListNodePoolControllerStatus retrieves all controller statuses for a nodepool
func (r *StatusRepository) ListNodePoolControllerStatus(ctx context.Context, nodepoolID uuid.UUID) ([]*models.NodePoolControllerStatus, error) {
	query := `
		SELECT nodepool_id, controller_name, observed_generation, conditions,
			   metadata, last_error, updated_at
		FROM nodepool_controller_status
		WHERE nodepool_id = $1
		ORDER BY controller_name`

	rows, err := r.client.QueryContext(ctx, query, nodepoolID)
	if err != nil {
		r.logger.Error("Failed to list nodepool controller status",
			zap.String("nodepool_id", nodepoolID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to list nodepool controller status: %w", err)
	}
	defer rows.Close()

	var statuses []*models.NodePoolControllerStatus
	for rows.Next() {
		var status models.NodePoolControllerStatus
		err := rows.Scan(
			&status.NodePoolID,
			&status.ControllerName,
			&status.ObservedGeneration,
			&status.Conditions,
			&status.Metadata,
			&status.LastError,
			&status.LastUpdated,
		)
		if err != nil {
			r.logger.Error("Failed to scan nodepool controller status row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan nodepool controller status: %w", err)
		}
		statuses = append(statuses, &status)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating nodepool controller status rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating nodepool controller status: %w", err)
	}

	return statuses, nil
}

// ListNodePoolControllerStatusByCluster retrieves all nodepool controller statuses for a cluster
func (r *StatusRepository) ListNodePoolControllerStatusByCluster(ctx context.Context, clusterID uuid.UUID) ([]*models.NodePoolControllerStatus, error) {
	query := `
		SELECT npcs.nodepool_id, npcs.controller_name, npcs.observed_generation,
			   npcs.conditions, npcs.metadata, npcs.last_error, npcs.updated_at
		FROM nodepool_controller_status npcs
		JOIN nodepools np ON npcs.nodepool_id = np.id
		WHERE np.cluster_id = $1 AND np.deleted_at IS NULL
		ORDER BY np.name, npcs.controller_name`

	rows, err := r.client.QueryContext(ctx, query, clusterID)
	if err != nil {
		r.logger.Error("Failed to list nodepool controller status by cluster",
			zap.String("cluster_id", clusterID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to list nodepool controller status by cluster: %w", err)
	}
	defer rows.Close()

	var statuses []*models.NodePoolControllerStatus
	for rows.Next() {
		var status models.NodePoolControllerStatus
		err := rows.Scan(
			&status.NodePoolID,
			&status.ControllerName,
			&status.ObservedGeneration,
			&status.Conditions,
			&status.Metadata,
			&status.LastError,
			&status.LastUpdated,
		)
		if err != nil {
			r.logger.Error("Failed to scan nodepool controller status row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan nodepool controller status: %w", err)
		}
		statuses = append(statuses, &status)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating nodepool controller status rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating nodepool controller status: %w", err)
	}

	return statuses, nil
}

// DeleteNodePoolControllerStatus deletes status for a specific nodepool controller
func (r *StatusRepository) DeleteNodePoolControllerStatus(ctx context.Context, nodepoolID uuid.UUID, controllerName string) error {
	query := `DELETE FROM nodepool_controller_status WHERE nodepool_id = $1 AND controller_name = $2`

	result, err := r.client.ExecContext(ctx, query, nodepoolID, controllerName)
	if err != nil {
		r.logger.Error("Failed to delete nodepool controller status",
			zap.String("nodepool_id", nodepoolID.String()),
			zap.String("controller_name", controllerName),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete nodepool controller status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	r.logger.Debug("NodePool controller status deleted",
		zap.String("nodepool_id", nodepoolID.String()),
		zap.String("controller_name", controllerName),
		zap.Int64("rows_affected", rowsAffected),
	)

	return nil
}

// DeleteAllNodePoolControllerStatus deletes all controller status for a nodepool
func (r *StatusRepository) DeleteAllNodePoolControllerStatus(ctx context.Context, nodepoolID uuid.UUID) error {
	query := `DELETE FROM nodepool_controller_status WHERE nodepool_id = $1`

	result, err := r.client.ExecContext(ctx, query, nodepoolID)
	if err != nil {
		r.logger.Error("Failed to delete all nodepool controller status",
			zap.String("nodepool_id", nodepoolID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete all nodepool controller status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	r.logger.Info("All nodepool controller status deleted",
		zap.String("nodepool_id", nodepoolID.String()),
		zap.Int64("rows_affected", rowsAffected),
	)

	return nil
}

// CreateClusterEvent creates a new cluster event
func (r *StatusRepository) CreateClusterEvent(ctx context.Context, event *models.ClusterEvent) error {
	event.BeforeCreate()

	query := `
		INSERT INTO cluster_events (id, cluster_id, event_type, controller_name, metadata, published_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.client.ExecContext(ctx, query,
		event.ID,
		event.ClusterID,
		event.EventType,
		"cluster",     // Use a default controller name
		event.Changes, // Store changes as metadata
		event.PublishedAt,
	)

	if err != nil {
		r.logger.Error("Failed to create cluster event",
			zap.String("cluster_id", event.ClusterID.String()),
			zap.String("event_type", event.EventType),
			zap.Error(err),
		)
		return fmt.Errorf("failed to create cluster event: %w", err)
	}

	r.logger.Debug("Cluster event created",
		zap.String("event_id", event.ID.String()),
		zap.String("cluster_id", event.ClusterID.String()),
		zap.String("event_type", event.EventType),
	)

	return nil
}

// ListClusterEvents retrieves events for a cluster
func (r *StatusRepository) ListClusterEvents(ctx context.Context, clusterID uuid.UUID, limit int) ([]*models.ClusterEvent, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, cluster_id, event_type, metadata, published_at
		FROM cluster_events
		WHERE cluster_id = $1
		ORDER BY published_at DESC
		LIMIT $2`

	rows, err := r.client.QueryContext(ctx, query, clusterID, limit)
	if err != nil {
		r.logger.Error("Failed to list cluster events",
			zap.String("cluster_id", clusterID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to list cluster events: %w", err)
	}
	defer rows.Close()

	var events []*models.ClusterEvent
	for rows.Next() {
		var event models.ClusterEvent
		err := rows.Scan(
			&event.ID,
			&event.ClusterID,
			&event.EventType,
			&event.Changes, // Read metadata as changes
			&event.PublishedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan cluster event row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan cluster event: %w", err)
		}
		// Set generation to 0 as default since it's not stored in this schema
		event.Generation = 0
		events = append(events, &event)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating cluster event rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating cluster events: %w", err)
	}

	return events, nil
}

// getClusterErrors retrieves detailed error information for a cluster
func (r *StatusRepository) getClusterErrors(ctx context.Context, clusterID uuid.UUID) ([]models.ErrorInfo, error) {
	query := `
		SELECT last_error
		FROM controller_status
		WHERE cluster_id = $1 AND last_error IS NOT NULL
		UNION ALL
		SELECT npcs.last_error
		FROM nodepool_controller_status npcs
		JOIN nodepools np ON npcs.nodepool_id = np.id
		WHERE np.cluster_id = $1 AND npcs.last_error IS NOT NULL AND np.deleted_at IS NULL`

	rows, err := r.client.QueryContext(ctx, query, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster errors: %w", err)
	}
	defer rows.Close()

	var errors []models.ErrorInfo
	for rows.Next() {
		var errorInfo models.ErrorInfo
		err := rows.Scan(&errorInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to scan error info: %w", err)
		}
		errors = append(errors, errorInfo)
	}

	return errors, rows.Err()
}
