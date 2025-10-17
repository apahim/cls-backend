# Event Architecture

This document explains the event-driven architecture of CLS Backend, including the fan-out Pub/Sub design, controller integration patterns, and event message structures.

## Overview

CLS Backend uses a **controller-agnostic fan-out architecture** where events are published to a single topic and controllers self-filter to determine which events they should process.

**Key Benefits:**
- **Zero Controller Awareness**: Backend has no hardcoded controller lists
- **Auto-Discovery**: New controllers work immediately without backend changes
- **True Scalability**: Add controllers by creating Pub/Sub subscription only
- **Self-Filtering**: Controllers decide which events to process
- **Simplified Context**: Events contain necessary cluster information

## Fan-Out Architecture

### Before: Controller-Specific Events

```
Reconciliation → Multiple Events (per controller type)
├── gcp-environment-validation event
├── aws-environment-validation event
└── azure-environment-validation event
```

**Problems:**
- Backend needed to know all controller types
- Adding controllers required backend changes
- Complex event routing logic
- Maintenance overhead

### After: Fan-Out Events

```
Reconciliation → Single Event (fan-out)
└── cluster.reconcile event → All Controllers (self-filter)
```

**Benefits:**
- Single event per cluster change
- Controllers self-filter based on platform and dependencies
- Zero maintenance of controller lists
- Automatic support for new controllers

## Pub/Sub Architecture

### Required Topics

#### cluster-events
Primary fan-out channel for all cluster lifecycle events:

```yaml
topic: cluster-events
description: Fan-out channel for all cluster lifecycle events
subscribers: All controllers create their own subscriptions
retention: 7 days
```

### Controller Subscriptions

Each controller creates its own subscription:

```yaml
# Example subscriptions
gcp-environment-validation-sub → cluster-events
aws-environment-validation-sub → cluster-events
networking-controller-sub → cluster-events
compute-controller-sub → cluster-events
```

**Subscription Pattern:**
- **Name**: `{controller-name}-sub`
- **Topic**: `cluster-events`
- **Filter**: Controllers filter in application code
- **Ack Deadline**: 600 seconds (10 minutes)
- **Retry Policy**: Exponential backoff

## Event Types

### Cluster Lifecycle Events

#### cluster.created
Sent when a new cluster is created:

```json
{
  "type": "cluster.created",
  "cluster_id": "abc-123-def",
  "generation": 1,
  "timestamp": "2025-10-17T16:00:00Z",
  "cluster": {
    "id": "abc-123-def",
    "name": "my-cluster",
    "spec": {
      "platform": {
        "type": "gcp",
        "gcp": {
          "projectID": "my-project",
          "region": "us-central1"
        }
      }
    },
    "created_by": "user@example.com",
    "generation": 1
  },
  "metadata": {
    "triggered_by": "api_request",
    "user_email": "user@example.com"
  }
}
```

#### cluster.updated
Sent when cluster specification is updated:

```json
{
  "type": "cluster.updated",
  "cluster_id": "abc-123-def",
  "generation": 2,
  "timestamp": "2025-10-17T16:05:00Z",
  "cluster": {
    /* updated cluster data */
  },
  "changes": {
    "spec.platform.gcp.region": {
      "from": "us-central1",
      "to": "us-west1"
    }
  },
  "metadata": {
    "triggered_by": "api_request",
    "user_email": "user@example.com"
  }
}
```

#### cluster.deleted
Sent when cluster is deleted:

```json
{
  "type": "cluster.deleted",
  "cluster_id": "abc-123-def",
  "generation": 2,
  "timestamp": "2025-10-17T16:10:00Z",
  "cluster": {
    /* final cluster state */
  },
  "metadata": {
    "triggered_by": "api_request",
    "user_email": "user@example.com",
    "force_delete": false
  }
}
```

### Reconciliation Events

#### cluster.reconcile
Sent for periodic reconciliation or reactive reconciliation:

