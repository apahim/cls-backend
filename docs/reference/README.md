# Reference Documentation

This section contains detailed technical reference materials for CLS Backend, including API specifications, data models, and system interfaces.

## API Reference

- **[Complete API Reference](api.md)** - Full REST API specification with examples
- **[OpenAPI Specification](openapi.md)** - OpenAPI/Swagger specification and documentation
- **[Status System Reference](../developer-guide/status-system.md)** - Kubernetes-like status structures and aggregation
- **[Event System Reference](../developer-guide/event-architecture.md)** - Pub/Sub events and message formats

## Quick API Overview

CLS Backend provides a REST API for cluster lifecycle management with simplified, single-tenant endpoints.

### Base URL Structure

```
http(s)://<host>/api/v1
```

### Authentication

All requests require the `X-User-Email` header for user context:

```bash
X-User-Email: user@example.com
```

### Core Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/clusters` | List clusters with pagination and filtering |
| POST | `/clusters` | Create a new cluster |
| GET | `/clusters/{id}` | Get cluster details with aggregated status |
| PUT | `/clusters/{id}` | Update cluster specification |
| DELETE | `/clusters/{id}` | Delete cluster (with optional force) |
| GET | `/clusters/{id}/status` | Get detailed cluster status |
| PUT | `/clusters/{id}/status` | Update cluster status (controllers only) |

### Utility Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Service health check |
| GET | `/info` | Service version and build information |
| GET | `/metrics` | Prometheus metrics (port 8081) |

## Architecture Overview

### Simplified Single-Tenant Design

CLS Backend uses a **simplified single-tenant architecture** with:

- **Clean API Design**: Simple `/api/v1/clusters` endpoints without organization scoping
- **External Authorization Ready**: Integration points via `created_by` field and request headers
- **Fan-Out Events**: Controller-agnostic Pub/Sub architecture
- **Binary State Reconciliation**: Simple 30s/5m intervals based on cluster state
- **Kubernetes-like Status**: Rich status conditions and phases

### Key Design Principles

1. **Zero Controller Awareness**: Backend has no hardcoded controller lists
2. **Auto-Discovery**: New controllers work immediately without backend changes
3. **Self-Filtering**: Controllers decide which events to process
4. **Generation Tracking**: Optimistic concurrency control prevents stale updates
5. **Hybrid Status**: Cached status with lazy recalculation for performance

## Data Models

### Cluster Resource

```json
{
  "id": "uuid",
  "name": "cluster-name",
  "spec": {
    "platform": {
      "type": "gcp|aws|azure",
      "gcp": { /* platform-specific config */ }
    }
  },
  "status": {
    "observedGeneration": 1,
    "conditions": [
      {
        "type": "Ready|Available",
        "status": "True|False|Unknown",
        "lastTransitionTime": "2025-10-17T00:00:00Z",
        "reason": "AllControllersReady",
        "message": "Human-readable message"
      }
    ],
    "phase": "Pending|Progressing|Ready|Failed",
    "message": "Overall status summary",
    "reason": "Machine-readable reason",
    "lastUpdateTime": "2025-10-17T00:00:00Z"
  },
  "created_by": "user@example.com",
  "generation": 1,
  "created_at": "2025-10-17T00:00:00Z",
  "updated_at": "2025-10-17T00:00:00Z"
}
```

### Controller Status

Controllers report status using this structure:

```json
{
  "cluster_id": "uuid",
  "controller_name": "gcp-environment-validation",
  "observed_generation": 1,
  "conditions": [
    {
      "type": "Available",
      "status": "True",
      "lastTransitionTime": "2025-10-17T00:00:00Z",
      "reason": "WorkCompleted",
      "message": "Environment validation completed successfully"
    }
  ],
  "metadata": {
    "platform": "gcp",
    "region": "us-central1"
  },
  "last_error": null,
  "updated_at": "2025-10-17T00:00:00Z"
}
```

## Event System

### Fan-Out Architecture

**Single Topic Design:**
```
Cluster Change → cluster-events topic → All Controllers (self-filter)
```

**Event Types:**
- `cluster.created` - New cluster created
- `cluster.updated` - Cluster specification updated
- `cluster.deleted` - Cluster deleted
- `cluster.reconcile` - Periodic or reactive reconciliation

### Event Message Format

