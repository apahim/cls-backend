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

func setupStatusTestRepository(t *testing.T) *Repository {
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

	// Run a minimal migration to create the status tables
	ctx := context.Background()
	_, err = repo.GetClient().ExecContext(ctx, `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

		CREATE TABLE IF NOT EXISTS clusters (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			name VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL DEFAULT 'default',
			generation BIGINT NOT NULL DEFAULT 1,
			resource_version VARCHAR(255) NOT NULL DEFAULT uuid_generate_v4()::text,
			spec JSONB NOT NULL,
			overall_status VARCHAR(50) NOT NULL DEFAULT 'Pending',
			overall_health VARCHAR(50) NOT NULL DEFAULT 'Unknown',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMP NULL,
			status_dirty BOOLEAN DEFAULT FALSE,

			CONSTRAINT clusters_name_namespace_unique UNIQUE(name, namespace)
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

		CREATE TABLE IF NOT EXISTS controller_status (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
			controller_name VARCHAR(255) NOT NULL,
			observed_generation BIGINT NOT NULL DEFAULT 0,
			conditions JSONB,
			last_error JSONB,
			metadata JSONB,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

			CONSTRAINT controller_status_cluster_controller_unique UNIQUE(cluster_id, controller_name)
		);

		CREATE TABLE IF NOT EXISTS nodepool_controller_status (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			nodepool_id UUID NOT NULL REFERENCES nodepools(id) ON DELETE CASCADE,
			controller_name VARCHAR(255) NOT NULL,
			observed_generation BIGINT NOT NULL DEFAULT 0,
			conditions JSONB,
			last_error JSONB,
			metadata JSONB,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

			CONSTRAINT nodepool_controller_status_nodepool_controller_unique UNIQUE(nodepool_id, controller_name)
		);

		CREATE TABLE IF NOT EXISTS cluster_events (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			cluster_id VARCHAR(255) NOT NULL,
			nodepool_id VARCHAR(255),
			controller_name VARCHAR(255) NOT NULL,
			event_type VARCHAR(50) NOT NULL,
			event_data JSONB,
			published_at TIMESTAMP NOT NULL DEFAULT NOW(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);

		CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ language 'plpgsql';

		DROP TRIGGER IF EXISTS update_controller_status_updated_at ON controller_status;
		CREATE TRIGGER update_controller_status_updated_at
			BEFORE UPDATE ON controller_status
			FOR EACH ROW
			EXECUTE PROCEDURE update_updated_at_column();

		DROP TRIGGER IF EXISTS update_nodepool_controller_status_updated_at ON nodepool_controller_status;
		CREATE TRIGGER update_nodepool_controller_status_updated_at
			BEFORE UPDATE ON nodepool_controller_status
			FOR EACH ROW
			EXECUTE PROCEDURE update_updated_at_column();
	`)
	utils.AssertError(t, err, false, "Should create test schema")

	return repo
}

func createTestClusterControllerStatus(clusterID uuid.UUID) *models.ClusterControllerStatus {
	return &models.ClusterControllerStatus{
		ClusterID:          clusterID,
		ControllerName:     "test-controller",
		ObservedGeneration: 1,
		Conditions: models.ConditionList{
			{
				Type:               "Available",
				Status:             "True",
				Reason:             "Ready",
				Message:            "Controller is ready",
				LastTransitionTime: time.Now(),
			},
		},
		LastError: nil,
		Metadata: models.JSONB(map[string]interface{}{
			"version": "1.0.0",
		}),
	}
}

func createTestNodePoolControllerStatus(nodepoolID uuid.UUID) *models.NodePoolControllerStatus {
	return &models.NodePoolControllerStatus{
		NodePoolID:         nodepoolID,
		ControllerName:     "test-controller",
		ObservedGeneration: 1,
		Conditions: models.ConditionList{
			{
				Type:               "Available",
				Status:             "True",
				Reason:             "Ready",
				Message:            "Controller is ready",
				LastTransitionTime: time.Now(),
			},
		},
		LastError: nil,
		Metadata: models.JSONB(map[string]interface{}{
			"version": "1.0.0",
		}),
	}
}

func createTestClusterEvent() *models.ClusterEvent {
	return &models.ClusterEvent{
		ID:        uuid.New(),
		ClusterID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		EventType: "StatusUpdate",
		Changes: models.JSONB(map[string]interface{}{
			"status": "Ready",
		}),
		PublishedAt: time.Now(),
	}
}

