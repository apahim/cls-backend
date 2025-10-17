# API Usage Guide

This guide covers common patterns and best practices for using the CLS Backend API.

## Authentication

All API requests require the `X-User-Email` header to provide user context:

```bash
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters
```

## Cluster Lifecycle

### Creating Clusters

#### Basic Cluster

```bash
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "name": "basic-cluster",
    "spec": {
      "platform": {
        "type": "gcp"
      }
    }
  }'
```

#### Detailed GCP Cluster

```bash
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "name": "production-cluster",
    "target_project_id": "my-gcp-project",
    "spec": {
      "infraID": "prod-cluster-infra",
      "platform": {
        "type": "gcp",
        "gcp": {
          "projectID": "my-gcp-project",
          "region": "us-central1",
          "zone": "us-central1-a"
        }
      },
      "release": {
        "image": "quay.io/openshift-release-dev/ocp-release:4.14.0",
        "version": "4.14.0"
      },
      "networking": {
        "clusterNetwork": [
          {"cidr": "10.128.0.0/14", "hostPrefix": 23}
        ],
        "serviceNetwork": ["172.30.0.0/16"]
      },
      "dns": {
        "baseDomain": "example.com"
      }
    }
  }'
```

### Listing Clusters

#### Basic List

```bash
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters
```

#### Paginated List

```bash
curl -H "X-User-Email: user@example.com" \
  "http://localhost:8080/api/v1/clusters?limit=10&offset=20"
```

#### Filtered List

```bash
# Filter by platform
curl -H "X-User-Email: user@example.com" \
  "http://localhost:8080/api/v1/clusters?platform=gcp"

# Filter by status
curl -H "X-User-Email: user@example.com" \
  "http://localhost:8080/api/v1/clusters?status=Ready"
```

### Updating Clusters

Updates increment the generation counter and create a new resource version:

```bash
curl -X PUT http://localhost:8080/api/v1/clusters/{cluster-id} \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "spec": {
      "platform": {
        "type": "gcp",
        "gcp": {
          "region": "us-west1"
        }
      }
    }
  }'
```

### Deleting Clusters

```bash
# Standard delete (requires cluster in Pending or Error state)
curl -X DELETE \
  -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/{cluster-id}

# Force delete (ignores cluster state)
curl -X DELETE \
  -H "X-User-Email: user@example.com" \
  "http://localhost:8080/api/v1/clusters/{cluster-id}?force=true"
```

## Status Monitoring

### Getting Cluster Status

```bash
# Full cluster details (includes status)
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/{cluster-id}

# Status endpoint only
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/{cluster-id}/status
```

### Understanding Status Response

```json
{
  "status": {
    "observedGeneration": 2,
    "conditions": [
      {
        "type": "Ready",
        "status": "True",
        "lastTransitionTime": "2025-01-01T00:00:00Z",
        "reason": "AllControllersReady",
        "message": "All 3 controllers are ready"
      },
      {
        "type": "Available",
        "status": "True",
        "lastTransitionTime": "2025-01-01T00:00:00Z",
        "reason": "AllControllersReady",
        "message": "All 3 controllers are available"
      }
    ],
    "phase": "Ready",
    "message": "Cluster is ready with 3 controllers operational",
    "reason": "AllControllersReady",
    "lastUpdateTime": "2025-01-01T00:00:00Z"
  }
}
```

## Error Handling

### HTTP Status Codes

| Code | Description | Example |
|------|-------------|---------|
| 200 | Success | GET, PUT operations |
| 201 | Created | POST operations |
| 400 | Bad Request | Invalid JSON, missing required fields |
| 404 | Not Found | Cluster doesn't exist |
| 409 | Conflict | Cluster name already exists |
| 500 | Server Error | Database connection issues |

### Error Response Format

```json
{
  "error": "Cluster with name 'my-cluster' already exists"
}
```

## Best Practices

### Naming Conventions

- Use descriptive cluster names
- Include environment indicators (dev, staging, prod)
- Avoid special characters except hyphens
- Keep names under 50 characters

Examples:
```
production-web-cluster
staging-api-cluster-v2
dev-john-test-cluster
```

### Polling for Status

When waiting for cluster operations to complete:

