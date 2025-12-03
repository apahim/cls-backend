package models

import (
	"time"

	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
)

// ReconciliationSchedule represents a cluster-centric reconciliation schedule for fan-out events
type ReconciliationSchedule struct {
	ID                int64      `json:"id" db:"id"`
	ClusterID         uuid.UUID  `json:"cluster_id" db:"cluster_id"`
	LastReconciledAt  *time.Time `json:"last_reconciled_at" db:"last_reconciled_at"`
	NextReconcileAt   *time.Time `json:"next_reconcile_at" db:"next_reconcile_at"`
	ReconcileInterval string     `json:"reconcile_interval" db:"reconcile_interval"` // PostgreSQL INTERVAL as string
	Enabled           bool       `json:"enabled" db:"enabled"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`

	// Health-aware interval configuration
	HealthyInterval   string     `json:"healthy_interval" db:"healthy_interval"`     // PostgreSQL INTERVAL as string
	UnhealthyInterval string     `json:"unhealthy_interval" db:"unhealthy_interval"` // PostgreSQL INTERVAL as string
	AdaptiveEnabled   bool       `json:"adaptive_enabled" db:"adaptive_enabled"`
	LastHealthCheck   *time.Time `json:"last_health_check" db:"last_health_check"`
	IsHealthy         *bool      `json:"is_healthy" db:"is_healthy"` // NULL = unknown, true = healthy, false = unhealthy
}

// ReconciliationTarget represents a cluster that needs reconciliation (fan-out to all controllers)
type ReconciliationTarget struct {
	ClusterID         uuid.UUID  `json:"cluster_id" db:"cluster_id"`
	Reason            string     `json:"reason" db:"reason"`
	LastReconciledAt  *time.Time `json:"last_reconciled_at" db:"last_reconciled_at"`
	ClusterGeneration int64      `json:"cluster_generation" db:"cluster_generation"`
}

// ReconciliationEvent represents an event published for reconciliation (fan-out to all controllers)
type ReconciliationEvent struct {
	Type       string                 `json:"type"`
	ClusterID  string                 `json:"cluster_id"`
	Reason     string                 `json:"reason"`
	Generation int64                  `json:"generation"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ReconciliationConfig represents reconciliation configuration
type ReconciliationConfig struct {
	Enabled         bool          `json:"enabled"`
	CheckInterval   time.Duration `json:"check_interval"`   // How often to check for reconciliation needs
	DefaultInterval time.Duration `json:"default_interval"` // Default reconciliation interval for new schedules
	MaxConcurrent   int           `json:"max_concurrent"`   // Max concurrent reconciliation events to publish (fan-out to all controllers)
}

// DefaultReconciliationConfig returns the default reconciliation configuration
func DefaultReconciliationConfig() *ReconciliationConfig {
	return &ReconciliationConfig{
		Enabled:         true,
		CheckInterval:   1 * time.Minute, // Check every minute
		DefaultInterval: 5 * time.Minute, // Default 5-minute reconciliation
		MaxConcurrent:   50,              // Max 50 concurrent reconciliations (fan-out to all controllers)
	}
}

// Validate validates the reconciliation configuration
func (rc *ReconciliationConfig) Validate() error {
	if rc.CheckInterval <= 0 {
		return utils.NewValidationError("INVALID_CHECK_INTERVAL", "check_interval must be positive", rc.CheckInterval)
	}
	if rc.DefaultInterval <= 0 {
		return utils.NewValidationError("INVALID_DEFAULT_INTERVAL", "default_interval must be positive", rc.DefaultInterval)
	}
	if rc.MaxConcurrent <= 0 {
		return utils.NewValidationError("INVALID_MAX_CONCURRENT", "max_concurrent must be positive", rc.MaxConcurrent)
	}
	return nil
}

// ReactiveReconciliationConfig represents configuration for reactive reconciliation (database-driven)
type ReactiveReconciliationConfig struct {
	ID                 int64         `json:"id" db:"id"`
	Enabled            bool          `json:"enabled" db:"enabled"`
	ChangeTypes        []string      `json:"change_types" db:"change_types"`                   // ["spec", "status", "controller_status"]
	DebounceInterval   time.Duration `json:"debounce_interval" db:"debounce_interval"`         // Prevent rapid-fire events
	MaxEventsPerMinute int           `json:"max_events_per_minute" db:"max_events_per_minute"` // Rate limiting
	CreatedAt          time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at" db:"updated_at"`
}

// DatabaseChangeNotification represents a change notification from database triggers (fan-out to all controllers)
type DatabaseChangeNotification struct {
	ClusterID  string    `json:"cluster_id"`
	ChangeType string    `json:"change_type"` // "spec", "status", "controller_status"
	Reason     string    `json:"reason"`
	Timestamp  time.Time `json:"timestamp"`
}
