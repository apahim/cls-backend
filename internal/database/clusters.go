package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/apahim/cls-backend/internal/models"
	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ClustersRepository handles database operations for clusters
type ClustersRepository struct {
	client           *Client
	logger           *utils.Logger
	statusAggregator *StatusAggregator
}

// NewClustersRepository creates a new clusters repository
func NewClustersRepository(client *Client) *ClustersRepository {
	return &ClustersRepository{
		client:           client,
		logger:           utils.NewLogger("clusters_repo"),
		statusAggregator: NewStatusAggregator(client),
	}
}

// isPrivilegedSystemUser checks if the user is a privileged system user that can access all clusters
func (r *ClustersRepository) isPrivilegedSystemUser(userEmail string) bool {
	// System users (controllers, operators, etc.) can access all clusters
	if strings.HasSuffix(userEmail, "@system.local") {
		return true
	}

	// Known privileged system users
	privilegedUsers := []string{
		"controller@system.local",
		"operator@system.local",
		"scheduler@system.local",
		"system@system.local",
	}

	for _, privilegedUser := range privilegedUsers {
		if userEmail == privilegedUser {
			return true
		}
	}

	return false
}

// Create creates a new cluster
func (r *ClustersRepository) Create(ctx context.Context, cluster *models.Cluster) error {
	if cluster.ID == uuid.Nil {
		cluster.ID = uuid.New()
	}

	cluster.CreatedAt = time.Now()
	cluster.UpdatedAt = time.Now()

	query := `
		INSERT INTO clusters (
			id, name, target_project_id, created_by,
			generation, resource_version, spec, status_dirty,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)`

	_, err := r.client.ExecContext(ctx, query,
		cluster.ID,
		cluster.Name,
		cluster.TargetProjectID,
		cluster.CreatedBy,
		cluster.Generation,
		cluster.ResourceVersion,
		cluster.Spec,
		true, // status_dirty = true for new clusters
		cluster.CreatedAt,
		cluster.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to create cluster",
			zap.String("cluster_name", cluster.Name),
			zap.Error(err),
		)
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	r.logger.Info("Cluster created successfully",
		zap.String("cluster_id", cluster.ID.String()),
		zap.String("cluster_name", cluster.Name),
	)

	return nil
}