```json
{
  "type": "cluster.reconcile",
  "cluster_id": "abc-123-def",
  "generation": 2,
  "timestamp": "2025-10-17T16:00:00Z",
  "reason": "generation_mismatch|periodic_reconciliation|spec_change",
  "cluster": {
    /* current cluster data */
  },
  "metadata": {
    "scheduled_by": "reactive_reconciliation|reconciliation_scheduler",
    "change_type": "spec|status|controller_status",
    "interval": "30s|5m"
  }
}
```

### Event Attributes

Events include Pub/Sub attributes for filtering:

```json
{
  "event_type": "cluster.reconcile",
  "cluster_id": "abc-123-def",
  "platform": "gcp",
  "generation": "2",
  "reason": "generation_mismatch"
}
```

## Controller Self-Filtering

Controllers use **preConditions** to determine if they should process events:

### Platform Filtering

```go
func (c *GCPController) shouldProcessEvent(event ClusterEvent) bool {
    // Only process GCP clusters
    if event.Cluster.Spec.Platform.Type != "gcp" {
        return false
    }
    return true
}

func (c *AWSController) shouldProcessEvent(event ClusterEvent) bool {
    // Only process AWS clusters
    if event.Cluster.Spec.Platform.Type != "aws" {
        return false
    }
    return true
}
```

### Dependency Filtering

```go
func (c *NetworkingController) shouldProcessEvent(event ClusterEvent) bool {
    // Only process if environment validation is complete
    envStatus := c.getControllerStatus(event.ClusterID, "environment-validation")
    if envStatus == nil || !envStatus.IsReady() {
        c.logger.Debug("Waiting for environment validation",
            "cluster_id", event.ClusterID)
        return false
    }
    return true
}
```

### Generation Filtering

```go
func (c *Controller) shouldProcessEvent(event ClusterEvent) bool {
    // Only process current generation
    lastProcessedGen := c.getLastProcessedGeneration(event.ClusterID)
    if event.Generation <= lastProcessedGen {
        c.logger.Debug("Already processed this generation",
            "cluster_id", event.ClusterID,
            "event_generation", event.Generation,
            "last_processed", lastProcessedGen)
        return false
    }
    return true
}
```

### User Context Filtering

```go
func (c *Controller) shouldProcessEvent(event ClusterEvent) bool {
    // Future: Filter by user context for multi-tenancy
    if c.config.EnableUserFiltering {
        allowedUsers := c.getUserAllowList(event.ClusterID)
        if !contains(allowedUsers, event.Metadata.UserEmail) {
            return false
        }
    }
    return true
}
```

## Event Publishing

### Backend Event Publishing

The backend publishes events for all cluster lifecycle operations:

```go
// Event publisher in internal/pubsub/publisher.go
func (p *Publisher) PublishClusterEvent(eventType string, cluster *models.Cluster, metadata map[string]interface{}) error {
    event := ClusterEvent{
        Type:      eventType,
        ClusterID: cluster.ID,
        Generation: cluster.Generation,
        Timestamp: time.Now().UTC(),
        Cluster:   cluster,
        Metadata:  metadata,
    }

    message := &pubsub.Message{
        Data: event.ToJSON(),
        Attributes: map[string]string{
            "event_type":  eventType,
            "cluster_id":  cluster.ID,
            "platform":    cluster.Spec.Platform.Type,
            "generation":  strconv.FormatInt(cluster.Generation, 10),
        },
    }

    result := p.topic.Publish(ctx, message)
    _, err := result.Get(ctx)
    return err
}
```

### Reactive Event Publishing

Events are published immediately when changes occur:

```go
// In API handlers
func (h *ClusterHandler) CreateCluster(c *gin.Context) {
    // ... validate and create cluster ...

    // Publish creation event
    h.eventPublisher.PublishClusterEvent("cluster.created", cluster, map[string]interface{}{
        "triggered_by": "api_request",
        "user_email":   c.GetHeader("X-User-Email"),
    })
}

func (h *ClusterHandler) UpdateCluster(c *gin.Context) {
    // ... validate and update cluster ...

    // Publish update event
    h.eventPublisher.PublishClusterEvent("cluster.updated", updatedCluster, map[string]interface{}{
        "triggered_by": "api_request",
        "user_email":   c.GetHeader("X-User-Email"),
        "changes":      calculateChanges(oldCluster, updatedCluster),
    })
}
```

