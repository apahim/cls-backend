package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Status represents the status of a resource
type Status string

// Health represents the health of a resource
type Health string

// Status constants
const (
	StatusPending  Status = "Pending"
	StatusReady    Status = "Ready"
	StatusError    Status = "Error"
	StatusDeleting Status = "Deleting"
	StatusUnknown  Status = "Unknown"
)

// Health constants
const (
	HealthHealthy   Health = "Healthy"
	HealthDegraded  Health = "Degraded"
	HealthUnhealthy Health = "Unhealthy"
	HealthUnknown   Health = "Unknown"
)

// Cluster represents a cluster in the database
type Cluster struct {
	ID              uuid.UUID          `json:"id" db:"id"`
	Name            string             `json:"name" db:"name"`
	TargetProjectID string             `json:"target_project_id" db:"target_project_id"`
	CreatedBy       string             `json:"created_by" db:"created_by"`
	Generation      int64              `json:"generation" db:"generation"`
	ResourceVersion string             `json:"resource_version" db:"resource_version"`
	Spec            ClusterSpec        `json:"spec" db:"spec"`
	Status          *ClusterStatusInfo `json:"status,omitempty" db:"status"`
	CreatedAt       time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at" db:"updated_at"`
	DeletedAt       *time.Time         `json:"deleted_at,omitempty" db:"deleted_at"`

	// Status management field
	StatusDirty bool `json:"-" db:"status_dirty"`
}

// ClusterWithObservedGeneration extends Cluster with observed generation for API responses
type ClusterWithObservedGeneration struct {
	Cluster
	ObservedGeneration *int64 `json:"observed_generation"`
}

// ClusterSpec represents the cluster specification
type ClusterSpec struct {
	InfraID                  string         `json:"infraID"`
	Platform                 PlatformSpec   `json:"platform"`
	Release                  ReleaseSpec    `json:"release"`
	Networking               NetworkingSpec `json:"networking"`
	DNS                      DNSSpec        `json:"dns"`
	ServiceAccountSigningKey string         `json:"serviceAccountSigningKey,omitempty"` // Base64-encoded PEM private key
	IssuerURL                string         `json:"issuerURL,omitempty"`                // OIDC issuer URL
}

// PlatformSpec represents platform-specific configuration
type PlatformSpec struct {
	Type string   `json:"type"`
	GCP  *GCPSpec `json:"gcp,omitempty"`
}

// GCPSpec represents GCP platform configuration
type GCPSpec struct {
	ProjectID        string                  `json:"projectID"`
	Region           string                  `json:"region"`
	Network          string                  `json:"network"`
	Subnet           string                  `json:"subnet"`
	EndpointAccess   string                  `json:"endpointAccess,omitempty"`
	WorkloadIdentity *WorkloadIdentityConfig `json:"workloadIdentity,omitempty"`
}

// WorkloadIdentityConfig represents GCP Workload Identity Federation configuration
type WorkloadIdentityConfig struct {
	ProjectNumber      string                 `json:"projectNumber"`
	PoolID             string                 `json:"poolID"`
	ProviderID         string                 `json:"providerID"`
	ServiceAccountsRef *WIFServiceAccountsRef `json:"serviceAccountsRef,omitempty"`
}

// WIFServiceAccountsRef represents GCP service account references for WIF
type WIFServiceAccountsRef struct {
	NodePoolEmail        string `json:"nodePoolEmail"`
	ControlPlaneEmail    string `json:"controlPlaneEmail"`
	CloudControllerEmail string `json:"cloudControllerEmail"`
}

// ReleaseSpec represents the OpenShift release configuration
type ReleaseSpec struct {
	Image   string `json:"image"`
	Version string `json:"version"`
}

// NetworkingSpec represents cluster networking configuration
type NetworkingSpec struct {
	ClusterNetwork []NetworkEntry `json:"clusterNetwork"`
	ServiceNetwork []string       `json:"serviceNetwork"`
	PodCIDR        string         `json:"podCIDR,omitempty"`
	ServiceCIDR    string         `json:"serviceCIDR,omitempty"`
}

// NetworkEntry represents a network CIDR entry
type NetworkEntry struct {
	CIDR       string `json:"cidr"`
	HostPrefix int    `json:"hostPrefix,omitempty"`
}

// DNSSpec represents DNS configuration
type DNSSpec struct {
	BaseDomain  string `json:"baseDomain"`
	PublicZone  string `json:"publicZone,omitempty"`
	PrivateZone string `json:"privateZone,omitempty"`
}

// ClusterStatusInfo represents the Kubernetes-like status block for clusters
type ClusterStatusInfo struct {
	ObservedGeneration int64       `json:"observedGeneration"`
	Conditions         []Condition `json:"conditions"`
	Phase              string      `json:"phase,omitempty"`   // Current lifecycle phase
	Message            string      `json:"message,omitempty"` // Human-readable status message
	Reason             string      `json:"reason,omitempty"`  // Machine-readable reason
	LastUpdateTime     time.Time   `json:"lastUpdateTime"`
}

