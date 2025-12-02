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

func setupTestRepository(t *testing.T) *Repository {
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

	// Run a minimal migration to create the clusters table
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

		CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ language 'plpgsql';

		DROP TRIGGER IF EXISTS update_clusters_updated_at ON clusters;
		CREATE TRIGGER update_clusters_updated_at
			BEFORE UPDATE ON clusters
			FOR EACH ROW
			EXECUTE PROCEDURE update_updated_at_column();
	`)
	utils.AssertError(t, err, false, "Should create test schema")

	return repo
}

func createTestCluster() *models.Cluster {
	return &models.Cluster{
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
	}
}

func TestClustersRepository_Create(t *testing.T) {
	repo := setupTestRepository(t)
	defer repo.Close()

	cluster := createTestCluster()

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster)
	utils.AssertError(t, err, false, "Should create cluster without error")

	// Verify cluster was created
	utils.AssertNotEqual(t, uuid.Nil, cluster.ID, "Cluster ID should be set")
	utils.AssertFalse(t, cluster.CreatedAt.IsZero(), "CreatedAt should be set")
	utils.AssertFalse(t, cluster.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestClustersRepository_CreateDuplicate(t *testing.T) {
	repo := setupTestRepository(t)
	defer repo.Close()

	cluster1 := createTestCluster()
	cluster2 := createTestCluster()
	cluster2.Name = cluster1.Name

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster1)
	utils.AssertError(t, err, false, "Should create first cluster")

	err = repo.Clusters.Create(ctx, cluster2)
	utils.AssertError(t, err, true, "Should fail to create duplicate cluster")
}

func TestClustersRepository_GetByID(t *testing.T) {
	repo := setupTestRepository(t)
	defer repo.Close()

	cluster := createTestCluster()

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster)
	utils.AssertError(t, err, false, "Should create cluster")

	// Get existing cluster
	retrieved, err := repo.Clusters.GetByID(ctx, cluster.ID, "")
	utils.AssertError(t, err, false, "Should get cluster by ID")
	utils.AssertNotNil(t, retrieved, "Retrieved cluster should not be nil")
	utils.AssertEqual(t, cluster.ID, retrieved.ID, "Cluster ID should match")
	utils.AssertEqual(t, cluster.Name, retrieved.Name, "Cluster name should match")

	// Get non-existent cluster
	nonExistentID := uuid.New()
	retrieved, err = repo.Clusters.GetByID(ctx, nonExistentID, "")
	utils.AssertError(t, err, true, "Should return error for non-existent cluster")
	utils.AssertEqual(t, models.ErrClusterNotFound, err, "Should return ErrClusterNotFound")
	utils.AssertNil(t, retrieved, "Retrieved cluster should be nil")
}

func TestClustersRepository_GetByName(t *testing.T) {
	repo := setupTestRepository(t)
	defer repo.Close()

	cluster := createTestCluster()

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster)
	utils.AssertError(t, err, false, "Should create cluster")

	// Get existing cluster
	retrieved, err := repo.Clusters.GetByName(ctx, cluster.Name, "")
	utils.AssertError(t, err, false, "Should get cluster by name")
	utils.AssertNotNil(t, retrieved, "Retrieved cluster should not be nil")
	utils.AssertEqual(t, cluster.ID, retrieved.ID, "Cluster ID should match")
	utils.AssertEqual(t, cluster.Name, retrieved.Name, "Cluster name should match")

	// Get non-existent cluster
	retrieved, err = repo.Clusters.GetByName(ctx, "non-existent", "")
	utils.AssertError(t, err, true, "Should return error for non-existent cluster")
	utils.AssertEqual(t, models.ErrClusterNotFound, err, "Should return ErrClusterNotFound")
	utils.AssertNil(t, retrieved, "Retrieved cluster should be nil")
}

func TestClustersRepository_List(t *testing.T) {
	repo := setupTestRepository(t)
	defer repo.Close()

	// Create test clusters
	cluster1 := createTestCluster()
	cluster1.Name = "cluster-1"

	cluster2 := createTestCluster()
	cluster2.Name = "cluster-2"

	cluster3 := createTestCluster()
	cluster3.Name = "cluster-3"

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster1)
	utils.AssertError(t, err, false, "Should create cluster 1")
	err = repo.Clusters.Create(ctx, cluster2)
	utils.AssertError(t, err, false, "Should create cluster 2")
	err = repo.Clusters.Create(ctx, cluster3)
	utils.AssertError(t, err, false, "Should create cluster 3")

	// List all clusters
	clusters, err := repo.Clusters.List(ctx, "", nil)
	utils.AssertError(t, err, false, "Should list all clusters")
	utils.AssertEqual(t, 3, len(clusters), "Should have 3 clusters")

	// List with pagination
	opts := &models.ListOptions{Limit: 2}
	clusters, err = repo.Clusters.List(ctx, "", opts)
	utils.AssertError(t, err, false, "Should list with limit")
	utils.AssertEqual(t, 2, len(clusters), "Should have 2 clusters with limit")

	opts = &models.ListOptions{Limit: 2, Offset: 1}
	clusters, err = repo.Clusters.List(ctx, "", opts)
	utils.AssertError(t, err, false, "Should list with offset")
	utils.AssertEqual(t, 2, len(clusters), "Should have 2 clusters with offset")
}

func TestClustersRepository_Update(t *testing.T) {
	repo := setupTestRepository(t)
	defer repo.Close()

	cluster := createTestCluster()

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster)
	utils.AssertError(t, err, false, "Should create cluster")

	originalUpdatedAt := cluster.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure different timestamp

	// Update cluster
	cluster.Generation = 2

	err = repo.Clusters.Update(ctx, cluster, "")
	utils.AssertError(t, err, false, "Should update cluster")

	// Verify update
	utils.AssertEqual(t, int64(2), cluster.Generation, "Generation should be updated")
	utils.AssertTrue(t, cluster.UpdatedAt.After(originalUpdatedAt), "UpdatedAt should be updated")

	// Verify in database
	retrieved, err := repo.Clusters.GetByID(ctx, cluster.ID, "")
	utils.AssertError(t, err, false, "Should get updated cluster")
	utils.AssertEqual(t, int64(2), retrieved.Generation, "Generation should be persisted")

	// Update non-existent cluster
	nonExistent := createTestCluster()
	err = repo.Clusters.Update(ctx, nonExistent, "")
	utils.AssertError(t, err, true, "Should fail to update non-existent cluster")
	utils.AssertEqual(t, models.ErrClusterNotFound, err, "Should return ErrClusterNotFound")
}

// TestClustersRepository_UpdateStatus removed - UpdateStatus method no longer exists
// Status updates now happen via controller status tracking and aggregation

func TestClustersRepository_Delete(t *testing.T) {
	repo := setupTestRepository(t)
	defer repo.Close()

	cluster := createTestCluster()

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster)
	utils.AssertError(t, err, false, "Should create cluster")

	// Delete cluster
	err = repo.Clusters.Delete(ctx, cluster.ID, "")
	utils.AssertError(t, err, false, "Should delete cluster")

	// Verify cluster is soft deleted
	retrieved, err := repo.Clusters.GetByID(ctx, cluster.ID, "")
	utils.AssertError(t, err, true, "Should not find deleted cluster")
	utils.AssertEqual(t, models.ErrClusterNotFound, err, "Should return ErrClusterNotFound")
	utils.AssertNil(t, retrieved, "Retrieved cluster should be nil")

	// Delete non-existent cluster
	nonExistentID := uuid.New()
	err = repo.Clusters.Delete(ctx, nonExistentID, "")
	utils.AssertError(t, err, true, "Should fail to delete non-existent cluster")
	utils.AssertEqual(t, models.ErrClusterNotFound, err, "Should return ErrClusterNotFound")
}

func TestClustersRepository_Count(t *testing.T) {
	repo := setupTestRepository(t)
	defer repo.Close()

	ctx := context.Background()

	// Initial count should be 0
	count, err := repo.Clusters.Count(ctx, "")
	utils.AssertError(t, err, false, "Should count clusters")
	utils.AssertEqual(t, int64(0), count, "Initial count should be 0")

	// Create clusters
	cluster1 := createTestCluster()
	cluster1.Name = "cluster-1"

	cluster2 := createTestCluster()
	cluster2.Name = "cluster-2"

	err = repo.Clusters.Create(ctx, cluster1)
	utils.AssertError(t, err, false, "Should create cluster 1")
	err = repo.Clusters.Create(ctx, cluster2)
	utils.AssertError(t, err, false, "Should create cluster 2")

	// Count all clusters
	count, err = repo.Clusters.Count(ctx, "")
	utils.AssertError(t, err, false, "Should count all clusters")
	utils.AssertEqual(t, int64(2), count, "Should have 2 clusters")

	// Delete a cluster and verify count
	err = repo.Clusters.Delete(ctx, cluster1.ID, "")
	utils.AssertError(t, err, false, "Should delete cluster")

	count, err = repo.Clusters.Count(ctx, "")
	utils.AssertError(t, err, false, "Should count after delete")
	utils.AssertEqual(t, int64(1), count, "Should have 1 cluster after delete")
}
