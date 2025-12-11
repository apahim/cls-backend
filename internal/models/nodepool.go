package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NodePool represents a node pool in the database
type NodePool struct {
	ID              uuid.UUID           `json:"id" db:"id"`
	ClusterID       uuid.UUID           `json:"cluster_id" db:"cluster_id"`
	Name            string              `json:"name" db:"name"`
	CreatedBy       string              `json:"created_by,omitempty" db:"created_by"` // User who created the nodepool
	Generation      int64               `json:"generation" db:"generation"`
	ResourceVersion string              `json:"resource_version" db:"resource_version"`
	Spec            NodePoolSpec        `json:"spec" db:"spec"`
	Status          *NodePoolStatusInfo `json:"status,omitempty" db:"status"` // Aggregated Kubernetes-like status
	CreatedAt       time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at" db:"updated_at"`
	DeletedAt       *time.Time          `json:"deleted_at,omitempty" db:"deleted_at"`

	// Status management (not serialized to JSON)
	StatusDirty bool `json:"-" db:"status_dirty"` // Triggers status recalculation when TRUE
}

// NodePoolSpec represents the node pool specification
type NodePoolSpec struct {
	Replicas         *int32               `json:"replicas,omitempty"`
	Management       NodePoolManagement   `json:"management"`
	Platform         NodePoolPlatformSpec `json:"platform"`
	Release          NodePoolReleaseSpec  `json:"release"`
	NodeDrainTimeout string               `json:"nodeDrainTimeout,omitempty"`
}

// NodePoolManagement represents node pool management configuration
type NodePoolManagement struct {
	UpgradeType string                 `json:"upgradeType,omitempty"` // Replace, InPlace
	AutoRepair  bool                   `json:"autoRepair"`
	Replace     *NodePoolReplaceConfig `json:"replace,omitempty"`
}

// NodePoolReplaceConfig represents node replacement configuration
type NodePoolReplaceConfig struct {
	Strategy      string               `json:"strategy,omitempty"` // RollingUpdate, OnDelete
	RollingUpdate *RollingUpdateConfig `json:"rollingUpdate,omitempty"`
}

// RollingUpdateConfig represents rolling update configuration
type RollingUpdateConfig struct {
	MaxUnavailable *string `json:"maxUnavailable,omitempty"`
	MaxSurge       *string `json:"maxSurge,omitempty"`
}

// NodePoolPlatformSpec represents platform-specific node pool configuration
type NodePoolPlatformSpec struct {
	Type string           `json:"type"`
	GCP  *NodePoolGCPSpec `json:"gcp,omitempty"`
}

