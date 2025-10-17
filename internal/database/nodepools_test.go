package database

import (
	"context"
	"testing"
	"time"

	"github.com/apahim/cls-backend/internal/config"
	"github.com/apahim/cls-backend/internal/models"
	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
)

func setupNodePoolsTestRepository(t *testing.T) *Repository {
	utils.SkipIfNoTestDB(t)

	testDBURL := utils.SetupTestDB(t)
	cfg := config.DatabaseConfig{
		URL:             testDBURL,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	repo, err := NewRepository(cfg)
	utils.AssertError(t, err, false, "Should create repository")

	// Run a minimal migration to create the nodepools table
	ctx := context.Background()
	_, err = repo.GetClient().ExecContext(ctx, `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

		CREATE TABLE IF NOT EXISTS clusters (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			name VARCHAR(255) NOT NULL UNIQUE,
			generation BIGINT NOT NULL DEFAULT 1,
			resource_version VARCHAR(255) NOT NULL DEFAULT uuid_generate_v4()::text,
			spec JSONB NOT NULL,
			overall_status VARCHAR(50) NOT NULL DEFAULT 'Pending',
			overall_health VARCHAR(50) NOT NULL DEFAULT 'Unknown',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMP NULL,
			status_dirty BOOLEAN DEFAULT FALSE
		);

		CREATE TABLE IF NOT EXISTS nodepools (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			generation BIGINT NOT NULL DEFAULT 1,
			resource_version VARCHAR(255) NOT NULL DEFAULT uuid_generate_v4()::text,
			spec JSONB NOT NULL,
			overall_status VARCHAR(50) NOT NULL DEFAULT 'Pending',
			overall_health VARCHAR(50) NOT NULL DEFAULT 'Unknown',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMP NULL,
			status_dirty BOOLEAN DEFAULT FALSE,

			CONSTRAINT nodepools_cluster_name_unique UNIQUE(cluster_id, name)
		);

		CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ language 'plpgsql';

		DROP TRIGGER IF EXISTS update_nodepools_updated_at ON nodepools;
		CREATE TRIGGER update_nodepools_updated_at
			BEFORE UPDATE ON nodepools
			FOR EACH ROW
			EXECUTE PROCEDURE update_updated_at_column();
	`)
	utils.AssertError(t, err, false, "Should create test schema")

	return repo
}

func createTestNodePool(clusterID uuid.UUID) *models.NodePool {
	return &models.NodePool{
		ID:              uuid.New(),
		ClusterID:       clusterID,
		Name:            "test-nodepool",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.NodePoolSpec{
			ClusterName: "test-cluster",
			Platform: models.NodePoolPlatformSpec{
				Type: "GCP",
				GCP: &models.NodePoolGCPSpec{
					InstanceType: "n1-standard-4",
					DiskType:     "pd-standard",
					DiskSizeGB:   100,
				},
			},
			Replicas: func() *int32 { i := int32(3); return &i }(),
		},
		OverallStatus: models.StatusPending,
		OverallHealth: models.HealthUnknown,
	}
}

func createTestClusterForNodePools(repo *Repository, ctx context.Context) *models.Cluster {
	cluster := &models.Cluster{
		ID:              uuid.New(),
		Name:            "test-cluster",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.ClusterSpec{
			InfraID: "test-infra",
			Platform: models.PlatformSpec{
				Type: "GCP",
				GCP: &models.GCPSpec{
					ProjectID: "test-project",
					Region:    "us-central1",
				},
			},
			Release: models.ReleaseSpec{
				Image: "test-image",
			},
		},
		OverallStatus: models.StatusPending,
		OverallHealth: models.HealthUnknown,
	}

	err := repo.Clusters.Create(ctx, cluster)
	if err != nil {
		panic(err)
	}
	return cluster
}

func TestNodePoolsRepository_Create(t *testing.T) {
	repo := setupNodePoolsTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	nodepool := createTestNodePool(cluster.ID)

	err := repo.NodePools.Create(ctx, nodepool)
	utils.AssertError(t, err, false, "Should create nodepool without error")

	// Verify nodepool was created
	utils.AssertNotEqual(t, uuid.Nil, nodepool.ID, "NodePool ID should be set")
	utils.AssertFalse(t, nodepool.CreatedAt.IsZero(), "CreatedAt should be set")
	utils.AssertFalse(t, nodepool.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestNodePoolsRepository_CreateDuplicate(t *testing.T) {
	repo := setupNodePoolsTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	nodepool1 := createTestNodePool(cluster.ID)
	nodepool2 := createTestNodePool(cluster.ID)
	nodepool2.Name = nodepool1.Name

	err := repo.NodePools.Create(ctx, nodepool1)
	utils.AssertError(t, err, false, "Should create first nodepool")

	err = repo.NodePools.Create(ctx, nodepool2)
	utils.AssertError(t, err, true, "Should fail to create duplicate nodepool")
}

func TestNodePoolsRepository_GetByID(t *testing.T) {
	repo := setupNodePoolsTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	nodepool := createTestNodePool(cluster.ID)

	err := repo.NodePools.Create(ctx, nodepool)
	utils.AssertError(t, err, false, "Should create nodepool")

	// Get existing nodepool
	retrieved, err := repo.NodePools.GetByID(ctx, nodepool.ID)
	utils.AssertError(t, err, false, "Should get nodepool by ID")
	utils.AssertNotNil(t, retrieved, "Retrieved nodepool should not be nil")
	utils.AssertEqual(t, nodepool.ID, retrieved.ID, "NodePool ID should match")
	utils.AssertEqual(t, nodepool.Name, retrieved.Name, "NodePool name should match")
	utils.AssertEqual(t, nodepool.ClusterID, retrieved.ClusterID, "NodePool cluster ID should match")

	// Get non-existent nodepool
	nonExistentID := uuid.New()
	retrieved, err = repo.NodePools.GetByID(ctx, nonExistentID)
	utils.AssertError(t, err, true, "Should return error for non-existent nodepool")
	utils.AssertEqual(t, models.ErrNodePoolNotFound, err, "Should return ErrNodePoolNotFound")
	utils.AssertNil(t, retrieved, "Retrieved nodepool should be nil")
}

func TestNodePoolsRepository_GetByClusterAndName(t *testing.T) {
	repo := setupNodePoolsTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	nodepool := createTestNodePool(cluster.ID)

	err := repo.NodePools.Create(ctx, nodepool)
	utils.AssertError(t, err, false, "Should create nodepool")

	// Get existing nodepool
	retrieved, err := repo.NodePools.GetByClusterAndName(ctx, cluster.ID, nodepool.Name)
	utils.AssertError(t, err, false, "Should get nodepool by cluster and name")
	utils.AssertNotNil(t, retrieved, "Retrieved nodepool should not be nil")
	utils.AssertEqual(t, nodepool.ID, retrieved.ID, "NodePool ID should match")
	utils.AssertEqual(t, nodepool.Name, retrieved.Name, "NodePool name should match")

	// Get non-existent nodepool
	retrieved, err = repo.NodePools.GetByClusterAndName(ctx, cluster.ID, "non-existent")
	utils.AssertError(t, err, true, "Should return error for non-existent nodepool")
	utils.AssertEqual(t, models.ErrNodePoolNotFound, err, "Should return ErrNodePoolNotFound")
	utils.AssertNil(t, retrieved, "Retrieved nodepool should be nil")
}

func TestNodePoolsRepository_ListByCluster(t *testing.T) {
	repo := setupNodePoolsTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster1 := createTestClusterForNodePools(repo, ctx)
	cluster2 := createTestClusterForNodePools(repo, ctx)
	cluster2.Name = "test-cluster-2"
	err := repo.Clusters.Create(ctx, cluster2)
	utils.AssertError(t, err, false, "Should create second cluster")

	// Create nodepools for cluster1
	nodepool1 := createTestNodePool(cluster1.ID)
	nodepool1.Name = "nodepool-1"
	nodepool1.OverallStatus = models.StatusReady

	nodepool2 := createTestNodePool(cluster1.ID)
	nodepool2.Name = "nodepool-2"
	nodepool2.OverallStatus = models.StatusPending

	// Create nodepool for cluster2
	nodepool3 := createTestNodePool(cluster2.ID)
	nodepool3.Name = "nodepool-3"

	err = repo.NodePools.Create(ctx, nodepool1)
	utils.AssertError(t, err, false, "Should create nodepool 1")
	err = repo.NodePools.Create(ctx, nodepool2)
	utils.AssertError(t, err, false, "Should create nodepool 2")
	err = repo.NodePools.Create(ctx, nodepool3)
	utils.AssertError(t, err, false, "Should create nodepool 3")

	// List nodepools for cluster1
	nodepools, err := repo.NodePools.ListByCluster(ctx, cluster1.ID, nil)
	utils.AssertError(t, err, false, "Should list nodepools for cluster1")
	utils.AssertEqual(t, 2, len(nodepools), "Should have 2 nodepools for cluster1")

	// List nodepools for cluster2
	nodepools, err = repo.NodePools.ListByCluster(ctx, cluster2.ID, nil)
	utils.AssertError(t, err, false, "Should list nodepools for cluster2")
	utils.AssertEqual(t, 1, len(nodepools), "Should have 1 nodepool for cluster2")

	// List with status filter
	opts := &models.ListOptions{Status: string(models.StatusReady)}
	nodepools, err = repo.NodePools.ListByCluster(ctx, cluster1.ID, opts)
	utils.AssertError(t, err, false, "Should list ready nodepools")
	utils.AssertEqual(t, 1, len(nodepools), "Should have 1 ready nodepool")
	utils.AssertEqual(t, "nodepool-1", nodepools[0].Name, "Should be nodepool-1")

	// List with pagination
	opts = &models.ListOptions{Limit: 1}
	nodepools, err = repo.NodePools.ListByCluster(ctx, cluster1.ID, opts)
	utils.AssertError(t, err, false, "Should list with limit")
	utils.AssertEqual(t, 1, len(nodepools), "Should have 1 nodepool with limit")
}

func TestNodePoolsRepository_List(t *testing.T) {
	repo := setupNodePoolsTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)

	// Create test nodepools
	nodepool1 := createTestNodePool(cluster.ID)
	nodepool1.Name = "nodepool-1"
	nodepool1.OverallStatus = models.StatusReady

	nodepool2 := createTestNodePool(cluster.ID)
	nodepool2.Name = "nodepool-2"
	nodepool2.OverallStatus = models.StatusPending

	err := repo.NodePools.Create(ctx, nodepool1)
	utils.AssertError(t, err, false, "Should create nodepool 1")
	err = repo.NodePools.Create(ctx, nodepool2)
	utils.AssertError(t, err, false, "Should create nodepool 2")

	// List all nodepools
	nodepools, err := repo.NodePools.List(ctx, nil)
	utils.AssertError(t, err, false, "Should list all nodepools")
	utils.AssertEqual(t, 2, len(nodepools), "Should have 2 nodepools")

	// List with status filter
	opts := &models.ListOptions{Status: string(models.StatusReady)}
	nodepools, err = repo.NodePools.List(ctx, opts)
	utils.AssertError(t, err, false, "Should list ready nodepools")
	utils.AssertEqual(t, 1, len(nodepools), "Should have 1 ready nodepool")

	// List with pagination
	opts = &models.ListOptions{Limit: 1, Offset: 1}
	nodepools, err = repo.NodePools.List(ctx, opts)
	utils.AssertError(t, err, false, "Should list with pagination")
	utils.AssertEqual(t, 1, len(nodepools), "Should have 1 nodepool with pagination")
}

func TestNodePoolsRepository_Update(t *testing.T) {
	repo := setupNodePoolsTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	nodepool := createTestNodePool(cluster.ID)

	err := repo.NodePools.Create(ctx, nodepool)
	utils.AssertError(t, err, false, "Should create nodepool")

	originalUpdatedAt := nodepool.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure different timestamp

	// Update nodepool
	nodepool.Generation = 2
	nodepool.OverallStatus = models.StatusReady
	nodepool.OverallHealth = models.HealthHealthy

	err = repo.NodePools.Update(ctx, nodepool)
	utils.AssertError(t, err, false, "Should update nodepool")

	// Verify update
	utils.AssertEqual(t, int64(2), nodepool.Generation, "Generation should be updated")
	utils.AssertTrue(t, nodepool.UpdatedAt.After(originalUpdatedAt), "UpdatedAt should be updated")

	// Verify in database
	retrieved, err := repo.NodePools.GetByID(ctx, nodepool.ID)
	utils.AssertError(t, err, false, "Should get updated nodepool")
	utils.AssertEqual(t, int64(2), retrieved.Generation, "Generation should be persisted")
	utils.AssertEqual(t, models.StatusReady, retrieved.OverallStatus, "Status should be persisted")

	// Update non-existent nodepool
	nonExistent := createTestNodePool(cluster.ID)
	err = repo.NodePools.Update(ctx, nonExistent)
	utils.AssertError(t, err, true, "Should fail to update non-existent nodepool")
	utils.AssertEqual(t, models.ErrNodePoolNotFound, err, "Should return ErrNodePoolNotFound")
}

func TestNodePoolsRepository_UpdateStatus(t *testing.T) {
	repo := setupNodePoolsTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	nodepool := createTestNodePool(cluster.ID)

	err := repo.NodePools.Create(ctx, nodepool)
	utils.AssertError(t, err, false, "Should create nodepool")

	// Update status
	err = repo.NodePools.UpdateStatus(ctx, nodepool.ID, string(models.StatusReady), string(models.HealthHealthy))
	utils.AssertError(t, err, false, "Should update status")

	// Verify update
	retrieved, err := repo.NodePools.GetByID(ctx, nodepool.ID)
	utils.AssertError(t, err, false, "Should get updated nodepool")
	utils.AssertEqual(t, models.StatusReady, retrieved.OverallStatus, "Status should be updated")
	utils.AssertEqual(t, models.HealthHealthy, retrieved.OverallHealth, "Health should be updated")

	// Update non-existent nodepool
	nonExistentID := uuid.New()
	err = repo.NodePools.UpdateStatus(ctx, nonExistentID, string(models.StatusReady), string(models.HealthHealthy))
	utils.AssertError(t, err, true, "Should fail to update non-existent nodepool")
	utils.AssertEqual(t, models.ErrNodePoolNotFound, err, "Should return ErrNodePoolNotFound")
}

func TestNodePoolsRepository_Delete(t *testing.T) {
	repo := setupNodePoolsTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	nodepool := createTestNodePool(cluster.ID)

	err := repo.NodePools.Create(ctx, nodepool)
	utils.AssertError(t, err, false, "Should create nodepool")

	// Delete nodepool
	err = repo.NodePools.Delete(ctx, nodepool.ID)
	utils.AssertError(t, err, false, "Should delete nodepool")

	// Verify nodepool is soft deleted
	retrieved, err := repo.NodePools.GetByID(ctx, nodepool.ID)
	utils.AssertError(t, err, true, "Should not find deleted nodepool")
	utils.AssertEqual(t, models.ErrNodePoolNotFound, err, "Should return ErrNodePoolNotFound")
	utils.AssertNil(t, retrieved, "Retrieved nodepool should be nil")

	// Delete non-existent nodepool
	nonExistentID := uuid.New()
	err = repo.NodePools.Delete(ctx, nonExistentID)
	utils.AssertError(t, err, true, "Should fail to delete non-existent nodepool")
	utils.AssertEqual(t, models.ErrNodePoolNotFound, err, "Should return ErrNodePoolNotFound")
}

func TestNodePoolsRepository_Count(t *testing.T) {
	repo := setupNodePoolsTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)

	// Initial count should be 0
	count, err := repo.NodePools.Count(ctx, nil)
	utils.AssertError(t, err, false, "Should count nodepools")
	utils.AssertEqual(t, int64(0), count, "Initial count should be 0")

	// Create nodepools
	nodepool1 := createTestNodePool(cluster.ID)
	nodepool1.Name = "nodepool-1"

	nodepool2 := createTestNodePool(cluster.ID)
	nodepool2.Name = "nodepool-2"

	err = repo.NodePools.Create(ctx, nodepool1)
	utils.AssertError(t, err, false, "Should create nodepool 1")
	err = repo.NodePools.Create(ctx, nodepool2)
	utils.AssertError(t, err, false, "Should create nodepool 2")

	// Count all nodepools
	count, err = repo.NodePools.Count(ctx, nil)
	utils.AssertError(t, err, false, "Should count all nodepools")
	utils.AssertEqual(t, int64(2), count, "Should have 2 nodepools")

	// Count with filter
	opts := &models.ListOptions{Status: string(models.StatusPending)}
	count, err = repo.NodePools.Count(ctx, opts)
	utils.AssertError(t, err, false, "Should count filtered nodepools")
	utils.AssertEqual(t, int64(2), count, "Should have 2 pending nodepools")

	// Delete a nodepool and verify count
	err = repo.NodePools.Delete(ctx, nodepool1.ID)
	utils.AssertError(t, err, false, "Should delete nodepool")

	count, err = repo.NodePools.Count(ctx, nil)
	utils.AssertError(t, err, false, "Should count after delete")
	utils.AssertEqual(t, int64(1), count, "Should have 1 nodepool after delete")
}

func TestNodePoolsRepository_CountByCluster(t *testing.T) {
	repo := setupNodePoolsTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster1 := createTestClusterForNodePools(repo, ctx)
	cluster2 := createTestClusterForNodePools(repo, ctx)
	cluster2.Name = "test-cluster-2"
	err := repo.Clusters.Create(ctx, cluster2)
	utils.AssertError(t, err, false, "Should create second cluster")

	// Initial count should be 0
	count, err := repo.NodePools.CountByCluster(ctx, cluster1.ID)
	utils.AssertError(t, err, false, "Should count nodepools for cluster1")
	utils.AssertEqual(t, int64(0), count, "Initial count should be 0")

	// Create nodepools for cluster1
	nodepool1 := createTestNodePool(cluster1.ID)
	nodepool1.Name = "nodepool-1"

	nodepool2 := createTestNodePool(cluster1.ID)
	nodepool2.Name = "nodepool-2"

	// Create nodepool for cluster2
	nodepool3 := createTestNodePool(cluster2.ID)
	nodepool3.Name = "nodepool-3"

	err = repo.NodePools.Create(ctx, nodepool1)
	utils.AssertError(t, err, false, "Should create nodepool 1")
	err = repo.NodePools.Create(ctx, nodepool2)
	utils.AssertError(t, err, false, "Should create nodepool 2")
	err = repo.NodePools.Create(ctx, nodepool3)
	utils.AssertError(t, err, false, "Should create nodepool 3")

	// Count nodepools for cluster1
	count, err = repo.NodePools.CountByCluster(ctx, cluster1.ID)
	utils.AssertError(t, err, false, "Should count nodepools for cluster1")
	utils.AssertEqual(t, int64(2), count, "Should have 2 nodepools for cluster1")

	// Count nodepools for cluster2
	count, err = repo.NodePools.CountByCluster(ctx, cluster2.ID)
	utils.AssertError(t, err, false, "Should count nodepools for cluster2")
	utils.AssertEqual(t, int64(1), count, "Should have 1 nodepool for cluster2")
}
