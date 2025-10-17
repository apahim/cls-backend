# API Development Guide

This guide covers how to add new API endpoints, modify existing ones, and follow API development best practices in CLS Backend.

## API Architecture Overview

CLS Backend uses a layered architecture for API development:

```
HTTP Request → Middleware → Handler → Service → Repository → Database
     ↓             ↓          ↓         ↓          ↓          ↓
   Routing    Auth/CORS   Business   Domain     Data      Storage
             Validation   Logic      Logic     Access
```

## Project Structure

```
internal/
├── api/
│   ├── handlers/          # HTTP request handlers
│   │   ├── clusters.go    # Cluster CRUD operations
│   │   ├── health.go      # Health check endpoints
│   │   └── info.go        # Service information
│   ├── middleware/        # HTTP middleware
│   │   ├── auth.go        # Authentication
│   │   ├── cors.go        # CORS handling
│   │   └── logging.go     # Request logging
│   ├── types/            # Request/response types
│   │   └── requests.go
│   └── server.go         # HTTP server setup
├── database/             # Data access layer
│   ├── clusters.go       # Cluster repository
│   └── repository.go     # Repository interfaces
├── models/               # Domain models
│   └── cluster.go
└── pubsub/              # Event publishing
    └── publisher.go
```

## Adding a New API Endpoint

Let's walk through adding a new endpoint to manage NodePools as an example.

### Step 1: Define the Domain Model

First, create or update the domain model:

```go
// internal/models/nodepool.go
package models

import (
    "time"
    "encoding/json"
)

type NodePool struct {
    ID          string                 `json:"id" db:"id"`
    Name        string                 `json:"name" db:"name"`
    ClusterID   string                 `json:"cluster_id" db:"cluster_id"`
    Spec        map[string]interface{} `json:"spec" db:"spec"`
    Status      *NodePoolStatusInfo    `json:"status" db:"status"`
    CreatedBy   string                 `json:"created_by" db:"created_by"`
    Generation  int64                  `json:"generation" db:"generation"`
    CreatedAt   time.Time             `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time             `json:"updated_at" db:"updated_at"`
}

type NodePoolStatusInfo struct {
    ObservedGeneration int64              `json:"observedGeneration"`
    Conditions         []Condition        `json:"conditions"`
    Phase              string             `json:"phase"`
    Message            string             `json:"message"`
    Reason             string             `json:"reason"`
    LastUpdateTime     time.Time         `json:"lastUpdateTime"`
}

func (n *NodePool) Validate() error {
    if n.Name == "" {
        return errors.New("name is required")
    }
    if n.ClusterID == "" {
        return errors.New("cluster_id is required")
    }
    if n.CreatedBy == "" {
        return errors.New("created_by is required")
    }
    return nil
}
```

### Step 2: Create Database Schema

Add database migration for the new table:

```sql
-- internal/database/migrations/002_add_nodepools.sql
CREATE TABLE nodepools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    spec JSONB NOT NULL DEFAULT '{}',
    status JSONB,
    status_dirty BOOLEAN DEFAULT true,
    created_by VARCHAR(255) NOT NULL,
    generation BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(cluster_id, name),
    CONSTRAINT valid_name CHECK (name ~ '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$')
);

CREATE INDEX idx_nodepools_cluster_id ON nodepools(cluster_id);
CREATE INDEX idx_nodepools_created_by ON nodepools(created_by);
```

### Step 3: Implement Repository Layer

Create the data access layer:

```go
// internal/database/nodepools.go
package database

import (
    "context"
    "database/sql"
    "encoding/json"
    "github.com/your-org/cls-backend/internal/models"
)

type NodePoolRepository interface {
    CreateNodePool(ctx context.Context, nodepool *models.NodePool) (*models.NodePool, error)
    GetNodePool(ctx context.Context, id, userEmail string) (*models.NodePool, error)
    UpdateNodePool(ctx context.Context, nodepool *models.NodePool) (*models.NodePool, error)
    DeleteNodePool(ctx context.Context, id, userEmail string) error
    ListNodePools(ctx context.Context, clusterID, userEmail string, limit, offset int) ([]*models.NodePool, int64, error)
}