// NodePoolGCPSpec represents GCP-specific node pool configuration
type NodePoolGCPSpec struct {
	InstanceType   string            `json:"instanceType"`
	RootVolume     *RootVolumeSpec   `json:"rootVolume,omitempty"`
	Subnet         string            `json:"subnet,omitempty"`
	ServiceAccount string            `json:"serviceAccount,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	Taints         []TaintSpec       `json:"taints,omitempty"`
	DiskType       string            `json:"diskType,omitempty"`   // For backward compatibility
	DiskSizeGB     int               `json:"diskSizeGB,omitempty"` // For backward compatibility
}

// RootVolumeSpec represents root volume configuration
type RootVolumeSpec struct {
	Size int    `json:"size"` // Size in GB
	Type string `json:"type"` // pd-standard, pd-ssd, pd-balanced
}

// TaintSpec represents a node taint
type TaintSpec struct {
	Key    string `json:"key"`
	Value  string `json:"value,omitempty"`
	Effect string `json:"effect"` // NoSchedule, PreferNoSchedule, NoExecute
}

// NodePoolReleaseSpec represents the release configuration for node pool
type NodePoolReleaseSpec struct {
	Image   string `json:"image"`
	Version string `json:"version"`
}

// NodePoolStatusInfo represents Kubernetes-like status for nodepools
// This structure mirrors ClusterStatusInfo for consistent status aggregation
type NodePoolStatusInfo struct {
	ObservedGeneration int64       `json:"observedGeneration"` // Last generation processed by controllers
	Conditions         []Condition `json:"conditions"`         // Ready, Available conditions
	Phase              string      `json:"phase,omitempty"`    // Pending, Progressing, Ready, Failed
	Message            string      `json:"message,omitempty"`  // Human-readable status message
	Reason             string      `json:"reason,omitempty"`   // Machine-readable reason code
	LastUpdateTime     time.Time   `json:"lastUpdateTime"`     // When status was last calculated
}

// Value implements driver.Valuer for NodePoolStatusInfo (database serialization)
func (npsi NodePoolStatusInfo) Value() (driver.Value, error) {
	return json.Marshal(npsi)
}

// Scan implements sql.Scanner for NodePoolStatusInfo (database deserialization)
func (npsi *NodePoolStatusInfo) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into NodePoolStatusInfo", value)
	}
	return json.Unmarshal(bytes, npsi)
}

// ToJSON converts NodePoolStatusInfo to JSON bytes for database storage
func (npsi *NodePoolStatusInfo) ToJSON() ([]byte, error) {
	return json.Marshal(npsi)
}

// NodePoolStatus - DEPRECATED: Use NodePoolStatusInfo instead
// This struct is kept for backward compatibility only. New code should use
// NodePoolStatusInfo which provides Kubernetes-like status aggregation with
// Ready/Available conditions, phase tracking, and generation awareness.
type NodePoolStatus struct {
	Phase               string      `json:"phase"`
	Health              string      `json:"health"`
	ReadyReplicas       int32       `json:"readyReplicas"`
	AvailableReplicas   int32       `json:"availableReplicas"`
	UnavailableReplicas int32       `json:"unavailableReplicas"`
	Conditions          []Condition `json:"conditions"`
	Errors              []ErrorInfo `json:"errors"`
	LastStatusUpdate    *time.Time  `json:"lastStatusUpdate,omitempty"`
}

// Value implements the driver.Valuer interface for NodePoolSpec
func (nps NodePoolSpec) Value() (driver.Value, error) {
	return json.Marshal(nps)
}

// Scan implements the sql.Scanner interface for NodePoolSpec
func (nps *NodePoolSpec) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into NodePoolSpec", value)
	}

	return json.Unmarshal(bytes, nps)
}

// NodePoolListRequest represents a request to list node pools
type NodePoolListRequest struct {
	ClusterID uuid.UUID `form:"-"`
	Status    string    `form:"status"`
	Limit     int       `form:"limit"`
	Offset    int       `form:"offset"`
}

// NodePoolCreateRequest represents a request to create a node pool
type NodePoolCreateRequest struct {
	Name string       `json:"name" binding:"required"`
	Spec NodePoolSpec `json:"spec" binding:"required"`
}

// NodePoolUpdateRequest represents a request to update a node pool
type NodePoolUpdateRequest struct {
	Spec NodePoolSpec `json:"spec" binding:"required"`
}

// TableName returns the table name for the NodePool model
func (NodePool) TableName() string {
	return "nodepools"
}

// BeforeCreate sets default values before creating a node pool
func (np *NodePool) BeforeCreate() {
	if np.ID == uuid.Nil {
		np.ID = uuid.New()
	}
	if np.Generation == 0 {
		np.Generation = 1
	}
	if np.ResourceVersion == "" {
		np.ResourceVersion = uuid.New().String()
	}
	now := time.Now()
	np.CreatedAt = now
	np.UpdatedAt = now
}

// BeforeUpdate sets values before updating a node pool
func (np *NodePool) BeforeUpdate() {
	np.Generation++
	np.ResourceVersion = uuid.New().String()
	np.UpdatedAt = time.Now()
}

// IsDeleted returns true if the node pool is soft deleted
func (np *NodePool) IsDeleted() bool {
	return np.DeletedAt != nil
}

// SoftDelete marks the node pool as deleted
func (np *NodePool) SoftDelete() {
	now := time.Now()
	np.DeletedAt = &now
	np.UpdatedAt = now
}
