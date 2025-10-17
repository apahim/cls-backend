package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/apahim/cls-backend/internal/config"
	"github.com/apahim/cls-backend/internal/utils"

	_ "github.com/lib/pq" // PostgreSQL driver
	"go.uber.org/zap"
)

// Client represents the database client
type Client struct {
	db     *sql.DB
	logger *utils.Logger
	config config.DatabaseConfig
	tx     *sql.Tx // Optional transaction for transaction-aware operations
}

// NewClient creates a new database client
func NewClient(cfg config.DatabaseConfig) (*Client, error) {
	logger := utils.NewLogger("database")

	db, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	client := &Client{
		db:     db,
		logger: logger,
		config: cfg,
	}

	// Test the connection
	if err := client.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Database connection established",
		zap.Int("max_open_conns", cfg.MaxOpenConns),
		zap.Int("max_idle_conns", cfg.MaxIdleConns),
		zap.Duration("conn_max_lifetime", cfg.ConnMaxLifetime),
	)

	return client, nil
}

// Close closes the database connection
func (c *Client) Close() error {
	c.logger.Info("Closing database connection")
	return c.db.Close()
}

// Ping tests the database connection
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.db.PingContext(ctx)
}

// DB returns the underlying sql.DB instance
func (c *Client) DB() *sql.DB {
	return c.db
}

// Transaction executes a function within a database transaction
func (c *Client) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				c.logger.Error("Failed to rollback transaction after panic",
					zap.Any("panic", p),
					zap.Error(rollbackErr),
				)
			}
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			c.logger.Error("Failed to rollback transaction",
				zap.Error(err),
				zap.Error(rollbackErr),
			)
			return fmt.Errorf("transaction failed: %w (rollback error: %v)", err, rollbackErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ExecContext executes a query without returning rows
func (c *Client) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	var result sql.Result
	var err error

	if c.tx != nil {
		// Use transaction if available
		result, err = c.tx.ExecContext(ctx, query, args...)
	} else {
		// Use regular connection
		result, err = c.db.ExecContext(ctx, query, args...)
	}
	duration := time.Since(start)

	c.logQuery("EXEC", query, duration, err, args...)
	return result, err
}

// QueryContext executes a query that returns rows
func (c *Client) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	var rows *sql.Rows
	var err error

	if c.tx != nil {
		// Use transaction if available
		rows, err = c.tx.QueryContext(ctx, query, args...)
	} else {
		// Use regular connection
		rows, err = c.db.QueryContext(ctx, query, args...)
	}
	duration := time.Since(start)

	c.logQuery("QUERY", query, duration, err, args...)
	return rows, err
}

// QueryRowContext executes a query that returns a single row
func (c *Client) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	start := time.Now()
	var row *sql.Row

	if c.tx != nil {
		// Use transaction if available
		row = c.tx.QueryRowContext(ctx, query, args...)
	} else {
		// Use regular connection
		row = c.db.QueryRowContext(ctx, query, args...)
	}
	duration := time.Since(start)

	c.logQuery("QUERY_ROW", query, duration, nil, args...)
	return row
}

// Stats returns database statistics
func (c *Client) Stats() sql.DBStats {
	return c.db.Stats()
}

// Health returns database health information
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	stats := c.Stats()

	// Check if we can ping the database
	pingErr := c.Ping(ctx)

	status := &HealthStatus{
		Status:       "healthy",
		MaxOpenConns: stats.MaxOpenConnections,
		OpenConns:    stats.OpenConnections,
		InUseConns:   stats.InUse,
		IdleConns:    stats.Idle,
		WaitCount:    stats.WaitCount,
		WaitDuration: stats.WaitDuration,
		MaxIdleCount: stats.MaxIdleClosed,
		MaxLifeCount: stats.MaxLifetimeClosed,
		PingError:    nil,
	}

	if pingErr != nil {
		status.Status = "unhealthy"
		status.PingError = pingErr
	}

	// Check for concerning metrics
	if stats.WaitCount > 100 {
		status.Status = "degraded"
		status.Issues = append(status.Issues, "High connection wait count")
	}

	if stats.OpenConnections >= stats.MaxOpenConnections {
		status.Status = "degraded"
		status.Issues = append(status.Issues, "Connection pool exhausted")
	}

	return status, nil
}

// HealthStatus represents database health status
type HealthStatus struct {
	Status       string        `json:"status"`
	MaxOpenConns int           `json:"max_open_conns"`
	OpenConns    int           `json:"open_conns"`
	InUseConns   int           `json:"in_use_conns"`
	IdleConns    int           `json:"idle_conns"`
	WaitCount    int64         `json:"wait_count"`
	WaitDuration time.Duration `json:"wait_duration"`
	MaxIdleCount int64         `json:"max_idle_count"`
	MaxLifeCount int64         `json:"max_life_count"`
	Issues       []string      `json:"issues,omitempty"`
	PingError    error         `json:"ping_error,omitempty"`
}

// logQuery logs database queries with performance information
func (c *Client) logQuery(operation, query string, duration time.Duration, err error, args ...interface{}) {
	fields := []zap.Field{
		zap.String("operation", operation),
		zap.Duration("duration", duration),
		zap.String("query", query),
	}

	if len(args) > 0 {
		fields = append(fields, zap.Int("arg_count", len(args)))
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
		c.logger.Error("Database query failed", fields...)
	} else {
		// Log slow queries
		if duration > 100*time.Millisecond {
			fields = append(fields, zap.String("performance", "slow"))
			c.logger.Warn("Slow database query", fields...)
		} else {
			c.logger.Debug("Database query executed", fields...)
		}
	}
}

// Note: Database migrations are handled by Kubernetes Jobs in production.
// See deploy/kubernetes/migration-job.yaml for the actual migration system.
