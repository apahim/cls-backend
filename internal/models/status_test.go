package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
)

func TestConditionList_Value(t *testing.T) {
	conditions := ConditionList{
		{
			Type:               "Available",
			Status:             "True",
			Reason:             "Ready",
			Message:            "Resource is ready",
			LastTransitionTime: time.Now(),
		},
		{
			Type:               "Progressing",
			Status:             "False",
			Reason:             "Complete",
			Message:            "Resource provisioning complete",
			LastTransitionTime: time.Now(),
		},
	}

	value, err := conditions.Value()
	utils.AssertError(t, err, false, "Value() should not return error")

	jsonBytes, ok := value.([]byte)
	utils.AssertTrue(t, ok, "Value should return []byte")

	var decoded []Condition
	err = json.Unmarshal(jsonBytes, &decoded)
	utils.AssertError(t, err, false, "Should be valid JSON")
	utils.AssertEqual(t, 2, len(decoded), "Should have 2 conditions")
	utils.AssertEqual(t, "Available", decoded[0].Type, "First condition type")
	utils.AssertEqual(t, "Progressing", decoded[1].Type, "Second condition type")
}

func TestConditionList_ValueNil(t *testing.T) {
	var conditions ConditionList
	conditions = nil

	value, err := conditions.Value()
	utils.AssertError(t, err, false, "Value() should not return error for nil")

	jsonBytes, ok := value.([]byte)
	utils.AssertTrue(t, ok, "Value should return []byte")

	var decoded []Condition
	err = json.Unmarshal(jsonBytes, &decoded)
	utils.AssertError(t, err, false, "Should be valid JSON")
	utils.AssertEqual(t, 0, len(decoded), "Should have 0 conditions for nil input")
}

func TestConditionList_Scan(t *testing.T) {
	originalConditions := []Condition{
		{
			Type:    "Available",
			Status:  "True",
			Reason:  "Ready",
			Message: "Resource is ready",
		},
		{
			Type:    "Progressing",
			Status:  "False",
			Reason:  "Complete",
			Message: "Resource provisioning complete",
		},
	}

	jsonBytes, err := json.Marshal(originalConditions)
	utils.AssertError(t, err, false, "Should marshal to JSON")

	var conditions ConditionList
	err = conditions.Scan(jsonBytes)
	utils.AssertError(t, err, false, "Should scan from []byte")
	utils.AssertEqual(t, 2, len(conditions), "Should have 2 conditions")
	utils.AssertEqual(t, "Available", conditions[0].Type, "First condition type")
	utils.AssertEqual(t, "Progressing", conditions[1].Type, "Second condition type")
}

func TestConditionList_ScanNil(t *testing.T) {
	var conditions ConditionList
	err := conditions.Scan(nil)
	utils.AssertError(t, err, false, "Should handle nil value")
	utils.AssertEqual(t, 0, len(conditions), "Should have 0 conditions for nil")
}

func TestConditionList_ScanInvalid(t *testing.T) {
	var conditions ConditionList
	err := conditions.Scan("invalid")
	utils.AssertError(t, err, true, "Should return error for invalid type")
}

func TestErrorInfo_Value(t *testing.T) {
	errorInfo := &ErrorInfo{
		ErrorType:      ErrorTypeFatal,
		ErrorCode:      "500",
		Message:        "Internal server error",
		UserActionable: false,
		Timestamp:      time.Now(),
	}

	value, err := errorInfo.Value()
	utils.AssertError(t, err, false, "Value() should not return error")

	jsonBytes, ok := value.([]byte)
	utils.AssertTrue(t, ok, "Value should return []byte")

	var decoded ErrorInfo
	err = json.Unmarshal(jsonBytes, &decoded)
	utils.AssertError(t, err, false, "Should be valid JSON")
	utils.AssertEqual(t, ErrorTypeFatal, decoded.ErrorType, "Error type should match")
	utils.AssertEqual(t, "500", decoded.ErrorCode, "Error code should match")
}

func TestErrorInfo_ValueNil(t *testing.T) {
	var errorInfo *ErrorInfo
	value, err := errorInfo.Value()
	utils.AssertError(t, err, false, "Value() should not return error for nil")
	utils.AssertNil(t, value, "Value should be nil for nil ErrorInfo")
}

func TestErrorInfo_Scan(t *testing.T) {
	originalError := ErrorInfo{
		ErrorType:      ErrorTypeConfiguration,
		ErrorCode:      "400",
		Message:        "Invalid configuration",
		UserActionable: true,
		Timestamp:      time.Now(),
	}

	jsonBytes, err := json.Marshal(originalError)
	utils.AssertError(t, err, false, "Should marshal to JSON")

	var errorInfo ErrorInfo
	err = errorInfo.Scan(jsonBytes)
	utils.AssertError(t, err, false, "Should scan from []byte")
	utils.AssertEqual(t, ErrorTypeConfiguration, errorInfo.ErrorType, "Error type should match")
	utils.AssertEqual(t, "400", errorInfo.ErrorCode, "Error code should match")
	utils.AssertEqual(t, true, errorInfo.UserActionable, "User actionable should match")
}

