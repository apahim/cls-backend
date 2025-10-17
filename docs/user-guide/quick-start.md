# Quick Start Guide

Get up and running with CLS Backend in 5 minutes.

## Prerequisites

- Go 1.21+
- PostgreSQL 13+
- Google Cloud Project with Pub/Sub enabled

## Installation

### 1. Clone and Build

```bash
git clone https://github.com/your-org/cls-backend.git
cd cls-backend
make build
```

### 2. Configure Environment

```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/cls_backend"
export GOOGLE_CLOUD_PROJECT="your-project-id"
export DISABLE_AUTH=true  # For local development only
```

### 3. Start the Service

```bash
./bin/backend-api
```

You should see output like:
```
INFO Starting CLS Backend server on :8080
INFO Database connection established
INFO Pub/Sub client initialized
```

## Basic Usage

### Health Check

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "healthy",
  "components": {
    "database": "healthy",
    "pubsub": "healthy"
  }
}
```

### Create Your First Cluster

```bash
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "name": "my-first-cluster",
    "spec": {
      "platform": {
        "type": "gcp",
        "gcp": {
          "projectID": "my-project",
          "region": "us-central1"
        }
      }
    }
  }'
```

Response:
```json
{
  "id": "abc-123-def",
  "name": "my-first-cluster",
  "generation": 1,
  "status": {
    "phase": "Pending",
    "conditions": [
      {
        "type": "Ready",
        "status": "False",
        "reason": "NoControllers",
        "message": "No controllers have reported status yet"
      }
    ]
  },
  "created_at": "2025-01-01T00:00:00Z"
}
```

### List Clusters

```bash
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters
```

### Get Cluster Details

```bash
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/abc-123-def
```

### Update Cluster

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

### Delete Cluster

```bash
curl -X DELETE \
  -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/abc-123-def
```

## Understanding Status

Clusters have Kubernetes-like status structures:

- **Phase**: `Pending` → `Progressing` → `Ready` (or `Failed`)
- **Conditions**: Array of detailed status conditions
- **Generation**: Version counter (increments on updates)

### Status Phases

| Phase | Description |
|-------|-------------|
| `Pending` | No controllers have reported status yet |
| `Progressing` | Some controllers working, not fully ready |
| `Ready` | All controllers operational |
| `Failed` | No controllers operational |

## Configuration

### Environment Variables

**Required:**
- `DATABASE_URL` - PostgreSQL connection string
- `GOOGLE_CLOUD_PROJECT` - GCP project ID

**Optional:**
- `DISABLE_AUTH=true` - Disable authentication (development only)
- `LOG_LEVEL=debug` - Set logging level
- `PORT=8080` - HTTP server port

### Authentication

In development, authentication is disabled with `DISABLE_AUTH=true`.

In production, all requests require the `X-User-Email` header:
```bash
X-User-Email: user@example.com
```

## Troubleshooting

### Service Won't Start

1. **Database connection failed**:
   - Verify `DATABASE_URL` is correct
   - Ensure PostgreSQL is running
   - Check database permissions

2. **Pub/Sub initialization failed**:
   - Verify `GOOGLE_CLOUD_PROJECT` is set
   - Check GCP authentication
   - Ensure Pub/Sub API is enabled

### API Returns 404

- Verify the service is running on the expected port
- Check the endpoint URL (use `/api/v1/clusters`, not organization-scoped)
- Ensure you're including the `X-User-Email` header

### API Returns Empty Results

- Check that you're using the same `X-User-Email` for create and list operations
- Verify clusters were created successfully (check the response)

## Next Steps

- Read the [API Usage Guide](api-usage.md) for more patterns
- Check the [Troubleshooting Guide](troubleshooting.md) for common issues
- See the [API Reference](../reference/api.md) for complete documentation
- Learn about [deployment](../deployment/) for production usage