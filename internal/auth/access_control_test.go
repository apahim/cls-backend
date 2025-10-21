package auth

import (
	"testing"

	"github.com/apahim/cls-backend/internal/models"
	"github.com/google/uuid"
)

func TestGetAccessLevel(t *testing.T) {
	tests := []struct {
		name     string
		userCtx  *UserContext
		expected AccessLevel
	}{
		{
			name: "controller should have system access",
			userCtx: &UserContext{
				Email:        "controller@system.local",
				IsController: true,
			},
			expected: SystemAccess,
		},
		{
			name: "regular user should have user access",
			userCtx: &UserContext{
				Email:        "user@example.com",
				IsController: false,
			},
			expected: UserAccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAccessLevel(tt.userCtx)
			if result != tt.expected {
				t.Errorf("GetAccessLevel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCanAccessCluster(t *testing.T) {
	cluster := &models.Cluster{
		ID:        uuid.New(),
		CreatedBy: "user@example.com",
		Name:      "test-cluster",
	}

	tests := []struct {
		name     string
		userCtx  *UserContext
		cluster  *models.Cluster
		expected bool
	}{
		{
			name: "controller can access any cluster",
			userCtx: &UserContext{
				Email:        "controller@system.local",
				IsController: true,
			},
			cluster:  cluster,
			expected: true,
		},
		{
			name: "user can access own cluster",
			userCtx: &UserContext{
				Email:        "user@example.com",
				IsController: false,
			},
			cluster:  cluster,
			expected: true,
		},
		{
			name: "user cannot access other user's cluster",
			userCtx: &UserContext{
				Email:        "other@example.com",
				IsController: false,
			},
			cluster:  cluster,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanAccessCluster(tt.userCtx, tt.cluster)
			if result != tt.expected {
				t.Errorf("CanAccessCluster() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCanReportStatus(t *testing.T) {
	tests := []struct {
		name     string
		userCtx  *UserContext
		expected bool
	}{
		{
			name: "controller can report status",
			userCtx: &UserContext{
				Email:        "controller@system.local",
				IsController: true,
			},
			expected: true,
		},
		{
			name: "regular user cannot report status",
			userCtx: &UserContext{
				Email:        "user@example.com",
				IsController: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanReportStatus(tt.userCtx)
			if result != tt.expected {
				t.Errorf("CanReportStatus() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsSystemUser(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{
			name:     "controller email should be system user",
			email:    "controller@system.local",
			expected: true,
		},
		{
			name:     "regular email should not be system user",
			email:    "user@example.com",
			expected: false,
		},
		{
			name:     "empty email should not be system user",
			email:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSystemUser(tt.email)
			if result != tt.expected {
				t.Errorf("IsSystemUser() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNewUserContext(t *testing.T) {
	tests := []struct {
		name             string
		email            string
		expectedEmail    string
		expectedIsController bool
	}{
		{
			name:             "controller email creates controller context",
			email:            "controller@system.local",
			expectedEmail:    "controller@system.local",
			expectedIsController: true,
		},
		{
			name:             "regular email creates user context",
			email:            "user@example.com",
			expectedEmail:    "user@example.com",
			expectedIsController: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewUserContext(tt.email)
			if result.Email != tt.expectedEmail {
				t.Errorf("NewUserContext().Email = %v, want %v", result.Email, tt.expectedEmail)
			}
			if result.IsController != tt.expectedIsController {
				t.Errorf("NewUserContext().IsController = %v, want %v", result.IsController, tt.expectedIsController)
			}
		})
	}
}