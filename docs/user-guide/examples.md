# API Usage Examples

This guide provides real-world examples and common patterns for using the CLS Backend API effectively.

## Common Workflows

### 1. Complete Cluster Lifecycle

This example shows a complete cluster lifecycle from creation to deletion.

#### Step 1: Create a GCP Cluster

```bash
# Create a basic GCP cluster
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "name": "production-cluster",
    "spec": {
      "platform": {
        "type": "gcp",
        "gcp": {
          "projectID": "my-gcp-project",
          "region": "us-central1",
          "zone": "us-central1-a"
        }
      },
      "networking": {
        "clusterNetwork": [{
          "cidr": "10.128.0.0/14",
          "hostPrefix": 23
        }],
        "serviceNetwork": ["172.30.0.0/16"]
      },
      "dns": {
        "baseDomain": "example.com"
      }
    }
  }'
```

**Expected Response:**
```json
{
  "id": "abc-123-def-456",
  "name": "production-cluster",
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
  "created_by": "user@example.com",
  "created_at": "2025-10-17T10:00:00Z"
}
```

#### Step 2: Monitor Cluster Status

```bash
# Check cluster status periodically
CLUSTER_ID="abc-123-def-456"

curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/$CLUSTER_ID/status
```

**Response (during provisioning):**
```json
{
  "cluster_id": "abc-123-def-456",
  "status": {
    "phase": "Progressing",
    "conditions": [
      {
        "type": "Ready",
        "status": "False",
        "reason": "ControllerProgressing",
        "message": "1 of 3 controllers are ready"
      }
    ]
  },
  "controllers": [
    {
      "controller_name": "gcp-environment-validation",
      "conditions": [
        {
          "type": "Available",
          "status": "True",
          "reason": "ValidationCompleted"
        }
      ]
    }
  ]
}
```

#### Step 3: Update Cluster Configuration

```bash
# Update cluster region
curl -X PUT http://localhost:8080/api/v1/clusters/$CLUSTER_ID \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "spec": {
      "platform": {
        "type": "gcp",
        "gcp": {
          "projectID": "my-gcp-project",
          "region": "us-west1",
          "zone": "us-west1-a"
        }
      }
    }
  }'
```

**Expected Response:**
```json
{
  "id": "abc-123-def-456",
  "name": "production-cluster",
  "generation": 2,
  "status": {
    "phase": "Progressing",
    "observedGeneration": 1,
    "conditions": [
      {
        "type": "Ready",
        "status": "Unknown",
        "reason": "GenerationMismatch",
        "message": "Controllers need to process generation 2"
      }
    ]
  }
}
```

#### Step 4: Clean Up

```bash
# Delete cluster
curl -X DELETE \
  -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/$CLUSTER_ID
```

### 2. Multi-Platform Cluster Management

Managing clusters across different cloud platforms.

#### AWS Cluster

```bash
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "name": "aws-cluster",
    "spec": {
      "platform": {
        "type": "aws",
        "aws": {
          "region": "us-east-1",
          "accountID": "123456789012"
        }
      },
      "networking": {
        "clusterNetwork": [{
          "cidr": "10.0.0.0/16",
          "hostPrefix": 24
        }],
        "serviceNetwork": ["172.31.0.0/16"]
      }
    }
  }'
```

#### Azure Cluster

```bash
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "name": "azure-cluster",
    "spec": {
      "platform": {
        "type": "azure",
        "azure": {
          "subscriptionID": "12345678-1234-1234-1234-123456789012",
          "resourceGroup": "my-resource-group",
          "region": "eastus"
        }
      }
    }
  }'
```

### 3. Batch Operations

Working with multiple clusters efficiently.

#### List All Clusters with Filtering

```bash
# List all clusters
curl -H "X-User-Email: user@example.com" \
  "http://localhost:8080/api/v1/clusters?limit=50"

# Filter by platform
curl -H "X-User-Email: user@example.com" \
  "http://localhost:8080/api/v1/clusters?platform=gcp&limit=20"

# Filter by status phase
curl -H "X-User-Email: user@example.com" \
  "http://localhost:8080/api/v1/clusters?status=Ready"
```

#### Batch Status Check

