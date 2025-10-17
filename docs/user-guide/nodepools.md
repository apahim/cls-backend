# NodePools User Guide

This guide covers working with NodePools in CLS Backend, which manage groups of compute nodes within clusters.

## Overview

NodePools are collections of worker nodes within a cluster that share common configuration. They provide the compute resources where your workloads run and can be scaled independently based on demand.

### Key Features

- **Independent Scaling**: Scale node pools independently based on workload requirements
- **Heterogeneous Configuration**: Different node types within the same cluster
- **Lifecycle Management**: Independent creation, updates, and deletion
- **Controller Integration**: Platform-specific controllers manage actual infrastructure

## API Endpoints

All nodepool operations use the simplified single-tenant API structure:

### Base URL
```
/api/v1/nodepools
```

### Authentication
Include your user email in the request header:
```
X-User-Email: user@example.com
```

## NodePool Operations

### 1. Create NodePool

Create a new nodepool within an existing cluster.

**Endpoint:** `POST /api/v1/clusters/{clusterId}/nodepools`

**Request Body:**
```json
{
  "name": "worker-pool-1",
  "cluster_id": "550e8400-e29b-41d4-a716-446655440000",
  "spec": {
    "platform": {
      "type": "gcp",
      "gcp": {
        "machineType": "n1-standard-4",
        "diskSize": 100,
        "diskType": "pd-ssd",
        "zones": ["us-central1-a", "us-central1-b"],
        "preemptible": false
      }
    },
    "autoscaling": {
      "enabled": true,
      "minNodes": 1,
      "maxNodes": 10
    },
    "nodeCount": 3
  }
}
```

**Response:**
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "name": "worker-pool-1",
  "cluster_id": "550e8400-e29b-41d4-a716-446655440000",
  "generation": 1,
  "resource_version": "abc123",
  "spec": { ... },
  "created_at": "2025-10-17T10:00:00Z",
  "updated_at": "2025-10-17T10:00:00Z",
  "created_by": "user@example.com"
}
```

### 2. List NodePools

List all nodepools with optional filtering and pagination.

**Endpoint:** `GET /api/v1/clusters/{clusterId}/nodepools`

**Query Parameters:**
- `limit` (int): Maximum number of results (1-100, default: 50)
- `offset` (int): Number of results to skip (default: 0)
- `status` (string): Filter by status phase
- `health` (string): Filter by health status

**Example:**
```bash
curl -H "X-User-Email: user@example.com" \
  "http://localhost:8080/api/v1/nodepools?limit=10&status=Ready"
```

**Response:**
```json
{
  "nodepools": [
    {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "name": "worker-pool-1",
      "cluster_id": "550e8400-e29b-41d4-a716-446655440000",
      "generation": 1,
      "spec": { ... },
      "created_at": "2025-10-17T10:00:00Z"
    }
  ],
  "total": 1,
  "limit": 10,
  "offset": 0
}
```

### 3. Get NodePool

Retrieve details of a specific nodepool.

**Endpoint:** `GET /api/v1/nodepools/{id}`

**Response:**
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "name": "worker-pool-1",
  "cluster_id": "550e8400-e29b-41d4-a716-446655440000",
  "generation": 2,
  "resource_version": "def456",
  "spec": {
    "platform": {
      "type": "gcp",
      "gcp": {
        "machineType": "n1-standard-4",
        "diskSize": 100,
        "diskType": "pd-ssd"
      }
    },
    "nodeCount": 5
  },
  "created_at": "2025-10-17T10:00:00Z",
  "updated_at": "2025-10-17T10:30:00Z",
  "created_by": "user@example.com"
}
```

### 4. Update NodePool

Update an existing nodepool specification.

**Endpoint:** `PUT /api/v1/nodepools/{id}`

**Request Body:**
```json
{
  "name": "worker-pool-1-updated",
  "spec": {
    "platform": {
      "type": "gcp",
      "gcp": {
        "machineType": "n1-standard-8",
        "diskSize": 200
      }
    },
    "nodeCount": 5
  }
}
```

**Response:** Updated nodepool object with incremented generation.

### 5. Delete NodePool

Delete a nodepool.

**Endpoint:** `DELETE /api/v1/nodepools/{id}`

**Response:** `204 No Content`

## NodePool Status

### Get NodePool Status

**Endpoint:** `GET /api/v1/nodepools/{id}/status`

**Response:**
```json
{
  "nodepool_id": "123e4567-e89b-12d3-a456-426614174000",
  "controller_status": [
    {
      "controller_name": "gcp-nodepool-controller",
      "observed_generation": 2,
      "conditions": [
        {
          "type": "Ready",
          "status": "True",
          "lastTransitionTime": "2025-10-17T10:30:00Z",
          "reason": "NodesReady",
          "message": "All 5 nodes are ready"
        }
      ],
      "metadata": {
        "platform": "gcp",
        "region": "us-central1",
        "active_nodes": 5,
        "desired_nodes": 5
      },
      "updated_at": "2025-10-17T10:30:00Z"
    }
  ]
}
```

### Update NodePool Status (Controllers Only)

Controllers use this endpoint to report nodepool status.

**Endpoint:** `PUT /api/v1/nodepools/{id}/status`

**Request Body:**
```json
{
  "controller_name": "gcp-nodepool-controller",
  "observed_generation": 2,
  "conditions": [
    {
      "type": "Ready",
      "status": "True",
      "lastTransitionTime": "2025-10-17T10:30:00Z",
      "reason": "NodesReady",
      "message": "All nodes are ready and healthy"
    }
  ],
  "metadata": {
    "platform": "gcp",
    "region": "us-central1",
    "active_nodes": 5,
    "desired_nodes": 5,
    "node_ages": "2h30m,1h45m,1h15m,45m,30m"
  }
}
```

