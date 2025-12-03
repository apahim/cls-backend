package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/apahim/cls-backend/internal/auth"
	"github.com/apahim/cls-backend/internal/database"
	"github.com/apahim/cls-backend/internal/middleware"
	"github.com/apahim/cls-backend/internal/models"
	"github.com/apahim/cls-backend/internal/services"
	"github.com/apahim/cls-backend/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ClusterHandler handles cluster operations
type ClusterHandler struct {
	clusterService   *services.ClusterService
	statusRepository *database.StatusRepository
	logger           *zap.Logger
}

// NewClusterHandler creates a new cluster handler
func NewClusterHandler(clusterService *services.ClusterService, statusRepository *database.StatusRepository) *ClusterHandler {
	return &ClusterHandler{
		clusterService:   clusterService,
		statusRepository: statusRepository,
		logger:           zap.L().Named("cluster_handler"),
	}
}

// RegisterRoutes registers cluster routes
func (h *ClusterHandler) RegisterRoutes(router *gin.RouterGroup) {
	clusters := router.Group("/clusters")
	{
		clusters.GET("", h.ListClusters)
		clusters.POST("", h.CreateCluster)
		clusters.GET("/:cluster_id", h.GetCluster)
		clusters.PUT("/:cluster_id", h.UpdateCluster)
		clusters.DELETE("/:cluster_id", h.DeleteCluster)
		clusters.GET("/:cluster_id/status", h.GetClusterStatus)
		clusters.PUT("/:cluster_id/status", h.UpdateClusterStatus)
	}
}

// ListClusters lists all clusters with pagination
func (h *ClusterHandler) ListClusters(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Parse query parameters
	limit := 50 // default
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Check for created_by filter (for future authorization)
	createdBy := c.Query("created_by")

	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		h.logger.Error("No user context found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	h.logger.Info("Listing clusters",
		zap.String("user_email", userCtx.Email),
		zap.Bool("is_controller", userCtx.IsController),
		zap.Int("limit", limit),
		zap.Int("offset", offset),
		zap.String("created_by_filter", createdBy),
	)

	// Use access-level aware listing
	var clusters []*models.Cluster
	var total int64
	var err error

	if userCtx.IsController {
		// Controllers get system-wide access
		clusters, total, err = h.clusterService.ListAllClusters(ctx, limit, offset)
	} else {
		// Users get scoped access
		clusters, total, err = h.clusterService.ListClusters(ctx, userCtx.Email, limit, offset)
	}

	if err != nil {
		h.logger.Error("Failed to list clusters", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list clusters"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"clusters": clusters,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// CreateCluster creates a new cluster
func (h *ClusterHandler) CreateCluster(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Parse request body
	var req models.ClusterCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	// Validate required fields
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cluster name is required"})
		return
	}

	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		h.logger.Error("No user context found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	// Check if user can create clusters
	if !auth.CanCreateCluster(userCtx) {
		h.logger.Warn("User not authorized to create clusters",
			zap.String("user_email", userCtx.Email),
			zap.Bool("is_controller", userCtx.IsController),
		)
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorized to create clusters"})
		return
	}

	h.logger.Info("Creating cluster",
		zap.String("cluster_name", req.Name),
		zap.String("user_email", userCtx.Email),
		zap.Bool("is_controller", userCtx.IsController),
	)

	// Create cluster
	cluster, err := h.clusterService.CreateCluster(ctx, &req, userCtx.Email)
	if err != nil {
		h.logger.Error("Failed to create cluster",
			zap.String("cluster_name", req.Name),
			zap.String("user_email", userCtx.Email),
			zap.Error(err),
		)

		// Convert database errors to appropriate API errors
		apiErr := utils.ConvertDBError(err)
		if apiErr.Code != "" {
			c.JSON(apiErr.HTTPStatus(), gin.H{"error": apiErr})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create cluster"})
		}
		return
	}

	c.JSON(http.StatusCreated, cluster)
}

// GetCluster gets a specific cluster
func (h *ClusterHandler) GetCluster(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Extract cluster ID
	clusterIDStr := c.Param("cluster_id")
	if clusterIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cluster ID is required"})
		return
	}

	clusterID, err := uuid.Parse(clusterIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster ID format"})
		return
	}

	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		h.logger.Error("No user context found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	h.logger.Info("Getting cluster",
		zap.String("cluster_id", clusterIDStr),
		zap.String("user_email", userCtx.Email),
		zap.Bool("is_controller", userCtx.IsController),
	)

	// Get cluster with access control
	cluster, err := h.clusterService.GetClusterWithAccessControl(ctx, clusterID, userCtx)
	if err != nil {
		h.logger.Error("Failed to get cluster",
			zap.String("cluster_id", clusterIDStr),
			zap.Error(err),
		)

		if err.Error() == "cluster not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get cluster"})
		}
		return
	}

	c.JSON(http.StatusOK, cluster)
}