func TestErrorInfo_ScanNil(t *testing.T) {
	var errorInfo ErrorInfo
	err := errorInfo.Scan(nil)
	utils.AssertError(t, err, false, "Should handle nil value")
}

func TestErrorInfo_ScanInvalid(t *testing.T) {
	var errorInfo ErrorInfo
	err := errorInfo.Scan(123)
	utils.AssertError(t, err, true, "Should return error for invalid type")
}

func TestConditionList_HasCondition(t *testing.T) {
	conditions := ConditionList{
		{Type: "Available", Status: "True"},
		{Type: "Progressing", Status: "False"},
		{Type: "Degraded", Status: "Unknown"},
	}

	utils.AssertTrue(t, conditions.HasCondition("Available", "True"), "Should find Available=True")
	utils.AssertTrue(t, conditions.HasCondition("Progressing", "False"), "Should find Progressing=False")
	utils.AssertFalse(t, conditions.HasCondition("Available", "False"), "Should not find Available=False")
	utils.AssertFalse(t, conditions.HasCondition("NonExistent", "True"), "Should not find non-existent condition")
}

func TestConditionList_GetCondition(t *testing.T) {
	conditions := ConditionList{
		{Type: "Available", Status: "True", Message: "Ready"},
		{Type: "Progressing", Status: "False", Message: "Complete"},
	}

	available := conditions.GetCondition("Available")
	utils.AssertNotNil(t, available, "Should find Available condition")
	utils.AssertEqual(t, "Available", available.Type, "Condition type should match")
	utils.AssertEqual(t, "True", available.Status, "Condition status should match")
	utils.AssertEqual(t, "Ready", available.Message, "Condition message should match")

	nonExistent := conditions.GetCondition("NonExistent")
	if nonExistent != nil {
		t.Errorf("Expected nil but got %v. [Should not find non-existent condition]", nonExistent)
	}
}

func TestConditionList_SetCondition(t *testing.T) {
	var conditions ConditionList

	// Add new condition
	newCondition := Condition{
		Type:    "Available",
		Status:  "True",
		Reason:  "Ready",
		Message: "Resource is ready",
	}

	conditions.SetCondition(newCondition)
	utils.AssertEqual(t, 1, len(conditions), "Should have 1 condition after adding")
	utils.AssertEqual(t, "Available", conditions[0].Type, "Condition type should match")
	utils.AssertFalse(t, conditions[0].LastTransitionTime.IsZero(), "LastTransitionTime should be set")

	// Update existing condition with same status
	originalTransitionTime := conditions[0].LastTransitionTime
	time.Sleep(time.Millisecond) // Ensure different timestamp

	updateCondition := Condition{
		Type:    "Available",
		Status:  "True", // Same status
		Reason:  "StillReady",
		Message: "Resource is still ready",
	}

	conditions.SetCondition(updateCondition)
	utils.AssertEqual(t, 1, len(conditions), "Should still have 1 condition")
	utils.AssertEqual(t, "StillReady", conditions[0].Reason, "Reason should be updated")
	utils.AssertEqual(t, originalTransitionTime, conditions[0].LastTransitionTime, "Transition time should not change for same status")

	// Update existing condition with different status
	time.Sleep(time.Millisecond)
	changeCondition := Condition{
		Type:    "Available",
		Status:  "False", // Different status
		Reason:  "NotReady",
		Message: "Resource is not ready",
	}

	conditions.SetCondition(changeCondition)
	utils.AssertEqual(t, 1, len(conditions), "Should still have 1 condition")
	utils.AssertEqual(t, "False", conditions[0].Status, "Status should be updated")
	utils.AssertEqual(t, "NotReady", conditions[0].Reason, "Reason should be updated")
	utils.AssertTrue(t, conditions[0].LastTransitionTime.After(originalTransitionTime), "Transition time should be updated for status change")
}

func TestConditionList_RemoveCondition(t *testing.T) {
	conditions := ConditionList{
		{Type: "Available", Status: "True"},
		{Type: "Progressing", Status: "False"},
		{Type: "Degraded", Status: "Unknown"},
	}

	// Remove middle condition
	conditions.RemoveCondition("Progressing")
	utils.AssertEqual(t, 2, len(conditions), "Should have 2 conditions after removal")
	if conditions.GetCondition("Progressing") != nil {
		t.Errorf("Expected nil but got %v. [Progressing condition should be removed]", conditions.GetCondition("Progressing"))
	}
	utils.AssertNotNil(t, conditions.GetCondition("Available"), "Available condition should still exist")
	utils.AssertNotNil(t, conditions.GetCondition("Degraded"), "Degraded condition should still exist")

	// Remove non-existent condition
	conditions.RemoveCondition("NonExistent")
	utils.AssertEqual(t, 2, len(conditions), "Should still have 2 conditions")

	// Remove first condition
	conditions.RemoveCondition("Available")
	utils.AssertEqual(t, 1, len(conditions), "Should have 1 condition after removal")
	utils.AssertEqual(t, "Degraded", conditions[0].Type, "Remaining condition should be Degraded")
}