// GetByID retrieves a cluster by ID with client isolation (skipped for privileged system users)
func (r *ClustersRepository) GetByID(ctx context.Context, id uuid.UUID, createdBy string) (*models.Cluster, error) {
	var query string
	var args []interface{}

	if r.isPrivilegedSystemUser(createdBy) {
		// Privileged system users can access any cluster
		query = `
			SELECT id, name, target_project_id, created_by,
				   generation, resource_version, spec, status,
				   status_dirty, created_at, updated_at, deleted_at
			FROM clusters
			WHERE id = $1 AND deleted_at IS NULL`
		args = []interface{}{id}
	} else {
		// Regular users can only access their own clusters
		query = `
			SELECT id, name, target_project_id, created_by,
				   generation, resource_version, spec, status,
				   status_dirty, created_at, updated_at, deleted_at
			FROM clusters
			WHERE id = $1 AND created_by = $2 AND deleted_at IS NULL`
		args = []interface{}{id, createdBy}
	}

	var cluster models.Cluster
	err := r.client.QueryRowContext(ctx, query, args...).Scan(
		&cluster.ID,
		&cluster.Name,
		&cluster.TargetProjectID,
		&cluster.CreatedBy,
		&cluster.Generation,
		&cluster.ResourceVersion,
		&cluster.Spec,
		&cluster.Status,
		&cluster.StatusDirty,
		&cluster.CreatedAt,
		&cluster.UpdatedAt,
		&cluster.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrClusterNotFound
	}
	if err != nil {
		r.logger.Error("Failed to get cluster by ID",
			zap.String("cluster_id", id.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	// Enrich with real-time status
	if err := r.statusAggregator.EnrichClusterWithStatus(ctx, &cluster); err != nil {
		r.logger.Warn("Failed to enrich cluster with real-time status",
			zap.String("cluster_id", id.String()),
			zap.Error(err),
		)
		// Continue without failing - return cluster with existing status
	}

	return &cluster, nil
}

// GetByName retrieves a cluster by name with client isolation (skipped for privileged system users)
func (r *ClustersRepository) GetByName(ctx context.Context, name string, createdBy string) (*models.Cluster, error) {
	var query string
	var args []interface{}

	if r.isPrivilegedSystemUser(createdBy) {
		// Privileged system users can access any cluster
		query = `
			SELECT id, name, target_project_id, created_by,
				   generation, resource_version, spec, status,
				   status_dirty, created_at, updated_at, deleted_at
			FROM clusters
			WHERE name = $1 AND deleted_at IS NULL`
		args = []interface{}{name}
	} else {
		// Regular users can only access their own clusters
		query = `
			SELECT id, name, target_project_id, created_by,
				   generation, resource_version, spec, status,
				   status_dirty, created_at, updated_at, deleted_at
			FROM clusters
			WHERE name = $1 AND created_by = $2 AND deleted_at IS NULL`
		args = []interface{}{name, createdBy}
	}

	var cluster models.Cluster
	err := r.client.QueryRowContext(ctx, query, args...).Scan(
		&cluster.ID,
		&cluster.Name,
		&cluster.TargetProjectID,
		&cluster.CreatedBy,
		&cluster.Generation,
		&cluster.ResourceVersion,
		&cluster.Spec,
		&cluster.Status,
		&cluster.StatusDirty,
		&cluster.CreatedAt,
		&cluster.UpdatedAt,
		&cluster.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrClusterNotFound
	}
	if err != nil {
		r.logger.Error("Failed to get cluster by name",
			zap.String("cluster_name", name),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	// Enrich with real-time status
	if err := r.statusAggregator.EnrichClusterWithStatus(ctx, &cluster); err != nil {
		r.logger.Warn("Failed to enrich cluster with real-time status",
			zap.String("cluster_name", name),
			zap.Error(err),
		)
		// Continue without failing - return cluster with existing status
	}

	return &cluster, nil
}

// List retrieves clusters for a specific user with client isolation
func (r *ClustersRepository) List(ctx context.Context, createdBy string, opts *models.ListOptions) ([]*models.Cluster, error) {
	baseQuery := `
		SELECT id, name, target_project_id, created_by,
			   generation, resource_version, spec, status,
			   status_dirty, created_at, updated_at, deleted_at
		FROM clusters
		WHERE created_by = $1 AND deleted_at IS NULL`

	var args []interface{}
	args = append(args, createdBy)
	argIndex := 2

	// Build the complete query - base query already has the created_by filter
	query := baseQuery

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
		r.logger.Error("Failed to list clusters", zap.Error(err))
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}
	defer rows.Close()

	var clusters []*models.Cluster
	for rows.Next() {
		var cluster models.Cluster
		err := rows.Scan(
			&cluster.ID,
			&cluster.Name,
			&cluster.TargetProjectID,
			&cluster.CreatedBy,
			&cluster.Generation,
			&cluster.ResourceVersion,
			&cluster.Spec,
			&cluster.Status,
			&cluster.StatusDirty,
			&cluster.CreatedAt,
			&cluster.UpdatedAt,
			&cluster.DeletedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan cluster row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan cluster: %w", err)
		}
		clusters = append(clusters, &cluster)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating cluster rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating clusters: %w", err)
	}

	// Enrich all clusters with real-time status
	if err := r.statusAggregator.EnrichClustersWithStatus(ctx, clusters); err != nil {
		r.logger.Warn("Failed to enrich some clusters with real-time status",
			zap.Int("cluster_count", len(clusters)),
			zap.Error(err),
		)
		// Continue without failing - return clusters with existing status
	}

	return clusters, nil
}

// Update updates an existing cluster with client isolation
func (r *ClustersRepository) Update(ctx context.Context, cluster *models.Cluster, createdBy string) error {
	cluster.UpdatedAt = time.Now()

	query := `
		UPDATE clusters
		SET name = $2, generation = $3, resource_version = $4,
			spec = $5, updated_at = $6
		WHERE id = $1 AND created_by = $7 AND deleted_at IS NULL`

	result, err := r.client.ExecContext(ctx, query,
		cluster.ID,
		cluster.Name,
		cluster.Generation,
		cluster.ResourceVersion,
		cluster.Spec,
		cluster.UpdatedAt,
		createdBy,
	)

	if err != nil {
		r.logger.Error("Failed to update cluster",
			zap.String("cluster_id", cluster.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update cluster: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrClusterNotFound
	}

	r.logger.Info("Cluster updated successfully",
		zap.String("cluster_id", cluster.ID.String()),
		zap.String("cluster_name", cluster.Name),
	)

	return nil
}

// Delete performs a soft delete of a cluster with client isolation
func (r *ClustersRepository) Delete(ctx context.Context, id uuid.UUID, createdBy string) error {
	query := `
		UPDATE clusters
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND created_by = $2 AND deleted_at IS NULL`

	result, err := r.client.ExecContext(ctx, query, id, createdBy)
	if err != nil {
		r.logger.Error("Failed to delete cluster",
			zap.String("cluster_id", id.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrClusterNotFound
	}

	r.logger.Info("Cluster deleted successfully",
		zap.String("cluster_id", id.String()),
	)

	return nil
}

// Count returns the total number of clusters for a specific user
func (r *ClustersRepository) Count(ctx context.Context, createdBy string) (int64, error) {
	query := "SELECT COUNT(*) FROM clusters WHERE created_by = $1 AND deleted_at IS NULL"

	var count int64
	err := r.client.QueryRowContext(ctx, query, createdBy).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to count clusters", zap.Error(err))
		return 0, fmt.Errorf("failed to count clusters: %w", err)
	}

	return count, nil
}

// MarkDirtyStatus marks a cluster's status as dirty, requiring recalculation
func (r *ClustersRepository) MarkDirtyStatus(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE clusters
		SET status_dirty = TRUE, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.client.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to mark cluster status as dirty",
			zap.String("cluster_id", id.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to mark cluster status as dirty: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrClusterNotFound
	}

	r.logger.Debug("Marked cluster status as dirty",
		zap.String("cluster_id", id.String()),
	)

	return nil
}

// GetDirtyClusters retrieves clusters that need status aggregation
func (r *ClustersRepository) GetDirtyClusters(ctx context.Context, limit int) ([]*models.Cluster, error) {
	query := `
		SELECT id, name, target_project_id, created_by,
			   generation, resource_version, spec, status,
			   status_dirty, created_at, updated_at, deleted_at
		FROM clusters
		WHERE status_dirty = TRUE AND deleted_at IS NULL
		ORDER BY updated_at ASC
		LIMIT $1`

	rows, err := r.client.QueryContext(ctx, query, limit)
	if err != nil {
		r.logger.Error("Failed to get dirty clusters", zap.Error(err))
		return nil, fmt.Errorf("failed to get dirty clusters: %w", err)
	}
	defer rows.Close()

	var clusters []*models.Cluster
	for rows.Next() {
		var cluster models.Cluster
		err := rows.Scan(
			&cluster.ID,
			&cluster.Name,
			&cluster.TargetProjectID,
			&cluster.CreatedBy,
			&cluster.Generation,
			&cluster.ResourceVersion,
			&cluster.Spec,
			&cluster.Status,
			&cluster.StatusDirty,
			&cluster.CreatedAt,
			&cluster.UpdatedAt,
			&cluster.DeletedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan dirty cluster row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan dirty cluster: %w", err)
		}
		clusters = append(clusters, &cluster)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating dirty cluster rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating dirty clusters: %w", err)
	}

	r.logger.Debug("Retrieved dirty clusters",
		zap.Int("count", len(clusters)),
	)

	return clusters, nil
}

// ListByCreatedBy retrieves clusters created by a specific user (for future authorization)
func (r *ClustersRepository) ListByCreatedBy(ctx context.Context, createdBy string, opts *models.ListOptions) ([]*models.Cluster, error) {
	baseQuery := `
		SELECT id, name, target_project_id, created_by,
			   generation, resource_version, spec, status,
			   status_dirty, created_at, updated_at, deleted_at
		FROM clusters
		WHERE created_by = $1 AND deleted_at IS NULL`

	var args []interface{}
	args = append(args, createdBy)
	argIndex := 2

	// Add ordering
	query := baseQuery + " ORDER BY created_at DESC"

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
		r.logger.Error("Failed to list clusters by created_by",
			zap.String("created_by", createdBy),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}
	defer rows.Close()

	var clusters []*models.Cluster
	for rows.Next() {
		var cluster models.Cluster
		err := rows.Scan(
			&cluster.ID,
			&cluster.Name,
			&cluster.TargetProjectID,
			&cluster.CreatedBy,
			&cluster.Generation,
			&cluster.ResourceVersion,
			&cluster.Spec,
			&cluster.Status,
			&cluster.StatusDirty,
			&cluster.CreatedAt,
			&cluster.UpdatedAt,
			&cluster.DeletedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan cluster row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan cluster: %w", err)
		}
		clusters = append(clusters, &cluster)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating cluster rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating clusters: %w", err)
	}

	// Enrich all clusters with real-time status
	if err := r.statusAggregator.EnrichClustersWithStatus(ctx, clusters); err != nil {
		r.logger.Warn("Failed to enrich some clusters with real-time status",
			zap.String("created_by", createdBy),
			zap.Int("cluster_count", len(clusters)),
			zap.Error(err),
		)
		// Continue without failing - return clusters with existing status
	}

	return clusters, nil
}

// CountByCreatedBy returns the total number of clusters created by a specific user
func (r *ClustersRepository) CountByCreatedBy(ctx context.Context, createdBy string) (int64, error) {
	query := "SELECT COUNT(*) FROM clusters WHERE created_by = $1 AND deleted_at IS NULL"

	var count int64
	err := r.client.QueryRowContext(ctx, query, createdBy).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to count clusters by created_by",
			zap.String("created_by", createdBy),
			zap.Error(err),
		)
		return 0, fmt.Errorf("failed to count clusters: %w", err)
	}

	return count, nil
}

// System-wide access methods (for controllers)

// ListAll retrieves all clusters (system-wide access for controllers)
func (r *ClustersRepository) ListAll(ctx context.Context, opts *models.ListOptions) ([]*models.Cluster, error) {
	baseQuery := `
		SELECT id, name, target_project_id, created_by,
			   generation, resource_version, spec, status,
			   status_dirty, created_at, updated_at, deleted_at
		FROM clusters
		WHERE deleted_at IS NULL`

	var args []interface{}
	argIndex := 1

	// Add ordering
	query := baseQuery + " ORDER BY created_at DESC"

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
		r.logger.Error("Failed to list all clusters", zap.Error(err))
		return nil, fmt.Errorf("failed to list all clusters: %w", err)
	}
	defer rows.Close()

	var clusters []*models.Cluster
	for rows.Next() {
		var cluster models.Cluster
		err := rows.Scan(
			&cluster.ID,
			&cluster.Name,
			&cluster.TargetProjectID,
			&cluster.CreatedBy,
			&cluster.Generation,
			&cluster.ResourceVersion,
			&cluster.Spec,
			&cluster.Status,
			&cluster.StatusDirty,
			&cluster.CreatedAt,
			&cluster.UpdatedAt,
			&cluster.DeletedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan cluster row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan cluster: %w", err)
		}
		clusters = append(clusters, &cluster)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating cluster rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating clusters: %w", err)
	}

	// Enrich all clusters with real-time status
	if err := r.statusAggregator.EnrichClustersWithStatus(ctx, clusters); err != nil {
		r.logger.Warn("Failed to enrich some clusters with real-time status",
			zap.Int("cluster_count", len(clusters)),
			zap.Error(err),
		)
		// Continue without failing - return clusters with existing status
	}

	return clusters, nil
}

// CountAll returns the total number of clusters system-wide
func (r *ClustersRepository) CountAll(ctx context.Context) (int64, error) {
	query := "SELECT COUNT(*) FROM clusters WHERE deleted_at IS NULL"

	var count int64
	err := r.client.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to count all clusters", zap.Error(err))
		return 0, fmt.Errorf("failed to count all clusters: %w", err)
	}

	return count, nil
}

// GetByIDWithoutFilter retrieves a cluster by ID without access control filtering (for controllers)
func (r *ClustersRepository) GetByIDWithoutFilter(ctx context.Context, id uuid.UUID) (*models.Cluster, error) {
	query := `
		SELECT id, name, target_project_id, created_by,
			   generation, resource_version, spec, status,
			   status_dirty, created_at, updated_at, deleted_at
		FROM clusters
		WHERE id = $1 AND deleted_at IS NULL`

	var cluster models.Cluster
	err := r.client.QueryRowContext(ctx, query, id).Scan(
		&cluster.ID,
		&cluster.Name,
		&cluster.TargetProjectID,
		&cluster.CreatedBy,
		&cluster.Generation,
		&cluster.ResourceVersion,
		&cluster.Spec,
		&cluster.Status,
		&cluster.StatusDirty,
		&cluster.CreatedAt,
		&cluster.UpdatedAt,
		&cluster.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrClusterNotFound
	}
	if err != nil {
		r.logger.Error("Failed to get cluster by ID without filter",
			zap.String("cluster_id", id.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	// Enrich with real-time status
	if err := r.statusAggregator.EnrichClusterWithStatus(ctx, &cluster); err != nil {
		r.logger.Warn("Failed to enrich cluster with real-time status",
			zap.String("cluster_id", id.String()),
			zap.Error(err),
		)
		// Continue without failing - return cluster with existing status
	}

	return &cluster, nil
}

// UpdateWithoutFilter updates a cluster without access control filtering (for controllers)
func (r *ClustersRepository) UpdateWithoutFilter(ctx context.Context, cluster *models.Cluster) error {
	cluster.UpdatedAt = time.Now()

	query := `
		UPDATE clusters
		SET name = $2, generation = $3, resource_version = $4,
			spec = $5, updated_at = $6
		WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.client.ExecContext(ctx, query,
		cluster.ID,
		cluster.Name,
		cluster.Generation,
		cluster.ResourceVersion,
		cluster.Spec,
		cluster.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to update cluster without filter",
			zap.String("cluster_id", cluster.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update cluster: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrClusterNotFound
	}

	r.logger.Info("Cluster updated successfully without filter",
		zap.String("cluster_id", cluster.ID.String()),
		zap.String("cluster_name", cluster.Name),
	)

	return nil
}

// DeleteWithoutFilter performs a soft delete without access control filtering (for controllers)
func (r *ClustersRepository) DeleteWithoutFilter(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE clusters
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.client.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete cluster without filter",
			zap.String("cluster_id", id.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrClusterNotFound
	}

	r.logger.Info("Cluster deleted successfully without filter",
		zap.String("cluster_id", id.String()),
	)

	return nil
}