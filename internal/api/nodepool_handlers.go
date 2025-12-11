package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/apahim/cls-backend/internal/database"
	"github.com/apahim/cls-backend/internal/middleware"
	"github.com/apahim/cls-backend/internal/models"
	"github.com/apahim/cls-backend/internal/pubsub"
	"github.com/apahim/cls-backend/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NodePoolHandler handles nodepool-related HTTP requests
type NodePoolHandler struct {
	repository *database.Repository
	pubsub     *pubsub.Service
	logger     *utils.Logger
}

// NewNodePoolHandler creates a new nodepool handler
func NewNodePoolHandler(repository *database.Repository, pubsubService *pubsub.Service) *NodePoolHandler {
	return &NodePoolHandler{
		repository: repository,
		pubsub:     pubsubService,
		logger:     utils.NewLogger("nodepool_handler"),
	}
}

// RegisterRoutes registers nodepool routes with the router
func (h *NodePoolHandler) RegisterRoutes(r *gin.RouterGroup) {
	nodepools := r.Group("/nodepools")
	{
		nodepools.POST("", h.CreateNodePool)
		nodepools.GET("", h.ListNodePools)
		nodepools.GET("/:id", h.GetNodePool)
		nodepools.PUT("/:id", h.UpdateNodePool)
		nodepools.DELETE("/:id", h.DeleteNodePool)
		nodepools.GET("/:id/status", h.GetNodePoolStatus)
		nodepools.PUT("/:id/status", h.UpdateNodePoolStatus)
	}
}

// CreateNodePool creates a new nodepool
func (h *NodePoolHandler) CreateNodePool(c *gin.Context) {
	var req models.NodePool
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid request body",
			err.Error(),
		))
		return
	}

	// Validate required fields
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Validation failed",
			"nodepool name is required",
		))
		return
	}

	if req.ClusterID == uuid.Nil {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Validation failed",
			"cluster ID is required",
		))
		return
	}

	ctx := c.Request.Context()

	// Get user email from context for client isolation
	userEmail := c.GetString("user_email")
	if userEmail == "" {
		h.logger.Error("No user email found in context")
		c.JSON(http.StatusUnauthorized, utils.NewAPIError(
			utils.ErrCodeUnauthorized,
			"Authentication required",
			"",
		))
		return
	}

	// Verify cluster exists
	_, err := h.repository.Clusters.GetByID(ctx, req.ClusterID, userEmail)
	if err != nil {
		if err == models.ErrClusterNotFound {
			c.JSON(http.StatusBadRequest, utils.NewAPIError(
				utils.ErrCodeValidation,
				"Invalid cluster ID",
				"cluster not found",
			))
			return
		}

		h.logger.Error("Failed to verify cluster",
			zap.String("cluster_id", req.ClusterID.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to verify cluster",
			err.Error(),
		))
		return
	}

	// Set initial values
	req.ID = uuid.New()
	req.Generation = 1
	req.ResourceVersion = uuid.New().String()
	req.CreatedBy = userEmail

	// Create nodepool in database
	err = h.repository.NodePools.Create(ctx, &req)
	if err != nil {
		// Check for unique constraint violation (duplicate name in cluster)
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "UNIQUE constraint") {
			h.logger.Warn("NodePool name already exists in cluster",
				zap.String("nodepool_name", req.Name),
				zap.String("cluster_id", req.ClusterID.String()),
			)
			c.JSON(http.StatusConflict, utils.NewAPIError(
				utils.ErrCodeConflict,
				"NodePool already exists",
				fmt.Sprintf("a nodepool with name '%s' already exists in this cluster", req.Name),
			))
			return
		}

		h.logger.Error("Failed to create nodepool",
			zap.String("nodepool_name", req.Name),
			zap.String("cluster_id", req.ClusterID.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to create nodepool",
			err.Error(),
		))
		return
	}

	// Publish nodepool created event
	if h.pubsub != nil && h.pubsub.IsRunning() {
		if err := h.pubsub.GetPublisher().PublishNodePoolCreated(ctx, &req); err != nil {
			h.logger.Warn("Failed to publish nodepool created event",
				zap.String("nodepool_id", req.ID.String()),
				zap.Error(err),
			)
		}
	}

	h.logger.Info("NodePool created successfully",
		zap.String("nodepool_id", req.ID.String()),
		zap.String("nodepool_name", req.Name),
		zap.String("cluster_id", req.ClusterID.String()),
	)

	c.JSON(http.StatusCreated, req)
}