func TestClusterControllerStatus_IsHealthy(t *testing.T) {
	clusterID := uuid.New()

	// Healthy status
	healthyStatus := &ClusterControllerStatus{
		ClusterID:      clusterID,
		ControllerName: "test-controller",
		Conditions: ConditionList{
			{Type: "Available", Status: "True"},
		},
		LastError: nil,
	}
	utils.AssertTrue(t, healthyStatus.IsHealthy(), "Should be healthy with Available=True and no error")

	// Unhealthy - not available
	unavailableStatus := &ClusterControllerStatus{
		ClusterID:      clusterID,
		ControllerName: "test-controller",
		Conditions: ConditionList{
			{Type: "Available", Status: "False"},
		},
		LastError: nil,
	}
	utils.AssertFalse(t, unavailableStatus.IsHealthy(), "Should not be healthy with Available=False")

	// Unhealthy - has error
	errorStatus := &ClusterControllerStatus{
		ClusterID:      clusterID,
		ControllerName: "test-controller",
		Conditions: ConditionList{
			{Type: "Available", Status: "True"},
		},
		LastError: &ErrorInfo{ErrorType: ErrorTypeTransient, Message: "Temporary error"},
	}
	utils.AssertFalse(t, errorStatus.IsHealthy(), "Should not be healthy with error")
}

func TestClusterControllerStatus_IsReady(t *testing.T) {
	clusterID := uuid.New()

	readyStatus := &ClusterControllerStatus{
		ClusterID:      clusterID,
		ControllerName: "test-controller",
		Conditions: ConditionList{
			{Type: "Available", Status: "True"},
		},
	}
	utils.AssertTrue(t, readyStatus.IsReady(), "Should be ready with Available=True")

	notReadyStatus := &ClusterControllerStatus{
		ClusterID:      clusterID,
		ControllerName: "test-controller",
		Conditions: ConditionList{
			{Type: "Available", Status: "False"},
		},
	}
	utils.AssertFalse(t, notReadyStatus.IsReady(), "Should not be ready with Available=False")
}

func TestClusterControllerStatus_HasErrors(t *testing.T) {
	clusterID := uuid.New()

	noErrorStatus := &ClusterControllerStatus{
		ClusterID:      clusterID,
		ControllerName: "test-controller",
		LastError:      nil,
	}
	utils.AssertFalse(t, noErrorStatus.HasErrors(), "Should not have errors when LastError is nil")

	withErrorStatus := &ClusterControllerStatus{
		ClusterID:      clusterID,
		ControllerName: "test-controller",
		LastError:      &ErrorInfo{ErrorType: ErrorTypeTransient, Message: "Error occurred"},
	}
	utils.AssertTrue(t, withErrorStatus.HasErrors(), "Should have errors when LastError is not nil")
}

func TestStatusEvent(t *testing.T) {
	event := StatusEvent{
		ClusterID:          "cluster-123",
		NodePoolID:         "nodepool-456",
		ControllerName:     "test-controller",
		ObservedGeneration: 5,
		Conditions: []Condition{
			{Type: "Available", Status: "True"},
		},
		Metadata: JSONB(map[string]interface{}{
			"key": "value",
		}),
		LastError: &ErrorInfo{
			ErrorType: ErrorTypeTransient,
			Message:   "Temporary error",
		},
		Timestamp: time.Now(),
	}

	utils.AssertEqual(t, "cluster-123", event.ClusterID, "Cluster ID should match")
	utils.AssertEqual(t, "nodepool-456", event.NodePoolID, "NodePool ID should match")
	utils.AssertEqual(t, "test-controller", event.ControllerName, "Controller name should match")
	utils.AssertEqual(t, int64(5), event.ObservedGeneration, "Observed generation should match")
	utils.AssertEqual(t, 1, len(event.Conditions), "Should have 1 condition")
	utils.AssertEqual(t, "Available", event.Conditions[0].Type, "Condition type should match")
	utils.AssertNotNil(t, event.LastError, "Last error should not be nil")
	utils.AssertEqual(t, ErrorTypeTransient, event.LastError.ErrorType, "Error type should match")
}

func TestClusterEvent_BeforeCreate(t *testing.T) {
	event := &ClusterEvent{}
	event.BeforeCreate()

	utils.AssertNotEqual(t, uuid.Nil, event.ID, "ID should be generated")
	utils.AssertFalse(t, event.PublishedAt.IsZero(), "PublishedAt should be set")
}

func TestTableNames(t *testing.T) {
	utils.AssertEqual(t, "controller_status", ClusterControllerStatus{}.TableName(), "ClusterControllerStatus table name")
	utils.AssertEqual(t, "nodepool_controller_status", NodePoolControllerStatus{}.TableName(), "NodePoolControllerStatus table name")
	utils.AssertEqual(t, "cluster_events", ClusterEvent{}.TableName(), "ClusterEvent table name")
}
