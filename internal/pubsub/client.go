package pubsub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/apahim/cls-backend/internal/config"
	"github.com/apahim/cls-backend/internal/utils"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

// Client wraps Google Cloud Pub/Sub client with additional functionality
type Client struct {
	client          *pubsub.Client
	logger          *utils.Logger
	config          config.PubSubConfig
	topics          map[string]*pubsub.Topic
	subscriptions   map[string]*pubsub.Subscription
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	healthStatus    string
	lastHealthCheck time.Time
}

// NewClient creates a new Pub/Sub client
func NewClient(cfg config.PubSubConfig) (*Client, error) {
	logger := utils.NewLogger("pubsub_client")

	ctx, cancel := context.WithCancel(context.Background())

	var client *pubsub.Client
	var err error

	if cfg.EmulatorHost != "" {
		// Use emulator for local development
		logger.Info("Connecting to Pub/Sub emulator",
			zap.String("emulator_host", cfg.EmulatorHost),
		)
		client, err = pubsub.NewClient(ctx, cfg.ProjectID,
			option.WithEndpoint(cfg.EmulatorHost),
			option.WithoutAuthentication(),
		)
	} else {
		// Use production Pub/Sub
		logger.Info("Connecting to Google Cloud Pub/Sub",
			zap.String("project_id", cfg.ProjectID),
		)
		if cfg.CredentialsFile != "" {
			client, err = pubsub.NewClient(ctx, cfg.ProjectID,
				option.WithCredentialsFile(cfg.CredentialsFile),
			)
		} else {
			// Use default credentials (service account, etc.)
			client, err = pubsub.NewClient(ctx, cfg.ProjectID)
		}
	}

	if err != nil {
		cancel()
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return nil, fmt.Errorf("failed to create pubsub client: %w", err)
	}

	c := &Client{
		client:        client,
		logger:        logger,
		config:        cfg,
		topics:        make(map[string]*pubsub.Topic),
		subscriptions: make(map[string]*pubsub.Subscription),
		ctx:           ctx,
		cancel:        cancel,
		healthStatus:  "healthy",
	}

	// Initialize topics and subscriptions
	if err := c.initializeTopics(); err != nil {
		c.Close()
		return nil, fmt.Errorf("failed to initialize topics: %w", err)
	}

	if err := c.initializeSubscriptions(); err != nil {
		c.Close()
		return nil, fmt.Errorf("failed to initialize subscriptions: %w", err)
	}

	logger.Info("Pub/Sub client initialized successfully")
	return c, nil
}

// initializeTopics creates or ensures required topics exist (simplified for fan-out architecture)
func (c *Client) initializeTopics() error {
	requiredTopics := []string{
		c.config.ClusterEventsTopic,
	}

	for _, topicName := range requiredTopics {
		if topicName == "" {
			continue
		}

		topic, err := c.ensureTopic(topicName)
		if err != nil {
			return fmt.Errorf("failed to ensure topic %s: %w", topicName, err)
		}

		c.mu.Lock()
		c.topics[topicName] = topic
		c.mu.Unlock()

		c.logger.Info("Topic initialized", zap.String("topic", topicName))
	}

	return nil
}

// initializeSubscriptions creates or ensures required subscriptions exist (none needed for publisher-only service)
func (c *Client) initializeSubscriptions() error {
	// No subscriptions needed for simplified fan-out architecture
	// Controllers create their own subscriptions to cluster-events topic
	c.logger.Info("No backend subscriptions initialized (publisher-only mode)")
	return nil
}

// ensureTopic creates a topic if it doesn't exist
func (c *Client) ensureTopic(topicName string) (*pubsub.Topic, error) {
	topic := c.client.Topic(topicName)

	exists, err := topic.Exists(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check if topic exists: %w", err)
	}

	if !exists {
		c.logger.Info("Creating topic", zap.String("topic", topicName))
		topic, err = c.client.CreateTopic(c.ctx, topicName)
		if err != nil {
			return nil, fmt.Errorf("failed to create topic: %w", err)
		}
	}

	// Configure topic settings
	updateConfig := pubsub.TopicConfigToUpdate{
		RetentionDuration: 24 * time.Hour, // Retain messages for 24 hours
	}

	if _, err := topic.Update(c.ctx, updateConfig); err != nil {
		c.logger.Warn("Failed to update topic config",
			zap.String("topic", topicName),
			zap.Error(err),
		)
	}

	return topic, nil
}

