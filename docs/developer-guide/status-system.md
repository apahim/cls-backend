# Status System

This document explains the Kubernetes-like status system used in CLS Backend, including status aggregation, generation tracking, and the hybrid caching architecture.

## Overview

CLS Backend implements a sophisticated **Kubernetes-like status system** that aggregates status from multiple controllers into a unified cluster health view.

**Key Features:**
- **Kubernetes-like Structure**: Conditions, phases, and observed generation
- **Hybrid Calculation**: Lazy calculation with intelligent caching
- **Generation Aware**: Prevents stale status from affecting cluster state
- **Always Accurate**: Impossible to have stale data due to dirty tracking

## Status Structure

### Kubernetes-like Status Format

Clusters have a `status` field at the same level as `spec`:

```json
{
  "id": "cluster-id",
  "name": "my-cluster",
  "spec": { /* cluster specification */ },
  "status": {
    "observedGeneration": 2,
    "conditions": [
      {
        "type": "Ready",
        "status": "True",
        "lastTransitionTime": "2025-10-17T00:00:00Z",
        "reason": "AllControllersReady",
        "message": "All 3 controllers are ready"
      },
      {
        "type": "Available",
        "status": "True",
        "lastTransitionTime": "2025-10-17T00:00:00Z",
        "reason": "AllControllersReady",
        "message": "All 3 controllers are available"
      }
    ],
    "phase": "Ready",
    "message": "Cluster is ready with 3 controllers operational",
    "reason": "AllControllersReady",
    "lastUpdateTime": "2025-10-17T00:00:00Z"
  }
}
```

### Status Fields

#### Core Conditions (Always Present)

**Ready Condition**: *"Is the cluster ready to serve requests?"*
- `type`: "Ready"
- `status`: "True" | "False"
- `reason`: Indicates why the condition is in this state

**Available Condition**: *"Is the cluster available for use?"*
- `type`: "Available"
- `status`: "True" | "False"
- `reason`: Explains the availability status

#### Status Phases

Top-level operational states:

| Phase | Description |
|-------|-------------|
| `Pending` | No controllers have reported status yet |
| `Progressing` | Some controllers are working, but cluster isn't fully ready |
| `Ready` | All controllers have completed their work successfully |
| `Failed` | No controllers are working/ready |

#### Additional Fields

- **`observedGeneration`**: The cluster generation that was last processed
- **`message`**: Human-readable summary of current state
- **`reason`**: Machine-readable reason for the current phase
- **`lastUpdateTime`**: When the status was last calculated

## Hybrid Status Architecture

CLS Backend uses a **hybrid approach** that balances performance with accuracy:

```
GET /clusters/{id} → Check status_dirty → Return cached or calculate fresh
                           ↓
                   If dirty (status_dirty = true):
                   1. Read cluster from DB
                   2. Query controller status for current generation
                   3. Apply aggregation logic in Go
                   4. Build K8s-like status structure
                   5. Cache status in DB (mark clean)
                   6. Return enriched cluster data

                   If clean (status_dirty = false):
                   1. Read cluster with cached status from DB
                   2. Return cluster data immediately (fast path)
```

### Performance Benefits

- ✅ **Fast reads** when status is clean (cached) - <1ms response time
- ✅ **Accurate data** (real-time calculation when controllers update)
- ✅ **Resource efficient** (only calculates when necessary) - ~5-10ms when dirty
- ✅ **Always current** (impossible to have stale data due to dirty tracking)

## Status Aggregation Logic

### Controller Status Input

Controllers communicate state through **conditions**:

| Condition Type | Status | Meaning |
|----------------|--------|---------|
| `Available` | `True` | Controller has completed its work successfully |
| `Available` | `False` | Controller work is incomplete or failed |
| `Ready` | `True` | Controller is ready to handle requests |
| `Ready` | `False` | Controller is not ready |

**Key Rule**: A controller is considered "ready" for aggregation if it has `Available: True` condition.

### Aggregation Decision Tree

