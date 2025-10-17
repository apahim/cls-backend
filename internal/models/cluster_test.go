package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
)

func TestCluster(t *testing.T) {
	cluster := &Cluster{
		ID:              uuid.New(),
		Name:            "test-cluster",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: ClusterSpec{
			InfraID: "test-infra",
			Platform: PlatformSpec{
				Type: "GCP",
				GCP: &GCPSpec{
					ProjectID: "test-project",
					Region:    "us-central1",
					Zone:      "us-central1-a",
				},
			},
			Release: ReleaseSpec{
				Image: "quay.io/openshift-release-dev/ocp-release:4.14.0",
			},
		},
		Status: &ClusterStatusInfo{
			Phase: "Pending",
			Conditions: []Condition{},
			ObservedGeneration: 1,
			LastUpdateTime: time.Now(),
		},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Test that the cluster has all required fields
	utils.AssertNotEqual(t, uuid.Nil, cluster.ID, "Cluster ID should not be nil")
	utils.AssertEqual(t, "test-cluster", cluster.Name, "Cluster name")
	utils.AssertEqual(t, int64(1), cluster.Generation, "Cluster generation")
	utils.AssertNotNil(t, cluster.Status, "Cluster status should not be nil")
	utils.AssertEqual(t, "Pending", cluster.Status.Phase, "Cluster status phase")
	utils.AssertEqual(t, int64(1), cluster.Status.ObservedGeneration, "Cluster observed generation")

	// Test platform spec
	utils.AssertEqual(t, "GCP", cluster.Spec.Platform.Type, "Platform type")
	utils.AssertNotNil(t, cluster.Spec.Platform.GCP, "GCP spec should not be nil")
	utils.AssertEqual(t, "test-project", cluster.Spec.Platform.GCP.ProjectID, "GCP project ID")
	utils.AssertEqual(t, "us-central1", cluster.Spec.Platform.GCP.Region, "GCP region")
}

func TestClusterSpec_Value(t *testing.T) {
	spec := ClusterSpec{
		InfraID: "test-infra",
		Platform: PlatformSpec{
			Type: "GCP",
			GCP: &GCPSpec{
				ProjectID: "test-project",
				Region:    "us-central1",
			},
		},
		Release: ReleaseSpec{
			Image: "test-image",
		},
		Networking: NetworkingSpec{
			ServiceCIDR: "10.128.0.0/14",
			PodCIDR:     "10.132.0.0/14",
		},
		DNS: DNSSpec{
			BaseDomain: "example.com",
		},
	}

	value, err := spec.Value()
	utils.AssertError(t, err, false, "Value() should not return error")

	// Should return JSON bytes
	jsonBytes, ok := value.([]byte)
	utils.AssertTrue(t, ok, "Value should return []byte")

	// Should be valid JSON
	var decoded ClusterSpec
	err = json.Unmarshal(jsonBytes, &decoded)
	utils.AssertError(t, err, false, "Should be valid JSON")
	utils.AssertEqual(t, spec.InfraID, decoded.InfraID, "InfraID should match")
	utils.AssertEqual(t, spec.Platform.Type, decoded.Platform.Type, "Platform type should match")
}

func TestClusterSpec_Scan(t *testing.T) {
	originalSpec := ClusterSpec{
		InfraID: "test-infra",
		Platform: PlatformSpec{
			Type: "GCP",
			GCP: &GCPSpec{
				ProjectID: "test-project",
				Region:    "us-central1",
			},
		},
	}

	// Convert to JSON
	jsonBytes, err := json.Marshal(originalSpec)
	utils.AssertError(t, err, false, "Should marshal to JSON")

	// Test scanning from []byte
	var spec ClusterSpec
	err = spec.Scan(jsonBytes)
	utils.AssertError(t, err, false, "Should scan from []byte")
	utils.AssertEqual(t, originalSpec.InfraID, spec.InfraID, "InfraID should match")
	utils.AssertEqual(t, originalSpec.Platform.Type, spec.Platform.Type, "Platform type should match")

	// Test scanning from nil
	var nilSpec ClusterSpec
	err = nilSpec.Scan(nil)
	utils.AssertError(t, err, false, "Should handle nil value")

	// Test scanning from invalid type
	var invalidSpec ClusterSpec
	err = invalidSpec.Scan("invalid")
	utils.AssertError(t, err, true, "Should return error for invalid type")
}

func TestJSONB_Value(t *testing.T) {
	data := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	jsonb := JSONB(data)
	value, err := jsonb.Value()
	utils.AssertError(t, err, false, "Value() should not return error")

	jsonBytes, ok := value.([]byte)
	utils.AssertTrue(t, ok, "Value should return []byte")

	var decoded map[string]interface{}
	err = json.Unmarshal(jsonBytes, &decoded)
	utils.AssertError(t, err, false, "Should be valid JSON")
	utils.AssertEqual(t, "value1", decoded["key1"], "String value should match")
	utils.AssertEqual(t, float64(123), decoded["key2"], "Number value should match") // JSON unmarshals numbers as float64
	utils.AssertEqual(t, true, decoded["key3"], "Boolean value should match")
}

func TestJSONB_Scan(t *testing.T) {
	originalData := map[string]interface{}{
		"key1": "value1",
		"key2": float64(123),
		"key3": true,
	}

	jsonBytes, err := json.Marshal(originalData)
	utils.AssertError(t, err, false, "Should marshal to JSON")

	// Test scanning from []byte
	var jsonb JSONB
	err = jsonb.Scan(jsonBytes)
	utils.AssertError(t, err, false, "Should scan from []byte")

	utils.AssertNotNil(t, jsonb, "JSONB should not be nil")
	// Check that data was properly unmarshaled
	if jsonData, ok := jsonb["key1"]; ok {
		utils.AssertEqual(t, "value1", jsonData, "String value should match")
	}

	// Test scanning from nil
	var nilJSONB JSONB
	err = nilJSONB.Scan(nil)
	utils.AssertError(t, err, false, "Should handle nil value")
	if nilJSONB != nil {
		t.Errorf("Expected nil but got %v. [Should be nil for nil input]", nilJSONB)
	}

	// Test scanning from invalid type
	var invalidJSONB JSONB
	err = invalidJSONB.Scan(123)
	utils.AssertError(t, err, true, "Should return error for invalid type")
}

func TestCondition(t *testing.T) {
	now := time.Now()
	condition := Condition{
		Type:               "Available",
		Status:             "True",
		Reason:             "Ready",
		Message:            "Cluster is ready",
		LastTransitionTime: now,
	}

	utils.AssertEqual(t, "Available", condition.Type, "Condition type")
	utils.AssertEqual(t, "True", condition.Status, "Condition status")
	utils.AssertEqual(t, "Ready", condition.Reason, "Condition reason")
	utils.AssertEqual(t, "Cluster is ready", condition.Message, "Condition message")
	utils.AssertEqual(t, now, condition.LastTransitionTime, "Condition last transition time")
}

func TestErrorInfo(t *testing.T) {
	retryAfter := 5 * time.Minute
	errorInfo := ErrorInfo{
		ErrorType:      ErrorTypeFatal,
		ErrorCode:      "500",
		Message:        "Internal server error",
		Details:        map[string]string{"component": "api-server"},
		UserActionable: false,
		RetryAfter:     &retryAfter,
		Timestamp:      time.Now(),
	}

	utils.AssertEqual(t, ErrorTypeFatal, errorInfo.ErrorType, "Error type")
	utils.AssertEqual(t, "500", errorInfo.ErrorCode, "Error code")
	utils.AssertEqual(t, "Internal server error", errorInfo.Message, "Error message")
	utils.AssertEqual(t, false, errorInfo.UserActionable, "User actionable")
	utils.AssertNotNil(t, errorInfo.RetryAfter, "Retry after should not be nil")
	utils.AssertEqual(t, retryAfter, *errorInfo.RetryAfter, "Retry after duration")
}

func TestBeforeCreate(t *testing.T) {
	cluster := &Cluster{}
	cluster.BeforeCreate()

	utils.AssertNotEqual(t, uuid.Nil, cluster.ID, "ID should be generated")
	utils.AssertFalse(t, cluster.CreatedAt.IsZero(), "CreatedAt should be set")
	utils.AssertFalse(t, cluster.UpdatedAt.IsZero(), "UpdatedAt should be set")
	utils.AssertEqual(t, cluster.CreatedAt, cluster.UpdatedAt, "CreatedAt and UpdatedAt should be the same")
}

func TestBeforeUpdate(t *testing.T) {
	cluster := &Cluster{
		ID:        uuid.New(),
		CreatedAt: time.Now().Add(-time.Hour), // Set to 1 hour ago
		UpdatedAt: time.Now().Add(-time.Hour), // Set to 1 hour ago
	}

	originalCreatedAt := cluster.CreatedAt
	originalUpdatedAt := cluster.UpdatedAt

	// Sleep a tiny bit to ensure different timestamp
	time.Sleep(time.Millisecond)

	cluster.BeforeUpdate()

	utils.AssertEqual(t, originalCreatedAt, cluster.CreatedAt, "CreatedAt should not change")
	utils.AssertTrue(t, cluster.UpdatedAt.After(originalUpdatedAt), "UpdatedAt should be updated")
}

func TestTableName(t *testing.T) {
	cluster := Cluster{}
	utils.AssertEqual(t, "clusters", cluster.TableName(), "Table name should be 'clusters'")
}

func TestNodePool(t *testing.T) {
	clusterID := uuid.New()
	nodepool := &NodePool{
		ID:              uuid.New(),
		ClusterID:       clusterID,
		Name:            "test-nodepool",
		Generation:      1,
		ResourceVersion: "v1",
		Spec: NodePoolSpec{
			ClusterName: "test-cluster",
			Platform: NodePoolPlatformSpec{
				Type: "GCP",
				GCP: &NodePoolGCPSpec{
					InstanceType: "n1-standard-4",
					DiskType:     "pd-standard",
					DiskSizeGB:   100,
				},
			},
			Replicas: func() *int32 { i := int32(3); return &i }(),
		},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	utils.AssertNotEqual(t, uuid.Nil, nodepool.ID, "NodePool ID should not be nil")
	utils.AssertEqual(t, clusterID, nodepool.ClusterID, "NodePool cluster ID")
	utils.AssertEqual(t, "test-nodepool", nodepool.Name, "NodePool name")
	utils.AssertEqual(t, int64(1), nodepool.Generation, "NodePool generation")
	// NodePool doesn't have status fields in current implementation

	// Test nodepool spec
	utils.AssertEqual(t, "test-cluster", nodepool.Spec.ClusterName, "Cluster name in spec")
	utils.AssertEqual(t, "GCP", nodepool.Spec.Platform.Type, "Platform type")
	utils.AssertNotNil(t, nodepool.Spec.Platform.GCP, "GCP spec should not be nil")
	utils.AssertEqual(t, "n1-standard-4", nodepool.Spec.Platform.GCP.InstanceType, "Instance type")
	utils.AssertNotNil(t, nodepool.Spec.Replicas, "Replicas should not be nil")
	utils.AssertEqual(t, int32(3), *nodepool.Spec.Replicas, "Replicas count")
}

func TestNodePoolSpec_Value(t *testing.T) {
	spec := NodePoolSpec{
		ClusterName: "test-cluster",
		Platform: NodePoolPlatformSpec{
			Type: "GCP",
			GCP: &NodePoolGCPSpec{
				InstanceType: "n1-standard-4",
				DiskType:     "pd-standard",
				DiskSizeGB:   100,
			},
		},
		Replicas: func() *int32 { i := int32(3); return &i }(),
	}

	value, err := spec.Value()
	utils.AssertError(t, err, false, "Value() should not return error")

	jsonBytes, ok := value.([]byte)
	utils.AssertTrue(t, ok, "Value should return []byte")

	var decoded NodePoolSpec
	err = json.Unmarshal(jsonBytes, &decoded)
	utils.AssertError(t, err, false, "Should be valid JSON")
	utils.AssertEqual(t, spec.ClusterName, decoded.ClusterName, "Cluster name should match")
	utils.AssertNotNil(t, decoded.Replicas, "Decoded replicas should not be nil")
	utils.AssertEqual(t, *spec.Replicas, *decoded.Replicas, "Replicas should match")
}