// ListNodePools lists nodepools with optional cluster filtering
func (h *NodePoolHandler) ListNodePools(c *gin.Context) {
	// Parse query parameters
	opts := &models.ListOptions{
		Status: c.Query("status"),
		Health: c.Query("health"),
	}

	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			opts.Limit = l
		}
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			opts.Offset = o
		}
	}

	// Validate options
	if err := opts.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid query parameters",
			err.Error(),
		))
		return
	}

	ctx := c.Request.Context()

	// Get user email from context (required for client isolation)
	userEmail := c.GetString("user_email")
	if userEmail == "" {
		h.logger.Error("No user email found in context")
		c.JSON(http.StatusUnauthorized, utils.NewAPIError(
			utils.ErrCodeUnauthorized,
			"Authentication required",
			"",
		))
		return
	}

	// Parse optional clusterId parameter
	var nodepools []*models.NodePool
	var total int64
	var err error

	if clusterIDStr := c.Query("clusterId"); clusterIDStr != "" {
		// Cluster-specific listing
		clusterID, parseErr := uuid.Parse(clusterIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, utils.NewAPIError(
				utils.ErrCodeValidation,
				"Invalid cluster ID",
				parseErr.Error(),
			))
			return
		}

		nodepools, err = h.repository.NodePools.ListByCluster(ctx, clusterID, userEmail, opts)
		if err != nil {
			h.logger.Error("Failed to list nodepools by cluster",
				zap.String("cluster_id", clusterID.String()),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, utils.NewAPIError(
				utils.ErrCodeInternal,
				"Failed to list nodepools",
				err.Error(),
			))
			return
		}

		total, err = h.repository.NodePools.CountByCluster(ctx, clusterID)
		if err != nil {
			h.logger.Error("Failed to count nodepools by cluster",
				zap.String("cluster_id", clusterID.String()),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, utils.NewAPIError(
				utils.ErrCodeInternal,
				"Failed to count nodepools",
				err.Error(),
			))
			return
		}
	} else {
		// List all nodepools for user (across all clusters)
		nodepools, err = h.repository.NodePools.List(ctx, userEmail, opts)
		if err != nil {
			h.logger.Error("Failed to list all nodepools",
				zap.String("user_email", userEmail),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, utils.NewAPIError(
				utils.ErrCodeInternal,
				"Failed to list nodepools",
				err.Error(),
			))
			return
		}

		total, err = h.repository.NodePools.Count(ctx, userEmail, opts)
		if err != nil {
			h.logger.Error("Failed to count all nodepools",
				zap.String("user_email", userEmail),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, utils.NewAPIError(
				utils.ErrCodeInternal,
				"Failed to count nodepools",
				err.Error(),
			))
			return
		}
	}

	response := map[string]interface{}{
		"nodepools": nodepools,
		"total":     total,
		"limit":     opts.Limit,
		"offset":    opts.Offset,
	}

	c.JSON(http.StatusOK, response)
}

// GetNodePool retrieves a nodepool by ID
func (h *NodePoolHandler) GetNodePool(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid nodepool ID",
			err.Error(),
		))
		return
	}

	ctx := c.Request.Context()

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

	h.logger.Info("Getting nodepool",
		zap.String("nodepool_id", idParam),
		zap.String("user_email", userCtx.Email),
		zap.Bool("is_controller", userCtx.IsController),
	)

	var nodepool *models.NodePool
	if userCtx.IsController {
		// Controllers can access any nodepool
		nodepool, err = h.repository.NodePools.GetByIDInternal(ctx, id)
	} else {
		// Users can only access their own nodepools (via cluster ownership)
		nodepool, err = h.repository.NodePools.GetByID(ctx, id, userCtx.Email)
	}

	if err != nil {
		if err == models.ErrNodePoolNotFound {
			c.JSON(http.StatusNotFound, utils.NewAPIError(
				utils.ErrCodeNotFound,
				"NodePool not found",
				"",
			))
			return
		}

		h.logger.Error("Failed to get nodepool",
			zap.String("nodepool_id", id.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to get nodepool",
			err.Error(),
		))
		return
	}

	c.JSON(http.StatusOK, nodepool)
}