```go
// Aggregation logic (for current generation only)
func calculateStatus(cluster Cluster, controllers []ControllerStatus) ClusterStatus {
    currentGeneration := cluster.Generation

    // Count controllers at current generation only
    var totalCount, readyCount, errorCount int
    for _, controller := range controllers {
        if controller.ObservedGeneration != currentGeneration {
            continue // Skip stale controller status
        }
        totalCount++
        if hasAvailableTrue(controller.Conditions) {
            readyCount++
        }
        if controller.LastError != nil {
            errorCount++
        }
    }

    // Apply aggregation rules
    if totalCount == 0 {
        return buildStatus("Pending", "NoControllers",
            "No controllers have reported status yet")
    }

    if readyCount == totalCount && errorCount == 0 {
        return buildStatus("Ready", "AllControllersReady",
            fmt.Sprintf("All %d controllers are ready", totalCount))
    }

    if readyCount > 0 {
        reason := "PartialProgress"
        if errorCount > 0 {
            reason = "ControllersWithErrors"
        }
        return buildStatus("Progressing", reason,
            fmt.Sprintf("%d of %d controllers ready", readyCount, totalCount))
    }

    return buildStatus("Failed", "NoControllersReady",
        fmt.Sprintf("None of %d controllers are ready", totalCount))
}
```

### Condition Reasons

#### Ready Condition Reasons

| Reason | When Used | Description |
|--------|-----------|-------------|
| `AllControllersReady` | All controllers ready | All controllers have reported ready status |
| `PartialProgress` | Some controllers ready | Some controllers are ready, others still working |
| `NoControllersReady` | No controllers ready | No controllers have achieved ready status |
| `NoControllers` | No controllers exist | No controllers have reported status yet |

#### Available Condition Reasons

| Reason | When Used | Description |
|--------|-----------|-------------|
| `AllControllersReady` | All controllers available | All controllers are available and operational |
| `PartialProgress` | Some controllers available | Some controllers available, others becoming available |
| `ControllersWithErrors` | Available but errors exist | Some controllers available but error conditions exist |
| `NoControllersReady` | No controllers available | No controllers are available yet |
| `NoControllers` | No controllers exist | No controllers have reported status yet |

## Generation-Aware Aggregation

Status aggregation is **generation-aware** to prevent stale status from affecting cluster state.

### The Problem Without Generation Filtering

When a cluster spec is updated:
1. Cluster's `generation` field increments (1 → 2 → 3...)
2. Controllers start working on the new generation
3. Old controller status records remain in database
4. Without filtering, aggregation would mix old and new status
5. This could show misleading "Ready" when controllers are still working

### How Generation Filtering Works

```sql
-- Example scenario
-- Cluster generation: 2 (just updated)
-- Controller status in database:

controller_name                | observed_generation | available
gcp-environment-validation     | 1                   | True      -- OLD (stale)
gcp-environment-validation     | 2                   | False     -- NEW (working)
network-setup                  | 1                   | True      -- OLD (stale)

-- Aggregation result:
-- Without generation filtering: 2 total, 2 ready → "Ready" (WRONG!)
-- With generation filtering:    1 total, 0 ready → "Progressing" (CORRECT)
```

### Controller Implementation Requirements

Controllers must properly handle generation:

1. **Update observed_generation** when receiving cluster events
2. **Include observed_generation** in all status reports
3. **Work on current generation** and not rely on cached cluster data

```go
// Example controller status update
func (c *Controller) reportStatus(clusterID string, generation int64) {
    status := ControllerStatus{
        ClusterID:           clusterID,
        ControllerName:      c.name,
        ObservedGeneration:  generation, // CRITICAL: Match cluster generation
        Conditions: []Condition{
            {
                Type:    "Available",
                Status:  "True",
                Reason:  "WorkCompleted",
                Message: "Controller work completed successfully",
            },
        },
    }
    c.statusAPI.UpdateStatus(status)
}
```

## Status Scenarios

