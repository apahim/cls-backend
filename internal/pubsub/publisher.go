package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/apahim/cls-backend/internal/config"
	"github.com/apahim/cls-backend/internal/models"
	"github.com/apahim/cls-backend/internal/utils"
	"go.uber.org/zap"
)

// Publisher handles publishing events to Pub/Sub topics
type Publisher struct {
	client *Client
	logger *utils.Logger
	config config.PubSubConfig
}

// NewPublisher creates a new publisher
func NewPublisher(client *Client, cfg config.PubSubConfig) *Publisher {
	return &Publisher{
		client: client,
		logger: utils.NewLogger("pubsub_publisher"),
		config: cfg,
	}
}

// PublishClusterEvent publishes a lightweight cluster lifecycle event
func (p *Publisher) PublishClusterEvent(ctx context.Context, eventType string, cluster *models.Cluster) error {
	event := NewClusterEvent(eventType, cluster.ID, cluster.Generation)

	data, err := event.ToJSON()
	if err != nil {
		p.logger.Error("Failed to serialize cluster event",
			zap.String("event_type", eventType),
			zap.String("cluster_id", cluster.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to serialize cluster event: %w", err)
	}

	err = p.client.Publish(ctx, p.config.ClusterEventsTopic, data, event.GetAttributes())
	if err != nil {
		p.logger.Error("Failed to publish cluster event",
			zap.String("event_type", eventType),
			zap.String("cluster_id", cluster.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish cluster event: %w", err)
	}

	p.logger.Info("Lightweight cluster event published successfully",
		zap.String("event_type", eventType),
		zap.String("cluster_id", cluster.ID.String()),
		zap.String("cluster_name", cluster.Name),
		zap.Int64("generation", cluster.Generation),
	)

	return nil
}

// PublishNodePoolEvent publishes a lightweight nodepool lifecycle event
func (p *Publisher) PublishNodePoolEvent(ctx context.Context, eventType string, nodepool *models.NodePool) error {
	event := NewNodePoolEvent(eventType, nodepool.ClusterID, nodepool.ID, nodepool.Generation)

	data, err := event.ToJSON()
	if err != nil {
		p.logger.Error("Failed to serialize nodepool event",
			zap.String("event_type", eventType),
			zap.String("nodepool_id", nodepool.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to serialize nodepool event: %w", err)
	}

	err = p.client.Publish(ctx, p.config.ClusterEventsTopic, data, event.GetAttributes())
	if err != nil {
		p.logger.Error("Failed to publish nodepool event",
			zap.String("event_type", eventType),
			zap.String("nodepool_id", nodepool.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish nodepool event: %w", err)
	}

	p.logger.Info("Lightweight NodePool event published successfully",
		zap.String("event_type", eventType),
		zap.String("cluster_id", nodepool.ClusterID.String()),
		zap.String("nodepool_id", nodepool.ID.String()),
		zap.String("nodepool_name", nodepool.Name),
		zap.Int64("generation", nodepool.Generation),
	)

	return nil
}


// Convenience methods for common events

// PublishClusterCreated publishes a cluster created event
func (p *Publisher) PublishClusterCreated(ctx context.Context, cluster *models.Cluster) error {
	return p.PublishClusterEvent(ctx, EventTypeClusterCreated, cluster)
}

// PublishClusterUpdated publishes a cluster updated event
func (p *Publisher) PublishClusterUpdated(ctx context.Context, cluster *models.Cluster) error {
	return p.PublishClusterEvent(ctx, EventTypeClusterUpdated, cluster)
}

// PublishClusterDeleted publishes a cluster deleted event
func (p *Publisher) PublishClusterDeleted(ctx context.Context, cluster *models.Cluster) error {
	return p.PublishClusterEvent(ctx, EventTypeClusterDeleted, cluster)
}

// PublishNodePoolCreated publishes a nodepool created event
func (p *Publisher) PublishNodePoolCreated(ctx context.Context, nodepool *models.NodePool) error {
	return p.PublishNodePoolEvent(ctx, EventTypeNodePoolCreated, nodepool)
}

// PublishNodePoolUpdated publishes a nodepool updated event
func (p *Publisher) PublishNodePoolUpdated(ctx context.Context, nodepool *models.NodePool) error {
	return p.PublishNodePoolEvent(ctx, EventTypeNodePoolUpdated, nodepool)
}

// PublishNodePoolDeleted publishes a nodepool deleted event
func (p *Publisher) PublishNodePoolDeleted(ctx context.Context, nodepool *models.NodePool) error {
	return p.PublishNodePoolEvent(ctx, EventTypeNodePoolDeleted, nodepool)
}


// PublishReconciliationEvent publishes a reconciliation event (fan-out to all controllers)
func (p *Publisher) PublishReconciliationEvent(ctx context.Context, event *models.ReconciliationEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		p.logger.Error("Failed to serialize reconciliation event",
			zap.String("cluster_id", event.ClusterID),
			zap.String("reason", event.Reason),
			zap.Error(err),
		)
		return fmt.Errorf("failed to serialize reconciliation event: %w", err)
	}

	// Create attributes for filtering
	attributes := map[string]string{
		"event_type": event.Type,
		"reason":     event.Reason,
		"cluster_id": event.ClusterID,
	}

	err = p.client.Publish(ctx, p.config.ClusterEventsTopic, data, attributes)
	if err != nil {
		p.logger.Error("Failed to publish reconciliation event",
			zap.String("cluster_id", event.ClusterID),
			zap.String("reason", event.Reason),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish reconciliation event: %w", err)
	}

	p.logger.Debug("Reconciliation event published successfully (fan-out)",
		zap.String("cluster_id", event.ClusterID),
		zap.String("reason", event.Reason),
	)

	return nil
}