func (r *Repository) CreateNodePool(ctx context.Context, nodepool *models.NodePool) (*models.NodePool, error) {
    if err := nodepool.Validate(); err != nil {
        return nil, err
    }

    // Set initial values
    nodepool.Generation = 1
    nodepool.CreatedAt = time.Now()
    nodepool.UpdatedAt = time.Now()

    // Serialize spec
    specBytes, err := json.Marshal(nodepool.Spec)
    if err != nil {
        return nil, err
    }

    query := `
        INSERT INTO nodepools (name, cluster_id, spec, created_by, generation, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id`

    err = r.db.QueryRowContext(ctx, query,
        nodepool.Name,
        nodepool.ClusterID,
        specBytes,
        nodepool.CreatedBy,
        nodepool.Generation,
        nodepool.CreatedAt,
        nodepool.UpdatedAt,
    ).Scan(&nodepool.ID)

    if err != nil {
        return nil, err
    }

    return nodepool, nil
}

func (r *Repository) GetNodePool(ctx context.Context, id, userEmail string) (*models.NodePool, error) {
    query := `
        SELECT id, name, cluster_id, spec, status, created_by, generation, created_at, updated_at
        FROM nodepools
        WHERE id = $1 AND created_by = $2`

    var nodepool models.NodePool
    var specBytes, statusBytes []byte

    err := r.db.QueryRowContext(ctx, query, id, userEmail).Scan(
        &nodepool.ID,
        &nodepool.Name,
        &nodepool.ClusterID,
        &specBytes,
        &statusBytes,
        &nodepool.CreatedBy,
        &nodepool.Generation,
        &nodepool.CreatedAt,
        &nodepool.UpdatedAt,
    )

    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.New("nodepool not found")
        }
        return nil, err
    }

    // Deserialize spec and status
    if err := json.Unmarshal(specBytes, &nodepool.Spec); err != nil {
        return nil, err
    }

    if statusBytes != nil {
        var status models.NodePoolStatusInfo
        if err := json.Unmarshal(statusBytes, &status); err != nil {
            return nil, err
        }
        nodepool.Status = &status
    }

    return &nodepool, nil
}
```

### Step 4: Create Request/Response Types

Define API-specific types:

```go
// internal/api/types/nodepools.go
package types

type CreateNodePoolRequest struct {
    Name string                 `json:"name" binding:"required"`
    Spec map[string]interface{} `json:"spec" binding:"required"`
}

type UpdateNodePoolRequest struct {
    Spec map[string]interface{} `json:"spec" binding:"required"`
}

type ListNodePoolsResponse struct {
    NodePools []*models.NodePool `json:"nodepools"`
    Limit     int                `json:"limit"`
    Offset    int                `json:"offset"`
    Total     int64              `json:"total"`
}
```

### Step 5: Implement API Handlers

Create the HTTP handlers:

```go
// internal/api/handlers/nodepools.go
package handlers

import (
    "net/http"
    "strconv"
    "github.com/gin-gonic/gin"
    "github.com/your-org/cls-backend/internal/api/types"
    "github.com/your-org/cls-backend/internal/database"
    "github.com/your-org/cls-backend/internal/models"
    "github.com/your-org/cls-backend/internal/pubsub"
)

type NodePoolHandler struct {
    repo      database.NodePoolRepository
    publisher pubsub.Publisher
}

func NewNodePoolHandler(repo database.NodePoolRepository, publisher pubsub.Publisher) *NodePoolHandler {
    return &NodePoolHandler{
        repo:      repo,
        publisher: publisher,
    }
}

func (h *NodePoolHandler) CreateNodePool(c *gin.Context) {
    var req types.CreateNodePoolRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    clusterID := c.Param("clusterId")
    userEmail := c.GetHeader("X-User-Email")

    nodepool := &models.NodePool{
        Name:      req.Name,
        ClusterID: clusterID,
        Spec:      req.Spec,
        CreatedBy: userEmail,
    }

    created, err := h.repo.CreateNodePool(c.Request.Context(), nodepool)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Publish creation event
    if err := h.publisher.PublishNodePoolEvent(c.Request.Context(), "nodepool.created", created); err != nil {
        // Log error but don't fail the request
        // TODO: Add proper logging
    }

    c.JSON(http.StatusCreated, created)
}

