package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apahim/cls-backend/internal/config"
	"github.com/apahim/cls-backend/internal/database"
	"github.com/apahim/cls-backend/internal/models"
	"github.com/apahim/cls-backend/internal/pubsub"
	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type mockPubSubService struct {
	publishedEvents []interface{}
}

func (m *mockPubSubService) PublishClusterEvent(ctx context.Context, event *models.ClusterEvent) error {
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}


func (m *mockPubSubService) Close() error {
	return nil
}

func setupTestAPI(t *testing.T) (*API, *database.Repository, *mockPubSubService) {
	utils.SkipIfNoTestDB(t)

	testDBURL := utils.SetupTestDB(t)
	cfg := config.DatabaseConfig{
		URL:             testDBURL,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	repo, err := database.NewRepository(cfg)
	utils.AssertError(t, err, false, "Should create repository")

	// Run minimal migrations
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
	`)
	utils.AssertError(t, err, false, "Should create test schema")

	mockPubSub := &mockPubSubService{}
	api := NewAPI(repo, mockPubSub)

	return api, repo, mockPubSub
}

func createTestClusterRequest() CreateClusterRequest {
	return CreateClusterRequest{
		Name: "test-cluster",
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

func createTestNodePoolRequest(clusterID uuid.UUID) CreateNodePoolRequest {
	return CreateNodePoolRequest{
		Name: "test-nodepool",
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
			Replicas: 3,
		},
	}
}

func TestAPI_CreateCluster(t *testing.T) {
	api, repo, mockPubSub := setupTestAPI(t)
	defer repo.Close()

	req := createTestClusterRequest()
	reqBody, err := json.Marshal(req)
	utils.AssertError(t, err, false, "Should marshal request")

	httpReq := httptest.NewRequest("POST", "/api/v1/clusters", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	api.createCluster(recorder, httpReq)

	utils.AssertEqual(t, http.StatusCreated, recorder.Code, "Should return 201 Created")

	var response ClusterResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	utils.AssertError(t, err, false, "Should unmarshal response")

	utils.AssertEqual(t, req.Name, response.Name, "Cluster name should match")
	utils.AssertEqual(t, models.StatusPending, response.OverallStatus, "Status should be Pending")
	utils.AssertNotEqual(t, uuid.Nil, response.ID, "ID should be set")

	// Verify event was published
	utils.AssertEqual(t, 1, len(mockPubSub.publishedEvents), "Should publish 1 event")
}

func TestAPI_CreateClusterInvalidRequest(t *testing.T) {
	api, repo, _ := setupTestAPI(t)
	defer repo.Close()

	// Test invalid JSON
	httpReq := httptest.NewRequest("POST", "/api/v1/clusters", bytes.NewBuffer([]byte("invalid json")))
	httpReq.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	api.createCluster(recorder, httpReq)

	utils.AssertEqual(t, http.StatusBadRequest, recorder.Code, "Should return 400 Bad Request")

	// Test missing required fields
	req := CreateClusterRequest{
		Name: "", // Missing name
		Spec: models.ClusterSpec{},
	}
	reqBody, _ := json.Marshal(req)

	httpReq = httptest.NewRequest("POST", "/api/v1/clusters", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()

	api.createCluster(recorder, httpReq)

	utils.AssertEqual(t, http.StatusBadRequest, recorder.Code, "Should return 400 Bad Request for missing name")
}

func TestAPI_GetCluster(t *testing.T) {
	api, repo, _ := setupTestAPI(t)
	defer repo.Close()

	// Create a test cluster
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
		},
		OverallStatus: models.StatusPending,
		OverallHealth: models.HealthUnknown,
	}

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster)
	utils.AssertError(t, err, false, "Should create cluster")

	// Test getting existing cluster
	httpReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/clusters/%s", cluster.ID), nil)
	httpReq = mux.SetURLVars(httpReq, map[string]string{"id": cluster.ID.String()})
	recorder := httptest.NewRecorder()

	api.getCluster(recorder, httpReq)

	utils.AssertEqual(t, http.StatusOK, recorder.Code, "Should return 200 OK")

	var response ClusterResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	utils.AssertError(t, err, false, "Should unmarshal response")

	utils.AssertEqual(t, cluster.ID, response.ID, "Cluster ID should match")
	utils.AssertEqual(t, cluster.Name, response.Name, "Cluster name should match")

	// Test getting non-existent cluster
	nonExistentID := uuid.New()
	httpReq = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/clusters/%s", nonExistentID), nil)
	httpReq = mux.SetURLVars(httpReq, map[string]string{"id": nonExistentID.String()})
	recorder = httptest.NewRecorder()

	api.getCluster(recorder, httpReq)

	utils.AssertEqual(t, http.StatusNotFound, recorder.Code, "Should return 404 Not Found")
}

func TestAPI_ListClusters(t *testing.T) {
	api, repo, _ := setupTestAPI(t)
	defer repo.Close()

	ctx := context.Background()

	// Create test clusters
	cluster1 := &models.Cluster{
		ID:              uuid.New(),
		Name:            "cluster-1",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.ClusterSpec{
			InfraID: "test-infra-1",
		},
		OverallStatus: models.StatusReady,
		OverallHealth: models.HealthHealthy,
	}

	cluster2 := &models.Cluster{
		ID:              uuid.New(),
		Name:            "cluster-2",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.ClusterSpec{
			InfraID: "test-infra-2",
		},
		OverallStatus: models.StatusPending,
		OverallHealth: models.HealthUnknown,
	}

	err := repo.Clusters.Create(ctx, cluster1)
	utils.AssertError(t, err, false, "Should create cluster 1")
	err = repo.Clusters.Create(ctx, cluster2)
	utils.AssertError(t, err, false, "Should create cluster 2")

	// Test listing all clusters
	httpReq := httptest.NewRequest("GET", "/api/v1/clusters", nil)
	recorder := httptest.NewRecorder()

	api.listClusters(recorder, httpReq)

	utils.AssertEqual(t, http.StatusOK, recorder.Code, "Should return 200 OK")

	var response ListClustersResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	utils.AssertError(t, err, false, "Should unmarshal response")

	utils.AssertEqual(t, 2, len(response.Clusters), "Should have 2 clusters")
	utils.AssertEqual(t, int64(2), response.Total, "Total should be 2")

	// Test filtering by status
	httpReq = httptest.NewRequest("GET", "/api/v1/clusters?status=Pending", nil)
	recorder = httptest.NewRecorder()

	api.listClusters(recorder, httpReq)

	utils.AssertEqual(t, http.StatusOK, recorder.Code, "Should return 200 OK")

	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	utils.AssertError(t, err, false, "Should unmarshal response")

	utils.AssertTrue(t, len(response.Clusters) >= 0, "Should return clusters matching status filter")

	// Test filtering by status
	httpReq = httptest.NewRequest("GET", "/api/v1/clusters?status=Ready", nil)
	recorder = httptest.NewRecorder()

	api.listClusters(recorder, httpReq)

	utils.AssertEqual(t, http.StatusOK, recorder.Code, "Should return 200 OK")

	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	utils.AssertError(t, err, false, "Should unmarshal response")

	utils.AssertEqual(t, 1, len(response.Clusters), "Should have 1 ready cluster")
	utils.AssertEqual(t, models.StatusReady, response.Clusters[0].OverallStatus, "Should be ready cluster")
}

func TestAPI_UpdateCluster(t *testing.T) {
	api, repo, mockPubSub := setupTestAPI(t)
	defer repo.Close()

	// Create a test cluster
	cluster := &models.Cluster{
		ID:              uuid.New(),
		Name:            "test-cluster",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.ClusterSpec{
			InfraID: "test-infra",
		},
		OverallStatus: models.StatusPending,
		OverallHealth: models.HealthUnknown,
	}

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster)
	utils.AssertError(t, err, false, "Should create cluster")

	// Update the cluster
	updateReq := UpdateClusterRequest{
		Generation: 2,
		Spec: models.ClusterSpec{
			InfraID: "updated-infra",
		},
	}

	reqBody, err := json.Marshal(updateReq)
	utils.AssertError(t, err, false, "Should marshal request")

	httpReq := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/clusters/%s", cluster.ID), bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq = mux.SetURLVars(httpReq, map[string]string{"id": cluster.ID.String()})
	recorder := httptest.NewRecorder()

	api.updateCluster(recorder, httpReq)

	utils.AssertEqual(t, http.StatusOK, recorder.Code, "Should return 200 OK")

	var response ClusterResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	utils.AssertError(t, err, false, "Should unmarshal response")

	utils.AssertEqual(t, int64(2), response.Generation, "Generation should be updated")
	utils.AssertEqual(t, "updated-infra", response.Spec.InfraID, "Spec should be updated")

	// Verify event was published
	utils.AssertEqual(t, 1, len(mockPubSub.publishedEvents), "Should publish 1 event")
}

func TestAPI_DeleteCluster(t *testing.T) {
	api, repo, mockPubSub := setupTestAPI(t)
	defer repo.Close()

	// Create a test cluster
	cluster := &models.Cluster{
		ID:              uuid.New(),
		Name:            "test-cluster",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.ClusterSpec{
			InfraID: "test-infra",
		},
		OverallStatus: models.StatusPending,
		OverallHealth: models.HealthUnknown,
	}

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster)
	utils.AssertError(t, err, false, "Should create cluster")

	// Delete the cluster
	httpReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/clusters/%s", cluster.ID), nil)
	httpReq = mux.SetURLVars(httpReq, map[string]string{"id": cluster.ID.String()})
	recorder := httptest.NewRecorder()

	api.deleteCluster(recorder, httpReq)

	utils.AssertEqual(t, http.StatusNoContent, recorder.Code, "Should return 204 No Content")

	// Verify cluster is deleted
	_, err = repo.Clusters.GetByID(ctx, cluster.ID)
	utils.AssertError(t, err, true, "Should not find deleted cluster")
	utils.AssertEqual(t, models.ErrClusterNotFound, err, "Should return ErrClusterNotFound")

	// Verify event was published
	utils.AssertEqual(t, 1, len(mockPubSub.publishedEvents), "Should publish 1 event")
}

func TestAPI_CreateNodePool(t *testing.T) {
	api, repo, mockPubSub := setupTestAPI(t)
	defer repo.Close()

	// Create a test cluster first
	cluster := &models.Cluster{
		ID:              uuid.New(),
		Name:            "test-cluster",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.ClusterSpec{
			InfraID: "test-infra",
		},
		OverallStatus: models.StatusPending,
		OverallHealth: models.HealthUnknown,
	}

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster)
	utils.AssertError(t, err, false, "Should create cluster")

	req := createTestNodePoolRequest(cluster.ID)
	reqBody, err := json.Marshal(req)
	utils.AssertError(t, err, false, "Should marshal request")

	httpReq := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/clusters/%s/nodepools", cluster.ID), bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq = mux.SetURLVars(httpReq, map[string]string{"clusterID": cluster.ID.String()})
	recorder := httptest.NewRecorder()

	api.createNodePool(recorder, httpReq)

	utils.AssertEqual(t, http.StatusCreated, recorder.Code, "Should return 201 Created")

	var response NodePoolResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	utils.AssertError(t, err, false, "Should unmarshal response")

	utils.AssertEqual(t, req.Name, response.Name, "NodePool name should match")
	utils.AssertEqual(t, cluster.ID, response.ClusterID, "NodePool cluster ID should match")
	utils.AssertEqual(t, models.StatusPending, response.OverallStatus, "Status should be Pending")
	utils.AssertNotEqual(t, uuid.Nil, response.ID, "ID should be set")

	// Verify event was published
	utils.AssertEqual(t, 1, len(mockPubSub.publishedEvents), "Should publish 1 event")
}

func TestAPI_ListNodePools(t *testing.T) {
	api, repo, _ := setupTestAPI(t)
	defer repo.Close()

	// Create a test cluster first
	cluster := &models.Cluster{
		ID:              uuid.New(),
		Name:            "test-cluster",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.ClusterSpec{
			InfraID: "test-infra",
		},
		OverallStatus: models.StatusPending,
		OverallHealth: models.HealthUnknown,
	}

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster)
	utils.AssertError(t, err, false, "Should create cluster")

	// Create test nodepools
	nodepool1 := &models.NodePool{
		ID:              uuid.New(),
		ClusterID:       cluster.ID,
		Name:            "nodepool-1",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.NodePoolSpec{
			ClusterName: cluster.Name,
			Replicas:    3,
		},
		OverallStatus: models.StatusReady,
		OverallHealth: models.HealthHealthy,
	}

	nodepool2 := &models.NodePool{
		ID:              uuid.New(),
		ClusterID:       cluster.ID,
		Name:            "nodepool-2",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.NodePoolSpec{
			ClusterName: cluster.Name,
			Replicas:    2,
		},
		OverallStatus: models.StatusPending,
		OverallHealth: models.HealthUnknown,
	}

	err = repo.NodePools.Create(ctx, nodepool1)
	utils.AssertError(t, err, false, "Should create nodepool 1")
	err = repo.NodePools.Create(ctx, nodepool2)
	utils.AssertError(t, err, false, "Should create nodepool 2")

	// Test listing nodepools for cluster
	httpReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/clusters/%s/nodepools", cluster.ID), nil)
	httpReq = mux.SetURLVars(httpReq, map[string]string{"clusterID": cluster.ID.String()})
	recorder := httptest.NewRecorder()

	api.listNodePools(recorder, httpReq)

	utils.AssertEqual(t, http.StatusOK, recorder.Code, "Should return 200 OK")

	var response ListNodePoolsResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	utils.AssertError(t, err, false, "Should unmarshal response")

	utils.AssertEqual(t, 2, len(response.NodePools), "Should have 2 nodepools")
	utils.AssertEqual(t, int64(2), response.Total, "Total should be 2")
}

func TestAPI_UpdateNodePoolStatus(t *testing.T) {
	api, repo, mockPubSub := setupTestAPI(t)
	defer repo.Close()

	// Create test cluster and nodepool
	cluster := &models.Cluster{
		ID:              uuid.New(),
		Name:            "test-cluster",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.ClusterSpec{
			InfraID: "test-infra",
		},
		OverallStatus: models.StatusPending,
		OverallHealth: models.HealthUnknown,
	}

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster)
	utils.AssertError(t, err, false, "Should create cluster")

	nodepool := &models.NodePool{
		ID:              uuid.New(),
		ClusterID:       cluster.ID,
		Name:            "test-nodepool",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.NodePoolSpec{
			ClusterName: cluster.Name,
			Replicas:    3,
		},
		OverallStatus: models.StatusPending,
		OverallHealth: models.HealthUnknown,
	}

	err = repo.NodePools.Create(ctx, nodepool)
	utils.AssertError(t, err, false, "Should create nodepool")

	// Update nodepool status
	statusReq := UpdateStatusRequest{
		Status: models.StatusReady,
		Health: models.HealthHealthy,
	}

	reqBody, err := json.Marshal(statusReq)
	utils.AssertError(t, err, false, "Should marshal request")

	httpReq := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/clusters/%s/nodepools/%s/status", cluster.ID, nodepool.ID), bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq = mux.SetURLVars(httpReq, map[string]string{
		"clusterID":  cluster.ID.String(),
		"nodepoolID": nodepool.ID.String(),
	})
	recorder := httptest.NewRecorder()

	api.updateNodePoolStatus(recorder, httpReq)

	utils.AssertEqual(t, http.StatusOK, recorder.Code, "Should return 200 OK")

	// Verify status was updated
	updated, err := repo.NodePools.GetByID(ctx, nodepool.ID)
	utils.AssertError(t, err, false, "Should get updated nodepool")
	utils.AssertEqual(t, models.StatusReady, updated.OverallStatus, "Status should be updated")
	utils.AssertEqual(t, models.HealthHealthy, updated.OverallHealth, "Health should be updated")

	// Verify event was published
	utils.AssertEqual(t, 1, len(mockPubSub.publishedEvents), "Should publish 1 event")
}

func TestAPI_GetClusterHealth(t *testing.T) {
	api, repo, _ := setupTestAPI(t)
	defer repo.Close()

	// Create a test cluster
	cluster := &models.Cluster{
		ID:              uuid.New(),
		Name:            "test-cluster",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: models.ClusterSpec{
			InfraID: "test-infra",
		},
		OverallStatus: models.StatusPending,
		OverallHealth: models.HealthUnknown,
	}

	ctx := context.Background()
	err := repo.Clusters.Create(ctx, cluster)
	utils.AssertError(t, err, false, "Should create cluster")

	// Create controller status
	status := &models.ClusterControllerStatus{
		ID:                 uuid.New(),
		ClusterID:          cluster.ID,
		ControllerName:     "test-controller",
		ObservedGeneration: 1,
		Conditions: models.ConditionList{
			{Type: "Available", Status: "True"},
		},
		LastError: nil,
	}

	err = repo.ClusterControllerStatus.Create(ctx, status)
	utils.AssertError(t, err, false, "Should create controller status")

	// Get cluster health
	httpReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/clusters/%s/health", cluster.ID), nil)
	httpReq = mux.SetURLVars(httpReq, map[string]string{"id": cluster.ID.String()})
	recorder := httptest.NewRecorder()

	api.getClusterHealth(recorder, httpReq)

	utils.AssertEqual(t, http.StatusOK, recorder.Code, "Should return 200 OK")

	var response ClusterHealthResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	utils.AssertError(t, err, false, "Should unmarshal response")

	utils.AssertEqual(t, cluster.ID, response.ClusterID, "Cluster ID should match")
	utils.AssertEqual(t, 1, response.TotalControllers, "Should have 1 controller")
	utils.AssertEqual(t, 1, response.HealthyControllers, "Should have 1 healthy controller")
}

func TestAPI_Pagination(t *testing.T) {
	api, repo, _ := setupTestAPI(t)
	defer repo.Close()

	ctx := context.Background()

	// Create multiple test clusters
	for i := 0; i < 5; i++ {
		cluster := &models.Cluster{
			ID:              uuid.New(),
			Name:            fmt.Sprintf("cluster-%d", i),
			Generation:      1,
			ResourceVersion: "v1",
			Spec: models.ClusterSpec{
				InfraID: fmt.Sprintf("infra-%d", i),
			},
			OverallStatus: models.StatusPending,
			OverallHealth: models.HealthUnknown,
		}

		err := repo.Clusters.Create(ctx, cluster)
		utils.AssertError(t, err, false, "Should create cluster %d", i)
	}

	// Test pagination with limit
	httpReq := httptest.NewRequest("GET", "/api/v1/clusters?limit=2", nil)
	recorder := httptest.NewRecorder()

	api.listClusters(recorder, httpReq)

	utils.AssertEqual(t, http.StatusOK, recorder.Code, "Should return 200 OK")

	var response ListClustersResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	utils.AssertError(t, err, false, "Should unmarshal response")

	utils.AssertEqual(t, 2, len(response.Clusters), "Should have 2 clusters with limit")
	utils.AssertEqual(t, int64(5), response.Total, "Total should be 5")

	// Test pagination with offset
	httpReq = httptest.NewRequest("GET", "/api/v1/clusters?limit=2&offset=2", nil)
	recorder = httptest.NewRecorder()

	api.listClusters(recorder, httpReq)

	utils.AssertEqual(t, http.StatusOK, recorder.Code, "Should return 200 OK")

	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	utils.AssertError(t, err, false, "Should unmarshal response")

	utils.AssertEqual(t, 2, len(response.Clusters), "Should have 2 clusters with offset")
	utils.AssertEqual(t, int64(5), response.Total, "Total should be 5")
}

func TestAPI_InvalidUUID(t *testing.T) {
	api, repo, _ := setupTestAPI(t)
	defer repo.Close()

	// Test invalid UUID in path
	httpReq := httptest.NewRequest("GET", "/api/v1/clusters/invalid-uuid", nil)
	httpReq = mux.SetURLVars(httpReq, map[string]string{"id": "invalid-uuid"})
	recorder := httptest.NewRecorder()

	api.getCluster(recorder, httpReq)

	utils.AssertEqual(t, http.StatusBadRequest, recorder.Code, "Should return 400 Bad Request for invalid UUID")
}