// UpdateCluster updates a cluster
func (h *ClusterHandler) UpdateCluster(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Extract cluster ID
	clusterIDStr := c.Param("cluster_id")
	if clusterIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cluster ID is required"})
		return
	}

	clusterID, err := uuid.Parse(clusterIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster ID format"})
		return
	}

	// Parse request body
	var req models.ClusterUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		h.logger.Error("No user context found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	h.logger.Info("Updating cluster",
		zap.String("cluster_id", clusterIDStr),
		zap.String("user_email", userCtx.Email),
		zap.Bool("is_controller", userCtx.IsController),
	)

	// Update cluster with access control
	cluster, err := h.clusterService.UpdateClusterWithAccessControl(ctx, clusterID, &req, userCtx)
	if err != nil {
		h.logger.Error("Failed to update cluster",
			zap.String("cluster_id", clusterIDStr),
			zap.Error(err),
		)

		if err.Error() == "cluster not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update cluster"})
		}
		return
	}

	c.JSON(http.StatusOK, cluster)
}

// DeleteCluster deletes a cluster
func (h *ClusterHandler) DeleteCluster(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Extract cluster ID
	clusterIDStr := c.Param("cluster_id")
	if clusterIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cluster ID is required"})
		return
	}

	clusterID, err := uuid.Parse(clusterIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster ID format"})
		return
	}

	force := c.Query("force") == "true"

	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		h.logger.Error("No user context found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	h.logger.Info("Deleting cluster",
		zap.String("cluster_id", clusterIDStr),
		zap.String("user_email", userCtx.Email),
		zap.Bool("is_controller", userCtx.IsController),
		zap.Bool("force", force),
	)

	// Delete cluster with access control
	err = h.clusterService.DeleteClusterWithAccessControl(ctx, clusterID, force, userCtx)
	if err != nil {
		h.logger.Error("Failed to delete cluster",
			zap.String("cluster_id", clusterIDStr),
			zap.Error(err),
		)

		if err.Error() == "cluster not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete cluster"})
		}
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":    "cluster deletion initiated",
		"cluster_id": clusterIDStr,
	})
}

// GetClusterStatus retrieves cluster status information
func (h *ClusterHandler) GetClusterStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Extract cluster ID
	clusterIDStr := c.Param("cluster_id")
	if clusterIDStr == "" {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Cluster ID is required",
			"",
		))
		return
	}

	clusterID, err := uuid.Parse(clusterIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid cluster ID format",
			err.Error(),
		))
		return
	}

	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		h.logger.Error("No user context found")
		c.JSON(http.StatusUnauthorized, utils.NewAPIError(
			utils.ErrCodeUnauthorized,
			"Authentication required",
			"",
		))
		return
	}

	h.logger.Info("Getting cluster status",
		zap.String("cluster_id", clusterIDStr),
		zap.String("user_email", userCtx.Email),
		zap.Bool("is_controller", userCtx.IsController),
	)

	// Get cluster first to verify it exists and user has access
	cluster, err := h.clusterService.GetClusterWithAccessControl(ctx, clusterID, userCtx)
	if err != nil {
		h.logger.Error("Failed to get cluster for status",
			zap.String("cluster_id", clusterIDStr),
			zap.Error(err),
		)

		if err.Error() == "cluster not found" {
			c.JSON(http.StatusNotFound, utils.NewAPIError(
				utils.ErrCodeNotFound,
				"Cluster not found",
				"",
			))
		} else {
			c.JSON(http.StatusInternalServerError, utils.NewAPIError(
				utils.ErrCodeInternal,
				"Failed to get cluster",
				err.Error(),
			))
		}
		return
	}

	// Get individual controller status reports
	controllerStatuses, err := h.statusRepository.ListClusterControllerStatus(ctx, clusterID)
	if err != nil {
		h.logger.Error("Failed to get controller status reports",
			zap.String("cluster_id", clusterIDStr),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to get controller status reports",
			err.Error(),
		))
		return
	}

	h.logger.Info("Retrieved controller status reports",
		zap.String("cluster_id", clusterIDStr),
		zap.Int("controller_count", len(controllerStatuses)),
	)

	response := gin.H{
		"cluster_id":        clusterIDStr,
		"status":            cluster.Status,     // K8s-like aggregated status
		"controller_status": controllerStatuses, // Individual controller reports
	}

	c.JSON(http.StatusOK, response)
}

