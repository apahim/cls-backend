package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ClusterControllerStatus represents the status of a controller for a cluster
type ClusterControllerStatus struct {
	ClusterID          uuid.UUID   `json:"cluster_id" db:"cluster_id"`
	ControllerName     string      `json:"controller_name" db:"controller_name"`
	ObservedGeneration int64       `json:"observed_generation" db:"observed_generation"`
	Conditions         ConditionList `json:"conditions" db:"conditions"`
	Metadata           JSONB       `json:"metadata,omitempty" db:"metadata"`
	LastError          *ErrorInfo  `json:"last_error,omitempty" db:"last_error"`
	LastUpdated        time.Time   `json:"last_updated" db:"updated_at"`
}

// NodePoolControllerStatus represents the status of a controller for a node pool
type NodePoolControllerStatus struct {
	NodePoolID         uuid.UUID   `json:"nodepool_id" db:"nodepool_id"`
	ControllerName     string      `json:"controller_name" db:"controller_name"`
	ObservedGeneration int64       `json:"observed_generation" db:"observed_generation"`
	Conditions         ConditionList `json:"conditions" db:"conditions"`
	Metadata           JSONB       `json:"metadata,omitempty" db:"metadata"`
	LastError          *ErrorInfo  `json:"last_error,omitempty" db:"last_error"`
	LastUpdated        time.Time   `json:"last_updated" db:"updated_at"`
}

// ClusterEvent represents a cluster change event
type ClusterEvent struct {
	ID           uuid.UUID `json:"id" db:"id"`
	ClusterID    uuid.UUID `json:"cluster_id" db:"cluster_id"`
	EventType    string    `json:"event_type" db:"event_type"` // created, updated, deleted
	Generation   int64     `json:"generation" db:"generation"`
	Changes      JSONB     `json:"changes,omitempty" db:"changes"`
	PublishedAt  time.Time `json:"published_at" db:"published_at"`
}

// StatusEvent represents a status update event from controllers
type StatusEvent struct {
	ClusterID          string      `json:"clusterId"`
	NodePoolID         string      `json:"nodePoolId,omitempty"`
	ControllerName     string      `json:"controllerName"`
	ObservedGeneration int64       `json:"observedGeneration"`
	Conditions         []Condition `json:"conditions"`
	Metadata           JSONB       `json:"metadata,omitempty"`
	LastError          *ErrorInfo  `json:"lastError,omitempty"`
	Timestamp          time.Time   `json:"timestamp"`
}

// ConditionList represents a list of conditions that can be stored in PostgreSQL
type ConditionList []Condition

// Value implements the driver.Valuer interface for ConditionList
func (cl ConditionList) Value() (driver.Value, error) {
	if cl == nil {
		return json.Marshal([]Condition{})
	}
	return json.Marshal(cl)
}

// Scan implements the sql.Scanner interface for ConditionList
func (cl *ConditionList) Scan(value interface{}) error {
	if value == nil {
		*cl = []Condition{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into ConditionList", value)
	}

	var conditions []Condition
	if err := json.Unmarshal(bytes, &conditions); err != nil {
		return err
	}

	*cl = conditions
	return nil
}

// Value implements the driver.Valuer interface for ErrorInfo
func (ei *ErrorInfo) Value() (driver.Value, error) {
	if ei == nil {
		return nil, nil
	}
	return json.Marshal(ei)
}

// Scan implements the sql.Scanner interface for ErrorInfo
func (ei *ErrorInfo) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into ErrorInfo", value)
	}

	return json.Unmarshal(bytes, ei)
}

