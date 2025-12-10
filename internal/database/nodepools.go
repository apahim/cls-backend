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

// NodePoolsRepository handles database operations for nodepools
type NodePoolsRepository struct {
	client *Client
	logger *utils.Logger
}

// NewNodePoolsRepository creates a new nodepools repository
func NewNodePoolsRepository(client *Client) *NodePoolsRepository {
	return &NodePoolsRepository{
		client: client,
		logger: utils.NewLogger("nodepools_repo"),
	}
}

// Create creates a new nodepool
func (r *NodePoolsRepository) Create(ctx context.Context, nodepool *models.NodePool) error {
	if nodepool.ID == uuid.Nil {
		nodepool.ID = uuid.New()
	}

	nodepool.CreatedAt = time.Now()
	nodepool.UpdatedAt = time.Now()

	query := `
		INSERT INTO nodepools (
			id, cluster_id, name, generation, resource_version, spec,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)`

	_, err := r.client.ExecContext(ctx, query,
		nodepool.ID,
		nodepool.ClusterID,
		nodepool.Name,
		nodepool.Generation,
		nodepool.ResourceVersion,
		nodepool.Spec,
		nodepool.CreatedAt,
		nodepool.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to create nodepool",
			zap.String("nodepool_name", nodepool.Name),
			zap.String("cluster_id", nodepool.ClusterID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to create nodepool: %w", err)
	}

	r.logger.Info("NodePool created successfully",
		zap.String("nodepool_id", nodepool.ID.String()),
		zap.String("nodepool_name", nodepool.Name),
		zap.String("cluster_id", nodepool.ClusterID.String()),
	)

	return nil
}

// GetByID retrieves a nodepool by ID with client isolation (through cluster ownership)
func (r *NodePoolsRepository) GetByID(ctx context.Context, id uuid.UUID, createdBy string) (*models.NodePool, error) {
	query := `
		SELECT np.id, np.cluster_id, np.name, np.generation, np.resource_version, np.spec,
			   np.created_at, np.updated_at, np.deleted_at
		FROM nodepools np
		INNER JOIN clusters c ON np.cluster_id = c.id
		WHERE np.id = $1 AND c.created_by = $2 AND np.deleted_at IS NULL AND c.deleted_at IS NULL`

	var nodepool models.NodePool
	err := r.client.QueryRowContext(ctx, query, id, createdBy).Scan(
		&nodepool.ID,
		&nodepool.ClusterID,
		&nodepool.Name,
		&nodepool.Generation,
		&nodepool.ResourceVersion,
		&nodepool.Spec,
		&nodepool.CreatedAt,
		&nodepool.UpdatedAt,
		&nodepool.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrNodePoolNotFound
	}
	if err != nil {
		r.logger.Error("Failed to get nodepool by ID",
			zap.String("nodepool_id", id.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get nodepool: %w", err)
	}

	return &nodepool, nil
}

// GetByIDInternal retrieves a nodepool by ID without client isolation (for internal use only, e.g., scheduler)
func (r *NodePoolsRepository) GetByIDInternal(ctx context.Context, id uuid.UUID) (*models.NodePool, error) {
	query := `
		SELECT id, cluster_id, name, generation, resource_version, spec,
		       created_at, updated_at, deleted_at
		FROM nodepools
		WHERE id = $1 AND deleted_at IS NULL`

	var nodepool models.NodePool
	err := r.client.QueryRowContext(ctx, query, id).Scan(
		&nodepool.ID,
		&nodepool.ClusterID,
		&nodepool.Name,
		&nodepool.Generation,
		&nodepool.ResourceVersion,
		&nodepool.Spec,
		&nodepool.CreatedAt,
		&nodepool.UpdatedAt,
		&nodepool.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrNodePoolNotFound
	}
	if err != nil {
		r.logger.Error("Failed to get nodepool by ID (internal)",
			zap.String("nodepool_id", id.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get nodepool: %w", err)
	}

	return &nodepool, nil
}

// GetByClusterAndName retrieves a nodepool by cluster ID and name with client isolation
func (r *NodePoolsRepository) GetByClusterAndName(ctx context.Context, clusterID uuid.UUID, name string, createdBy string) (*models.NodePool, error) {
	query := `
		SELECT np.id, np.cluster_id, np.name, np.generation, np.resource_version, np.spec,
			   np.created_at, np.updated_at, np.deleted_at
		FROM nodepools np
		INNER JOIN clusters c ON np.cluster_id = c.id
		WHERE np.cluster_id = $1 AND np.name = $2 AND c.created_by = $3 AND np.deleted_at IS NULL AND c.deleted_at IS NULL`

	var nodepool models.NodePool
	err := r.client.QueryRowContext(ctx, query, clusterID, name, createdBy).Scan(
		&nodepool.ID,
		&nodepool.ClusterID,
		&nodepool.Name,
		&nodepool.Generation,
		&nodepool.ResourceVersion,
		&nodepool.Spec,
		&nodepool.CreatedAt,
		&nodepool.UpdatedAt,
		&nodepool.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrNodePoolNotFound
	}
	if err != nil {
		r.logger.Error("Failed to get nodepool by cluster and name",
			zap.String("cluster_id", clusterID.String()),
			zap.String("nodepool_name", name),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get nodepool: %w", err)
	}

	return &nodepool, nil
}

// ListByCluster retrieves all nodepools for a cluster with client isolation
func (r *NodePoolsRepository) ListByCluster(ctx context.Context, clusterID uuid.UUID, createdBy string, opts *models.ListOptions) ([]*models.NodePool, error) {
	baseQuery := `
		SELECT np.id, np.cluster_id, np.name, np.generation, np.resource_version, np.spec,
			   np.created_at, np.updated_at, np.deleted_at
		FROM nodepools np
		INNER JOIN clusters c ON np.cluster_id = c.id
		WHERE np.cluster_id = $1 AND c.created_by = $2 AND np.deleted_at IS NULL AND c.deleted_at IS NULL`

	var args []interface{}
	args = append(args, clusterID, createdBy)
	var conditions []string
	argIndex := 3

	// Note: Status and Health filters removed - use status.phase instead if needed

	// Build the complete query
	query := baseQuery
	if len(conditions) > 0 {
		query += " AND " + conditions[0]
		for _, condition := range conditions[1:] {
			query += " AND " + condition
		}
	}

	// Add ordering
	query += " ORDER BY created_at DESC"

	// Add pagination
	if opts != nil {
		if opts.Limit > 0 {
			query += fmt.Sprintf(" LIMIT $%d", argIndex)
			args = append(args, opts.Limit)
			argIndex++
		}
		if opts.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argIndex)
			args = append(args, opts.Offset)
			argIndex++
		}
	}

	rows, err := r.client.QueryContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to list nodepools by cluster",
			zap.String("cluster_id", clusterID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to list nodepools: %w", err)
	}
	defer rows.Close()

	var nodepools []*models.NodePool
	for rows.Next() {
		var nodepool models.NodePool
		err := rows.Scan(
			&nodepool.ID,
			&nodepool.ClusterID,
			&nodepool.Name,
			&nodepool.Generation,
			&nodepool.ResourceVersion,
			&nodepool.Spec,
			&nodepool.CreatedAt,
			&nodepool.UpdatedAt,
			&nodepool.DeletedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan nodepool row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan nodepool: %w", err)
		}
		nodepools = append(nodepools, &nodepool)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating nodepool rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating nodepools: %w", err)
	}

	return nodepools, nil
}

// List retrieves nodepools with optional filtering and client isolation
func (r *NodePoolsRepository) List(ctx context.Context, createdBy string, opts *models.ListOptions) ([]*models.NodePool, error) {
	baseQuery := `
		SELECT np.id, np.cluster_id, np.name, np.generation, np.resource_version, np.spec,
			   np.created_at, np.updated_at, np.deleted_at
		FROM nodepools np
		INNER JOIN clusters c ON np.cluster_id = c.id
		WHERE np.deleted_at IS NULL AND c.deleted_at IS NULL AND c.created_by = $1`

	var args []interface{}
	args = append(args, createdBy)
	var conditions []string
	argIndex := 2 // Start at 2 because $1 is createdBy

	// Note: Status and Health filters removed - use status.phase instead if needed

	// Build the complete query
	query := baseQuery
	if len(conditions) > 0 {
		query += " AND " + conditions[0]
		for _, condition := range conditions[1:] {
			query += " AND " + condition
		}
	}

	// Add ordering
	query += " ORDER BY created_at DESC"

	// Add pagination
	if opts != nil {
		if opts.Limit > 0 {
			query += fmt.Sprintf(" LIMIT $%d", argIndex)
			args = append(args, opts.Limit)
			argIndex++
		}
		if opts.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argIndex)
			args = append(args, opts.Offset)
			argIndex++
		}
	}

	rows, err := r.client.QueryContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to list nodepools", zap.Error(err))
		return nil, fmt.Errorf("failed to list nodepools: %w", err)
	}
	defer rows.Close()

	var nodepools []*models.NodePool
	for rows.Next() {
		var nodepool models.NodePool
		err := rows.Scan(
			&nodepool.ID,
			&nodepool.ClusterID,
			&nodepool.Name,
			&nodepool.Generation,
			&nodepool.ResourceVersion,
			&nodepool.Spec,
			&nodepool.CreatedAt,
			&nodepool.UpdatedAt,
			&nodepool.DeletedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan nodepool row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan nodepool: %w", err)
		}
		nodepools = append(nodepools, &nodepool)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating nodepool rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating nodepools: %w", err)
	}

	return nodepools, nil
}

// Update updates an existing nodepool with client isolation
func (r *NodePoolsRepository) Update(ctx context.Context, nodepool *models.NodePool, createdBy string) error {
	nodepool.UpdatedAt = time.Now()

	query := `
		UPDATE nodepools
		SET name = $3, generation = $4, resource_version = $5, spec = $6,
			updated_at = $7
		WHERE id = $1 AND cluster_id = $2 AND deleted_at IS NULL
		AND EXISTS (SELECT 1 FROM clusters c WHERE c.id = $2 AND c.created_by = $8 AND c.deleted_at IS NULL)`

	result, err := r.client.ExecContext(ctx, query,
		nodepool.ID,
		nodepool.ClusterID,
		nodepool.Name,
		nodepool.Generation,
		nodepool.ResourceVersion,
		nodepool.Spec,
		nodepool.UpdatedAt,
		createdBy,
	)

	if err != nil {
		r.logger.Error("Failed to update nodepool",
			zap.String("nodepool_id", nodepool.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update nodepool: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrNodePoolNotFound
	}

	r.logger.Info("NodePool updated successfully",
		zap.String("nodepool_id", nodepool.ID.String()),
		zap.String("nodepool_name", nodepool.Name),
	)

	return nil
}

// UpdateStatus is deprecated - status is now managed through controller_status table
// This method is kept for backwards compatibility but does nothing
func (r *NodePoolsRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, health string) error {
	r.logger.Warn("UpdateStatus called with deprecated overall_status/overall_health fields - ignoring",
		zap.String("nodepool_id", id.String()),
	)
	return nil
}

// Delete performs a soft delete of a nodepool with client isolation
func (r *NodePoolsRepository) Delete(ctx context.Context, id uuid.UUID, createdBy string) error {
	query := `
		UPDATE nodepools
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		AND EXISTS (SELECT 1 FROM clusters c WHERE c.id = nodepools.cluster_id AND c.created_by = $2 AND c.deleted_at IS NULL)`

	result, err := r.client.ExecContext(ctx, query, id, createdBy)
	if err != nil {
		r.logger.Error("Failed to delete nodepool",
			zap.String("nodepool_id", id.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete nodepool: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrNodePoolNotFound
	}

	r.logger.Info("NodePool deleted successfully",
		zap.String("nodepool_id", id.String()),
	)

	return nil
}

// DeleteByCluster deletes all nodepools for a cluster (soft delete)
func (r *NodePoolsRepository) DeleteByCluster(ctx context.Context, clusterID uuid.UUID) error {
	query := `
		UPDATE nodepools
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE cluster_id = $1 AND deleted_at IS NULL`

	result, err := r.client.ExecContext(ctx, query, clusterID)
	if err != nil {
		r.logger.Error("Failed to delete nodepools by cluster",
			zap.String("cluster_id", clusterID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete nodepools by cluster: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	r.logger.Info("NodePools deleted successfully",
		zap.String("cluster_id", clusterID.String()),
		zap.Int64("rows_affected", rowsAffected),
	)

	return nil
}

// Count returns the total number of nodepools matching the filter criteria
func (r *NodePoolsRepository) Count(ctx context.Context, createdBy string, opts *models.ListOptions) (int64, error) {
	baseQuery := `SELECT COUNT(*)
		FROM nodepools np
		INNER JOIN clusters c ON np.cluster_id = c.id
		WHERE np.deleted_at IS NULL AND c.deleted_at IS NULL AND c.created_by = $1`

	var args []interface{}
	args = append(args, createdBy)
	var conditions []string

	// Note: Status and Health filters removed - use status.phase instead if needed

	// Build the complete query
	query := baseQuery
	if len(conditions) > 0 {
		query += " AND " + conditions[0]
		for _, condition := range conditions[1:] {
			query += " AND " + condition
		}
	}

	var count int64
	err := r.client.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to count nodepools",
			zap.String("created_by", createdBy),
			zap.Error(err))
		return 0, fmt.Errorf("failed to count nodepools: %w", err)
	}

	return count, nil
}

// CountByCluster returns the total number of nodepools for a specific cluster
func (r *NodePoolsRepository) CountByCluster(ctx context.Context, clusterID uuid.UUID) (int64, error) {
	query := "SELECT COUNT(*) FROM nodepools WHERE cluster_id = $1 AND deleted_at IS NULL"

	var count int64
	err := r.client.QueryRowContext(ctx, query, clusterID).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to count nodepools by cluster",
			zap.String("cluster_id", clusterID.String()),
			zap.Error(err),
		)
		return 0, fmt.Errorf("failed to count nodepools by cluster: %w", err)
	}

	return count, nil
}
