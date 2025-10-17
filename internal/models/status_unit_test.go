package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
)

// Unit tests for status models - no external dependencies

func TestConditionListSerialization(t *testing.T) {
	conditions := ConditionList{
		{
			Type:               "Available",
			Status:             "True",
			Reason:             "Ready",
			Message:            "System is ready",
			LastTransitionTime: time.Now(),
		},
		{
			Type:               "Progressing",
			Status:             "False",
			Reason:             "Complete",
			Message:            "Operation completed",
			LastTransitionTime: time.Now(),
		},
	}

	// Test Value() method
	value, err := conditions.Value()
	utils.AssertError(t, err, false, "Value() should not return error")

	jsonBytes, ok := value.([]byte)
	utils.AssertTrue(t, ok, "Value should be []byte")

	// Test Scan() method
	var scannedConditions ConditionList
	err = scannedConditions.Scan(jsonBytes)
	utils.AssertError(t, err, false, "Scan() should not return error")

	utils.AssertEqual(t, 2, len(scannedConditions), "Should have 2 conditions")
	utils.AssertEqual(t, "Available", scannedConditions[0].Type, "First condition type should match")
	utils.AssertEqual(t, "Progressing", scannedConditions[1].Type, "Second condition type should match")
}

func TestConditionListHelpers(t *testing.T) {
	conditions := ConditionList{
		{
			Type:               "Available",
			Status:             "True",
			Reason:             "Ready",
			Message:            "System is ready",
			LastTransitionTime: time.Now(),
		},
		{
			Type:               "Progressing",
			Status:             "False",
			Reason:             "Complete",
			Message:            "Operation completed",
			LastTransitionTime: time.Now(),
		},
	}

	// Test HasCondition
	utils.AssertTrue(t, conditions.HasCondition("Available", "True"), "Should have Available=True condition")
	utils.AssertTrue(t, conditions.HasCondition("Progressing", "False"), "Should have Progressing=False condition")
	utils.AssertFalse(t, conditions.HasCondition("Unknown", "True"), "Should not have Unknown condition")

	// Test GetCondition
	available := conditions.GetCondition("Available")
	utils.AssertNotNil(t, available, "Available condition should be found")
	utils.AssertEqual(t, "Available", available.Type, "Condition type should match")

	unknown := conditions.GetCondition("Unknown")
	if unknown != nil {
		t.Errorf("Expected nil but got %v. [Unknown condition should not be found]", unknown)
	}

	// Test SetCondition (replace existing)
	newAvailable := Condition{
		Type:               "Available",
		Status:             "False",
		Reason:             "NotReady",
		Message:            "System is not ready",
		LastTransitionTime: time.Now(),
	}
	conditions.SetCondition(newAvailable)

	updated := conditions.GetCondition("Available")
	utils.AssertNotNil(t, updated, "Available condition should still exist")
	utils.AssertEqual(t, "False", updated.Status, "Status should be updated")
	utils.AssertEqual(t, "NotReady", updated.Reason, "Reason should be updated")

	// Test SetCondition (add new)
	newCondition := Condition{
		Type:               "Degraded",
		Status:             "True",
		Reason:             "ServiceDown",
		Message:            "Service is down",
		LastTransitionTime: time.Now(),
	}
	conditions.SetCondition(newCondition)

	utils.AssertEqual(t, 3, len(conditions), "Should have 3 conditions after adding")
	utils.AssertTrue(t, conditions.HasCondition("Degraded", "True"), "Should have Degraded=True condition")

	// Test RemoveCondition
	conditions.RemoveCondition("Progressing")
	utils.AssertEqual(t, 2, len(conditions), "Should have 2 conditions after removal")
	utils.AssertFalse(t, conditions.HasCondition("Progressing", "False"), "Should not have Progressing condition")
}

func TestClusterControllerStatusHelpers(t *testing.T) {
	status := ClusterControllerStatus{
		ClusterID:      uuid.New(),
		ControllerName: "test-controller",
		Conditions: ConditionList{
			{
				Type:   "Available",
				Status: "True",
				Reason: "Ready",
			},
			{
				Type:   "Healthy",
				Status: "True",
				Reason: "Running",
			},
		},
		LastError: nil,
	}

	// Test IsHealthy
	utils.AssertTrue(t, status.IsHealthy(), "Should be healthy with no errors")

	// Test with error
	status.LastError = &ErrorInfo{
		ErrorType: ErrorTypeTransient,
		Message:   "Temporary issue",
	}
	utils.AssertFalse(t, status.IsHealthy(), "Should not be healthy with error")

	// Test IsReady
	status.LastError = nil
	utils.AssertTrue(t, status.IsReady(), "Should be ready with Available=True")

	// Test with Available=False
	status.Conditions.SetCondition(Condition{
		Type:   "Available",
		Status: "False",
		Reason: "NotReady",
	})
	utils.AssertFalse(t, status.IsReady(), "Should not be ready with Available=False")

	// Test HasErrors
	utils.AssertFalse(t, status.HasErrors(), "Should not have errors")
	status.LastError = &ErrorInfo{
		ErrorType: ErrorTypeFatal,
		Message:   "Fatal error",
	}
	utils.AssertTrue(t, status.HasErrors(), "Should have errors")
}

