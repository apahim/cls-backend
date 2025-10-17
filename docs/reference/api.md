# Complete API Reference

This document provides the complete REST API specification for CLS Backend with the simplified single-tenant architecture.

## Base URL

```
http(s)://<host>/api/v1
```

## Authentication

All API requests require the `X-User-Email` header for user context:

```bash
X-User-Email: user@example.com
```

**Development Mode**: Set `DISABLE_AUTH=true` to bypass authentication (testing only)
**Production Mode**: External authorization system provides user context via headers

## Core API Endpoints

### 1. List Clusters

Get a paginated list of clusters for the authenticated user.

```http
GET /clusters
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | integer | 50 | Maximum results (1-100) |
| `offset` | integer | 0 | Number of results to skip |
| `platform` | string | - | Filter by platform (gcp, aws, azure) |
| `status` | string | - | Filter by status phase |

**Request Example:**

```bash
curl -H "X-User-Email: user@example.com" \
  "http://localhost:8080/api/v1/clusters?limit=10&platform=gcp"
```

**Response (200 OK):**

```json
{
  "clusters": [
    {
      "id": "abc-123-def",
      "name": "production-cluster",
      "spec": {
        "platform": {
          "type": "gcp",
          "gcp": {
            "projectID": "my-project",
            "region": "us-central1"
          }
        }
      },
      "status": {
        "observedGeneration": 1,
        "conditions": [
          {
            "type": "Ready",
            "status": "True",
            "lastTransitionTime": "2025-10-17T00:00:00Z",
            "reason": "AllControllersReady",
            "message": "All 3 controllers are ready"
          }
        ],
        "phase": "Ready",
        "message": "Cluster is ready with 3 controllers operational",
        "lastUpdateTime": "2025-10-17T00:00:00Z"
      },
      "created_by": "user@example.com",
      "generation": 1,
      "created_at": "2025-10-17T00:00:00Z",
      "updated_at": "2025-10-17T00:00:00Z"
    }
  ],
  "limit": 10,
  "offset": 0,
  "total": 1
}
```

### 2. Create Cluster

Create a new cluster with the specified configuration.

```http
POST /clusters
```

**Request Body:**

```json
{
  "name": "string (required)",
  "target_project_id": "string (optional)",
  "spec": {
    "platform": {
      "type": "gcp|aws|azure (required)",
      "gcp": {
        "projectID": "string",
        "region": "string",
        "zone": "string (optional)"
      }
    },
    "release": {
      "image": "string (optional)",
      "version": "string (optional)"
    },
    "networking": {
      "clusterNetwork": [
        {
          "cidr": "string",
          "hostPrefix": "integer"
        }
      ],
      "serviceNetwork": ["string"]
    },
    "dns": {
      "baseDomain": "string (optional)"
    }
  }
}
```

**Request Example:**

```bash
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "name": "my-cluster",
    "spec": {
      "platform": {
        "type": "gcp",
        "gcp": {
          "projectID": "my-project",
          "region": "us-central1",
          "zone": "us-central1-a"
        }
      },
      "dns": {
        "baseDomain": "example.com"
      }
    }
  }'
```

**Response (201 Created):**

```json
{
  "id": "new-cluster-uuid",
  "name": "my-cluster",
  "spec": {
    "platform": {
      "type": "gcp",
      "gcp": {
        "projectID": "my-project",
        "region": "us-central1",
        "zone": "us-central1-a"
      }
    },
    "dns": {
      "baseDomain": "example.com"
    }
  },
  "status": {
    "observedGeneration": 1,
    "conditions": [
      {
        "type": "Ready",
        "status": "False",
        "lastTransitionTime": "2025-10-17T00:00:00Z",
        "reason": "NoControllers",
        "message": "No controllers have reported status yet"
      }
    ],
    "phase": "Pending",
    "message": "Waiting for controllers to report status",
    "lastUpdateTime": "2025-10-17T00:00:00Z"
  },
  "created_by": "user@example.com",
  "generation": 1,
  "created_at": "2025-10-17T00:00:00Z",
  "updated_at": "2025-10-17T00:00:00Z"
}
```

### 3. Get Cluster

Get detailed information about a specific cluster.

```http
GET /clusters/{id}
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Cluster UUID |

**Request Example:**

```bash
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/abc-123-def
```

**Response (200 OK):**

Returns the complete cluster object with aggregated status (same format as create response).

**Response (404 Not Found):**

```json
{
  "error": "Cluster not found"
}
```

### 4. Update Cluster

Update the cluster specification. This increments the generation counter.

```http
PUT /clusters/{id}
```

**Request Body:**

```json
{
  "spec": {
    "platform": {
      "type": "gcp",
      "gcp": {
        "projectID": "updated-project",
        "region": "us-west1"
      }
    }
  }
}
```

