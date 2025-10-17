package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/apahim/cls-backend/internal/config"
	"github.com/apahim/cls-backend/internal/utils"
	"go.uber.org/zap"
)

// Repository provides access to all database repositories
type Repository struct {
	client *Client
	logger *utils.Logger

	Clusters         *ClustersRepository
	NodePools        *NodePoolsRepository
	Status           *StatusRepository
	Reconciliation   *ReconciliationRepository
	StatusAggregator *StatusAggregator
}

// NewRepository creates a new repository manager
func NewRepository(cfg config.DatabaseConfig) (*Repository, error) {
	logger := utils.NewLogger("repository")

	// Create database client
	client, err := NewClient(cfg)
	if err != nil {
		logger.Error("Failed to create database client", zap.Error(err))
		return nil, fmt.Errorf("failed to create database client: %w", err)
	}

	// Create repositories
	reconciliationRepo := NewReconciliationRepository(client)
	statusRepo := NewStatusRepository(client)

	// Wire up the reconciliation updater to avoid circular dependency
	statusRepo.SetReconciliationUpdater(reconciliationRepo)

	repo := &Repository{
		client:           client,
		logger:           logger,
		Clusters:         NewClustersRepository(client),
		NodePools:        NewNodePoolsRepository(client),
		Status:           statusRepo,
		Reconciliation:   reconciliationRepo,
		StatusAggregator: NewStatusAggregator(client),
	}

	logger.Info("Repository initialized successfully")
	return repo, nil
}

// Close closes all database connections
func (r *Repository) Close() error {
	r.logger.Info("Closing repository connections")
	return r.client.Close()
}

// Health returns the health status of the database
func (r *Repository) Health(ctx context.Context) (*HealthStatus, error) {
	return r.client.Health(ctx)
}

// Stats returns database connection statistics
func (r *Repository) Stats() interface{} {
	return r.client.Stats()
}

// Note: Database migrations are handled by Kubernetes Jobs in production.
// See deploy/kubernetes/migration-job.yaml for the actual migration system.

// Transaction executes a function within a database transaction
func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	return r.client.Transaction(ctx, func(tx *sql.Tx) error {
		// Create a transaction-aware client
		txClient := &Client{
			db:     nil, // tx will be used instead
			logger: r.client.logger,
			config: r.client.config,
			tx:     tx, // Store the transaction
		}

		// Create transaction-aware repositories
		txReconciliationRepo := NewReconciliationRepository(txClient)
		txStatusRepo := NewStatusRepository(txClient)

		// Wire up the reconciliation updater for transaction repositories
		txStatusRepo.SetReconciliationUpdater(txReconciliationRepo)

		// Create a transaction repository with transaction-aware repositories
		txRepo := &Repository{
			client:           txClient,
			logger:           r.logger,
			Clusters:         NewClustersRepository(txClient),
			NodePools:        NewNodePoolsRepository(txClient),
			Status:           txStatusRepo,
			Reconciliation:   txReconciliationRepo,
			StatusAggregator: NewStatusAggregator(txClient),
		}

		return fn(txRepo)
	})
}

// GetClient returns the underlying database client
func (r *Repository) GetClient() *Client {
	return r.client
}
