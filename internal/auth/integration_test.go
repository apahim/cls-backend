package auth

import (
	"testing"

	"github.com/apahim/cls-backend/internal/models"
	"github.com/google/uuid"
)

// TestAccessControlFlow demonstrates the complete access control flow
func TestAccessControlFlow(t *testing.T) {
	// Create test cluster
	cluster := &models.Cluster{
		ID:        uuid.New(),
		Name:      "test-cluster",
		CreatedBy: "alice@example.com",
	}

	// Test scenarios
	scenarios := []struct {
		name            string
		email           string
		expectedAccess  AccessLevel
		canAccess       bool
		canUpdate       bool
		canDelete       bool
		canReportStatus bool
	}{
		{
			name:            "Controller System Access",
			email:           "controller@system.local",
			expectedAccess:  SystemAccess,
			canAccess:       true,
			canUpdate:       true,
			canDelete:       true,
			canReportStatus: true,
		},
		{
			name:            "Cluster Owner Access",
			email:           "alice@example.com",
			expectedAccess:  UserAccess,
			canAccess:       true,
			canUpdate:       true,
			canDelete:       true,
			canReportStatus: false,
		},
		{
			name:            "Different User Access",
			email:           "bob@example.com",
			expectedAccess:  UserAccess,
			canAccess:       false,
			canUpdate:       false,
			canDelete:       false,
			canReportStatus: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Create user context
			userCtx := NewUserContext(scenario.email)

			// Test access level
			accessLevel := GetAccessLevel(userCtx)
			if accessLevel != scenario.expectedAccess {
				t.Errorf("Expected access level %v, got %v", scenario.expectedAccess, accessLevel)
			}

			// Test cluster access
			canAccess := CanAccessCluster(userCtx, cluster)
			if canAccess != scenario.canAccess {
				t.Errorf("Expected CanAccessCluster %v, got %v", scenario.canAccess, canAccess)
			}

			// Test update permission
			canUpdate := CanUpdateCluster(userCtx, cluster)
			if canUpdate != scenario.canUpdate {
				t.Errorf("Expected CanUpdateCluster %v, got %v", scenario.canUpdate, canUpdate)
			}

			// Test delete permission
			canDelete := CanDeleteCluster(userCtx, cluster)
			if canDelete != scenario.canDelete {
				t.Errorf("Expected CanDeleteCluster %v, got %v", scenario.canDelete, canDelete)
			}

			// Test status reporting permission
			canReportStatus := CanReportStatus(userCtx)
			if canReportStatus != scenario.canReportStatus {
				t.Errorf("Expected CanReportStatus %v, got %v", scenario.canReportStatus, canReportStatus)
			}

			t.Logf("✅ %s: Access=%v, CanAccess=%v, CanUpdate=%v, CanDelete=%v, CanReportStatus=%v",
				scenario.name, accessLevel, canAccess, canUpdate, canDelete, canReportStatus)
		})
	}
}

// TestControllerPrivileges specifically tests controller privileges
func TestControllerPrivileges(t *testing.T) {
	controllerCtx := NewUserContext("controller@system.local")

	// Test clusters from different users
	clusters := []*models.Cluster{
		{ID: uuid.New(), Name: "user1-cluster", CreatedBy: "user1@example.com"},
		{ID: uuid.New(), Name: "user2-cluster", CreatedBy: "user2@example.com"},
		{ID: uuid.New(), Name: "user3-cluster", CreatedBy: "user3@example.com"},
	}

	for _, cluster := range clusters {
		if !CanAccessCluster(controllerCtx, cluster) {
			t.Errorf("Controller should be able to access cluster %s created by %s", cluster.Name, cluster.CreatedBy)
		}

		if !CanUpdateCluster(controllerCtx, cluster) {
			t.Errorf("Controller should be able to update cluster %s created by %s", cluster.Name, cluster.CreatedBy)
		}

		if !CanDeleteCluster(controllerCtx, cluster) {
			t.Errorf("Controller should be able to delete cluster %s created by %s", cluster.Name, cluster.CreatedBy)
		}
	}

	if !CanReportStatus(controllerCtx) {
		t.Error("Controller should be able to report status")
	}

	if !CanCreateCluster(controllerCtx) {
		t.Error("Controller should be able to create clusters")
	}

	t.Log("✅ Controller has system-wide privileges confirmed")
}

// TestUserIsolation tests that users can only access their own resources
func TestUserIsolation(t *testing.T) {
	user1Ctx := NewUserContext("user1@example.com")
	user2Ctx := NewUserContext("user2@example.com")

	user1Cluster := &models.Cluster{
		ID:        uuid.New(),
		Name:      "user1-cluster",
		CreatedBy: "user1@example.com",
	}

	user2Cluster := &models.Cluster{
		ID:        uuid.New(),
		Name:      "user2-cluster",
		CreatedBy: "user2@example.com",
	}

	// User1 should access own cluster
	if !CanAccessCluster(user1Ctx, user1Cluster) {
		t.Error("User1 should be able to access own cluster")
	}

	// User1 should NOT access user2's cluster
	if CanAccessCluster(user1Ctx, user2Cluster) {
		t.Error("User1 should NOT be able to access user2's cluster")
	}

	// User2 should access own cluster
	if !CanAccessCluster(user2Ctx, user2Cluster) {
		t.Error("User2 should be able to access own cluster")
	}

	// User2 should NOT access user1's cluster
	if CanAccessCluster(user2Ctx, user1Cluster) {
		t.Error("User2 should NOT be able to access user1's cluster")
	}

	// Neither user should be able to report status
	if CanReportStatus(user1Ctx) {
		t.Error("User1 should NOT be able to report status")
	}

	if CanReportStatus(user2Ctx) {
		t.Error("User2 should NOT be able to report status")
	}

	t.Log("✅ User isolation working correctly")
}