// UpdateNodePool updates an existing nodepool
func (h *NodePoolHandler) UpdateNodePool(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid nodepool ID",
			err.Error(),
		))
		return
	}

	var req models.NodePoolUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid request body",
			err.Error(),
		))
		return
	}

	ctx := c.Request.Context()

	// Get user email from context for client isolation
	userEmail := c.GetString("user_email")
	if userEmail == "" {
		h.logger.Error("No user email found in context")
		c.JSON(http.StatusUnauthorized, utils.NewAPIError(
			utils.ErrCodeUnauthorized,
			"Authentication required",
			"",
		))
		return
	}

	// Get existing nodepool
	existing, err := h.repository.NodePools.GetByID(ctx, id, userEmail)
	if err != nil {
		if err == models.ErrNodePoolNotFound {
			c.JSON(http.StatusNotFound, utils.NewAPIError(
				utils.ErrCodeNotFound,
				"NodePool not found",
				"",
			))
			return
		}

		h.logger.Error("Failed to get nodepool",
			zap.String("nodepool_id", id.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to get nodepool",
			err.Error(),
		))
		return
	}

	// Track changes for event publishing
	hasChanges := existing.Spec != req.Spec

	// Update only mutable fields on existing object
	// This preserves all immutable fields: name, created_by, cluster_id, id, created_at
	existing.Spec = req.Spec
	existing.Generation = existing.Generation + 1
	existing.ResourceVersion = uuid.New().String()
	existing.UpdatedAt = time.Now()

	// Update nodepool in database
	err = h.repository.NodePools.Update(ctx, existing, userEmail)
	if err != nil {
		h.logger.Error("Failed to update nodepool",
			zap.String("nodepool_id", id.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to update nodepool",
			err.Error(),
		))
		return
	}

	// Publish nodepool updated event if there were changes
	if hasChanges && h.pubsub != nil && h.pubsub.IsRunning() {
		if err := h.pubsub.GetPublisher().PublishNodePoolUpdated(ctx, existing); err != nil {
			h.logger.Warn("Failed to publish nodepool updated event",
				zap.String("nodepool_id", existing.ID.String()),
				zap.Error(err),
			)
		}
	}

	h.logger.Info("NodePool updated successfully",
		zap.String("nodepool_id", existing.ID.String()),
		zap.String("nodepool_name", existing.Name),
		zap.Int64("generation", existing.Generation),
	)

	c.JSON(http.StatusOK, existing)
}

// DeleteNodePool deletes a nodepool
func (h *NodePoolHandler) DeleteNodePool(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid nodepool ID",
			err.Error(),
		))
		return
	}

	ctx := c.Request.Context()

	// Get user email from context for client isolation
	userEmail := c.GetString("user_email")
	if userEmail == "" {
		h.logger.Error("No user email found in context")
		c.JSON(http.StatusUnauthorized, utils.NewAPIError(
			utils.ErrCodeUnauthorized,
			"Authentication required",
			"",
		))
		return
	}

	// Get existing nodepool for event publishing
	nodepool, err := h.repository.NodePools.GetByID(ctx, id, userEmail)
	if err != nil {
		if err == models.ErrNodePoolNotFound {
			c.JSON(http.StatusNotFound, utils.NewAPIError(
				utils.ErrCodeNotFound,
				"NodePool not found",
				"",
			))
			return
		}

		h.logger.Error("Failed to get nodepool",
			zap.String("nodepool_id", id.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to get nodepool",
			err.Error(),
		))
		return
	}

	// Delete nodepool in database (soft delete)
	err = h.repository.NodePools.Delete(ctx, id, userEmail)
	if err != nil {
		h.logger.Error("Failed to delete nodepool",
			zap.String("nodepool_id", id.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to delete nodepool",
			err.Error(),
		))
		return
	}

	// Delete controller status
	if err := h.repository.Status.DeleteAllNodePoolControllerStatus(ctx, id); err != nil {
		h.logger.Warn("Failed to delete nodepool controller status",
			zap.String("nodepool_id", id.String()),
			zap.Error(err),
		)
	}

	// Publish nodepool deleted event
	if h.pubsub != nil && h.pubsub.IsRunning() {
		if err := h.pubsub.GetPublisher().PublishNodePoolDeleted(ctx, nodepool); err != nil {
			h.logger.Warn("Failed to publish nodepool deleted event",
				zap.String("nodepool_id", nodepool.ID.String()),
				zap.Error(err),
			)
		}
	}

	h.logger.Info("NodePool deleted successfully",
		zap.String("nodepool_id", id.String()),
		zap.String("nodepool_name", nodepool.Name),
	)

	c.JSON(http.StatusAccepted, gin.H{
		"message":     "nodepool deletion initiated",
		"nodepool_id": id.String(),
	})
}