func (h *NodePoolHandler) GetNodePool(c *gin.Context) {
    id := c.Param("id")
    userEmail := c.GetHeader("X-User-Email")

    nodepool, err := h.repo.GetNodePool(c.Request.Context(), id, userEmail)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "NodePool not found"})
        return
    }

    c.JSON(http.StatusOK, nodepool)
}

func (h *NodePoolHandler) UpdateNodePool(c *gin.Context) {
    var req types.UpdateNodePoolRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    id := c.Param("id")
    userEmail := c.GetHeader("X-User-Email")

    // Get existing nodepool
    nodepool, err := h.repo.GetNodePool(c.Request.Context(), id, userEmail)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "NodePool not found"})
        return
    }

    // Update spec and increment generation
    nodepool.Spec = req.Spec
    nodepool.Generation++
    nodepool.UpdatedAt = time.Now()

    updated, err := h.repo.UpdateNodePool(c.Request.Context(), nodepool)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Publish update event
    if err := h.publisher.PublishNodePoolEvent(c.Request.Context(), "nodepool.updated", updated); err != nil {
        // Log error but don't fail the request
    }

    c.JSON(http.StatusOK, updated)
}

func (h *NodePoolHandler) DeleteNodePool(c *gin.Context) {
    id := c.Param("id")
    userEmail := c.GetHeader("X-User-Email")

    // Get nodepool before deletion for event
    nodepool, err := h.repo.GetNodePool(c.Request.Context(), id, userEmail)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "NodePool not found"})
        return
    }

    err = h.repo.DeleteNodePool(c.Request.Context(), id, userEmail)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete nodepool"})
        return
    }

    // Publish deletion event
    if err := h.publisher.PublishNodePoolEvent(c.Request.Context(), "nodepool.deleted", nodepool); err != nil {
        // Log error but don't fail the request
    }

    c.JSON(http.StatusOK, gin.H{"message": "NodePool deleted successfully"})
}

func (h *NodePoolHandler) ListNodePools(c *gin.Context) {
    clusterID := c.Param("clusterId")
    userEmail := c.GetHeader("X-User-Email")

    // Parse query parameters
    limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
    offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

    // Validate limits
    if limit > 100 {
        limit = 100
    }
    if limit < 1 {
        limit = 50
    }

    nodepools, total, err := h.repo.ListNodePools(c.Request.Context(), clusterID, userEmail, limit, offset)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list nodepools"})
        return
    }

    response := types.ListNodePoolsResponse{
        NodePools: nodepools,
        Limit:     limit,
        Offset:    offset,
        Total:     total,
    }

    c.JSON(http.StatusOK, response)
}
```

### Step 6: Register Routes

Add routes to the server:

```go
// internal/api/server.go (add to setupRoutes function)

func (s *Server) setupRoutes() {
    // ... existing routes ...

    // NodePool routes
    nodepoolHandler := handlers.NewNodePoolHandler(s.repo, s.publisher)

    v1.GET("/clusters/:clusterId/nodepools", nodepoolHandler.ListNodePools)
    v1.POST("/clusters/:clusterId/nodepools", nodepoolHandler.CreateNodePool)
    v1.GET("/nodepools/:id", nodepoolHandler.GetNodePool)
    v1.PUT("/nodepools/:id", nodepoolHandler.UpdateNodePool)
    v1.DELETE("/nodepools/:id", nodepoolHandler.DeleteNodePool)
}
```

### Step 7: Add Tests

Create comprehensive tests:

```go
// internal/api/handlers/nodepools_test.go
package handlers

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestNodePoolHandler_CreateNodePool(t *testing.T) {
    gin.SetMode(gin.TestMode)

    mockRepo := &MockNodePoolRepository{}
    mockPublisher := &MockPublisher{}
    handler := NewNodePoolHandler(mockRepo, mockPublisher)

    expectedNodePool := &models.NodePool{
        ID:        "test-id",
        Name:      "test-nodepool",
        ClusterID: "cluster-id",
        Generation: 1,
    }

    mockRepo.On("CreateNodePool", mock.Anything, mock.Anything).Return(expectedNodePool, nil)
    mockPublisher.On("PublishNodePoolEvent", mock.Anything, "nodepool.created", expectedNodePool).Return(nil)

    requestBody := map[string]interface{}{
        "name": "test-nodepool",
        "spec": map[string]interface{}{
            "replicas": 3,
        },
    }

    body, _ := json.Marshal(requestBody)
    req := httptest.NewRequest("POST", "/api/v1/clusters/cluster-id/nodepools", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-User-Email", "test@example.com")

    w := httptest.NewRecorder()
    router := gin.New()
    router.POST("/api/v1/clusters/:clusterId/nodepools", handler.CreateNodePool)
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusCreated, w.Code)

    var response models.NodePool
    err := json.Unmarshal(w.Body.Bytes(), &response)
    assert.NoError(t, err)
    assert.Equal(t, expectedNodePool.ID, response.ID)

    mockRepo.AssertExpectations(t)
    mockPublisher.AssertExpectations(t)
}
```

## API Best Practices

### 1. Input Validation

Always validate input at multiple layers:

```go
// Model validation
func (n *NodePool) Validate() error {
    if n.Name == "" {
        return errors.New("name is required")
    }
    if !isValidName(n.Name) {
        return errors.New("name must contain only lowercase letters, numbers, and hyphens")
    }
    return nil
}