```json
{
  "type": "cluster.created|cluster.updated|cluster.deleted|cluster.reconcile",
  "cluster_id": "uuid",
  "generation": 1,
  "timestamp": "2025-10-17T16:00:00Z",
  "cluster": { /* complete cluster object */ },
  "metadata": {
    "triggered_by": "api_request|reconciliation_scheduler",
    "user_email": "user@example.com",
    "reason": "spec_change|generation_mismatch|periodic_reconciliation"
  }
}
```

## Status System

### Status Phases

| Phase | Description | Controller State |
|-------|-------------|------------------|
| `Pending` | No controllers have reported status yet | 0 controllers reporting |
| `Progressing` | Some controllers working toward ready state | Partial controllers ready |
| `Ready` | All controllers operational | All controllers ready |
| `Failed` | No controllers operational | No controllers ready |

### Status Conditions

**Always Present:**
- **Ready**: Indicates if cluster is ready to serve requests
- **Available**: Indicates if cluster is available for use

**Condition Status Values:**
- `True` - Condition is satisfied
- `False` - Condition is not satisfied
- `Unknown` - Condition state is unknown

### Generation-Aware Aggregation

Status aggregation only considers controller status for the current cluster generation:

```sql
-- Only count controllers reporting for current generation
WHERE cluster_id = ? AND observed_generation = current_cluster_generation
```

This prevents stale controller status from affecting cluster state calculations.

## Integration Patterns

### Controller Integration

Controllers integrate with CLS Backend by:

1. **Creating Pub/Sub Subscription** to `cluster-events` topic
2. **Self-Filtering Events** based on platform and dependencies
3. **Processing Cluster Changes** when filters match
4. **Reporting Status** via PUT `/clusters/{id}/status`
5. **Updating Generation** to match current cluster generation

### External Authorization

Ready for integration with external authorization systems:

- **User Context**: `X-User-Email` header provides user identification
- **Ownership Tracking**: `created_by` field enables user-based filtering
- **API Gateway Ready**: Designed for external authentication/authorization
- **Future Multi-Tenancy**: Database schema supports organization extension

## Performance Characteristics

### API Performance

- **Simple Endpoints**: Direct database access without complex joins
- **Hybrid Status**: ~1ms cached, ~5-10ms when dirty
- **Connection Pooling**: Configurable database connection management
- **Pagination**: Efficient large dataset handling

### Reconciliation Performance

- **Binary State**: Simple 30s vs 5m decision logic
- **Event Fan-Out**: Single event per cluster change
- **Controller Independence**: No backend processing overhead
- **Lazy Status**: Calculation only when needed

### Database Performance

- **JSONB Optimization**: PostgreSQL-optimized JSON storage
- **Proper Indexing**: Fast lookups on common patterns
- **Generation Filtering**: Prevents processing stale data
- **Connection Pooling**: Efficient resource utilization

## Error Handling

### HTTP Status Codes

| Code | Usage | Description |
|------|-------|-------------|
| `200` | Success | GET, PUT operations completed |
| `201` | Created | POST operations completed |
| `400` | Bad Request | Invalid JSON, missing fields, validation errors |
| `401` | Unauthorized | Missing or invalid authentication |
| `404` | Not Found | Cluster or resource doesn't exist |
| `409` | Conflict | Cluster name conflicts, concurrent updates |
| `500` | Server Error | Database issues, internal errors |

### Error Response Format

```json
{
  "error": "Human-readable error message",
  "code": "MACHINE_READABLE_ERROR_CODE",
  "details": {
    "field": "Additional error context"
  }
}
```

## Rate Limiting and Pagination

### Pagination Parameters

- `limit`: Maximum results per request (default: 50, max: 100)
- `offset`: Number of results to skip (default: 0)

### Response Format

```json
{
  "clusters": [ /* cluster objects */ ],
  "limit": 50,
  "offset": 0,
  "total": 150
}
```

## Security Considerations

### Authentication

- **Development**: `DISABLE_AUTH=true` for local testing
- **Production**: External authorization via API Gateway
- **Headers**: User context provided via `X-User-Email`

### Data Isolation

- **User Filtering**: All operations filtered by `created_by`
- **Generation Safety**: Prevents concurrent update conflicts
- **Input Validation**: All inputs validated and sanitized

This reference documentation provides the technical foundation for understanding and integrating with CLS Backend's simplified single-tenant architecture.