**Request Example:**

```bash
curl -X PUT http://localhost:8080/api/v1/clusters/abc-123-def \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "spec": {
      "platform": {
        "type": "gcp",
        "gcp": {
          "projectID": "my-project",
          "region": "us-west1"
        }
      }
    }
  }'
```

**Response (200 OK):**

Returns the updated cluster with incremented generation.

### 5. Delete Cluster

Delete a cluster. By default, only clusters in certain states can be deleted.

```http
DELETE /clusters/{id}
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `force` | boolean | false | Force delete regardless of state |

**Request Examples:**

```bash
# Standard delete
curl -X DELETE \
  -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/abc-123-def

# Force delete
curl -X DELETE \
  -H "X-User-Email: user@example.com" \
  "http://localhost:8080/api/v1/clusters/abc-123-def?force=true"
```

**Response (200 OK):**

```json
{
  "message": "Cluster deleted successfully"
}
```

**Response (409 Conflict):**

```json
{
  "error": "Cannot delete cluster in Ready state. Use force=true to override."
}
```

### 6. Get Cluster Status

Get detailed cluster status including individual controller status.

```http
GET /clusters/{id}/status
```

**Request Example:**

```bash
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/abc-123-def/status
```

**Response (200 OK):**

```json
{
  "cluster_id": "abc-123-def",
  "generation": 1,
  "status": {
    "observedGeneration": 1,
    "conditions": [
      {
        "type": "Ready",
        "status": "True",
        "lastTransitionTime": "2025-10-17T00:00:00Z",
        "reason": "AllControllersReady",
        "message": "All 3 controllers are ready"
      }
    ],
    "phase": "Ready",
    "message": "Cluster is ready with 3 controllers operational",
    "lastUpdateTime": "2025-10-17T00:00:00Z"
  },
  "controllers": [
    {
      "controller_name": "gcp-environment-validation",
      "observed_generation": 1,
      "conditions": [
        {
          "type": "Available",
          "status": "True",
          "lastTransitionTime": "2025-10-17T00:00:00Z",
          "reason": "ValidationCompleted",
          "message": "GCP environment validation completed successfully"
        }
      ],
      "metadata": {
        "platform": "gcp",
        "region": "us-central1"
      },
      "updated_at": "2025-10-17T00:00:00Z"
    }
  ]
}
```

### 7. Update Cluster Status (Controllers Only)

This endpoint is used by controllers to report their status.

```http
PUT /clusters/{id}/status
```

**Request Body:**

```json
{
  "controller_name": "string (required)",
  "observed_generation": "integer (required)",
  "conditions": [
    {
      "type": "Available|Ready|Progressing|Failed",
      "status": "True|False|Unknown",
      "lastTransitionTime": "2025-10-17T00:00:00Z",
      "reason": "string",
      "message": "string"
    }
  ],
  "metadata": {
    "platform": "string",
    "region": "string"
  },
  "last_error": {
    "code": "string",
    "message": "string",
    "timestamp": "2025-10-17T00:00:00Z"
  }
}
```

**Request Example:**

```bash
curl -X PUT http://localhost:8080/api/v1/clusters/abc-123-def/status \
  -H "Content-Type: application/json" \
  -H "X-User-Email: controller@system.local" \
  -d '{
    "controller_name": "gcp-environment-validation",
    "observed_generation": 1,
    "conditions": [
      {
        "type": "Available",
        "status": "True",
        "lastTransitionTime": "2025-10-17T00:00:00Z",
        "reason": "ValidationCompleted",
        "message": "GCP environment validation completed successfully"
      }
    ],
    "metadata": {
      "platform": "gcp",
      "region": "us-central1"
    }
  }'
```

**Response (200 OK):**

```json
{
  "message": "Controller status updated successfully"
}
```

## Utility Endpoints

### Health Check

Check service health including dependencies.

```http
GET /health
```

**Response (200 OK):**

```json
{
  "status": "healthy",
  "components": {
    "database": "healthy",
    "pubsub": "healthy"
  },
  "timestamp": "2025-10-17T00:00:00Z"
}
```

**Response (503 Service Unavailable):**

```json
{
  "status": "unhealthy",
  "components": {
    "database": "unhealthy",
    "pubsub": "healthy"
  },
  "timestamp": "2025-10-17T00:00:00Z"
}
```

### Service Information

Get service version and build information.

```http
GET /info
```

**Response (200 OK):**

```json
{
  "service": "cls-backend",
  "version": "v1.0.0",
  "git_commit": "a1b2c3d",
  "build_time": "2025-10-17T00:00:00Z",
  "api_version": "v1",
  "environment": "production"
}
```

### Metrics

Prometheus metrics endpoint (available on port 8081).

```http
GET /metrics
```

**Response (200 OK):**

```
# HELP cls_backend_clusters_total Total number of clusters
# TYPE cls_backend_clusters_total gauge
cls_backend_clusters_total 42

