package pubsub

import (
	"context"
	"fmt"
	"sync"

	"github.com/apahim/cls-backend/internal/config"
	"github.com/apahim/cls-backend/internal/utils"
	"go.uber.org/zap"
)

// Service manages Pub/Sub operations for the CLS backend (simplified for fan-out architecture)
type Service struct {
	client    *Client
	publisher *Publisher
	logger    *utils.Logger
	config    config.PubSubConfig

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	status string
}

// NewService creates a new Pub/Sub service (publisher-only for fan-out architecture)
func NewService(cfg config.PubSubConfig) (*Service, error) {
	logger := utils.NewLogger("pubsub_service")

	// Create Pub/Sub client
	client, err := NewClient(cfg)
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return nil, fmt.Errorf("failed to create pubsub client: %w", err)
	}

	// Create publisher
	publisher := NewPublisher(client, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	service := &Service{
		client:    client,
		publisher: publisher,
		logger:    logger,
		config:    cfg,
		ctx:       ctx,
		cancel:    cancel,
		status:    "initialized",
	}

	logger.Info("Pub/Sub service created successfully (publisher-only)")
	return service, nil
}

// Start starts the Pub/Sub service (publisher-only for fan-out architecture)
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == "running" {
		return fmt.Errorf("service is already running")
	}

	s.logger.Info("Starting Pub/Sub service (publisher-only)")

	s.status = "running"
	s.logger.Info("Pub/Sub service started successfully")
	return nil
}

// Stop stops the Pub/Sub service
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != "running" {
		return fmt.Errorf("service is not running")
	}

	s.logger.Info("Stopping Pub/Sub service")

	// Cancel context to stop all operations
	s.cancel()

	// Close the client
	if err := s.client.Close(); err != nil {
		s.logger.Error("Failed to close Pub/Sub client", zap.Error(err))
		return fmt.Errorf("failed to close pubsub client: %w", err)
	}

	s.status = "stopped"
	s.logger.Info("Pub/Sub service stopped successfully")
	return nil
}

// GetPublisher returns the publisher instance
func (s *Service) GetPublisher() *Publisher {
	return s.publisher
}


// GetClient returns the Pub/Sub client
func (s *Service) GetClient() *Client {
	return s.client
}

// Health returns the health status of the Pub/Sub service
func (s *Service) Health(ctx context.Context) (string, error) {
	s.mu.RLock()
	status := s.status
	s.mu.RUnlock()

	if status != "running" {
		return "unhealthy", fmt.Errorf("service is not running (status: %s)", status)
	}

	// Check client health
	clientHealth, err := s.client.Health(ctx)
	if err != nil {
		return "unhealthy", fmt.Errorf("client health check failed: %w", err)
	}

	if clientHealth != "healthy" {
		return "degraded", fmt.Errorf("client is %s", clientHealth)
	}

	return "healthy", nil
}

// GetStatus returns the current service status
func (s *Service) GetStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// IsRunning returns true if the service is running
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status == "running"
}