// ClusterStatusSummary represents aggregated cluster status
type ClusterStatusSummary struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	Name              string     `json:"name" db:"name"`
	Namespace         string     `json:"namespace" db:"namespace"`
	Generation        int64      `json:"generation" db:"generation"`
	TotalControllers  int        `json:"total_controllers" db:"total_controllers"`
	ReadyControllers  int        `json:"ready_controllers" db:"ready_controllers"`
	LastStatusUpdate  *time.Time `json:"last_status_update" db:"last_status_update"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

// StatusAggregationRequest represents a request to aggregate status
type StatusAggregationRequest struct {
	ClusterID uuid.UUID `json:"cluster_id"`
	Force     bool      `json:"force,omitempty"`
}

// StatusAggregationResult represents the result of status aggregation
type StatusAggregationResult struct {
	ClusterID          uuid.UUID              `json:"cluster_id"`
	ObservedGeneration int64                  `json:"observed_generation"`
	ControllerCount    int                    `json:"controller_count"`
	ReadyCount         int                    `json:"ready_count"`
	Conditions         []Condition            `json:"conditions"`
	Errors             []ErrorInfo            `json:"errors"`
	ControllerDetails  map[string]interface{} `json:"controller_details"`
	AggregatedAt       time.Time              `json:"aggregated_at"`
}

// TableName returns the table name for ClusterControllerStatus
func (ClusterControllerStatus) TableName() string {
	return "controller_status"
}

// TableName returns the table name for NodePoolControllerStatus
func (NodePoolControllerStatus) TableName() string {
	return "nodepool_controller_status"
}

// TableName returns the table name for ClusterEvent
func (ClusterEvent) TableName() string {
	return "cluster_events"
}

// BeforeCreate sets default values before creating a cluster event
func (ce *ClusterEvent) BeforeCreate() {
	if ce.ID == uuid.Nil {
		ce.ID = uuid.New()
	}
	if ce.PublishedAt.IsZero() {
		ce.PublishedAt = time.Now()
	}
}

// HasCondition checks if a condition type exists with the given status
func (cl ConditionList) HasCondition(conditionType, status string) bool {
	for _, condition := range cl {
		if condition.Type == conditionType && condition.Status == status {
			return true
		}
	}
	return false
}

// GetCondition returns a condition by type
func (cl ConditionList) GetCondition(conditionType string) *Condition {
	for _, condition := range cl {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

// SetCondition adds or updates a condition
func (cl *ConditionList) SetCondition(condition Condition) {
	condition.LastTransitionTime = time.Now()

	for i, existing := range *cl {
		if existing.Type == condition.Type {
			// Only update transition time if status changed
			if existing.Status != condition.Status {
				condition.LastTransitionTime = time.Now()
			} else {
				condition.LastTransitionTime = existing.LastTransitionTime
			}
			(*cl)[i] = condition
			return
		}
	}

	// Add new condition
	*cl = append(*cl, condition)
}

// RemoveCondition removes a condition by type
func (cl *ConditionList) RemoveCondition(conditionType string) {
	for i, condition := range *cl {
		if condition.Type == conditionType {
			*cl = append((*cl)[:i], (*cl)[i+1:]...)
			return
		}
	}
}

// IsHealthy returns true if the controller status indicates healthy state
func (ccs *ClusterControllerStatus) IsHealthy() bool {
	return ccs.Conditions.HasCondition("Available", "True") && ccs.LastError == nil
}

// IsReady returns true if the controller is ready
func (ccs *ClusterControllerStatus) IsReady() bool {
	return ccs.Conditions.HasCondition("Available", "True")
}

// HasErrors returns true if the controller has errors
func (ccs *ClusterControllerStatus) HasErrors() bool {
	return ccs.LastError != nil
}

// IsHealthy returns true if the node pool controller status indicates healthy state
func (npcs *NodePoolControllerStatus) IsHealthy() bool {
	return npcs.Conditions.HasCondition("Available", "True") && npcs.LastError == nil
}

// IsReady returns true if the node pool controller is ready
func (npcs *NodePoolControllerStatus) IsReady() bool {
	return npcs.Conditions.HasCondition("Available", "True")
}

// HasErrors returns true if the node pool controller has errors
func (npcs *NodePoolControllerStatus) HasErrors() bool {
	return npcs.LastError != nil
}