# HELP cls_backend_http_requests_total Total HTTP requests
# TYPE cls_backend_http_requests_total counter
cls_backend_http_requests_total{method="GET",path="/clusters",status="200"} 1234
```

## Error Handling

### HTTP Status Codes

| Code | Description | Common Causes |
|------|-------------|---------------|
| `200` | OK | Successful GET, PUT operations |
| `201` | Created | Successful POST operations |
| `400` | Bad Request | Invalid JSON, missing required fields, validation errors |
| `401` | Unauthorized | Missing X-User-Email header in production mode |
| `404` | Not Found | Cluster doesn't exist or not accessible to user |
| `409` | Conflict | Cluster name already exists, concurrent update conflicts |
| `500` | Internal Server Error | Database connection issues, internal errors |

### Error Response Format

All error responses follow this format:

```json
{
  "error": "Human-readable error message",
  "code": "MACHINE_READABLE_ERROR_CODE",
  "details": {
    "field": "field-specific-error",
    "value": "invalid-value"
  }
}
```

### Common Error Examples

#### 400 Bad Request

```json
{
  "error": "Invalid cluster specification",
  "code": "INVALID_SPEC",
  "details": {
    "field": "spec.platform.type",
    "value": "invalid-platform",
    "allowed": ["gcp", "aws", "azure"]
  }
}
```

#### 409 Conflict

```json
{
  "error": "Cluster with name 'my-cluster' already exists",
  "code": "CLUSTER_NAME_EXISTS",
  "details": {
    "field": "name",
    "value": "my-cluster"
  }
}
```

## Request/Response Headers

### Common Request Headers

```http
Content-Type: application/json
X-User-Email: user@example.com
User-Agent: my-client/1.0.0
```

### Common Response Headers

```http
Content-Type: application/json
X-Request-ID: 123e4567-e89b-12d3-a456-426614174000
X-Response-Time: 15ms
```

## Rate Limiting

### Limits

- **List Operations**: Maximum 100 results per request
- **Pagination**: Use `limit` and `offset` for large datasets
- **Default Page Size**: 50 items
- **Maximum Page Size**: 100 items

### Rate Limit Headers (Future)

```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1635724800
```

## Authentication and Authorization

### Development Mode

```bash
export DISABLE_AUTH=true
# No X-User-Email header required
```

### Production Mode

```bash
export DISABLE_AUTH=false
# X-User-Email header required from external authorization
```

### External Authorization Integration

The API is designed for external authorization systems:

- **User Context**: Provided via `X-User-Email` header
- **Ownership**: Tracked via `created_by` field in database
- **Filtering**: All operations automatically filtered by user
- **Future**: Ready for organization-based multi-tenancy

## SDK Examples

### cURL Examples

```bash
# List clusters
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters

# Create cluster
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{"name": "test", "spec": {"platform": {"type": "gcp"}}}'

# Get cluster
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/abc-123-def

# Update cluster
curl -X PUT http://localhost:8080/api/v1/clusters/abc-123-def \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{"spec": {"platform": {"type": "gcp"}}}'

# Delete cluster
curl -X DELETE \
  -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/abc-123-def
```

### JavaScript Example

```javascript
const apiClient = {
  baseURL: 'http://localhost:8080/api/v1',
  userEmail: 'user@example.com',

  async request(method, path, body = null) {
    const response = await fetch(`${this.baseURL}${path}`, {
      method,
      headers: {
        'Content-Type': 'application/json',
        'X-User-Email': this.userEmail,
      },
      body: body ? JSON.stringify(body) : null,
    });

    if (!response.ok) {
      throw new Error(`API Error: ${response.status}`);
    }

    return response.json();
  },

  // List clusters
  async listClusters(limit = 50, offset = 0) {
    return this.request('GET', `/clusters?limit=${limit}&offset=${offset}`);
  },

  // Create cluster
  async createCluster(cluster) {
    return this.request('POST', '/clusters', cluster);
  },

  // Get cluster
  async getCluster(id) {
    return this.request('GET', `/clusters/${id}`);
  },

  // Update cluster
  async updateCluster(id, spec) {
    return this.request('PUT', `/clusters/${id}`, { spec });
  },

  // Delete cluster
  async deleteCluster(id, force = false) {
    const query = force ? '?force=true' : '';
    return this.request('DELETE', `/clusters/${id}${query}`);
  },
};

// Usage
const clusters = await apiClient.listClusters();
const cluster = await apiClient.createCluster({
  name: 'my-cluster',
  spec: { platform: { type: 'gcp' } }
});
```

This API reference provides comprehensive documentation for integrating with CLS Backend's simplified single-tenant architecture.