func TestClusterControllerStatusRepository_Create(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	status := createTestClusterControllerStatus(cluster.ID)

	err := repo.ClusterControllerStatus.Create(ctx, status)
	utils.AssertError(t, err, false, "Should create cluster controller status without error")

	// Verify status was created
	utils.AssertNotEqual(t, uuid.Nil, status.ID, "Status ID should be set")
	utils.AssertFalse(t, status.CreatedAt.IsZero(), "CreatedAt should be set")
	utils.AssertFalse(t, status.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestClusterControllerStatusRepository_CreateDuplicate(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	status1 := createTestClusterControllerStatus(cluster.ID)
	status2 := createTestClusterControllerStatus(cluster.ID)
	status2.ControllerName = status1.ControllerName

	err := repo.ClusterControllerStatus.Create(ctx, status1)
	utils.AssertError(t, err, false, "Should create first status")

	err = repo.ClusterControllerStatus.Create(ctx, status2)
	utils.AssertError(t, err, true, "Should fail to create duplicate status")
}

func TestClusterControllerStatusRepository_GetByClusterAndController(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	status := createTestClusterControllerStatus(cluster.ID)

	err := repo.ClusterControllerStatus.Create(ctx, status)
	utils.AssertError(t, err, false, "Should create status")

	// Get existing status
	retrieved, err := repo.ClusterControllerStatus.GetByClusterAndController(ctx, cluster.ID, status.ControllerName)
	utils.AssertError(t, err, false, "Should get status by cluster and controller")
	utils.AssertNotNil(t, retrieved, "Retrieved status should not be nil")
	utils.AssertEqual(t, status.ID, retrieved.ID, "Status ID should match")
	utils.AssertEqual(t, status.ControllerName, retrieved.ControllerName, "Controller name should match")
	utils.AssertEqual(t, cluster.ID, retrieved.ClusterID, "Cluster ID should match")

	// Get non-existent status
	retrieved, err = repo.ClusterControllerStatus.GetByClusterAndController(ctx, cluster.ID, "non-existent")
	utils.AssertError(t, err, true, "Should return error for non-existent status")
	utils.AssertEqual(t, models.ErrStatusNotFound, err, "Should return ErrStatusNotFound")
	utils.AssertNil(t, retrieved, "Retrieved status should be nil")
}

func TestClusterControllerStatusRepository_ListByCluster(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster1 := createTestClusterForNodePools(repo, ctx)
	cluster2 := createTestClusterForNodePools(repo, ctx)
	cluster2.Name = "test-cluster-2"
	err := repo.Clusters.Create(ctx, cluster2)
	utils.AssertError(t, err, false, "Should create second cluster")

	// Create statuses for cluster1
	status1 := createTestClusterControllerStatus(cluster1.ID)
	status1.ControllerName = "controller-1"

	status2 := createTestClusterControllerStatus(cluster1.ID)
	status2.ControllerName = "controller-2"

	// Create status for cluster2
	status3 := createTestClusterControllerStatus(cluster2.ID)
	status3.ControllerName = "controller-3"

	err = repo.ClusterControllerStatus.Create(ctx, status1)
	utils.AssertError(t, err, false, "Should create status 1")
	err = repo.ClusterControllerStatus.Create(ctx, status2)
	utils.AssertError(t, err, false, "Should create status 2")
	err = repo.ClusterControllerStatus.Create(ctx, status3)
	utils.AssertError(t, err, false, "Should create status 3")

	// List statuses for cluster1
	statuses, err := repo.ClusterControllerStatus.ListByCluster(ctx, cluster1.ID)
	utils.AssertError(t, err, false, "Should list statuses for cluster1")
	utils.AssertEqual(t, 2, len(statuses), "Should have 2 statuses for cluster1")

	// List statuses for cluster2
	statuses, err = repo.ClusterControllerStatus.ListByCluster(ctx, cluster2.ID)
	utils.AssertError(t, err, false, "Should list statuses for cluster2")
	utils.AssertEqual(t, 1, len(statuses), "Should have 1 status for cluster2")
}

func TestClusterControllerStatusRepository_Update(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	status := createTestClusterControllerStatus(cluster.ID)

	err := repo.ClusterControllerStatus.Create(ctx, status)
	utils.AssertError(t, err, false, "Should create status")

	originalUpdatedAt := status.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure different timestamp

	// Update status
	status.ObservedGeneration = 2
	status.Conditions.SetCondition(models.Condition{
		Type:    "Available",
		Status:  "False",
		Reason:  "NotReady",
		Message: "Controller is not ready",
	})

	err = repo.ClusterControllerStatus.Update(ctx, status)
	utils.AssertError(t, err, false, "Should update status")

	// Verify update
	utils.AssertEqual(t, int64(2), status.ObservedGeneration, "Generation should be updated")
	utils.AssertTrue(t, status.UpdatedAt.After(originalUpdatedAt), "UpdatedAt should be updated")

	// Verify in database
	retrieved, err := repo.ClusterControllerStatus.GetByClusterAndController(ctx, cluster.ID, status.ControllerName)
	utils.AssertError(t, err, false, "Should get updated status")
	utils.AssertEqual(t, int64(2), retrieved.ObservedGeneration, "Generation should be persisted")
	utils.AssertEqual(t, "False", retrieved.Conditions[0].Status, "Condition should be updated")

	// Update non-existent status
	nonExistent := createTestClusterControllerStatus(cluster.ID)
	nonExistent.ControllerName = "non-existent"
	err = repo.ClusterControllerStatus.Update(ctx, nonExistent)
	utils.AssertError(t, err, true, "Should fail to update non-existent status")
	utils.AssertEqual(t, models.ErrStatusNotFound, err, "Should return ErrStatusNotFound")
}

func TestClusterControllerStatusRepository_Delete(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	status := createTestClusterControllerStatus(cluster.ID)

	err := repo.ClusterControllerStatus.Create(ctx, status)
	utils.AssertError(t, err, false, "Should create status")

	// Delete status
	err = repo.ClusterControllerStatus.Delete(ctx, cluster.ID, status.ControllerName)
	utils.AssertError(t, err, false, "Should delete status")

	// Verify status is deleted
	retrieved, err := repo.ClusterControllerStatus.GetByClusterAndController(ctx, cluster.ID, status.ControllerName)
	utils.AssertError(t, err, true, "Should not find deleted status")
	utils.AssertEqual(t, models.ErrStatusNotFound, err, "Should return ErrStatusNotFound")
	utils.AssertNil(t, retrieved, "Retrieved status should be nil")

	// Delete non-existent status
	err = repo.ClusterControllerStatus.Delete(ctx, cluster.ID, "non-existent")
	utils.AssertError(t, err, true, "Should fail to delete non-existent status")
	utils.AssertEqual(t, models.ErrStatusNotFound, err, "Should return ErrStatusNotFound")
}

func TestNodePoolControllerStatusRepository_Create(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	nodepool := createTestNodePool(cluster.ID)
	err := repo.NodePools.Create(ctx, nodepool)
	utils.AssertError(t, err, false, "Should create nodepool")

	status := createTestNodePoolControllerStatus(nodepool.ID)

	err = repo.NodePoolControllerStatus.Create(ctx, status)
	utils.AssertError(t, err, false, "Should create nodepool controller status without error")

	// Verify status was created
	utils.AssertNotEqual(t, uuid.Nil, status.ID, "Status ID should be set")
	utils.AssertFalse(t, status.CreatedAt.IsZero(), "CreatedAt should be set")
	utils.AssertFalse(t, status.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestNodePoolControllerStatusRepository_GetByNodePoolAndController(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)
	nodepool := createTestNodePool(cluster.ID)
	err := repo.NodePools.Create(ctx, nodepool)
	utils.AssertError(t, err, false, "Should create nodepool")

	status := createTestNodePoolControllerStatus(nodepool.ID)

	err = repo.NodePoolControllerStatus.Create(ctx, status)
	utils.AssertError(t, err, false, "Should create status")

	// Get existing status
	retrieved, err := repo.NodePoolControllerStatus.GetByNodePoolAndController(ctx, nodepool.ID, status.ControllerName)
	utils.AssertError(t, err, false, "Should get status by nodepool and controller")
	utils.AssertNotNil(t, retrieved, "Retrieved status should not be nil")
	utils.AssertEqual(t, status.ID, retrieved.ID, "Status ID should match")
	utils.AssertEqual(t, status.ControllerName, retrieved.ControllerName, "Controller name should match")
	utils.AssertEqual(t, nodepool.ID, retrieved.NodePoolID, "NodePool ID should match")

	// Get non-existent status
	retrieved, err = repo.NodePoolControllerStatus.GetByNodePoolAndController(ctx, nodepool.ID, "non-existent")
	utils.AssertError(t, err, true, "Should return error for non-existent status")
	utils.AssertEqual(t, models.ErrStatusNotFound, err, "Should return ErrStatusNotFound")
	utils.AssertNil(t, retrieved, "Retrieved status should be nil")
}

func TestClusterEventsRepository_Create(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	event := createTestClusterEvent()

	err := repo.ClusterEvents.Create(ctx, event)
	utils.AssertError(t, err, false, "Should create cluster event without error")

	// Verify event was created
	utils.AssertNotEqual(t, uuid.Nil, event.ID, "Event ID should be set")
	utils.AssertFalse(t, event.CreatedAt.IsZero(), "CreatedAt should be set")
}

func TestClusterEventsRepository_List(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()

	// Create test events
	event1 := createTestClusterEvent()
	event1.ClusterID = "cluster-1"
	event1.EventType = "StatusUpdate"

	event2 := createTestClusterEvent()
	event2.ClusterID = "cluster-1"
	event2.EventType = "ErrorEvent"

	event3 := createTestClusterEvent()
	event3.ClusterID = "cluster-2"
	event3.EventType = "StatusUpdate"

	err := repo.ClusterEvents.Create(ctx, event1)
	utils.AssertError(t, err, false, "Should create event 1")
	err = repo.ClusterEvents.Create(ctx, event2)
	utils.AssertError(t, err, false, "Should create event 2")
	err = repo.ClusterEvents.Create(ctx, event3)
	utils.AssertError(t, err, false, "Should create event 3")

	// List all events
	events, err := repo.ClusterEvents.List(ctx, nil)
	utils.AssertError(t, err, false, "Should list all events")
	utils.AssertEqual(t, 3, len(events), "Should have 3 events")

	// List events by cluster
	opts := &models.ListOptions{ClusterID: "cluster-1"}
	events, err = repo.ClusterEvents.List(ctx, opts)
	utils.AssertError(t, err, false, "Should list events for cluster-1")
	utils.AssertEqual(t, 2, len(events), "Should have 2 events for cluster-1")

	// List events by type
	opts = &models.ListOptions{EventType: "StatusUpdate"}
	events, err = repo.ClusterEvents.List(ctx, opts)
	utils.AssertError(t, err, false, "Should list StatusUpdate events")
	utils.AssertEqual(t, 2, len(events), "Should have 2 StatusUpdate events")

	// List with pagination
	opts = &models.ListOptions{Limit: 2}
	events, err = repo.ClusterEvents.List(ctx, opts)
	utils.AssertError(t, err, false, "Should list with limit")
	utils.AssertEqual(t, 2, len(events), "Should have 2 events with limit")

	// List with offset
	opts = &models.ListOptions{Limit: 2, Offset: 1}
	events, err = repo.ClusterEvents.List(ctx, opts)
	utils.AssertError(t, err, false, "Should list with offset")
	utils.AssertEqual(t, 2, len(events), "Should have 2 events with offset")
}

func TestClusterEventsRepository_ListByCluster(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()

	// Create test events
	event1 := createTestClusterEvent()
	event1.ClusterID = "cluster-1"
	event1.NodePoolID = "nodepool-1"

	event2 := createTestClusterEvent()
	event2.ClusterID = "cluster-1"
	event2.NodePoolID = "nodepool-2"

	event3 := createTestClusterEvent()
	event3.ClusterID = "cluster-2"

	err := repo.ClusterEvents.Create(ctx, event1)
	utils.AssertError(t, err, false, "Should create event 1")
	err = repo.ClusterEvents.Create(ctx, event2)
	utils.AssertError(t, err, false, "Should create event 2")
	err = repo.ClusterEvents.Create(ctx, event3)
	utils.AssertError(t, err, false, "Should create event 3")

	// List events by cluster
	events, err := repo.ClusterEvents.ListByCluster(ctx, "cluster-1", nil)
	utils.AssertError(t, err, false, "Should list events for cluster-1")
	utils.AssertEqual(t, 2, len(events), "Should have 2 events for cluster-1")

	// List events by cluster with nodepool filter
	opts := &models.ListOptions{NodePoolID: "nodepool-1"}
	events, err = repo.ClusterEvents.ListByCluster(ctx, "cluster-1", opts)
	utils.AssertError(t, err, false, "Should list events for cluster-1 nodepool-1")
	utils.AssertEqual(t, 1, len(events), "Should have 1 event for cluster-1 nodepool-1")

	// List events for non-existent cluster
	events, err = repo.ClusterEvents.ListByCluster(ctx, "non-existent", nil)
	utils.AssertError(t, err, false, "Should list events for non-existent cluster")
	utils.AssertEqual(t, 0, len(events), "Should have 0 events for non-existent cluster")
}

func TestClusterEventsRepository_Count(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()

	// Initial count should be 0
	count, err := repo.ClusterEvents.Count(ctx, nil)
	utils.AssertError(t, err, false, "Should count events")
	utils.AssertEqual(t, int64(0), count, "Initial count should be 0")

	// Create events
	event1 := createTestClusterEvent()
	event1.ClusterID = "cluster-1"

	event2 := createTestClusterEvent()
	event2.ClusterID = "cluster-2"

	err = repo.ClusterEvents.Create(ctx, event1)
	utils.AssertError(t, err, false, "Should create event 1")
	err = repo.ClusterEvents.Create(ctx, event2)
	utils.AssertError(t, err, false, "Should create event 2")

	// Count all events
	count, err = repo.ClusterEvents.Count(ctx, nil)
	utils.AssertError(t, err, false, "Should count all events")
	utils.AssertEqual(t, int64(2), count, "Should have 2 events")

	// Count with filter
	opts := &models.ListOptions{ClusterID: "cluster-1"}
	count, err = repo.ClusterEvents.Count(ctx, opts)
	utils.AssertError(t, err, false, "Should count filtered events")
	utils.AssertEqual(t, int64(1), count, "Should have 1 event for cluster-1")
}

func TestStatusRepository_GetUnhealthyClusters(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster1 := createTestClusterForNodePools(repo, ctx)
	cluster2 := createTestClusterForNodePools(repo, ctx)
	cluster2.Name = "test-cluster-2"
	err := repo.Clusters.Create(ctx, cluster2)
	utils.AssertError(t, err, false, "Should create second cluster")

	// Create healthy status for cluster1
	healthyStatus := createTestClusterControllerStatus(cluster1.ID)
	healthyStatus.Conditions = models.ConditionList{
		{Type: "Available", Status: "True"},
	}
	healthyStatus.LastError = nil

	// Create unhealthy status for cluster2
	unhealthyStatus := createTestClusterControllerStatus(cluster2.ID)
	unhealthyStatus.Conditions = models.ConditionList{
		{Type: "Available", Status: "False"},
	}
	unhealthyStatus.LastError = &models.ErrorInfo{
		ErrorType: models.ErrorTypeTransient,
		Message:   "Controller error",
	}

	err = repo.ClusterControllerStatus.Create(ctx, healthyStatus)
	utils.AssertError(t, err, false, "Should create healthy status")
	err = repo.ClusterControllerStatus.Create(ctx, unhealthyStatus)
	utils.AssertError(t, err, false, "Should create unhealthy status")

	// Get unhealthy clusters
	clusters, err := repo.Status.GetUnhealthyClusters(ctx)
	utils.AssertError(t, err, false, "Should get unhealthy clusters")
	utils.AssertEqual(t, 1, len(clusters), "Should have 1 unhealthy cluster")
	utils.AssertEqual(t, cluster2.ID, clusters[0], "Should be cluster2")
}

func TestStatusRepository_GetClusterHealth(t *testing.T) {
	repo := setupStatusTestRepository(t)
	defer repo.Close()

	ctx := context.Background()
	cluster := createTestClusterForNodePools(repo, ctx)

	// Create controller statuses with different health states
	healthyStatus := createTestClusterControllerStatus(cluster.ID)
	healthyStatus.ControllerName = "healthy-controller"
	healthyStatus.Conditions = models.ConditionList{
		{Type: "Available", Status: "True"},
	}
	healthyStatus.LastError = nil

	unhealthyStatus := createTestClusterControllerStatus(cluster.ID)
	unhealthyStatus.ControllerName = "unhealthy-controller"
	unhealthyStatus.Conditions = models.ConditionList{
		{Type: "Available", Status: "False"},
	}

	err := repo.ClusterControllerStatus.Create(ctx, healthyStatus)
	utils.AssertError(t, err, false, "Should create healthy status")
	err = repo.ClusterControllerStatus.Create(ctx, unhealthyStatus)
	utils.AssertError(t, err, false, "Should create unhealthy status")

	// Get cluster health
	health, err := repo.Status.GetClusterHealth(ctx, cluster.ID)
	utils.AssertError(t, err, false, "Should get cluster health")
	utils.AssertNotNil(t, health, "Health should not be nil")
	utils.AssertEqual(t, cluster.ID, health.ClusterID, "Cluster ID should match")
	utils.AssertEqual(t, 2, health.TotalControllers, "Should have 2 controllers")
	utils.AssertEqual(t, 1, health.HealthyControllers, "Should have 1 healthy controller")
	utils.AssertEqual(t, 1, health.UnhealthyControllers, "Should have 1 unhealthy controller")
	utils.AssertEqual(t, models.HealthDegraded, health.OverallHealth, "Overall health should be degraded")
}