```bash
#!/bin/bash
CLUSTER_ID="your-cluster-id"
USER_EMAIL="user@example.com"

while true; do
  STATUS=$(curl -s -H "X-User-Email: $USER_EMAIL" \
    "http://localhost:8080/api/v1/clusters/$CLUSTER_ID" | \
    jq -r '.status.phase')

  echo "Current status: $STATUS"

  if [[ "$STATUS" == "Ready" ]]; then
    echo "Cluster is ready!"
    break
  elif [[ "$STATUS" == "Failed" ]]; then
    echo "Cluster failed to deploy"
    exit 1
  fi

  sleep 30
done
```

### Pagination

For large cluster lists, always use pagination:

```bash
#!/bin/bash
USER_EMAIL="user@example.com"
LIMIT=50
OFFSET=0

while true; do
  RESPONSE=$(curl -s -H "X-User-Email: $USER_EMAIL" \
    "http://localhost:8080/api/v1/clusters?limit=$LIMIT&offset=$OFFSET")

  CLUSTERS=$(echo "$RESPONSE" | jq '.clusters | length')
  TOTAL=$(echo "$RESPONSE" | jq '.total')

  # Process clusters here
  echo "Processing clusters $OFFSET to $((OFFSET + CLUSTERS))"

  OFFSET=$((OFFSET + LIMIT))
  if [[ $OFFSET -ge $TOTAL ]]; then
    break
  fi
done
```

## Common Patterns

### Batch Operations

```bash
#!/bin/bash
# Create multiple clusters
USER_EMAIL="user@example.com"

for i in {1..5}; do
  curl -X POST http://localhost:8080/api/v1/clusters \
    -H "Content-Type: application/json" \
    -H "X-User-Email: $USER_EMAIL" \
    -d "{
      \"name\": \"batch-cluster-$i\",
      \"spec\": {
        \"platform\": {
          \"type\": \"gcp\"
        }
      }
    }"
  sleep 1  # Rate limiting
done
```

### Health Monitoring

```bash
#!/bin/bash
# Monitor service health
while true; do
  HEALTH=$(curl -s http://localhost:8080/health | jq -r '.status')
  echo "$(date): Service health: $HEALTH"

  if [[ "$HEALTH" != "healthy" ]]; then
    echo "Service unhealthy, sending alert..."
    # Add alerting logic here
  fi

  sleep 60
done
```

## Integration Examples

### Shell Script Wrapper

```bash
#!/bin/bash
# cls-client.sh - Simple CLI wrapper

API_BASE="http://localhost:8080/api/v1"
USER_EMAIL="${CLS_USER_EMAIL:-user@example.com}"

case "$1" in
  "list")
    curl -H "X-User-Email: $USER_EMAIL" "$API_BASE/clusters"
    ;;
  "create")
    curl -X POST -H "Content-Type: application/json" \
         -H "X-User-Email: $USER_EMAIL" \
         -d "{\"name\": \"$2\", \"spec\": {\"platform\": {\"type\": \"gcp\"}}}" \
         "$API_BASE/clusters"
    ;;
  "get")
    curl -H "X-User-Email: $USER_EMAIL" "$API_BASE/clusters/$2"
    ;;
  "delete")
    curl -X DELETE -H "X-User-Email: $USER_EMAIL" "$API_BASE/clusters/$2"
    ;;
  *)
    echo "Usage: $0 {list|create|get|delete} [cluster-name|cluster-id]"
    exit 1
    ;;
esac
```

Usage:
```bash
export CLS_USER_EMAIL="user@example.com"
./cls-client.sh list
./cls-client.sh create my-cluster
./cls-client.sh get abc-123-def
./cls-client.sh delete abc-123-def
```

## Troubleshooting

### Common Issues

1. **401 Unauthorized**: Missing `X-User-Email` header
2. **404 Not Found**: Wrong endpoint URL or cluster doesn't exist
3. **409 Conflict**: Cluster name already exists
4. **Empty response**: Using wrong user email context

### Debug Requests

Add `-v` flag to curl for detailed request/response information:

```bash
curl -v -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters
```

## Next Steps

- Check the [Troubleshooting Guide](troubleshooting.md) for specific issues
- See the [API Reference](../reference/api.md) for complete endpoint documentation
- Learn about [deployment](../deployment/) for production usage