### Scenario 1: All Controllers Ready ✅

```
Total Controllers: 3
Ready Controllers: 3
Errors: 0
```

**Result:**
```json
{
  "phase": "Ready",
  "conditions": [
    {
      "type": "Ready",
      "status": "True",
      "reason": "AllControllersReady",
      "message": "All 3 controllers are ready"
    },
    {
      "type": "Available",
      "status": "True",
      "reason": "AllControllersReady",
      "message": "All 3 controllers are available"
    }
  ],
  "message": "Cluster is ready with 3 controllers operational"
}
```

### Scenario 2: Partial Progress, No Issues 🔄

```
Total Controllers: 3
Ready Controllers: 1
Errors: 0
```

**Result:**
```json
{
  "phase": "Progressing",
  "conditions": [
    {
      "type": "Ready",
      "status": "False",
      "reason": "PartialProgress",
      "message": "1 of 3 controllers are ready"
    },
    {
      "type": "Available",
      "status": "False",
      "reason": "PartialProgress",
      "message": "Controllers are still working (1 ready of 3)"
    }
  ],
  "message": "Cluster is progressing (1/3 controllers ready)"
}
```

### Scenario 3: Progress with Errors ⚠️

```
Total Controllers: 3
Ready Controllers: 1
Errors: 1
```

**Result:**
```json
{
  "phase": "Progressing",
  "conditions": [
    {
      "type": "Ready",
      "status": "False",
      "reason": "PartialProgress",
      "message": "1 of 3 controllers are ready"
    },
    {
      "type": "Available",
      "status": "False",
      "reason": "ControllersWithErrors",
      "message": "Some controllers have errors (1 ready of 3)"
    }
  ],
  "message": "Cluster is progressing but some controllers have errors (1/3 ready)"
}
```

### Scenario 4: Complete Failure ❌

```
Total Controllers: 3
Ready Controllers: 0
Errors: 2
```

**Result:**
```json
{
  "phase": "Failed",
  "conditions": [
    {
      "type": "Ready",
      "status": "False",
      "reason": "NoControllersReady",
      "message": "None of 3 controllers are ready"
    },
    {
      "type": "Available",
      "status": "False",
      "reason": "NoControllersReady",
      "message": "None of 3 controllers are available"
    }
  ],
  "message": "Cluster failed - no controllers are operational (3 controllers exist)"
}
```

### Scenario 5: No Controllers Yet ⏳

```
Total Controllers: 0
```

**Result:**
```json
{
  "phase": "Pending",
  "conditions": [
    {
      "type": "Ready",
      "status": "False",
      "reason": "NoControllers",
      "message": "No controllers have reported status yet"
    },
    {
      "type": "Available",
      "status": "False",
      "reason": "NoControllers",
      "message": "No controllers have reported status yet"
    }
  ],
  "message": "Waiting for controllers to report status"
}
```

## Database Schema

### Cluster Status Fields

```sql
-- Clusters table
ALTER TABLE clusters ADD COLUMN status JSONB;              -- Cached K8s-like status
ALTER TABLE clusters ADD COLUMN status_dirty BOOLEAN DEFAULT true; -- Dirty flag
```

### Controller Status Table

```sql
CREATE TABLE cluster_controller_status (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    controller_name VARCHAR(255) NOT NULL,
    observed_generation BIGINT NOT NULL DEFAULT 1,    -- Generation tracking
    conditions JSONB NOT NULL DEFAULT '[]',           -- K8s-like conditions
    metadata JSONB NOT NULL DEFAULT '{}',
    last_error JSONB,                                 -- Error tracking
    last_updated TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(cluster_id, controller_name)
);
```

### Dirty Tracking Mechanism

```sql
-- Automatic trigger to mark status dirty when controllers update
CREATE OR REPLACE FUNCTION mark_cluster_status_dirty()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE clusters
    SET status_dirty = true, updated_at = NOW()
    WHERE id = NEW.cluster_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER cluster_controller_status_dirty_trigger
    AFTER INSERT OR UPDATE ON cluster_controller_status
    FOR EACH ROW
    EXECUTE FUNCTION mark_cluster_status_dirty();
```