func TestStatusEventCreation(t *testing.T) {
	clusterID := uuid.New()
	nodepoolID := uuid.New()

	event := StatusEvent{
		ClusterID:          clusterID.String(),
		NodePoolID:         nodepoolID.String(),
		ControllerName:     "test-controller",
		ObservedGeneration: 5,
		Conditions: []Condition{
			{
				Type:   "Available",
				Status: "True",
				Reason: "Ready",
			},
		},
		Metadata: JSONB{
			"version": "1.2.3",
			"build":   "abc123",
		},
		LastError: &ErrorInfo{
			ErrorType: ErrorTypeTransient,
			Message:   "Temporary issue",
		},
		Timestamp: time.Now(),
	}

	utils.AssertEqual(t, clusterID.String(), event.ClusterID, "ClusterID should match")
	utils.AssertEqual(t, nodepoolID.String(), event.NodePoolID, "NodePoolID should match")
	utils.AssertEqual(t, "test-controller", event.ControllerName, "ControllerName should match")
	utils.AssertEqual(t, int64(5), event.ObservedGeneration, "ObservedGeneration should match")
	utils.AssertEqual(t, 1, len(event.Conditions), "Should have 1 condition")
	utils.AssertNotNil(t, event.Metadata, "Metadata should not be nil")
	utils.AssertNotNil(t, event.LastError, "LastError should not be nil")
}

func TestClusterEventCreation(t *testing.T) {
	clusterID := uuid.New()

	event := ClusterEvent{
		ID:         uuid.New(),
		ClusterID:  clusterID,
		EventType:  "created",
		Generation: 1,
		Changes: JSONB{
			"spec": map[string]interface{}{
				"replicas": 3,
			},
		},
		PublishedAt: time.Now(),
	}

	utils.AssertNotEqual(t, uuid.Nil, event.ID, "ID should be set")
	utils.AssertEqual(t, clusterID, event.ClusterID, "ClusterID should match")
	utils.AssertEqual(t, "created", event.EventType, "EventType should match")
	utils.AssertEqual(t, int64(1), event.Generation, "Generation should match")
	utils.AssertNotNil(t, event.Changes, "Changes should not be nil")
}

func TestErrorInfoSerialization(t *testing.T) {
	errorInfo := ErrorInfo{
		ControllerName: "test-controller",
		ErrorType:      ErrorTypeFatal,
		ErrorCode:      "E001",
		Message:        "Critical error occurred",
		UserActionable: true,
		Suggestions:    []string{"Check logs", "Restart service"},
		Details: map[string]string{
			"component": "api-server",
			"version":   "1.0.0",
		},
		RetryAfter: func() *time.Duration { d := 5 * time.Minute; return &d }(),
		Timestamp:  time.Now(),
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(errorInfo)
	utils.AssertError(t, err, false, "JSON marshal should not error")

	var unmarshaled ErrorInfo
	err = json.Unmarshal(jsonData, &unmarshaled)
	utils.AssertError(t, err, false, "JSON unmarshal should not error")

	utils.AssertEqual(t, errorInfo.ControllerName, unmarshaled.ControllerName, "ControllerName should match")
	utils.AssertEqual(t, errorInfo.ErrorType, unmarshaled.ErrorType, "ErrorType should match")
	utils.AssertEqual(t, errorInfo.ErrorCode, unmarshaled.ErrorCode, "ErrorCode should match")
	utils.AssertEqual(t, errorInfo.Message, unmarshaled.Message, "Message should match")
	utils.AssertEqual(t, errorInfo.UserActionable, unmarshaled.UserActionable, "UserActionable should match")
	utils.AssertEqual(t, len(errorInfo.Suggestions), len(unmarshaled.Suggestions), "Suggestions count should match")
	utils.AssertEqual(t, len(errorInfo.Details), len(unmarshaled.Details), "Details count should match")
}

func TestConditionListNilHandling(t *testing.T) {
	// Test Value() with nil
	var nilConditions ConditionList = nil
	value, err := nilConditions.Value()
	utils.AssertError(t, err, false, "Value() should handle nil")

	jsonBytes, ok := value.([]byte)
	utils.AssertTrue(t, ok, "Should return []byte")

	// Should be empty array JSON (ConditionList(nil).Value() returns marshaled empty slice)
	expectedBytes := []byte("[]")
	utils.AssertEqual(t, string(expectedBytes), string(jsonBytes), "Should serialize to empty array")

	// Test Scan() with nil
	var conditions ConditionList
	err = conditions.Scan(nil)
	utils.AssertError(t, err, false, "Scan() should handle nil")
	utils.AssertEqual(t, 0, len(conditions), "Should be empty slice")
}

// Note: Helper methods are already implemented in status.go