// Handler validation
func (h *NodePoolHandler) CreateNodePool(c *gin.Context) {
    var req types.CreateNodePoolRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid request format",
            "details": err.Error(),
        })
        return
    }

    // Additional business logic validation
    if err := validateNodePoolSpec(req.Spec); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid nodepool specification",
            "details": err.Error(),
        })
        return
    }
}
```

### 2. Error Handling

Use consistent error responses:

```go
// internal/api/errors/errors.go
package errors

type APIError struct {
    Code    string      `json:"code"`
    Message string      `json:"error"`
    Details interface{} `json:"details,omitempty"`
}

func NewValidationError(message string, details interface{}) *APIError {
    return &APIError{
        Code:    "VALIDATION_ERROR",
        Message: message,
        Details: details,
    }
}

func NewNotFoundError(resource string) *APIError {
    return &APIError{
        Code:    "NOT_FOUND",
        Message: fmt.Sprintf("%s not found", resource),
    }
}

// Usage in handlers
func (h *NodePoolHandler) GetNodePool(c *gin.Context) {
    nodepool, err := h.repo.GetNodePool(c.Request.Context(), id, userEmail)
    if err != nil {
        if strings.Contains(err.Error(), "not found") {
            c.JSON(http.StatusNotFound, NewNotFoundError("NodePool"))
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }

    c.JSON(http.StatusOK, nodepool)
}
```

### 3. Authentication Middleware

Ensure all endpoints use authentication:

```go
// internal/api/middleware/auth.go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        userEmail := c.GetHeader("X-User-Email")
        if userEmail == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
            c.Abort()
            return
        }

        // Validate email format
        if !isValidEmail(userEmail) {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user email format"})
            c.Abort()
            return
        }

        c.Next()
    }
}
```

### 4. Pagination

Implement consistent pagination:

```go
func ParsePaginationParams(c *gin.Context) (limit, offset int) {
    limit, _ = strconv.Atoi(c.DefaultQuery("limit", "50"))
    offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))

    // Validate and constrain
    if limit > 100 {
        limit = 100
    }
    if limit < 1 {
        limit = 50
    }
    if offset < 0 {
        offset = 0
    }

    return limit, offset
}

func (h *NodePoolHandler) ListNodePools(c *gin.Context) {
    limit, offset := ParsePaginationParams(c)

    nodepools, total, err := h.repo.ListNodePools(c.Request.Context(), clusterID, userEmail, limit, offset)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list nodepools"})
        return
    }

    response := gin.H{
        "nodepools": nodepools,
        "pagination": gin.H{
            "limit":  limit,
            "offset": offset,
            "total":  total,
        },
    }

    c.JSON(http.StatusOK, response)
}
```

### 5. Event Publishing

Consistently publish events for state changes:

```go
// internal/pubsub/publisher.go
func (p *Publisher) PublishNodePoolEvent(ctx context.Context, eventType string, nodepool *models.NodePool) error {
    event := &pubsub.Event{
        Type:      eventType,
        ID:        nodepool.ID,
        ClusterID: nodepool.ClusterID,
        Timestamp: time.Now(),
        Data: map[string]interface{}{
            "nodepool": nodepool,
        },
        Attributes: map[string]string{
            "event_type":  eventType,
            "nodepool_id": nodepool.ID,
            "cluster_id":  nodepool.ClusterID,
            "created_by":  nodepool.CreatedBy,
        },
    }

    return p.publish(ctx, "cluster-events", event)
}
```

## OpenAPI Documentation

### Adding Swagger Annotations

Use swaggo to generate OpenAPI documentation:

```go
// internal/api/handlers/nodepools.go