```bash
#!/bin/bash
# Check status of all clusters

USER_EMAIL="user@example.com"
BASE_URL="http://localhost:8080/api/v1"

# Get all cluster IDs
CLUSTER_IDS=$(curl -s -H "X-User-Email: $USER_EMAIL" \
  "$BASE_URL/clusters" | \
  jq -r '.clusters[].id')

# Check status of each cluster
for CLUSTER_ID in $CLUSTER_IDS; do
  echo "Cluster: $CLUSTER_ID"
  curl -s -H "X-User-Email: $USER_EMAIL" \
    "$BASE_URL/clusters/$CLUSTER_ID/status" | \
    jq '.status.phase'
  echo ""
done
```

## Error Handling Examples

### 1. Handling Validation Errors

```bash
# Invalid platform type
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "name": "invalid-cluster",
    "spec": {
      "platform": {
        "type": "invalid-platform"
      }
    }
  }'
```

**Error Response (400 Bad Request):**
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

### 2. Handling Conflicts

```bash
# Try to create cluster with duplicate name
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "name": "production-cluster",
    "spec": {"platform": {"type": "gcp"}}
  }'
```

**Error Response (409 Conflict):**
```json
{
  "error": "Cluster with name 'production-cluster' already exists",
  "code": "CLUSTER_NAME_EXISTS",
  "details": {
    "field": "name",
    "value": "production-cluster"
  }
}
```

### 3. Handling Not Found

```bash
# Try to access non-existent cluster
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/non-existent-id
```

**Error Response (404 Not Found):**
```json
{
  "error": "Cluster not found"
}
```

## Production Patterns

### 1. Health Monitoring Script

```bash
#!/bin/bash
# Production health monitoring script

set -e

USER_EMAIL="production@company.com"
BASE_URL="https://your-api-gateway-url/api/v1"
WEBHOOK_URL="https://your-alerting-webhook"

# Check service health
HEALTH=$(curl -s "$BASE_URL/../health" | jq -r '.status')

if [ "$HEALTH" != "healthy" ]; then
  curl -X POST "$WEBHOOK_URL" \
    -d "Service unhealthy: $HEALTH"
  exit 1
fi

# Check cluster status
FAILED_CLUSTERS=$(curl -s -H "X-User-Email: $USER_EMAIL" \
  "$BASE_URL/clusters?status=Failed" | \
  jq -r '.total')

if [ "$FAILED_CLUSTERS" -gt 0 ]; then
  curl -X POST "$WEBHOOK_URL" \
    -d "Alert: $FAILED_CLUSTERS clusters in Failed state"
fi

echo "Health check completed successfully"
```

### 2. Automated Cluster Creation

```bash
#!/bin/bash
# Automated cluster creation with retry logic

create_cluster() {
  local name=$1
  local region=$2
  local project=$3

  local payload=$(cat <<EOF
{
  "name": "$name",
  "spec": {
    "platform": {
      "type": "gcp",
      "gcp": {
        "projectID": "$project",
        "region": "$region"
      }
    }
  }
}
EOF
)

  # Create cluster with retry
  for i in {1..3}; do
    if response=$(curl -s -X POST "$BASE_URL/clusters" \
      -H "Content-Type: application/json" \
      -H "X-User-Email: $USER_EMAIL" \
      -d "$payload"); then

      cluster_id=$(echo "$response" | jq -r '.id')
      if [ "$cluster_id" != "null" ]; then
        echo "Created cluster: $cluster_id"
        return 0
      fi
    fi

    echo "Attempt $i failed, retrying..."
    sleep 5
  done

  echo "Failed to create cluster after 3 attempts"
  return 1
}

# Create multiple clusters
create_cluster "staging-cluster" "us-central1" "staging-project"
create_cluster "prod-cluster" "us-west1" "production-project"
```

### 3. Status Polling with Timeout

```bash
#!/bin/bash
# Wait for cluster to become ready with timeout

wait_for_ready() {
  local cluster_id=$1
  local timeout=${2:-300}  # 5 minutes default
  local start_time=$(date +%s)

  while true; do
    current_time=$(date +%s)
    elapsed=$((current_time - start_time))

    if [ $elapsed -gt $timeout ]; then
      echo "Timeout waiting for cluster to become ready"
      return 1
    fi

    status=$(curl -s -H "X-User-Email: $USER_EMAIL" \
      "$BASE_URL/clusters/$cluster_id/status" | \
      jq -r '.status.phase')

    case "$status" in
      "Ready")
        echo "Cluster is ready!"
        return 0
        ;;
      "Failed")
        echo "Cluster failed to provision"
        return 1
        ;;
      "Pending"|"Progressing")
        echo "Cluster status: $status (${elapsed}s elapsed)"
        sleep 10
        ;;
      *)
        echo "Unknown status: $status"
        sleep 10
        ;;
    esac
  done
}

# Usage
CLUSTER_ID="your-cluster-id"
wait_for_ready "$CLUSTER_ID" 600  # 10 minutes timeout
```

