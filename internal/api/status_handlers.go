package api

import (
	"net/http"
	"strconv"

	"github.com/apahim/cls-backend/internal/database"
	"github.com/apahim/cls-backend/internal/models"
	"github.com/apahim/cls-backend/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// StatusHandler handles status-related HTTP requests
type StatusHandler struct {
	repository *database.Repository
	logger     *utils.Logger
}

// NewStatusHandler creates a new status handler
func NewStatusHandler(repository *database.Repository) *StatusHandler {
	return &StatusHandler{
		repository: repository,
		logger:     utils.NewLogger("status_handler"),
	}
}

// RegisterRoutes registers status routes with the router
func (h *StatusHandler) RegisterRoutes(r *gin.RouterGroup) {
	status := r.Group("/status")
	{
		// Remove redundant routes - these are now handled by cluster/nodepool handlers:
		// ❌ status.GET("/clusters", h.GetClustersStatus) → Use GET /api/v1/clusters
		// ❌ status.GET("/clusters/:id/summary", h.GetClusterStatusSummary) → Use GET /api/v1/clusters/{id}/status

		// Keep operational routes:
		status.POST("/clusters/:id/aggregate", h.TriggerStatusAggregation)
		status.GET("/health", h.GetHealthStatus)
		status.GET("/errors", h.GetErrors)
	}
}



// TriggerStatusAggregation manually triggers status aggregation for a cluster
func (h *StatusHandler) TriggerStatusAggregation(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.NewAPIError(
			utils.ErrCodeValidation,
			"Invalid cluster ID",
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

	// Verify cluster exists
	_, err = h.repository.Clusters.GetByID(ctx, id, userEmail)
	if err != nil {
		if err == models.ErrClusterNotFound {
			c.JSON(http.StatusNotFound, utils.NewAPIError(
				utils.ErrCodeNotFound,
				"Cluster not found",
				"",
			))
			return
		}

		h.logger.Error("Failed to verify cluster",
			zap.String("cluster_id", id.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to verify cluster",
			err.Error(),
		))
		return
	}

	// Mark cluster as dirty to force recalculation on next GET
	err = h.repository.Clusters.MarkDirtyStatus(ctx, id)
	if err != nil {
		h.logger.Error("Failed to mark cluster status as dirty",
			zap.String("cluster_id", id.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to mark cluster status as dirty",
			err.Error(),
		))
		return
	}

	// Get the cluster to trigger status calculation
	cluster, err := h.repository.Clusters.GetByID(ctx, id, userEmail)
	if err != nil {
		h.logger.Error("Failed to get cluster after marking dirty",
			zap.String("cluster_id", id.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to get cluster",
			err.Error(),
		))
		return
	}

	// Build result from calculated status
	result := map[string]interface{}{
		"cluster_id":        cluster.ID.String(),
		"status":            cluster.Status, // K8s-like status structure
		"generation":        cluster.Generation,
		"last_updated":      cluster.UpdatedAt,
	}

	h.logger.Info("Status aggregation completed",
		zap.String("cluster_id", id.String()),
		zap.String("status_phase", cluster.Status.Phase),
	)

	c.JSON(http.StatusOK, result)
}

// GetHealthStatus returns the health status of the service and its dependencies
func (h *StatusHandler) GetHealthStatus(c *gin.Context) {
	ctx := c.Request.Context()

	// Check database health
	dbHealth, err := h.repository.Health(ctx)
	if err != nil {
		h.logger.Error("Database health check failed", zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"status": "unhealthy",
			"checks": map[string]interface{}{
				"database": map[string]interface{}{
					"status": "unhealthy",
					"error":  err.Error(),
				},
			},
		})
		return
	}

	healthStatus := "healthy"
	if dbHealth.Status != "healthy" {
		healthStatus = "degraded"
	}

	// Build health response
	response := map[string]interface{}{
		"status": healthStatus,
		"checks": map[string]interface{}{
			"database": map[string]interface{}{
				"status":         dbHealth.Status,
				"max_open_conns": dbHealth.MaxOpenConns,
				"open_conns":     dbHealth.OpenConns,
				"in_use_conns":   dbHealth.InUseConns,
				"idle_conns":     dbHealth.IdleConns,
				"wait_count":     dbHealth.WaitCount,
				"wait_duration":  dbHealth.WaitDuration,
				"max_idle_count": dbHealth.MaxIdleCount,
				"max_life_count": dbHealth.MaxLifeCount,
				"issues":         dbHealth.Issues,
			},
		},
	}

	statusCode := http.StatusOK
	if healthStatus == "degraded" {
		statusCode = http.StatusOK // Still return 200 for degraded but functional
	} else if healthStatus == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, response)
}