// CreateNodePool creates a new nodepool
// @Summary Create nodepool
// @Description Create a new nodepool for a cluster
// @Tags nodepools
// @Accept json
// @Produce json
// @Param clusterId path string true "Cluster ID"
// @Param X-User-Email header string true "User Email"
// @Param nodepool body types.CreateNodePoolRequest true "NodePool specification"
// @Success 201 {object} models.NodePool
// @Failure 400 {object} errors.APIError
// @Failure 401 {object} errors.APIError
// @Router /api/v1/clusters/{clusterId}/nodepools [post]
func (h *NodePoolHandler) CreateNodePool(c *gin.Context) {
    // ... implementation
}
```

### Generate Documentation

```bash
# Install swaggo
go install github.com/swaggo/swag/cmd/swag@latest

# Generate docs
swag init -g internal/api/server.go -o docs/swagger

# Serve docs at /swagger/index.html
```

## API Versioning Strategy

### URL-Based Versioning

Current approach uses URL path versioning:

```go
// Routes
v1 := router.Group("/api/v1")
v2 := router.Group("/api/v2")  // Future version

// Version-specific handlers
v1.GET("/clusters", handlersV1.ListClusters)
v2.GET("/clusters", handlersV2.ListClusters)
```

### Backward Compatibility

Maintain backward compatibility:

```go
// Support both old and new field names
type NodePoolResponse struct {
    ID       string `json:"id"`
    Name     string `json:"name"`

    // Deprecated: use 'cluster_id' instead
    Cluster  string `json:"cluster,omitempty"`

    ClusterID string `json:"cluster_id"`
}

func (n *NodePool) ToResponse() *NodePoolResponse {
    return &NodePoolResponse{
        ID:        n.ID,
        Name:      n.Name,
        Cluster:   n.ClusterID,  // For backward compatibility
        ClusterID: n.ClusterID,
    }
}
```

## Performance Considerations

### Database Query Optimization

```go
// Use prepared statements and proper indexing
func (r *Repository) ListNodePoolsOptimized(ctx context.Context, clusterID, userEmail string, limit, offset int) ([]*models.NodePool, int64, error) {
    // Count query (separate for efficiency)
    countQuery := `SELECT COUNT(*) FROM nodepools WHERE cluster_id = $1 AND created_by = $2`
    var total int64
    err := r.db.QueryRowContext(ctx, countQuery, clusterID, userEmail).Scan(&total)
    if err != nil {
        return nil, 0, err
    }

    // Main query with limit/offset
    query := `
        SELECT id, name, cluster_id, spec, status, created_by, generation, created_at, updated_at
        FROM nodepools
        WHERE cluster_id = $1 AND created_by = $2
        ORDER BY created_at DESC
        LIMIT $3 OFFSET $4`

    rows, err := r.db.QueryContext(ctx, query, clusterID, userEmail, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var nodepools []*models.NodePool
    for rows.Next() {
        // ... scan rows
        nodepools = append(nodepools, nodepool)
    }

    return nodepools, total, nil
}
```

### Response Caching

```go
// internal/api/middleware/cache.go
func CacheMiddleware(duration time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Only cache GET requests
        if c.Request.Method != "GET" {
            c.Next()
            return
        }

        // Create cache key
        key := fmt.Sprintf("%s:%s", c.Request.URL.Path, c.Request.URL.RawQuery)

        // Check cache
        if cached, found := cache.Get(key); found {
            c.Header("X-Cache", "HIT")
            c.JSON(http.StatusOK, cached)
            return
        }

        // Continue processing
        c.Next()

        // Cache successful responses
        if c.Writer.Status() == http.StatusOK {
            // Extract response body and cache it
            // Implementation depends on caching strategy
        }
    }
}
```

## Related Documentation

- **[Architecture Overview](architecture.md)** - System design and components
- **[Testing Guide](testing.md)** - Testing strategies and best practices
- **[Local Setup](local-setup.md)** - Development environment setup
- **[API Reference](../reference/api.md)** - Complete API specification