## API Client Examples

### Python Client

```python
import requests
import json
import time

class CLSClient:
    def __init__(self, base_url, user_email):
        self.base_url = base_url
        self.headers = {
            'Content-Type': 'application/json',
            'X-User-Email': user_email
        }

    def create_cluster(self, name, spec):
        payload = {'name': name, 'spec': spec}
        response = requests.post(
            f'{self.base_url}/clusters',
            headers=self.headers,
            json=payload
        )
        response.raise_for_status()
        return response.json()

    def get_cluster(self, cluster_id):
        response = requests.get(
            f'{self.base_url}/clusters/{cluster_id}',
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()

    def wait_for_ready(self, cluster_id, timeout=300):
        start_time = time.time()
        while time.time() - start_time < timeout:
            cluster = self.get_cluster(cluster_id)
            phase = cluster['status']['phase']

            if phase == 'Ready':
                return True
            elif phase == 'Failed':
                raise Exception('Cluster failed')

            time.sleep(10)

        raise Exception('Timeout waiting for cluster')

# Usage
client = CLSClient('http://localhost:8080/api/v1', 'user@example.com')

# Create cluster
cluster = client.create_cluster('test-cluster', {
    'platform': {'type': 'gcp'}
})

# Wait for ready
client.wait_for_ready(cluster['id'])
print(f"Cluster {cluster['id']} is ready!")
```

### JavaScript Client

```javascript
class CLSClient {
  constructor(baseUrl, userEmail) {
    this.baseUrl = baseUrl;
    this.headers = {
      'Content-Type': 'application/json',
      'X-User-Email': userEmail
    };
  }

  async request(method, path, body = null) {
    const response = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers: this.headers,
      body: body ? JSON.stringify(body) : null
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(`API Error: ${error.error}`);
    }

    return response.json();
  }

  async createCluster(name, spec) {
    return this.request('POST', '/clusters', { name, spec });
  }

  async getCluster(clusterId) {
    return this.request('GET', `/clusters/${clusterId}`);
  }

  async waitForReady(clusterId, timeout = 300000) {
    const startTime = Date.now();

    while (Date.now() - startTime < timeout) {
      const cluster = await this.getCluster(clusterId);
      const phase = cluster.status.phase;

      if (phase === 'Ready') return true;
      if (phase === 'Failed') throw new Error('Cluster failed');

      await new Promise(resolve => setTimeout(resolve, 10000));
    }

    throw new Error('Timeout waiting for cluster');
  }
}

// Usage
const client = new CLSClient('http://localhost:8080/api/v1', 'user@example.com');

async function main() {
  const cluster = await client.createCluster('test-cluster', {
    platform: { type: 'gcp' }
  });

  await client.waitForReady(cluster.id);
  console.log(`Cluster ${cluster.id} is ready!`);
}
```

## Best Practices

### 1. Error Handling
- Always check HTTP status codes
- Parse error responses for detailed information
- Implement retry logic for transient failures
- Use exponential backoff for rate limiting

### 2. Authentication
- Always include `X-User-Email` header in production
- Use service accounts for automated operations
- Rotate credentials regularly

### 3. Polling and Timeouts
- Use reasonable polling intervals (10-30 seconds)
- Set appropriate timeouts for operations
- Implement circuit breakers for resilience

### 4. Resource Management
- Clean up test clusters to avoid resource leaks
- Use descriptive naming conventions
- Tag clusters with metadata for tracking

## Related Documentation

- **[Quick Start](quick-start.md)** - Basic setup and first cluster
- **[API Usage](api-usage.md)** - Core API patterns and concepts
- **[API Reference](../reference/api.md)** - Complete API specification
- **[Troubleshooting](troubleshooting.md)** - Common issues and solutions