### Reconciliation Event Publishing

Scheduled reconciliation events are published by the reconciliation scheduler:

```go
// In internal/reconciliation/scheduler.go
func (s *Scheduler) publishReconciliationEvents() {
    clusters := s.findClustersNeedingReconciliation()

    for _, cluster := range clusters {
        reason := s.determineReconciliationReason(cluster)
        interval := s.getReconciliationInterval(cluster)

        s.eventPublisher.PublishClusterEvent("cluster.reconcile", cluster, map[string]interface{}{
            "scheduled_by": "reconciliation_scheduler",
            "reason":       reason,
            "interval":     interval,
        })
    }
}
```

## Controller Integration

### Subscription Setup

Controllers should create their own Pub/Sub subscriptions:

```go
// Controller subscription setup
func (c *MyController) setupSubscription() error {
    subscription := c.pubsubClient.Subscription(c.subscriptionName)
    exists, err := subscription.Exists(c.ctx)
    if err != nil {
        return err
    }

    if !exists {
        _, err = c.pubsubClient.CreateSubscription(c.ctx, c.subscriptionName, pubsub.SubscriptionConfig{
            Topic:       c.pubsubClient.Topic("cluster-events"),
            AckDeadline: 600 * time.Second, // 10 minutes
            RetryPolicy: &pubsub.RetryPolicy{
                MinimumBackoff: 10 * time.Second,
                MaximumBackoff: 600 * time.Second,
            },
        })
        if err != nil {
            return err
        }
    }

    return nil
}
```

### Event Processing

Controllers should implement self-filtering event processing:

```go
// Controller event processing
func (c *MyController) processEvents() {
    subscription := c.pubsubClient.Subscription(c.subscriptionName)

    err := subscription.Receive(c.ctx, func(ctx context.Context, msg *pubsub.Message) {
        event, err := parseClusterEvent(msg.Data)
        if err != nil {
            c.logger.Error("Failed to parse event", "error", err)
            msg.Nack()
            return
        }

        // Self-filtering logic
        if !c.shouldProcessEvent(event) {
            c.logger.Debug("Skipping event", "cluster_id", event.ClusterID, "reason", "filtered")
            msg.Ack()
            return
        }

        // Process the event
        err = c.handleClusterEvent(event)
        if err != nil {
            c.logger.Error("Failed to process event", "error", err, "cluster_id", event.ClusterID)
            msg.Nack()
            return
        }

        // Report status after processing
        c.reportStatus(event.ClusterID, event.Generation)

        msg.Ack()
    })
}
```

### Status Reporting

Controllers must report status after processing events:

```go
// Controller status reporting
func (c *MyController) reportStatus(clusterID string, generation int64) error {
    status := ControllerStatus{
        ClusterID:          clusterID,
        ControllerName:     c.name,
        ObservedGeneration: generation,
        Conditions: []Condition{
            {
                Type:    "Available",
                Status:  "True", // or "False" if work failed
                Reason:  "WorkCompleted",
                Message: "Controller work completed successfully",
            },
        },
        Metadata: c.generateMetadata(),
    }

    return c.statusClient.UpdateStatus(status)
}
```

## Message Ordering and Delivery

### Message Ordering

**Pub/Sub does not guarantee message ordering** within a subscription. Controllers must handle out-of-order events:

```go
func (c *Controller) handleClusterEvent(event ClusterEvent) error {
    // Check if we've already processed a newer generation
    lastProcessed := c.getLastProcessedGeneration(event.ClusterID)
    if event.Generation <= lastProcessed {
        c.logger.Info("Skipping older generation",
            "cluster_id", event.ClusterID,
            "event_generation", event.Generation,
            "last_processed", lastProcessed)
        return nil
    }

    // Process the event
    return c.processClusterEvent(event)
}
```

