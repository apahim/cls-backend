# Architecture Overview

This document provides a comprehensive overview of the CLS Backend system architecture, design principles, and core components.

## High-Level Architecture

CLS Backend is a **simplified single-tenant cluster lifecycle management service** with external authorization integration points.

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   API Gateway   │────│   CLS Backend    │────│   PostgreSQL    │
│  (External Auth)│    │                  │    │    Database     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                               │
                               │
                       ┌──────────────────┐
                       │  Google Cloud    │
                       │     Pub/Sub      │
                       └──────────────────┘
                               │
                       ┌──────────────────┐
                       │   Controllers    │
                       │ (Self-Filtering) │
                       └──────────────────┘
```

### Detailed Event Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CLS Backend Event Flow                            │
└─────────────────────────────────────────────────────────────────────────────┘

1. API Request Flow:
   ┌─────────┐   ┌──────────┐   ┌──────────┐   ┌─────────────┐   ┌─────────────┐
   │ Client  │──▶│ API      │──▶│ Database │──▶│ Pub/Sub     │──▶│ Controllers │
   │ Request │   │ Handler  │   │ Insert   │   │ Publish     │   │ Process     │
   └─────────┘   └──────────┘   └──────────┘   └─────────────┘   └─────────────┘

2. Status Aggregation:
   ┌─────────────┐   ┌──────────┐   ┌─────────────┐   ┌────────────┐
   │ Controllers │──▶│ Database │──▶│ Status      │──▶│ API        │
   │ Report      │   │ Update   │   │ Aggregation │   │ Response   │
   └─────────────┘   └──────────┘   └─────────────┘   └────────────┘

3. Reconciliation Cycle:
   ┌───────────┐   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
   │ Scheduler │──▶│ Find        │──▶│ Pub/Sub     │──▶│ Controllers │
   │ (30s)     │   │ Clusters    │   │ Events      │   │ Self-Filter │
   └───────────┘   └─────────────┘   └─────────────┘   └─────────────┘
```

## Design Principles

### 1. Simplified Single-Tenant Architecture

**Removed**: Complex organization-based multi-tenancy
**Added**: External authorization integration points

- **Clean API Endpoints**: Simple `/api/v1/clusters` paths
- **External Authorization Ready**: Uses `created_by` field for future external auth
- **Single-Tenant Database**: No organization isolation in database layer
- **Authorization via Gateway**: Designed for external API Gateway integration

### 2. Controller-Agnostic Fan-Out

**No hardcoded controller lists** - controllers discover and self-filter events:

- **Single Event per Change**: One `cluster.reconcile` event per cluster operation
- **Self-Filtering Controllers**: Controllers decide if they should act on events
- **Auto-Discovery**: New controllers work immediately without backend changes
- **Platform Aware**: Controllers filter by platform type (GCP, AWS, Azure)

### 3. Binary State Reconciliation

**Simplified from complex adaptive system** to two-state model:

- **Needs Attention**: 30-second intervals (new clusters, error states)
- **Stable**: 5-minute intervals (operational clusters)
- **Simple Logic**: Easy to understand and debug
- **Same Performance**: Fast response to issues, efficient for stable clusters

### 4. Kubernetes-like Status System

Rich status information with standard Kubernetes patterns:

- **Conditions**: Array of detailed status conditions (Ready, Available)
- **Phases**: High-level state (Pending, Progressing, Ready, Failed)
- **Generation Tracking**: Optimistic concurrency control
- **Hybrid Calculation**: Cached status with lazy recalculation

## System Components

### API Layer (`internal/api/`)

**HTTP server and request handling**

```go
// Main API components
├── server.go          // Gin server setup and middleware
├── handlers/
│   ├── clusters.go    // Cluster CRUD operations
│   ├── health.go      // Health check endpoints
│   └── info.go        // Service information
├── middleware/
│   ├── auth.go        // Authentication middleware
│   ├── cors.go        // CORS handling
│   └── logging.go     // Request logging
└── types/
    └── requests.go    // Request/response types
```

**Key Features:**
- **Gin Framework**: Fast HTTP router with middleware support
- **Simple Endpoints**: `/api/v1/clusters` without organization scoping
- **Authentication Middleware**: Extracts user context from headers
- **Error Handling**: Consistent error responses
- **Request Validation**: Input validation and sanitization

### Database Layer (`internal/database/`)

**Repository pattern with PostgreSQL**

```go
├── client.go          // Database connection and lifecycle
├── clusters.go        // Cluster repository operations
├── status_aggregator.go // Hybrid status calculation
├── migrations/
│   └── 001_complete_schema.sql // Complete database schema
└── repository.go      // Repository interface definitions
```