// UpdateClusterStatus handles controller status updates
func (h *ClusterHandler) UpdateClusterStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Extract cluster ID
	clusterIDStr := c.Param("cluster_id")
	if clusterIDStr == "" {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Cluster ID is required",
			"",
		))
		return
	}

	clusterID, err := uuid.Parse(clusterIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid cluster ID format",
			err.Error(),
		))
		return
	}

	var statusUpdate models.ClusterControllerStatus
	if err := c.ShouldBindJSON(&statusUpdate); err != nil {
		h.logger.Error("Invalid status update request", zap.Error(err))
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid status update format",
			err.Error(),
		))
		return
	}

	// Get user context from middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		h.logger.Error("No user context found")
		c.JSON(http.StatusUnauthorized, utils.NewAPIError(
			utils.ErrCodeUnauthorized,
			"Authentication required",
			"",
		))
		return
	}

	// Only controllers can report status
	if !auth.CanReportStatus(userCtx) {
		h.logger.Warn("User not authorized to report status",
			zap.String("user_email", userCtx.Email),
			zap.Bool("is_controller", userCtx.IsController),
		)
		c.JSON(http.StatusForbidden, utils.NewAPIError(
			utils.ErrCodeForbidden,
			"Only system controllers can report status",
			"",
		))
		return
	}

	h.logger.Info("Updating cluster status",
		zap.String("cluster_id", clusterIDStr),
		zap.String("controller_name", statusUpdate.ControllerName),
		zap.String("user_email", userCtx.Email),
		zap.Bool("is_controller", userCtx.IsController),
	)

	// Controllers can access any cluster for status reporting
	_, err = h.clusterService.GetClusterWithAccessControl(ctx, clusterID, userCtx)
	if err != nil {
		h.logger.Error("Failed to verify cluster for status update",
			zap.String("cluster_id", clusterIDStr),
			zap.Error(err),
		)

		if err.Error() == "cluster not found" {
			c.JSON(http.StatusNotFound, utils.NewAPIError(
				utils.ErrCodeNotFound,
				"Cluster not found",
				"",
			))
		} else {
			c.JSON(http.StatusInternalServerError, utils.NewAPIError(
				utils.ErrCodeInternal,
				"Failed to verify cluster",
				err.Error(),
			))
		}
		return
	}

	// Set cluster ID in status update
	statusUpdate.ClusterID = clusterID
	statusUpdate.LastUpdated = time.Now()

	// Ensure metadata is not nil to satisfy database NOT NULL constraint
	if statusUpdate.Metadata == nil {
		statusUpdate.Metadata = make(models.JSONB)
	}

	// Store the status update in the database
	err = h.statusRepository.UpsertClusterControllerStatus(ctx, &statusUpdate)
	if err != nil {
		h.logger.Error("Failed to store cluster status update",
			zap.String("cluster_id", clusterIDStr),
			zap.String("controller_name", statusUpdate.ControllerName),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to store status update",
			err.Error(),
		))
		return
	}

	h.logger.Info("Successfully stored cluster status update",
		zap.String("cluster_id", clusterIDStr),
		zap.String("controller_name", statusUpdate.ControllerName),
		zap.Int64("observed_generation", statusUpdate.ObservedGeneration),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":         "Status update stored successfully",
		"cluster_id":      clusterIDStr,
		"controller_name": statusUpdate.ControllerName,
	})
}
