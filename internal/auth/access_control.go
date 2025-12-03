package auth

import (
	"github.com/apahim/cls-backend/internal/models"
)

// AccessLevel represents the level of access a user has
type AccessLevel int

const (
	// UserAccess represents normal user access (only own resources)
	UserAccess AccessLevel = iota
	// SystemAccess represents system-wide access (all resources)
	SystemAccess
)

// UserContext represents the authenticated user context with privileges
type UserContext struct {
	Email        string
	IsController bool
}

// GetAccessLevel returns the access level for a user context
func GetAccessLevel(userCtx *UserContext) AccessLevel {
	if userCtx.IsController {
		return SystemAccess
	}
	return UserAccess
}

// CanAccessCluster determines if a user can access a specific cluster
func CanAccessCluster(userCtx *UserContext, cluster *models.Cluster) bool {
	if userCtx.IsController {
		return true // System-wide access for controllers
	}
	return cluster.CreatedBy == userCtx.Email // User-scoped access
}

// CanReportStatus determines if a user can report status for clusters
func CanReportStatus(userCtx *UserContext) bool {
	return userCtx.IsController // Only controllers can report status
}

// CanCreateCluster determines if a user can create clusters
func CanCreateCluster(userCtx *UserContext) bool {
	// Both users and controllers can create clusters
	// Controllers may create clusters on behalf of the system
	return true
}

// CanUpdateCluster determines if a user can update a specific cluster
func CanUpdateCluster(userCtx *UserContext, cluster *models.Cluster) bool {
	if userCtx.IsController {
		return true // Controllers can update any cluster
	}
	return cluster.CreatedBy == userCtx.Email // Users can only update their own clusters
}

// CanDeleteCluster determines if a user can delete a specific cluster
func CanDeleteCluster(userCtx *UserContext, cluster *models.Cluster) bool {
	if userCtx.IsController {
		return true // Controllers can delete any cluster
	}
	return cluster.CreatedBy == userCtx.Email // Users can only delete their own clusters
}

// IsSystemUser checks if the user is a system user (controller)
func IsSystemUser(email string) bool {
	return email == "controller@system.local"
}

// NewUserContext creates a new user context from an email
func NewUserContext(email string) *UserContext {
	return &UserContext{
		Email:        email,
		IsController: IsSystemUser(email),
	}
}
