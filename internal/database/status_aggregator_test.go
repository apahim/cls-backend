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