## Platform-Specific Configuration

### Google Cloud Platform (GCP)

```json
{
  "spec": {
    "platform": {
      "type": "gcp",
      "gcp": {
        "machineType": "n1-standard-4",
        "diskSize": 100,
        "diskType": "pd-ssd",
        "zones": ["us-central1-a", "us-central1-b"],
        "preemptible": false,
        "accelerators": [
          {
            "type": "nvidia-tesla-v100",
            "count": 1
          }
        ],
        "labels": {
          "environment": "production",
          "team": "platform"
        },
        "taints": [
          {
            "key": "workload-type",
            "value": "gpu",
            "effect": "NoSchedule"
          }
        ]
      }
    }
  }
}
```

### Amazon Web Services (AWS)

```json
{
  "spec": {
    "platform": {
      "type": "aws",
      "aws": {
        "instanceType": "m5.xlarge",
        "volumeSize": 100,
        "volumeType": "gp3",
        "availabilityZones": ["us-west-2a", "us-west-2b"],
        "spotInstances": false,
        "userData": "#!/bin/bash\necho 'Custom initialization'"
      }
    }
  }
}
```

### Microsoft Azure

```json
{
  "spec": {
    "platform": {
      "type": "azure",
      "azure": {
        "vmSize": "Standard_D4s_v3",
        "osDiskSizeGB": 100,
        "osDiskType": "Premium_LRS",
        "availabilityZones": ["1", "2"],
        "spotInstances": false
      }
    }
  }
}
```

## Autoscaling Configuration

Configure automatic scaling based on resource usage:

```json
{
  "spec": {
    "autoscaling": {
      "enabled": true,
      "minNodes": 1,
      "maxNodes": 20,
      "targetCPUUtilization": 70,
      "targetMemoryUtilization": 80,
      "scaleDownDelay": "10m",
      "scaleUpDelay": "3m"
    }
  }
}
```

## Complete Example Workflow

Here's a complete example of creating and managing a nodepool:

### 1. Create the NodePool

```bash
curl -X POST "http://localhost:8080/api/v1/nodepools" \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "name": "production-workers",
    "cluster_id": "550e8400-e29b-41d4-a716-446655440000",
    "spec": {
      "platform": {
        "type": "gcp",
        "gcp": {
          "machineType": "n1-standard-4",
          "diskSize": 100,
          "diskType": "pd-ssd",
          "zones": ["us-central1-a", "us-central1-b"]
        }
      },
      "autoscaling": {
        "enabled": true,
        "minNodes": 2,
        "maxNodes": 10
      },
      "nodeCount": 3
    }
  }'
```

### 2. Monitor Status

```bash
# Check nodepool status
curl -H "X-User-Email: user@example.com" \
  "http://localhost:8080/api/v1/nodepools/123e4567-e89b-12d3-a456-426614174000/status"
```

### 3. Scale the NodePool

```bash
curl -X PUT "http://localhost:8080/api/v1/nodepools/123e4567-e89b-12d3-a456-426614174000" \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{
    "name": "production-workers",
    "spec": {
      "platform": {
        "type": "gcp",
        "gcp": {
          "machineType": "n1-standard-4",
          "diskSize": 100
        }
      },
      "nodeCount": 6
    }
  }'
```

## Error Handling

### Common Error Responses

**400 Bad Request** - Invalid request data:
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "details": "nodepool name is required"
  }
}
```

**404 Not Found** - NodePool not found:
```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "NodePool not found",
    "details": ""
  }
}
```

**409 Conflict** - Resource conflict:
```json
{
  "error": {
    "code": "CONFLICT",
    "message": "NodePool with name 'production-workers' already exists in cluster",
    "details": ""
  }
}
```

## Best Practices

### 1. Naming Convention
- Use descriptive names: `production-workers`, `gpu-pool`, `spot-instances`
- Include purpose or workload type in the name
- Avoid special characters and spaces

### 2. Resource Planning
- Start with smaller node counts and scale up as needed
- Use autoscaling for dynamic workloads
- Consider resource requirements (CPU, memory, storage)

### 3. High Availability
- Distribute nodes across multiple availability zones
- Use appropriate node counts for redundancy
- Plan for rolling updates and maintenance

### 4. Cost Optimization
- Use appropriate machine types for workloads
- Consider spot/preemptible instances for non-critical workloads
- Enable autoscaling to avoid over-provisioning

### 5. Security
- Apply appropriate taints and labels
- Use network policies for isolation
- Keep node images updated

## Monitoring and Troubleshooting

### 1. Check NodePool Health
Monitor the `/status` endpoint regularly to ensure nodes are healthy and ready.

### 2. Review Controller Status
Each platform controller reports status with specific conditions and metadata.

### 3. Common Issues
- **Nodes not ready**: Check platform quotas and resource availability
- **Scaling issues**: Verify autoscaling configuration and resource limits
- **Network problems**: Ensure proper VPC/subnet configuration

### 4. Log Analysis
Controllers provide detailed logs for troubleshooting platform-specific issues.

## Integration with External Tools

### kubectl Integration
Once nodes are ready, they appear in your Kubernetes cluster:

```bash
kubectl get nodes -l nodepool=production-workers
```

### Monitoring Tools
NodePools integrate with standard Kubernetes monitoring:
- Prometheus metrics for node status
- Grafana dashboards for visualization
- Alert manager for notifications

For more detailed information, see:
- [Platform-Specific Controller Documentation](../developer-guide/controllers.md)
- [Status and Monitoring Guide](./monitoring.md)
- [API Reference](../reference/api.md)