// ClusterStatus represents the overall cluster status (DEPRECATED - use ClusterStatusInfo)
type ClusterStatus struct {
	Phase              string                      `json:"phase"`
	Health             string                      `json:"health"`
	Available          bool                        `json:"available"`
	Conditions         []Condition                 `json:"conditions"`
	Errors             []ErrorInfo                 `json:"errors"`
	ControllerStatuses map[string]ControllerStatus `json:"controllerStatuses"`
	Endpoints          *ClusterEndpoints           `json:"endpoints,omitempty"`
	LastStatusUpdate   *time.Time                  `json:"lastStatusUpdate,omitempty"`
}

// ClusterEndpoints represents cluster access endpoints
type ClusterEndpoints struct {
	APIServer  string `json:"apiServer,omitempty"`
	ConsoleURL string `json:"consoleURL,omitempty"`
	KubeConfig string `json:"kubeConfig,omitempty"`
}

// Condition represents a status condition
type Condition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"` // True, False, Unknown
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason"`
	Message            string    `json:"message"`
	Severity           string    `json:"severity,omitempty"` // Info, Warning, Error, Critical
}

// ErrorType represents the type of error
type ErrorType string

// Error type constants
const (
	ErrorTypeTransient     ErrorType = "Transient"
	ErrorTypeConfiguration ErrorType = "Configuration"
	ErrorTypeFatal         ErrorType = "Fatal"
	ErrorTypeSystem        ErrorType = "System"
)

// ErrorInfo represents error information
type ErrorInfo struct {
	ControllerName string            `json:"controllerName"`
	ErrorType      ErrorType         `json:"errorType"` // Transient, Configuration, Fatal, System
	ErrorCode      string            `json:"errorCode"`
	Message        string            `json:"message"`
	UserActionable bool              `json:"userActionable"`
	Suggestions    []string          `json:"suggestions,omitempty"`
	Details        map[string]string `json:"details,omitempty"`
	RetryAfter     *time.Duration    `json:"retryAfter,omitempty"`
	Timestamp      time.Time         `json:"timestamp"`
}

// ControllerStatus represents the status of a controller for a cluster
type ControllerStatus struct {
	ControllerName     string      `json:"controllerName"`
	ObservedGeneration int64       `json:"observedGeneration"`
	Conditions         []Condition `json:"conditions"`
	Metadata           JSONB       `json:"metadata,omitempty"`
	LastError          *ErrorInfo  `json:"lastError,omitempty"`
	LastUpdated        time.Time   `json:"lastUpdated"`
}

// JSONB represents a JSONB field that can be stored in PostgreSQL
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		// Return empty JSON object instead of nil to satisfy NOT NULL constraints
		return []byte("{}"), nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into JSONB", value)
	}

	return json.Unmarshal(bytes, j)
}

// Value implements the driver.Valuer interface for ClusterSpec
func (cs ClusterSpec) Value() (driver.Value, error) {
	return json.Marshal(cs)
}

// Scan implements the sql.Scanner interface for ClusterSpec
func (cs *ClusterSpec) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into ClusterSpec", value)
	}

	return json.Unmarshal(bytes, cs)
}

// Value implements the driver.Valuer interface for ClusterStatusInfo
func (csi ClusterStatusInfo) Value() (driver.Value, error) {
	return json.Marshal(csi)
}

// Scan implements the sql.Scanner interface for ClusterStatusInfo
func (csi *ClusterStatusInfo) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into ClusterStatusInfo", value)
	}

	return json.Unmarshal(bytes, csi)
}

// ToJSON converts ClusterStatusInfo to JSON bytes for database storage
func (csi *ClusterStatusInfo) ToJSON() ([]byte, error) {
	return json.Marshal(csi)
}

// ClusterListRequest represents a request to list clusters
type ClusterListRequest struct {
	Labels map[string]string `form:"-"`
	Status string            `form:"status"`
	Limit  int               `form:"limit"`
	Offset int               `form:"offset"`
}

// ClusterCreateRequest represents a request to create a cluster
type ClusterCreateRequest struct {
	Name            string      `json:"name" binding:"required"`
	TargetProjectID string      `json:"target_project_id,omitempty"`
	Spec            ClusterSpec `json:"spec" binding:"required"`
}

// ClusterUpdateRequest represents a request to update a cluster
type ClusterUpdateRequest struct {
	Spec ClusterSpec `json:"spec" binding:"required"`
}

// TableName returns the table name for the Cluster model
func (Cluster) TableName() string {
	return "clusters"
}

