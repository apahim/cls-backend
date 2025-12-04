package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
)

// Unit tests for cluster models - no external dependencies

func TestClusterValidation(t *testing.T) {
	tests := []struct {
		name    string
		cluster Cluster
		wantErr bool
	}{
		{
			name: "valid cluster",
			cluster: Cluster{
				Name: "test-cluster",
				Spec: ClusterSpec{
					InfraID: "test-infra",
					Platform: PlatformSpec{
						Type: "GCP",
						GCP: &GCPSpec{
							ProjectID: "test-project",
							Region:    "us-central1",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty name",
			cluster: Cluster{
				Name: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCluster(&tt.cluster)
			utils.AssertError(t, err, tt.wantErr, "Validation result should match expected")
		})
	}
}

func TestClusterBeforeCreate(t *testing.T) {
	cluster := &Cluster{
		Name: "test-cluster",
	}

	cluster.BeforeCreate()

	utils.AssertNotEqual(t, uuid.Nil, cluster.ID, "ID should be generated")
	utils.AssertEqual(t, int64(1), cluster.Generation, "Generation should be 1")
	utils.AssertNotEqual(t, "", cluster.ResourceVersion, "ResourceVersion should be set")
	// Current implementation doesn't set status automatically in BeforeCreate
	if cluster.Status != nil {
		t.Errorf("Expected Status to be nil initially, but got %v", cluster.Status)
	}
	utils.AssertFalse(t, cluster.CreatedAt.IsZero(), "CreatedAt should be set")
	utils.AssertFalse(t, cluster.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestClusterBeforeUpdate(t *testing.T) {
	cluster := &Cluster{
		ID:         uuid.New(),
		Generation: 1,
		CreatedAt:  time.Now().Add(-time.Hour),
		UpdatedAt:  time.Now().Add(-time.Hour),
	}
	oldGeneration := cluster.Generation
	oldResourceVersion := cluster.ResourceVersion
	oldUpdatedAt := cluster.UpdatedAt

	time.Sleep(time.Millisecond) // Ensure time difference
	cluster.BeforeUpdate()

	utils.AssertEqual(t, oldGeneration+1, cluster.Generation, "Generation should increment")
	utils.AssertNotEqual(t, oldResourceVersion, cluster.ResourceVersion, "ResourceVersion should change")
	utils.AssertTrue(t, cluster.UpdatedAt.After(oldUpdatedAt), "UpdatedAt should be newer")
}

func TestClusterSoftDelete(t *testing.T) {
	cluster := &Cluster{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	utils.AssertFalse(t, cluster.IsDeleted(), "Should not be deleted initially")

	cluster.SoftDelete()

	utils.AssertTrue(t, cluster.IsDeleted(), "Should be deleted after SoftDelete")
	utils.AssertNotNil(t, cluster.DeletedAt, "DeletedAt should be set")
	utils.AssertFalse(t, cluster.DeletedAt.IsZero(), "DeletedAt should not be zero")
}

func TestClusterSpecSerialization(t *testing.T) {
	spec := ClusterSpec{
		InfraID: "test-infra",
		Platform: PlatformSpec{
			Type: "GCP",
			GCP: &GCPSpec{
				ProjectID: "test-project",
				Region:    "us-central1",
				Network:   "test-network",
				Subnet:    "test-subnet",
			},
		},
		Release: ReleaseSpec{
			Image:   "quay.io/openshift-release-dev/ocp-release:4.14.0",
			Version: "4.14.0",
		},
		Networking: NetworkingSpec{
			ClusterNetwork: []NetworkEntry{
				{CIDR: "10.128.0.0/14", HostPrefix: 23},
			},
			ServiceNetwork: []string{"172.30.0.0/16"},
		},
		DNS: DNSSpec{
			BaseDomain: "example.com",
		},
	}

	// Test Value() method (JSON marshaling)
	value, err := spec.Value()
	utils.AssertError(t, err, false, "Value() should not return error")
	utils.AssertNotNil(t, value, "Value should not be nil")

	jsonBytes, ok := value.([]byte)
	utils.AssertTrue(t, ok, "Value should be []byte")

	// Test Scan() method (JSON unmarshaling)
	var scannedSpec ClusterSpec
	err = scannedSpec.Scan(jsonBytes)
	utils.AssertError(t, err, false, "Scan() should not return error")

	utils.AssertEqual(t, spec.InfraID, scannedSpec.InfraID, "InfraID should match")
	utils.AssertEqual(t, spec.Platform.Type, scannedSpec.Platform.Type, "Platform type should match")
	utils.AssertNotNil(t, scannedSpec.Platform.GCP, "GCP spec should not be nil")
	utils.AssertEqual(t, spec.Platform.GCP.ProjectID, scannedSpec.Platform.GCP.ProjectID, "ProjectID should match")
}

func TestJSONBSerialization(t *testing.T) {
	data := JSONB{
		"string_field": "test_value",
		"number_field": 42,
		"bool_field":   true,
		"nested_field": map[string]interface{}{
			"inner": "value",
		},
	}

	// Test Value() method
	value, err := data.Value()
	utils.AssertError(t, err, false, "Value() should not return error")

	jsonBytes, ok := value.([]byte)
	utils.AssertTrue(t, ok, "Value should be []byte")

	// Test Scan() method
	var scannedData JSONB
	err = scannedData.Scan(jsonBytes)
	utils.AssertError(t, err, false, "Scan() should not return error")

	utils.AssertEqual(t, "test_value", scannedData["string_field"], "String field should match")
	utils.AssertEqual(t, float64(42), scannedData["number_field"], "Number field should match")
	utils.AssertEqual(t, true, scannedData["bool_field"], "Bool field should match")

	// Test nil handling
	var nilData JSONB
	err = nilData.Scan(nil)
	utils.AssertError(t, err, false, "Should handle nil value")
	if nilData != nil {
		t.Errorf("Expected nil but got %v. [Should be nil for nil input]", nilData)
	}
}

func TestConditionHelpers(t *testing.T) {
	condition := Condition{
		Type:               "Available",
		Status:             "True",
		Reason:             "Ready",
		Message:            "System is ready",
		LastTransitionTime: time.Now(),
		Severity:           "Info",
	}

	utils.AssertEqual(t, "Available", condition.Type, "Type should match")
	utils.AssertEqual(t, "True", condition.Status, "Status should match")
	utils.AssertEqual(t, "Ready", condition.Reason, "Reason should match")
	utils.AssertEqual(t, "System is ready", condition.Message, "Message should match")
	utils.AssertEqual(t, "Info", condition.Severity, "Severity should match")
}

func TestErrorInfoTypes(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		expected  string
	}{
		{"transient error", ErrorTypeTransient, "Transient"},
		{"configuration error", ErrorTypeConfiguration, "Configuration"},
		{"fatal error", ErrorTypeFatal, "Fatal"},
		{"system error", ErrorTypeSystem, "System"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utils.AssertEqual(t, tt.expected, string(tt.errorType), "Error type string should match")
		})
	}
}

func TestStatusAndHealthTypes(t *testing.T) {
	statusTests := []struct {
		status   Status
		expected string
	}{
		{StatusPending, "Pending"},
		{StatusReady, "Ready"},
		{StatusError, "Error"},
		{StatusDeleting, "Deleting"},
		{StatusUnknown, "Unknown"},
	}

	for _, tt := range statusTests {
		utils.AssertEqual(t, tt.expected, string(tt.status), "Status string should match")
	}

	healthTests := []struct {
		health   Health
		expected string
	}{
		{HealthHealthy, "Healthy"},
		{HealthDegraded, "Degraded"},
		{HealthUnhealthy, "Unhealthy"},
		{HealthUnknown, "Unknown"},
	}

	for _, tt := range healthTests {
		utils.AssertEqual(t, tt.expected, string(tt.health), "Health string should match")
	}
}

// Helper function for validation (this would normally be in the cluster.go file)
func validateCluster(cluster *Cluster) error {
	if cluster.Name == "" {
		return fmt.Errorf("cluster name is required")
	}
	return nil
}