### At-Least-Once Delivery

Pub/Sub guarantees **at-least-once delivery**. Controllers must handle duplicate events:

```go
func (c *Controller) processClusterEvent(event ClusterEvent) error {
    // Check if we've already processed this exact event
    if c.hasProcessedEvent(event.ClusterID, event.Generation) {
        c.logger.Debug("Event already processed",
            "cluster_id", event.ClusterID,
            "generation", event.Generation)
        return nil
    }

    // Process the event (idempotent operations)
    err := c.doWork(event)
    if err != nil {
        return err
    }

    // Mark as processed
    c.markEventProcessed(event.ClusterID, event.Generation)
    return nil
}
```

## Error Handling and Retries

### Event Processing Errors

```go
func (c *Controller) handleEvent(ctx context.Context, msg *pubsub.Message) {
    event, err := parseEvent(msg.Data)
    if err != nil {
        c.logger.Error("Invalid event format", "error", err)
        msg.Ack() // Don't retry invalid events
        return
    }

    err = c.processEvent(event)
    if err != nil {
        // Determine if error is retryable
        if c.isRetryableError(err) {
            c.logger.Warn("Retryable error, will retry", "error", err)
            msg.Nack() // Retry the message
        } else {
            c.logger.Error("Non-retryable error", "error", err)
            // Report error status
            c.reportErrorStatus(event.ClusterID, event.Generation, err)
            msg.Ack() // Don't retry non-retryable errors
        }
        return
    }

    msg.Ack()
}
```

### Dead Letter Queues

Configure dead letter queues for failed events:

```go
subscriptionConfig := pubsub.SubscriptionConfig{
    Topic:       topic,
    AckDeadline: 600 * time.Second,
    DeadLetterPolicy: &pubsub.DeadLetterPolicy{
        DeadLetterTopic:     deadLetterTopic,
        MaxDeliveryAttempts: 5,
    },
    RetryPolicy: &pubsub.RetryPolicy{
        MinimumBackoff: 10 * time.Second,
        MaximumBackoff: 600 * time.Second,
    },
}
```

## Monitoring and Debugging

### Event Flow Monitoring

```bash
# Monitor Pub/Sub topic
gcloud pubsub topics list
gcloud pubsub topics describe cluster-events

# Monitor subscriptions
gcloud pubsub subscriptions list
gcloud pubsub subscriptions describe gcp-controller-sub

# Check message backlog
gcloud pubsub subscriptions describe gcp-controller-sub \
  --format="value(numUndeliveredMessages)"
```

### Event Tracing

Add correlation IDs to events for tracing:

```go
event := ClusterEvent{
    Type:          "cluster.created",
    ClusterID:     cluster.ID,
    CorrelationID: generateCorrelationID(),
    // ... other fields
}
```

### Debugging Tools

```bash
# Pull messages manually for debugging
gcloud pubsub subscriptions pull gcp-controller-sub --limit=1

# Publish test events
gcloud pubsub topics publish cluster-events \
  --message='{"type": "cluster.reconcile", "cluster_id": "test"}'
```

## Best Practices

### For Backend Developers

1. **Publish Events Immediately**: Don't batch or delay event publishing
2. **Include Complete Context**: Events should contain all necessary information
3. **Use Consistent Message Format**: Follow established event schemas
4. **Add Correlation IDs**: Enable event tracing across systems

### For Controller Developers

1. **Implement Self-Filtering**: Use preConditions to filter events
2. **Handle Duplicates**: Implement idempotent event processing
3. **Report Status**: Always report status after processing events
4. **Use Generation Tracking**: Process only current generation events

### For Operations

1. **Monitor Subscriptions**: Watch for message backlogs
2. **Set Up Alerting**: Alert on failed message processing
3. **Use Dead Letter Queues**: Handle permanently failed messages
4. **Track Processing Time**: Monitor event processing latency

This event architecture provides a scalable, maintainable foundation for controller integration while maintaining loose coupling and operational simplicity.