// BeforeCreate sets default values before creating a cluster
func (c *Cluster) BeforeCreate() {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.Generation == 0 {
		c.Generation = 1
	}
	if c.ResourceVersion == "" {
		c.ResourceVersion = uuid.New().String()
	}
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
}

// BeforeUpdate sets values before updating a cluster
func (c *Cluster) BeforeUpdate() {
	c.Generation++
	c.ResourceVersion = uuid.New().String()
	c.UpdatedAt = time.Now()
}

// IsDeleted returns true if the cluster is soft deleted
func (c *Cluster) IsDeleted() bool {
	return c.DeletedAt != nil
}

// SoftDelete marks the cluster as deleted
func (c *Cluster) SoftDelete() {
	now := time.Now()
	c.DeletedAt = &now
	c.UpdatedAt = now
}

// ClusterWithPermissions extends Cluster with user permissions information
type ClusterWithPermissions struct {
	*Cluster
	Permissions UserClusterPermissions `json:"permissions"`
}

// UserClusterPermissions represents user's permissions on a cluster
type UserClusterPermissions struct {
	CanView   bool   `json:"can_view"`
	CanEdit   bool   `json:"can_edit"`
	CanDelete bool   `json:"can_delete"`
	Role      string `json:"role"`
}

// BuildStatusFromAggregation creates a Kubernetes-like status from aggregation data
func (c *Cluster) BuildStatusFromAggregation(observedGeneration int64, readyCount, totalCount int, hasErrors bool) {
	now := time.Now()

	// Determine conditions based on aggregation data
	conditions := []Condition{}

	// Ready condition
	readyCondition := Condition{
		Type:               "Ready",
		LastTransitionTime: now,
	}

	// Available condition
	availableCondition := Condition{
		Type:               "Available",
		LastTransitionTime: now,
	}

	var phase, message, reason string

	if totalCount == 0 {
		// No controllers yet
		readyCondition.Status = "False"
		readyCondition.Reason = "NoControllers"
		readyCondition.Message = "No controllers have reported status yet"

		availableCondition.Status = "False"
		availableCondition.Reason = "NoControllers"
		availableCondition.Message = "No controllers have reported status yet"

		phase = "Pending"
		reason = "NoControllers"
		message = "Waiting for controllers to report status"

	} else if readyCount == totalCount && !hasErrors {
		// All controllers ready, no errors
		readyCondition.Status = "True"
		readyCondition.Reason = "AllControllersReady"
		readyCondition.Message = fmt.Sprintf("All %d controllers are ready", totalCount)

		availableCondition.Status = "True"
		availableCondition.Reason = "AllControllersReady"
		availableCondition.Message = fmt.Sprintf("All %d controllers are available", totalCount)

		phase = "Ready"
		reason = "AllControllersReady"
		message = fmt.Sprintf("Cluster is ready with %d controllers operational", totalCount)

	} else if readyCount > 0 {
		// Some controllers ready
		readyCondition.Status = "False"
		readyCondition.Reason = "PartialProgress"
		readyCondition.Message = fmt.Sprintf("%d of %d controllers are ready", readyCount, totalCount)

		if hasErrors {
			availableCondition.Status = "False"
			availableCondition.Reason = "ControllersWithErrors"
			availableCondition.Message = fmt.Sprintf("Some controllers have errors (%d ready of %d)", readyCount, totalCount)

			phase = "Progressing"
			reason = "ControllersWithErrors"
			message = fmt.Sprintf("Cluster is progressing but some controllers have errors (%d/%d ready)", readyCount, totalCount)
		} else {
			availableCondition.Status = "False"
			availableCondition.Reason = "PartialProgress"
			availableCondition.Message = fmt.Sprintf("Controllers are still working (%d ready of %d)", readyCount, totalCount)

			phase = "Progressing"
			reason = "PartialProgress"
			message = fmt.Sprintf("Cluster is progressing (%d/%d controllers ready)", readyCount, totalCount)
		}

	} else {
		// No controllers ready
		readyCondition.Status = "False"
		readyCondition.Reason = "NoControllersReady"
		readyCondition.Message = fmt.Sprintf("None of %d controllers are ready", totalCount)

		availableCondition.Status = "False"
		availableCondition.Reason = "NoControllersReady"
		availableCondition.Message = fmt.Sprintf("None of %d controllers are available", totalCount)

		phase = "Failed"
		reason = "NoControllersReady"
		message = fmt.Sprintf("Cluster failed - no controllers are operational (%d controllers exist)", totalCount)
	}

	conditions = append(conditions, readyCondition, availableCondition)

	// Create the Kubernetes-like status
	c.Status = &ClusterStatusInfo{
		ObservedGeneration: observedGeneration,
		Conditions:         conditions,
		Phase:              phase,
		Message:            message,
		Reason:             reason,
		LastUpdateTime:     now,
	}
}