**Key Features:**
- **Connection Pooling**: Efficient database connections
- **Repository Pattern**: Clean separation of data access
- **JSONB Support**: Rich data structures for specs and status
- **Hybrid Status**: Cached status with dirty tracking
- **Migration System**: Schema evolution support

### Event System (`internal/pubsub/`)

**Google Cloud Pub/Sub integration**

```go
├── service.go         // Pub/Sub client management
├── publisher.go       // Event publishing
├── subscriber.go      // Event subscription (for testing)
└── messages.go        // Event message structures
```

**Fan-Out Architecture:**
```
Cluster Change → Single Event → Pub/Sub Topic → All Controllers
                   cluster.reconcile   cluster-events    (self-filter)
```

**Event Types:**
- `cluster.created` - New cluster created
- `cluster.updated` - Cluster specification updated
- `cluster.deleted` - Cluster deleted
- `cluster.reconcile` - Periodic reconciliation trigger

### Reconciliation System (`internal/reconciliation/`)

**Binary state reconciliation scheduling**

```go
├── scheduler.go       // Reconciliation scheduler
└── service.go         // Reconciliation service interface
```

**Binary State Logic:**
```go
func clusterNeedsAttention(cluster Cluster) bool {
    // New clusters (< 2 hours old)
    if time.Since(cluster.CreatedAt) < 2*time.Hour {
        return true
    }

    // Error states
    if cluster.Status.Phase == "Failed" || cluster.Status.Phase == "Error" {
        return true
    }

    return false // Stable - use 5m interval
}
```

### Configuration (`internal/config/`)

**Environment-based configuration**

```go
├── config.go          // Configuration structure
└── validation.go      // Configuration validation
```

**Configuration Categories:**
- **Database**: Connection strings and pool settings
- **Pub/Sub**: Google Cloud project and topic configuration
- **Server**: HTTP server settings and timeouts
- **Authentication**: Auth enabled/disabled, external auth settings
- **Reconciliation**: Scheduling intervals and settings

### Models (`internal/models/`)

**Data structures and types**

```go
├── cluster.go         // Cluster resource definition
├── status.go          // Kubernetes-like status structures
├── events.go          // Event message definitions
└── controller.go      // Controller status definitions
```

**Cluster Model:**
```go
type Cluster struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Spec        map[string]interface{} `json:"spec"`
    Status      *ClusterStatusInfo     `json:"status"`
    CreatedBy   string                 `json:"created_by"`
    Generation  int64                  `json:"generation"`
    CreatedAt   time.Time             `json:"created_at"`
    UpdatedAt   time.Time             `json:"updated_at"`
}
```

## Data Flow

### 1. Cluster Creation

```
API Request → Validation → Database Insert → Event Publish → Controllers
     ↓              ↓             ↓              ↓             ↓
  POST /clusters  Spec Valid   New Cluster   cluster.created  Auto-Discovery
```

**Detailed Flow:**
1. **API Handler** receives POST request with cluster specification
2. **Validation** ensures required fields and valid structure
3. **Repository** inserts cluster with generation=1, status="Pending"
4. **Event Publisher** sends `cluster.created` event to Pub/Sub
5. **Controllers** receive event, filter by platform, start work
6. **Status Updates** flow back via controller status API

### 2. Status Aggregation

```
Controller Status → Database → Dirty Flag → Lazy Calculation → API Response
       ↓               ↓           ↓             ↓              ↓
   PUT /status    status_dirty=true  GET /cluster  Calculate   K8s Status
```

**Hybrid Status System:**
1. **Controllers** report status via `PUT /clusters/{id}/status`
2. **Database** stores controller status and marks cluster dirty
3. **API Requests** check dirty flag during GET operations
4. **Lazy Calculation** recalculates status only when dirty
5. **Caching** stores calculated status for fast subsequent requests

### 3. Reconciliation Scheduling

```
Scheduler → Find Clusters → Binary Decision → Publish Events → Controllers
    ↓            ↓              ↓              ↓              ↓
  Every 30s  Needs Attention?  30s vs 5m    cluster.reconcile  Self-Filter
```

**Binary State Reconciliation:**
1. **Scheduler** runs every 30 seconds
2. **Database Query** finds clusters needing reconciliation
3. **Binary Decision** determines 30s (attention) vs 5m (stable) intervals
4. **Event Publishing** sends reconciliation events
5. **Controllers** self-filter and process applicable events

## Database Schema

### Core Tables