## Implementation Guide

### Controller Status Reporting

Controllers should report status using this pattern:

```go
// Controller status update
func (c *MyController) UpdateStatus(clusterID string, generation int64) error {
    status := ControllerStatus{
        ClusterID:          clusterID,
        ControllerName:     "my-controller",
        ObservedGeneration: generation, // Critical: match cluster generation
        Conditions: []Condition{
            {
                Type:               "Available",
                Status:             "True", // or "False"
                LastTransitionTime: time.Now(),
                Reason:             "WorkCompleted",
                Message:            "Controller completed successfully",
            },
        },
        Metadata: map[string]interface{}{
            "platform": "gcp",
            "region":   "us-central1",
        },
    }

    return c.statusClient.UpdateControllerStatus(status)
}
```

### Status API Endpoints

```bash
# Get cluster with aggregated status
GET /api/v1/clusters/{id}

# Get detailed controller status (for debugging)
GET /api/v1/clusters/{id}/status

# Update controller status (controllers only)
PUT /api/v1/clusters/{id}/status
```

**Controller Access**: Controllers using `@system.local` email addresses have privileged access and can read/update status for all clusters, bypassing normal client isolation.

### Debugging Status Issues

#### Check Individual Controller Status

```bash
# Get raw controller status data
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/{cluster_id}/status
```

#### Verify Generation Matching

```sql
-- Check current cluster generation
SELECT id, name, generation, status_dirty FROM clusters WHERE id = 'cluster-uuid';

-- Check controller status for current generation
SELECT controller_name, observed_generation, conditions, last_error
FROM cluster_controller_status
WHERE cluster_id = 'cluster-uuid'
  AND observed_generation = 2;  -- replace with current generation
```

#### Force Status Recalculation

```sql
-- Manually mark status as dirty to force recalculation
UPDATE clusters SET status_dirty = true WHERE id = 'cluster-uuid';
```

## Performance Considerations

### Database Optimization

- **Indexes**: Proper indexes on `cluster_id` and `observed_generation`
- **JSONB Performance**: PostgreSQL optimized JSON storage for conditions
- **Connection Pooling**: Efficient database connections for status queries

### Caching Strategy

- **Lazy Calculation**: Status calculated only when needed
- **Dirty Tracking**: Avoids unnecessary recalculations
- **Fast Path**: Cached status served in <1ms
- **Calculation Path**: Fresh calculation in ~5-10ms

### Monitoring

```sql
-- Monitor status calculation frequency
SELECT
    COUNT(*) as total_clusters,
    SUM(CASE WHEN status_dirty THEN 1 ELSE 0 END) as dirty_clusters,
    ROUND(AVG(CASE WHEN status_dirty THEN 0 ELSE 1 END) * 100, 2) as cache_hit_rate
FROM clusters;
```

## Best Practices

### For Controller Developers

1. **Always Update Generation**: Set `observed_generation` to match cluster generation
2. **Report Available Condition**: Use `Available: True` when work is complete
3. **Include Error Information**: Set `last_error` for debugging
4. **Use Meaningful Messages**: Help operators understand status

### For API Users

1. **Use Hybrid Status**: Included in all GET cluster responses
2. **Check Conditions**: Look at individual conditions for details
3. **Monitor Phases**: Use phase for high-level state decisions
4. **Debug with Raw Status**: Use `/status` endpoint for detailed investigation

### For Operators

1. **Monitor Cache Hit Rate**: Track `status_dirty` flag usage
2. **Index Performance**: Ensure proper database indexes
3. **Generation Alignment**: Verify controllers report correct generation
4. **Error Tracking**: Monitor controller `last_error` fields

The status system provides rich, accurate, and performant cluster state information that follows Kubernetes conventions while optimizing for the specific needs of cluster lifecycle management.