// GetErrors returns error information across clusters
func (h *StatusHandler) GetErrors(c *gin.Context) {
	// This would use the cluster_error_summary view from migration 003
	// For now, we'll provide a basic implementation

	// Parse query parameters
	namespace := c.Query("namespace")
	errorType := c.Query("error_type")
	userActionable := c.Query("user_actionable")

	limit := 100
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	ctx := c.Request.Context()

	// Build query for cluster_error_summary view
	query := `
		SELECT cluster_id, cluster_name, namespace, controller_name,
			   error_type, error_code, error_message, user_actionable, error_time
		FROM cluster_error_summary
		WHERE 1=1`

	var args []interface{}
	argIndex := 1

	if namespace != "" {
		query += " AND namespace = $" + strconv.Itoa(argIndex)
		args = append(args, namespace)
		argIndex++
	}

	if errorType != "" {
		query += " AND error_type = $" + strconv.Itoa(argIndex)
		args = append(args, errorType)
		argIndex++
	}

	if userActionable != "" {
		isUserActionable := userActionable == "true"
		query += " AND user_actionable = $" + strconv.Itoa(argIndex)
		args = append(args, isUserActionable)
		argIndex++
	}

	query += " ORDER BY error_time DESC"
	query += " LIMIT $" + strconv.Itoa(argIndex)
	args = append(args, limit)
	argIndex++

	if offset > 0 {
		query += " OFFSET $" + strconv.Itoa(argIndex)
		args = append(args, offset)
	}

	rows, err := h.repository.GetClient().QueryContext(ctx, query, args...)
	if err != nil {
		h.logger.Error("Failed to query error summary", zap.Error(err))
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Failed to get error summary",
			err.Error(),
		))
		return
	}
	defer rows.Close()

	var errors []map[string]interface{}
	for rows.Next() {
		var errorInfo map[string]interface{} = make(map[string]interface{})
		var clusterID, clusterName, namespace, controllerName string
		var errorType, errorCode, errorMessage interface{}
		var userActionable interface{}
		var errorTime interface{}

		err := rows.Scan(
			&clusterID, &clusterName, &namespace, &controllerName,
			&errorType, &errorCode, &errorMessage, &userActionable, &errorTime,
		)
		if err != nil {
			h.logger.Error("Failed to scan error row", zap.Error(err))
			continue
		}

		errorInfo["cluster_id"] = clusterID
		errorInfo["cluster_name"] = clusterName
		errorInfo["namespace"] = namespace
		errorInfo["controller_name"] = controllerName
		errorInfo["error_type"] = errorType
		errorInfo["error_code"] = errorCode
		errorInfo["error_message"] = errorMessage
		errorInfo["user_actionable"] = userActionable
		errorInfo["error_time"] = errorTime

		errors = append(errors, errorInfo)
	}

	if err = rows.Err(); err != nil {
		h.logger.Error("Error iterating error rows", zap.Error(err))
		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Error processing error summary",
			err.Error(),
		))
		return
	}

	response := map[string]interface{}{
		"errors": errors,
		"limit":  limit,
		"offset": offset,
	}

	c.JSON(http.StatusOK, response)
}