#### Clusters Table
```sql
CREATE TABLE clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    spec JSONB NOT NULL DEFAULT '{}',
    status JSONB,                          -- Cached K8s-like status
    status_dirty BOOLEAN DEFAULT true,     -- Dirty flag for lazy calculation
    created_by VARCHAR(255) NOT NULL,      -- External auth integration point
    generation BIGINT NOT NULL DEFAULT 1,  -- Optimistic concurrency
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

#### Controller Status Table
```sql
CREATE TABLE controller_status (
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    controller_name VARCHAR(255) NOT NULL,
    observed_generation BIGINT NOT NULL DEFAULT 1,
    conditions JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}',
    last_error JSONB,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (cluster_id, controller_name)
);
```

#### Reconciliation Schedule Table
```sql
CREATE TABLE reconciliation_schedule (
    cluster_id UUID PRIMARY KEY REFERENCES clusters(id) ON DELETE CASCADE,
    next_reconcile_at TIMESTAMP WITH TIME ZONE NOT NULL,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### Key Database Features

- **JSONB Support**: Rich data structures for cluster specs and status
- **Foreign Key Constraints**: Data consistency and cascading deletes
- **Indexes**: Optimized queries for common access patterns
- **Triggers**: Automatic updates and reconciliation scheduling
- **Generation Tracking**: Optimistic concurrency control

## Security Model

### External Authorization Integration

**Ready for external authorization systems:**

```go
// Request context extraction
type RequestContext struct {
    UserEmail      string   // From X-User-Email header
    UserDomain     string   // Extracted from email domain
    UserRoles      []string // From external authorization system
    OrganizationID string   // Future: from external auth
}
```

**Authorization Integration Points:**
- **API Gateway**: External authentication and authorization
- **Request Headers**: User context passed to backend
- **Database Fields**: `created_by` field for ownership tracking
- **Future Extensions**: Organization fields ready for multi-tenancy

### Data Isolation

Current single-tenant model with future multi-tenant preparation:

- **User Context**: All operations filtered by `created_by` field
- **Database Design**: Ready for organization-based isolation
- **API Design**: External authorization via API Gateway
- **Event Security**: User context included in events for future filtering

## Performance Characteristics

### API Performance

- **Simple Endpoints**: Direct database access without complex joins
- **Hybrid Status**: ~1ms response time for cached status, ~5-10ms when dirty
- **Connection Pooling**: Efficient database connection management
- **Minimal Middleware**: Fast request processing

### Reconciliation Performance

- **Binary State Logic**: Simple decisions, fast execution
- **Event Fan-Out**: Single event per cluster change
- **Controller Self-Filtering**: No backend processing overhead
- **Intelligent Scheduling**: 30s for attention, 5m for stable clusters

### Database Performance

- **Optimized Indexes**: Fast lookups on common query patterns
- **JSONB Efficiency**: PostgreSQL-optimized JSON storage
- **Lazy Calculation**: Status calculated only when needed
- **Dirty Tracking**: Avoids unnecessary recalculations

## Extensibility

### Adding New Controllers

Controllers integrate with zero backend changes:

1. **Create Pub/Sub Subscription** to `cluster-events` topic
2. **Implement Self-Filtering** based on platform and dependencies
3. **Report Status** via standard status API
4. **Handle Events** according to preConditions

### Adding New Platforms

New platforms (AWS, Azure, etc.) supported automatically:

1. **Specify Platform** in cluster spec: `spec.platform.type = "aws"`
2. **Controllers Filter** events by platform type
3. **No Backend Changes** required for new platforms
4. **Automatic Discovery** of platform-specific controllers

### External Authorization

Ready for integration with external authorization systems:

1. **API Gateway Integration**: Handle authentication/authorization
2. **User Context Headers**: Pass user information to backend
3. **Database Fields**: Use `created_by` for ownership tracking
4. **Future Multi-Tenancy**: Organization fields ready for extension

## Monitoring and Observability

### Health Checks

```bash
# Service health
GET /health

# Service information
GET /api/v1/info
```

### Metrics (Future)

Prometheus metrics available on port 8081:
- HTTP request duration and status codes
- Database connection pool metrics
- Pub/Sub operation metrics
- Reconciliation timing and success rates

### Logging

Structured JSON logging with configurable levels:
- Request/response logging
- Database operation logging
- Event publishing/receiving
- Error tracking with context

## Testing Strategy

### Unit Tests

- **Repository Layer**: Database operations with test database
- **API Handlers**: HTTP request/response testing
- **Status Aggregation**: Logic validation with mock data
- **Event Publishing**: Pub/Sub integration testing

### Integration Tests

- **End-to-End API**: Complete request flows
- **Database Integration**: Real PostgreSQL testing
- **Event Integration**: Pub/Sub message handling
- **Status Calculation**: Real controller status scenarios

### Performance Tests

- **Load Testing**: API performance under load
- **Database Performance**: Query optimization validation
- **Concurrency Testing**: Multiple concurrent operations
- **Memory Usage**: Resource consumption monitoring

This architecture provides a solid foundation for cluster lifecycle management with simplicity, performance, and extensibility as core design goals.