// GetNodePoolStatus retrieves nodepool status information
// Returns both aggregated K8s-like status and individual controller status reports
func (h *NodePoolHandler) GetNodePoolStatus(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid nodepool ID",
			err.Error(),
		))
		return
	}

	ctx := c.Request.Context()

	// Get user context for access control
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

	h.logger.Info("Getting nodepool status",
		zap.String("nodepool_id", id.String()),
		zap.String("user_email", userCtx.Email),
		zap.Bool("is_controller", userCtx.IsController),
	)

	// Get nodepool to retrieve aggregated status
	nodepool, err := h.repository.NodePools.GetByID(ctx, id, userCtx.Email)
	if err != nil {
		h.logger.Error("Failed to get nodepool",
			zap.String("nodepool_id", id.String()),
			zap.Error(err),
		)

		if err.Error() == "nodepool not found" {
			c.JSON(http.StatusNotFound, utils.NewAPIError(
				utils.ErrCodeNotFound,
				"NodePool not found",
				"",
			))
		} else {
			c.JSON(http.StatusInternalServerError, utils.NewAPIError(
				utils.ErrCodeInternal,
				"Failed to get nodepool",
				err.Error(),
			))
		}
		return
	}

	// Get individual controller status reports
	controllerStatuses, err := h.repository.Status.ListNodePoolControllerStatus(ctx, id)
	if err != nil {
		h.logger.Error("Failed to get nodepool controller status",
			zap.String("nodepool_id", id.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to get nodepool controller status",
			err.Error(),
		))
		return
	}

	h.logger.Info("Retrieved nodepool status",
		zap.String("nodepool_id", id.String()),
		zap.Int("controller_count", len(controllerStatuses)),
	)

	// Return both aggregated status AND controller status (matching cluster pattern)
	response := gin.H{
		"nodepool_id":       id,
		"cluster_id":        nodepool.ClusterID,
		"status":            nodepool.Status,    // Aggregated K8s-like status
		"controller_status": controllerStatuses, // Individual controller reports
	}

	c.JSON(http.StatusOK, response)
}

// UpdateNodePoolStatus handles controller status updates for nodepools
func (h *NodePoolHandler) UpdateNodePoolStatus(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid nodepool ID",
			err.Error(),
		))
		return
	}

	var statusUpdate models.NodePoolControllerStatus
	if err := c.ShouldBindJSON(&statusUpdate); err != nil {
		h.logger.Error("Invalid status update request", zap.Error(err))
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid status update format",
			err.Error(),
		))
		return
	}

	// Validate required fields
	if statusUpdate.ControllerName == "" {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"controller_name is required",
			"",
		))
		return
	}

	if statusUpdate.Metadata == nil {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"metadata is required",
			"",
		))
		return
	}

	// Set nodepool ID from URL parameter
	statusUpdate.NodePoolID = id

	ctx := c.Request.Context()

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

	// Verify nodepool exists
	var nodepool *models.NodePool
	if userCtx.IsController {
		// Controllers can access any nodepool
		nodepool, err = h.repository.NodePools.GetByIDInternal(ctx, id)
	} else {
		// Users can only access their own nodepools (via cluster ownership)
		nodepool, err = h.repository.NodePools.GetByID(ctx, id, userCtx.Email)
	}
	if err != nil {
		if err == models.ErrNodePoolNotFound {
			c.JSON(http.StatusNotFound, utils.NewAPIError(
				utils.ErrCodeNotFound,
				"NodePool not found",
				"",
			))
			return
		}

		h.logger.Error("Failed to verify nodepool",
			zap.String("nodepool_id", id.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to verify nodepool",
			err.Error(),
		))
		return
	}

	// Update controller status
	err = h.repository.Status.UpsertNodePoolControllerStatus(ctx, &statusUpdate)
	if err != nil {
		h.logger.Error("Failed to update nodepool controller status",
			zap.String("nodepool_id", id.String()),
			zap.String("controller_name", statusUpdate.ControllerName),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to update nodepool status",
			err.Error(),
		))
		return
	}

	// Mark the cluster status as dirty to trigger recalculation on next GET
	if err := h.repository.Clusters.MarkDirtyStatus(ctx, nodepool.ClusterID); err != nil {
		h.logger.Warn("Failed to mark cluster status as dirty",
			zap.String("cluster_id", nodepool.ClusterID.String()),
			zap.Error(err),
		)
		// Don't fail the request for this - status will be recalculated eventually
	}

	// Status update completed - cluster marked as dirty for recalculation
	// No pub/sub events needed in simplified architecture (controllers report via API)

	h.logger.Info("NodePool controller status updated",
		zap.String("nodepool_id", id.String()),
		zap.String("cluster_id", nodepool.ClusterID.String()),
		zap.String("controller_name", statusUpdate.ControllerName),
		zap.Int64("observed_generation", statusUpdate.ObservedGeneration),
	)

	c.JSON(http.StatusOK, map[string]interface{}{
		"message":             "Status updated successfully",
		"nodepool_id":         id.String(),
		"cluster_id":          nodepool.ClusterID.String(),
		"controller_name":     statusUpdate.ControllerName,
		"observed_generation": statusUpdate.ObservedGeneration,
	})
}
