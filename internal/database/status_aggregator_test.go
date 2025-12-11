package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/apahim/cls-backend/internal/config"
	"github.com/apahim/cls-backend/internal/models"
	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
)

func setupStatusAggregatorTest(t *testing.T) (*Repository, uuid.UUID) {
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
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()

	// Create minimal schema for testing
	_, err = repo.GetClient().ExecContext(ctx, `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

		CREATE TABLE IF NOT EXISTS clusters (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			name VARCHAR(255) NOT NULL,
			generation BIGINT NOT NULL DEFAULT 1,
			created_by VARCHAR(255) NOT NULL DEFAULT 'test@example.com',
			spec JSONB NOT NULL,
			status JSONB,
			status_dirty BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMP NULL
		);

		CREATE TABLE IF NOT EXISTS controller_status (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
			controller_name VARCHAR(255) NOT NULL,
			observed_generation BIGINT NOT NULL DEFAULT 0,
			conditions JSONB,
			last_error JSONB,
			metadata JSONB NOT NULL DEFAULT '{}',
			last_updated TIMESTAMP NOT NULL DEFAULT NOW(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

			CONSTRAINT controller_status_cluster_controller_unique UNIQUE(cluster_id, controller_name)
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	// Create test cluster
	clusterID := uuid.New()
	cluster := &models.Cluster{
		ID:         clusterID,
		Name:       "test-cluster",
		Generation: 1,
		CreatedBy:  "test@example.com",
		Spec: models.ClusterSpec{
			Platform: models.PlatformSpec{Type: "gcp"},
		},
		StatusDirty: true,
	}

	err = repo.Clusters.Create(ctx, cluster)
	if err != nil {
		t.Fatalf("Failed to create test cluster: %v", err)
	}

	return repo, clusterID
}

func TestStatusAggregator_SQLQueryVariants(t *testing.T) {
	repo, clusterID := setupStatusAggregatorTest(t)
	defer repo.Close()

	ctx := context.Background()

	// Test Case 1: Controller with Available=True (should be counted as ready)
	t.Run("Controller_Available_True", func(t *testing.T) {
		// Create controller status exactly like the API response we saw
		controllerStatus := &models.ClusterControllerStatus{
			ClusterID:          clusterID,
			ControllerName:     "cls-hypershift-client",
			ObservedGeneration: 1,
			Conditions: models.ConditionList{
				{
					Type:               "Applied",
					Status:             "True",
					LastTransitionTime: time.Now(),
					Reason:             "RemoteResourcesCreated",
					Message:            "Created remote cluster resources successfully",
				},
				{
					Type:               "Available",
					Status:             "True",
					LastTransitionTime: time.Now(),
					Reason:             "DeploymentReady",
					Message:            "Remote deployment is available",
				},
				{
					Type:               "Healthy",
					Status:             "True",
					LastTransitionTime: time.Now(),
					Reason:             "AllReplicasReady",
					Message:            "All replicas are healthy",
				},
			},
			LastError: nil,
			Metadata:  models.JSONB(map[string]interface{}{"test": "data"}),
		}

		err := repo.Status.UpsertClusterControllerStatus(ctx, controllerStatus)
		if err != nil {
			t.Fatalf("Failed to create controller status: %v", err)
		}

		// Test different SQL query variants
		testCases := []struct {
			name  string
			query string
		}{
			{
				name: "Original_Broken_Query",
				query: `
					SELECT
						COUNT(*) AS total,
						COUNT(CASE WHEN
							EXISTS (
								SELECT 1 FROM jsonb_array_elements(conditions) AS condition
								WHERE condition->>'type' = 'Available' AND condition->>'status' = 'True'
							)
						THEN 1 END) AS ready,
						COUNT(CASE WHEN last_error IS NOT NULL THEN 1 END) AS errors
					FROM controller_status
					WHERE cluster_id = $1 AND observed_generation = $2`,
			},
			{
				name: "Fixed_With_Table_Reference",
				query: `
					SELECT
						COUNT(*) AS total,
						COUNT(CASE WHEN
							EXISTS (
								SELECT 1
								FROM jsonb_array_elements(controller_status.conditions) AS condition
								WHERE condition->>'type' = 'Available' AND condition->>'status' = 'True'
							)
						THEN 1 END) AS ready,
						COUNT(CASE WHEN last_error IS NOT NULL THEN 1 END) AS errors
					FROM controller_status
					WHERE cluster_id = $1 AND observed_generation = $2`,
			},
			{
				name: "Alternative_JSONB_Contains",
				query: `
					SELECT
						COUNT(*) AS total,
						COUNT(CASE WHEN
							conditions @> '[{"type": "Available", "status": "True"}]'
						THEN 1 END) AS ready,
						COUNT(CASE WHEN last_error IS NOT NULL THEN 1 END) AS errors
					FROM controller_status
					WHERE cluster_id = $1 AND observed_generation = $2`,
			},
			{
				name: "Simple_JSONB_Query",
				query: `
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
					WHERE cluster_id = $1 AND observed_generation = $2`,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				var total, ready, errors int
				err := repo.GetClient().QueryRowContext(ctx, tc.query, clusterID, int64(1)).Scan(&total, &ready, &errors)

				t.Logf("Query: %s", tc.name)
				t.Logf("Results - Total: %d, Ready: %d, Errors: %d", total, ready, errors)

				if err != nil {
					t.Errorf("Query failed: %v", err)
					return
				}

				// Validate expected results
				if total != 1 {
					t.Errorf("Expected 1 total controller, got %d", total)
				}
				if ready != 1 {
					t.Errorf("Expected 1 ready controller, got %d", ready)
				}
				if errors != 0 {
					t.Errorf("Expected 0 error controllers, got %d", errors)
				}
			})
		}
	})

	// Test Case 2: Controller with Available=False (should NOT be counted as ready)
	t.Run("Controller_Available_False", func(t *testing.T) {
		// Clear previous data
		_, err := repo.GetClient().ExecContext(ctx, "DELETE FROM controller_status WHERE cluster_id = $1", clusterID)
		if err != nil {
			t.Fatalf("Failed to clear controller status: %v", err)
		}

		// Create controller status with Available=False
		controllerStatus := &models.ClusterControllerStatus{
			ClusterID:          clusterID,
			ControllerName:     "failing-controller",
			ObservedGeneration: 1,
			Conditions: models.ConditionList{
				{
					Type:               "Applied",
					Status:             "False",
					LastTransitionTime: time.Now(),
					Reason:             "ResourceReconciliationFailed",
					Message:            "Failed to create resources",
				},
				{
					Type:               "Available",
					Status:             "False",
					LastTransitionTime: time.Now(),
					Reason:             "DeploymentFailed",
					Message:            "Deployment is not available",
				},
			},
			LastError: &models.ErrorInfo{
				Message: "Test error",
			},
			Metadata: models.JSONB(map[string]interface{}{"test": "data"}),
		}

		err = repo.Status.UpsertClusterControllerStatus(ctx, controllerStatus)
		if err != nil {
			t.Fatalf("Failed to create controller status: %v", err)
		}

		// Test the working query
		query := `
			SELECT
				COUNT(*) AS total,
				COUNT(CASE WHEN
					EXISTS (
						SELECT 1
						FROM jsonb_array_elements(controller_status.conditions) AS condition
						WHERE condition->>'type' = 'Available' AND condition->>'status' = 'True'
					)
				THEN 1 END) AS ready,
				COUNT(CASE WHEN last_error IS NOT NULL THEN 1 END) AS errors
			FROM controller_status
			WHERE cluster_id = $1 AND observed_generation = $2`

		var total, ready, errors int
		err = repo.GetClient().QueryRowContext(ctx, query, clusterID, int64(1)).Scan(&total, &ready, &errors)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		t.Logf("Available=False Results - Total: %d, Ready: %d, Errors: %d", total, ready, errors)

		// Validate expected results for failing controller
		if total != 1 {
			t.Errorf("Expected 1 total controller, got %d", total)
		}
		if ready != 0 {
			t.Errorf("Expected 0 ready controllers (Available=False), got %d", ready)
		}
		if errors != 1 {
			t.Errorf("Expected 1 error controller, got %d", errors)
		}
	})

	// Test Case 3: Test Status Aggregator directly
	t.Run("StatusAggregator_Integration", func(t *testing.T) {
		// Clear and recreate with Available=True
		_, err := repo.GetClient().ExecContext(ctx, "DELETE FROM controller_status WHERE cluster_id = $1", clusterID)
		if err != nil {
			t.Fatalf("Failed to clear controller status: %v", err)
		}

		controllerStatus := &models.ClusterControllerStatus{
			ClusterID:          clusterID,
			ControllerName:     "test-controller",
			ObservedGeneration: 1,
			Conditions: models.ConditionList{
				{
					Type:               "Available",
					Status:             "True",
					LastTransitionTime: time.Now(),
					Reason:             "Ready",
					Message:            "Controller is ready",
				},
			},
			LastError: nil,
			Metadata:  models.JSONB(map[string]interface{}{"test": "data"}),
		}

		err = repo.Status.UpsertClusterControllerStatus(ctx, controllerStatus)
		if err != nil {
			t.Fatalf("Failed to create controller status: %v", err)
		}

		// Get cluster for testing
		cluster, err := repo.Clusters.GetByID(ctx, clusterID, "test@example.com")
		if err != nil {
			t.Fatalf("Failed to get cluster: %v", err)
		}

		// Test status aggregator
		aggregator := NewStatusAggregator(repo.GetClient())
		result, err := aggregator.CalculateClusterStatus(ctx, cluster)
		if err != nil {
			t.Fatalf("StatusAggregator failed: %v", err)
		}

		t.Logf("StatusAggregator Results:")
		t.Logf("  Total Controllers: %d", result.TotalControllers)
		t.Logf("  Ready Controllers: %d", result.ReadyControllers)
		t.Logf("  Failed Controllers: %d", result.FailedControllers)
		t.Logf("  Has Errors: %t", result.HasErrors)
		t.Logf("  Status Phase: %s", result.Status.Phase)
		t.Logf("  Status Reason: %s", result.Status.Reason)

		// Validate status aggregator results
		if result.TotalControllers != 1 {
			t.Errorf("Expected 1 total controller, got %d", result.TotalControllers)
		}
		if result.ReadyControllers != 1 {
			t.Errorf("Expected 1 ready controller, got %d", result.ReadyControllers)
		}
		if result.Status.Phase != "Ready" {
			t.Errorf("Expected phase 'Ready', got '%s'", result.Status.Phase)
		}
	})
}

// =============================================================================
// NODEPOOL STATUS AGGREGATION TESTS
// =============================================================================

func setupNodePoolStatusAggregatorTest(t *testing.T) (*Repository, uuid.UUID, uuid.UUID) {
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
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()

	// Create minimal schema for testing
	_, err = repo.GetClient().ExecContext(ctx, `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

		CREATE TABLE IF NOT EXISTS clusters (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			name VARCHAR(255) NOT NULL,
			generation BIGINT NOT NULL DEFAULT 1,
			created_by VARCHAR(255) NOT NULL DEFAULT 'test@example.com',
			spec JSONB NOT NULL,
			status JSONB,
			status_dirty BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMP NULL
		);

		CREATE TABLE IF NOT EXISTS nodepools (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			generation BIGINT NOT NULL DEFAULT 1,
			resource_version VARCHAR(255) NOT NULL,
			spec JSONB NOT NULL,
			status JSONB,
			status_dirty BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMP NULL,
			UNIQUE(cluster_id, name)
		);

		CREATE TABLE IF NOT EXISTS nodepool_controller_status (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			nodepool_id UUID NOT NULL REFERENCES nodepools(id) ON DELETE CASCADE,
			controller_name VARCHAR(255) NOT NULL,
			observed_generation BIGINT NOT NULL DEFAULT 0,
			conditions JSONB,
			last_error JSONB,
			metadata JSONB NOT NULL DEFAULT '{}',
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			CONSTRAINT nodepool_controller_status_unique UNIQUE(nodepool_id, controller_name)
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	// Create test cluster
	clusterID := uuid.New()
	cluster := &models.Cluster{
		ID:         clusterID,
		Name:       "test-cluster",
		Generation: 1,
		CreatedBy:  "test@example.com",
		Spec: models.ClusterSpec{
			Platform: models.PlatformSpec{Type: "gcp"},
		},
		StatusDirty: true,
	}

	err = repo.Clusters.Create(ctx, cluster)
	if err != nil {
		t.Fatalf("Failed to create test cluster: %v", err)
	}

	// Create test nodepool
	nodepoolID := uuid.New()
	nodepool := &models.NodePool{
		ID:              nodepoolID,
		ClusterID:       clusterID,
		Name:            "test-nodepool",
		Generation:      1,
		ResourceVersion: uuid.New().String(),
		Spec: models.NodePoolSpec{
			Platform: models.NodePoolPlatformSpec{Type: "gcp"},
		},
		StatusDirty: true,
	}

	_, err = repo.GetClient().ExecContext(ctx, `
		INSERT INTO nodepools (id, cluster_id, name, generation, resource_version, spec, status_dirty)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, nodepool.ID, nodepool.ClusterID, nodepool.Name, nodepool.Generation,
		nodepool.ResourceVersion, nodepool.Spec, nodepool.StatusDirty)
	if err != nil {
		t.Fatalf("Failed to create test nodepool: %v", err)
	}

	return repo, clusterID, nodepoolID
}

func TestNodePoolStatusAggregator_NoControllers(t *testing.T) {
	repo, _, nodepoolID := setupNodePoolStatusAggregatorTest(t)
	defer repo.Close()

	ctx := context.Background()

	// Get nodepool
	var nodepool models.NodePool
	err := repo.GetClient().QueryRowContext(ctx, `
		SELECT id, cluster_id, name, generation, resource_version, spec, status, status_dirty
		FROM nodepools WHERE id = $1
	`, nodepoolID).Scan(
		&nodepool.ID, &nodepool.ClusterID, &nodepool.Name, &nodepool.Generation,
		&nodepool.ResourceVersion, &nodepool.Spec, &nodepool.Status, &nodepool.StatusDirty,
	)
	if err != nil {
		t.Fatalf("Failed to get nodepool: %v", err)
	}

	// Test status aggregator with no controllers
	aggregator := NewStatusAggregator(repo.GetClient())
	result, err := aggregator.CalculateNodePoolStatus(ctx, &nodepool)
	if err != nil {
		t.Fatalf("StatusAggregator failed: %v", err)
	}

	t.Logf("No Controllers Results:")
	t.Logf("  Phase: %s", result.Status.Phase)
	t.Logf("  Reason: %s", result.Status.Reason)
	t.Logf("  Message: %s", result.Status.Message)
	t.Logf("  Total Controllers: %d", result.TotalControllers)

	// Validate results
	if result.TotalControllers != 0 {
		t.Errorf("Expected 0 total controllers, got %d", result.TotalControllers)
	}
	if result.Status.Phase != "Pending" {
		t.Errorf("Expected phase 'Pending', got '%s'", result.Status.Phase)
	}
	if result.Status.Reason != "NoControllers" {
		t.Errorf("Expected reason 'NoControllers', got '%s'", result.Status.Reason)
	}
	if len(result.Status.Conditions) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(result.Status.Conditions))
	}

	// Check Ready condition
	readyCondition := result.Status.Conditions[0]
	if readyCondition.Type != "Ready" {
		t.Errorf("Expected first condition type 'Ready', got '%s'", readyCondition.Type)
	}
	if readyCondition.Status != "False" {
		t.Errorf("Expected Ready status 'False', got '%s'", readyCondition.Status)
	}

	// Check Available condition
	availableCondition := result.Status.Conditions[1]
	if availableCondition.Type != "Available" {
		t.Errorf("Expected second condition type 'Available', got '%s'", availableCondition.Type)
	}
	if availableCondition.Status != "False" {
		t.Errorf("Expected Available status 'False', got '%s'", availableCondition.Status)
	}
}

func TestNodePoolStatusAggregator_AllReady(t *testing.T) {
	repo, _, nodepoolID := setupNodePoolStatusAggregatorTest(t)
	defer repo.Close()

	ctx := context.Background()

	// Create controller status with Available=True
	_, err := repo.GetClient().ExecContext(ctx, `
		INSERT INTO nodepool_controller_status (nodepool_id, controller_name, observed_generation, conditions, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`, nodepoolID, "nodepool-gcp-provisioner", int64(1),
		models.ConditionList{
			{
				Type:               "Ready",
				Status:             "True",
				LastTransitionTime: time.Now(),
				Reason:             "NodesProvisioned",
				Message:            "All nodes provisioned successfully",
			},
			{
				Type:               "Available",
				Status:             "True",
				LastTransitionTime: time.Now(),
				Reason:             "NodesAvailable",
				Message:            "All nodes are available",
			},
		},
		models.JSONB(map[string]interface{}{"nodes": 3}),
	)
	if err != nil {
		t.Fatalf("Failed to create controller status: %v", err)
	}

	// Get nodepool
	var nodepool models.NodePool
	err = repo.GetClient().QueryRowContext(ctx, `
		SELECT id, cluster_id, name, generation, resource_version, spec, status, status_dirty
		FROM nodepools WHERE id = $1
	`, nodepoolID).Scan(
		&nodepool.ID, &nodepool.ClusterID, &nodepool.Name, &nodepool.Generation,
		&nodepool.ResourceVersion, &nodepool.Spec, &nodepool.Status, &nodepool.StatusDirty,
	)
	if err != nil {
		t.Fatalf("Failed to get nodepool: %v", err)
	}

	// Test status aggregator
	aggregator := NewStatusAggregator(repo.GetClient())
	result, err := aggregator.CalculateNodePoolStatus(ctx, &nodepool)
	if err != nil {
		t.Fatalf("StatusAggregator failed: %v", err)
	}

	t.Logf("All Ready Results:")
	t.Logf("  Phase: %s", result.Status.Phase)
	t.Logf("  Reason: %s", result.Status.Reason)
	t.Logf("  Message: %s", result.Status.Message)
	t.Logf("  Total Controllers: %d", result.TotalControllers)
	t.Logf("  Ready Controllers: %d", result.ReadyControllers)

	// Validate results
	if result.TotalControllers != 1 {
		t.Errorf("Expected 1 total controller, got %d", result.TotalControllers)
	}
	if result.ReadyControllers != 1 {
		t.Errorf("Expected 1 ready controller, got %d", result.ReadyControllers)
	}
	if result.Status.Phase != "Ready" {
		t.Errorf("Expected phase 'Ready', got '%s'", result.Status.Phase)
	}
	if result.Status.Reason != "AllControllersReady" {
		t.Errorf("Expected reason 'AllControllersReady', got '%s'", result.Status.Reason)
	}

	// Check Ready condition
	readyCondition := result.Status.Conditions[0]
	if readyCondition.Type != "Ready" || readyCondition.Status != "True" {
		t.Errorf("Expected Ready=True, got %s=%s", readyCondition.Type, readyCondition.Status)
	}

	// Check Available condition
	availableCondition := result.Status.Conditions[1]
	if availableCondition.Type != "Available" || availableCondition.Status != "True" {
		t.Errorf("Expected Available=True, got %s=%s", availableCondition.Type, availableCondition.Status)
	}
}

func TestNodePoolStatusAggregator_PartialProgress(t *testing.T) {
	repo, _, nodepoolID := setupNodePoolStatusAggregatorTest(t)
	defer repo.Close()

	ctx := context.Background()

	// Create two controllers - one ready, one not ready
	_, err := repo.GetClient().ExecContext(ctx, `
		INSERT INTO nodepool_controller_status (nodepool_id, controller_name, observed_generation, conditions, metadata)
		VALUES
			($1, $2, $3, $4, $5),
			($1, $6, $3, $7, $5)
	`, nodepoolID, "controller-1", int64(1),
		models.ConditionList{
			{Type: "Available", Status: "True", LastTransitionTime: time.Now(), Reason: "Ready", Message: "Ready"},
		},
		models.JSONB(map[string]interface{}{}),
		"controller-2",
		models.ConditionList{
			{Type: "Available", Status: "False", LastTransitionTime: time.Now(), Reason: "Progressing", Message: "Working"},
		},
	)
	if err != nil {
		t.Fatalf("Failed to create controller statuses: %v", err)
	}

	// Get nodepool
	var nodepool models.NodePool
	err = repo.GetClient().QueryRowContext(ctx, `
		SELECT id, cluster_id, name, generation, resource_version, spec, status, status_dirty
		FROM nodepools WHERE id = $1
	`, nodepoolID).Scan(
		&nodepool.ID, &nodepool.ClusterID, &nodepool.Name, &nodepool.Generation,
		&nodepool.ResourceVersion, &nodepool.Spec, &nodepool.Status, &nodepool.StatusDirty,
	)
	if err != nil {
		t.Fatalf("Failed to get nodepool: %v", err)
	}

	// Test status aggregator
	aggregator := NewStatusAggregator(repo.GetClient())
	result, err := aggregator.CalculateNodePoolStatus(ctx, &nodepool)
	if err != nil {
		t.Fatalf("StatusAggregator failed: %v", err)
	}

	t.Logf("Partial Progress Results:")
	t.Logf("  Phase: %s", result.Status.Phase)
	t.Logf("  Reason: %s", result.Status.Reason)
	t.Logf("  Total Controllers: %d", result.TotalControllers)
	t.Logf("  Ready Controllers: %d", result.ReadyControllers)

	// Validate results
	if result.TotalControllers != 2 {
		t.Errorf("Expected 2 total controllers, got %d", result.TotalControllers)
	}
	if result.ReadyControllers != 1 {
		t.Errorf("Expected 1 ready controller, got %d", result.ReadyControllers)
	}
	if result.Status.Phase != "Progressing" {
		t.Errorf("Expected phase 'Progressing', got '%s'", result.Status.Phase)
	}
	if result.Status.Reason != "PartialProgress" {
		t.Errorf("Expected reason 'PartialProgress', got '%s'", result.Status.Reason)
	}

	// Check Ready condition is False
	readyCondition := result.Status.Conditions[0]
	if readyCondition.Status != "False" {
		t.Errorf("Expected Ready=False for partial progress, got %s", readyCondition.Status)
	}
}

func TestNodePoolStatusAggregator_EnrichWithStatus(t *testing.T) {
	repo, _, nodepoolID := setupNodePoolStatusAggregatorTest(t)
	defer repo.Close()

	ctx := context.Background()

	// Create controller status
	_, err := repo.GetClient().ExecContext(ctx, `
		INSERT INTO nodepool_controller_status (nodepool_id, controller_name, observed_generation, conditions, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`, nodepoolID, "test-controller", int64(1),
		models.ConditionList{
			{Type: "Available", Status: "True", LastTransitionTime: time.Now(), Reason: "Ready", Message: "Ready"},
		},
		models.JSONB(map[string]interface{}{}),
	)
	if err != nil {
		t.Fatalf("Failed to create controller status: %v", err)
	}

	// Get nodepool with dirty flag
	var nodepool models.NodePool
	err = repo.GetClient().QueryRowContext(ctx, `
		SELECT id, cluster_id, name, generation, resource_version, spec, status, status_dirty
		FROM nodepools WHERE id = $1
	`, nodepoolID).Scan(
		&nodepool.ID, &nodepool.ClusterID, &nodepool.Name, &nodepool.Generation,
		&nodepool.ResourceVersion, &nodepool.Spec, &nodepool.Status, &nodepool.StatusDirty,
	)
	if err != nil {
		t.Fatalf("Failed to get nodepool: %v", err)
	}

	// Verify status is dirty
	if !nodepool.StatusDirty {
		t.Fatal("Expected nodepool to be dirty initially")
	}

	// Test enrichment
	aggregator := NewStatusAggregator(repo.GetClient())
	err = aggregator.EnrichNodePoolWithStatus(ctx, &nodepool)
	if err != nil {
		t.Fatalf("EnrichNodePoolWithStatus failed: %v", err)
	}

	// Verify status was calculated
	if nodepool.Status == nil {
		t.Fatal("Expected status to be populated")
	}
	if nodepool.Status.Phase != "Ready" {
		t.Errorf("Expected phase 'Ready', got '%s'", nodepool.Status.Phase)
	}

	// Verify status was marked clean in memory
	if nodepool.StatusDirty {
		t.Error("Expected nodepool to be marked clean after enrichment")
	}

	// Verify status was cached in database
	var dbStatusDirty bool
	err = repo.GetClient().QueryRowContext(ctx, `
		SELECT status_dirty FROM nodepools WHERE id = $1
	`, nodepoolID).Scan(&dbStatusDirty)
	if err != nil {
		t.Fatalf("Failed to check database status: %v", err)
	}
	if dbStatusDirty {
		t.Error("Expected status_dirty to be FALSE in database after enrichment")
	}

	// Test that calling again uses cached status (fast path)
	nodepool.StatusDirty = false // Simulate clean status
	oldStatus := nodepool.Status
	err = aggregator.EnrichNodePoolWithStatus(ctx, &nodepool)
	if err != nil {
		t.Fatalf("EnrichNodePoolWithStatus (clean) failed: %v", err)
	}
	if nodepool.Status != oldStatus {
		t.Error("Expected cached status to be reused (pointer should be same)")
	}
}

func TestNodePoolStatusAggregator_BatchEnrichment(t *testing.T) {
	repo, clusterID, _ := setupNodePoolStatusAggregatorTest(t)
	defer repo.Close()

	ctx := context.Background()

	// Create multiple nodepools
	nodepools := []*models.NodePool{}
	for i := 0; i < 3; i++ {
		npID := uuid.New()
		_, err := repo.GetClient().ExecContext(ctx, `
			INSERT INTO nodepools (id, cluster_id, name, generation, resource_version, spec, status_dirty)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, npID, clusterID, fmt.Sprintf("nodepool-%d", i), int64(1), uuid.New().String(),
			models.NodePoolSpec{Platform: models.NodePoolPlatformSpec{Type: "gcp"}}, true)
		if err != nil {
			t.Fatalf("Failed to create nodepool %d: %v", i, err)
		}

		// Create controller status for each nodepool
		_, err = repo.GetClient().ExecContext(ctx, `
			INSERT INTO nodepool_controller_status (nodepool_id, controller_name, observed_generation, conditions, metadata)
			VALUES ($1, $2, $3, $4, $5)
		`, npID, "controller", int64(1),
			models.ConditionList{
				{Type: "Available", Status: "True", LastTransitionTime: time.Now(), Reason: "Ready", Message: "Ready"},
			},
			models.JSONB(map[string]interface{}{}),
		)
		if err != nil {
			t.Fatalf("Failed to create controller status for nodepool %d: %v", i, err)
		}

		var np models.NodePool
		err = repo.GetClient().QueryRowContext(ctx, `
			SELECT id, cluster_id, name, generation, resource_version, spec, status, status_dirty
			FROM nodepools WHERE id = $1
		`, npID).Scan(&np.ID, &np.ClusterID, &np.Name, &np.Generation,
			&np.ResourceVersion, &np.Spec, &np.Status, &np.StatusDirty)
		if err != nil {
			t.Fatalf("Failed to get nodepool %d: %v", i, err)
		}
		nodepools = append(nodepools, &np)
	}

	// Test batch enrichment
	aggregator := NewStatusAggregator(repo.GetClient())
	err := aggregator.EnrichNodePoolsWithStatus(ctx, nodepools)
	if err != nil {
		t.Fatalf("EnrichNodePoolsWithStatus failed: %v", err)
	}

	// Verify all nodepools were enriched
	for i, np := range nodepools {
		if np.Status == nil {
			t.Errorf("Nodepool %d: Expected status to be populated", i)
		}
		if np.Status.Phase != "Ready" {
			t.Errorf("Nodepool %d: Expected phase 'Ready', got '%s'", i, np.Status.Phase)
		}
		if np.StatusDirty {
			t.Errorf("Nodepool %d: Expected to be marked clean", i)
		}
	}

	t.Logf("Successfully enriched %d nodepools in batch", len(nodepools))
}
