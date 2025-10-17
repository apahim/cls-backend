package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
)

// Unit tests for nodepool models - no external dependencies

func TestNodePoolValidation(t *testing.T) {
	tests := []struct {
		name     string
		nodepool NodePool
		wantErr  bool
	}{
		{
			name: "valid nodepool",
			nodepool: NodePool{
				Name:      "test-nodepool",
				ClusterID: uuid.New(),
				Spec: NodePoolSpec{
					Replicas: func() *int32 { i := int32(3); return &i }(),
					Platform: NodePoolPlatformSpec{
						Type: "GCP",
						GCP: &NodePoolGCPSpec{
							InstanceType: "n1-standard-4",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty name",
			nodepool: NodePool{
				Name:      "",
				ClusterID: uuid.New(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNodePool(&tt.nodepool)
			utils.AssertError(t, err, tt.wantErr, "Validation result should match expected")
		})
	}
}

func TestNodePoolBeforeCreate(t *testing.T) {
	nodepool := &NodePool{
		Name:      "test-nodepool",
		ClusterID: uuid.New(),
	}

	nodepool.BeforeCreate()

	utils.AssertNotEqual(t, uuid.Nil, nodepool.ID, "ID should be generated")
	utils.AssertEqual(t, int64(1), nodepool.Generation, "Generation should be 1")
	utils.AssertNotEqual(t, "", nodepool.ResourceVersion, "ResourceVersion should be set")
	// Current implementation doesn't have status fields for NodePool
	utils.AssertFalse(t, nodepool.CreatedAt.IsZero(), "CreatedAt should be set")
	utils.AssertFalse(t, nodepool.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestNodePoolBeforeUpdate(t *testing.T) {
	nodepool := &NodePool{
		ID:         uuid.New(),
		Generation: 1,
		CreatedAt:  time.Now().Add(-time.Hour),
		UpdatedAt:  time.Now().Add(-time.Hour),
	}
	oldGeneration := nodepool.Generation
	oldResourceVersion := nodepool.ResourceVersion
	oldUpdatedAt := nodepool.UpdatedAt

	time.Sleep(time.Millisecond)
	nodepool.BeforeUpdate()

	utils.AssertEqual(t, oldGeneration+1, nodepool.Generation, "Generation should increment")
	utils.AssertNotEqual(t, oldResourceVersion, nodepool.ResourceVersion, "ResourceVersion should change")
	utils.AssertTrue(t, nodepool.UpdatedAt.After(oldUpdatedAt), "UpdatedAt should be newer")
}

func TestNodePoolSoftDelete(t *testing.T) {
	nodepool := &NodePool{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	utils.AssertFalse(t, nodepool.IsDeleted(), "Should not be deleted initially")

	nodepool.SoftDelete()

	utils.AssertTrue(t, nodepool.IsDeleted(), "Should be deleted after SoftDelete")
	utils.AssertNotNil(t, nodepool.DeletedAt, "DeletedAt should be set")
}

func TestNodePoolSpecSerialization(t *testing.T) {
	spec := NodePoolSpec{
		Replicas: func() *int32 { i := int32(3); return &i }(),
		Management: NodePoolManagement{
			UpgradeType: "Replace",
			AutoRepair:  true,
		},
		Platform: NodePoolPlatformSpec{
			Type: "GCP",
			GCP: &NodePoolGCPSpec{
				InstanceType: "n1-standard-4",
				RootVolume: &RootVolumeSpec{
					Size: 100,
					Type: "pd-standard",
				},
			},
		},
		Release: NodePoolReleaseSpec{
			Image:   "quay.io/openshift-release-dev/ocp-release:4.14.0",
			Version: "4.14.0",
		},
	}

	// Test Value() method
	value, err := spec.Value()
	utils.AssertError(t, err, false, "Value() should not return error")
	utils.AssertNotNil(t, value, "Value should not be nil")

	jsonBytes, ok := value.([]byte)
	utils.AssertTrue(t, ok, "Value should be []byte")

	// Test Scan() method
	var scannedSpec NodePoolSpec
	err = scannedSpec.Scan(jsonBytes)
	utils.AssertError(t, err, false, "Scan() should not return error")

	utils.AssertNotNil(t, scannedSpec.Replicas, "Replicas should not be nil")
	utils.AssertEqual(t, int32(3), *scannedSpec.Replicas, "Replicas should match")
	utils.AssertEqual(t, spec.Platform.Type, scannedSpec.Platform.Type, "Platform type should match")
}

func TestNodePoolGCPSpecFields(t *testing.T) {
	spec := NodePoolGCPSpec{
		InstanceType:   "n1-standard-4",
		Subnet:         "default",
		ServiceAccount: "default@project.iam.gserviceaccount.com",
		DiskType:       "pd-standard", // Backward compatibility field
		DiskSizeGB:     100,           // Backward compatibility field
		RootVolume: &RootVolumeSpec{
			Size: 100,
			Type: "pd-standard",
		},
		Labels: map[string]string{
			"env":  "test",
			"team": "platform",
		},
		Taints: []TaintSpec{
			{
				Key:    "node-role",
				Value:  "worker",
				Effect: "NoSchedule",
			},
		},
	}

	utils.AssertEqual(t, "n1-standard-4", spec.InstanceType, "Instance type should match")
	utils.AssertEqual(t, "default", spec.Subnet, "Subnet should match")
	utils.AssertEqual(t, "pd-standard", spec.DiskType, "DiskType should be set for backward compatibility")
	utils.AssertEqual(t, 100, spec.DiskSizeGB, "DiskSizeGB should be set for backward compatibility")
	utils.AssertNotNil(t, spec.RootVolume, "RootVolume should not be nil")
	utils.AssertEqual(t, 100, spec.RootVolume.Size, "RootVolume size should match")
	utils.AssertEqual(t, 2, len(spec.Labels), "Should have 2 labels")
	utils.AssertEqual(t, 1, len(spec.Taints), "Should have 1 taint")
}

func TestTaintSpec(t *testing.T) {
	taint := TaintSpec{
		Key:    "special-node",
		Value:  "true",
		Effect: "NoSchedule",
	}

	utils.AssertEqual(t, "special-node", taint.Key, "Taint key should match")
	utils.AssertEqual(t, "true", taint.Value, "Taint value should match")
	utils.AssertEqual(t, "NoSchedule", taint.Effect, "Taint effect should match")
}

func TestRollingUpdateConfig(t *testing.T) {
	maxUnavailable := "25%"
	maxSurge := "25%"

	config := RollingUpdateConfig{
		MaxUnavailable: &maxUnavailable,
		MaxSurge:       &maxSurge,
	}

	utils.AssertNotNil(t, config.MaxUnavailable, "MaxUnavailable should not be nil")
	utils.AssertNotNil(t, config.MaxSurge, "MaxSurge should not be nil")
	utils.AssertEqual(t, "25%", *config.MaxUnavailable, "MaxUnavailable should match")
	utils.AssertEqual(t, "25%", *config.MaxSurge, "MaxSurge should match")
}

func TestNodePoolTableName(t *testing.T) {
	nodepool := NodePool{}
	utils.AssertEqual(t, "nodepools", nodepool.TableName(), "Table name should be 'nodepools'")
}

// Helper function for validation
func validateNodePool(nodepool *NodePool) error {
	if nodepool.Name == "" {
		return fmt.Errorf("nodepool name is required")
	}
	if nodepool.ClusterID == uuid.Nil {
		return fmt.Errorf("cluster ID is required")
	}
	return nil
}