// ensureSubscription creates a subscription if it doesn't exist
func (c *Client) ensureSubscription(subName, topicName string) (*pubsub.Subscription, error) {
	sub := c.client.Subscription(subName)

	exists, err := sub.Exists(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check if subscription exists: %w", err)
	}

	if !exists {
		c.logger.Info("Creating subscription",
			zap.String("subscription", subName),
			zap.String("topic", topicName),
		)

		topic := c.client.Topic(topicName)
		sub, err = c.client.CreateSubscription(c.ctx, subName, pubsub.SubscriptionConfig{
			Topic:       topic,
			AckDeadline: 30 * time.Second,
			RetryPolicy: &pubsub.RetryPolicy{
				MinimumBackoff: 10 * time.Second,
				MaximumBackoff: 10 * time.Minute,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create subscription: %w", err)
		}
	}

	// Configure subscription settings - commenting out for now as the field might not exist in this version
	// updateConfig := pubsub.SubscriptionConfigToUpdate{
	// 	MaxDeliveryAttempts: 5,
	// }

	// if _, err := sub.Update(c.ctx, updateConfig); err != nil {
	//	c.logger.Warn("Failed to update subscription config",
	//		zap.String("subscription", subName),
	//		zap.Error(err),
	//	)
	// }

	return sub, nil
}

// GetTopic returns a topic by name
func (c *Client) GetTopic(topicName string) (*pubsub.Topic, error) {
	c.mu.RLock()
	topic, exists := c.topics[topicName]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("topic %s not found", topicName)
	}

	return topic, nil
}

// GetSubscription returns a subscription by name
func (c *Client) GetSubscription(subName string) (*pubsub.Subscription, error) {
	c.mu.RLock()
	sub, exists := c.subscriptions[subName]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("subscription %s not found", subName)
	}

	return sub, nil
}

// Publish publishes a message to a topic
func (c *Client) Publish(ctx context.Context, topicName string, data []byte, attributes map[string]string) error {
	topic, err := c.GetTopic(topicName)
	if err != nil {
		return fmt.Errorf("failed to get topic: %w", err)
	}

	msg := &pubsub.Message{
		Data:       data,
		Attributes: attributes,
	}

	result := topic.Publish(ctx, msg)

	// Wait for the publish to complete
	msgID, err := result.Get(ctx)
	if err != nil {
		c.logger.Error("Failed to publish message",
			zap.String("topic", topicName),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish message: %w", err)
	}

	c.logger.Debug("Message published successfully",
		zap.String("topic", topicName),
		zap.String("message_id", msgID),
		zap.Int("data_size", len(data)),
	)

	return nil
}

// Subscribe starts receiving messages from a subscription
func (c *Client) Subscribe(ctx context.Context, subName string, handler MessageHandler) error {
	sub, err := c.GetSubscription(subName)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	c.logger.Info("Starting subscription",
		zap.String("subscription", subName),
	)

	// Configure subscription receive settings
	sub.ReceiveSettings.NumGoroutines = c.config.MaxConcurrentHandlers
	sub.ReceiveSettings.MaxOutstandingMessages = c.config.MaxOutstandingMessages

	// Start receiving messages
	return sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		start := time.Now()

		// Create message wrapper
		message := &Message{
			ID:          msg.ID,
			Data:        msg.Data,
			Attributes:  msg.Attributes,
			PublishTime: msg.PublishTime,
		}

		// Handle the message
		err := handler.HandleMessage(ctx, message)
		duration := time.Since(start)

		if err != nil {
			c.logger.Error("Message handler failed",
				zap.String("subscription", subName),
				zap.String("message_id", msg.ID),
				zap.Duration("duration", duration),
				zap.Error(err),
			)
			msg.Nack()
			return
		}

		c.logger.Debug("Message processed successfully",
			zap.String("subscription", subName),
			zap.String("message_id", msg.ID),
			zap.Duration("duration", duration),
		)
		msg.Ack()
	})
}

// Health checks the health of the Pub/Sub client
func (c *Client) Health(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Return cached status if checked recently
	if time.Since(c.lastHealthCheck) < 30*time.Second {
		return c.healthStatus, nil
	}

	// Try to list topics to verify connectivity
	it := c.client.Topics(ctx)
	_, err := it.Next()
	if err != nil && err.Error() != "no more items in iterator" {
		c.healthStatus = "unhealthy"
		c.lastHealthCheck = time.Now()
		return c.healthStatus, fmt.Errorf("pubsub health check failed: %w", err)
	}

	c.healthStatus = "healthy"
	c.lastHealthCheck = time.Now()
	return c.healthStatus, nil
}

// Close closes the Pub/Sub client and all topics/subscriptions
func (c *Client) Close() error {
	c.logger.Info("Closing Pub/Sub client")

	// Cancel context to stop all operations
	c.cancel()

	// Close all topics
	c.mu.Lock()
	for name, topic := range c.topics {
		topic.Stop()
		c.logger.Debug("Topic stopped", zap.String("topic", name))
	}

	// Close all subscriptions (they will stop automatically with context cancellation)
	for name := range c.subscriptions {
		c.logger.Debug("Subscription stopped", zap.String("subscription", name))
	}
	c.mu.Unlock()

	// Close the client
	if err := c.client.Close(); err != nil {
		c.logger.Error("Failed to close Pub/Sub client", zap.Error(err))
		return fmt.Errorf("failed to close pubsub client: %w", err)
	}

	c.logger.Info("Pub/Sub client closed successfully")
	return nil
}

// GetConfig returns the client configuration
func (c *Client) GetConfig() config.PubSubConfig {
	return c.config
}
