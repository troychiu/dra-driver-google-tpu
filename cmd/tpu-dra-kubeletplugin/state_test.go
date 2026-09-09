/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager"
	"sigs.k8s.io/yaml"
	cdiapi "tags.cncf.io/container-device-interface/pkg/cdi"
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

const foreignDriverName = "example.com/nic"

func tpuDeviceState(chipCount int, deviceNames ...string) *DeviceState {
	allocatable := AllocatableDevices{}
	for i, name := range deviceNames {
		allocatable[name] = &AllocatableDevice{
			UUID:        name,
			name:        name,
			index:       i,
			allocatable: true,
		}
	}
	return &DeviceState{
		cdi:         &CDIHandler{},
		allocatable: allocatable,
		tm: &tpuManager{
			DevDirectory: "/dev",
			devices:      allocatable,
			tpuChipCount: chipCount,
		},
	}
}

func claimWithResults(results ...resourceapi.DeviceRequestAllocationResult) *resourceapi.ResourceClaim {
	return &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "claim", Namespace: "default", UID: "claim-uid"},
		Status: resourceapi.ResourceClaimStatus{
			Allocation: &resourceapi.AllocationResult{
				Devices: resourceapi.DeviceAllocationResult{Results: results},
			},
		},
	}
}

func result(driver, pool, device, request string) resourceapi.DeviceRequestAllocationResult {
	return resourceapi.DeviceRequestAllocationResult{
		Driver:  driver,
		Pool:    pool,
		Device:  device,
		Request: request,
	}
}

// A ResourceClaim can be satisfied by more than one driver. The TPU plugin must
// apply its full-chip count, its allocatable lookup and its CDI edits to the
// results it owns, and leave the rest alone.
func TestPrepareDevicesIgnoresForeignAllocationResults(t *testing.T) {
	tests := []struct {
		name        string
		state       *DeviceState
		results     []resourceapi.DeviceRequestAllocationResult
		wantErr     bool
		wantDevices []string
	}{
		{
			name:  "all results owned by this driver",
			state: tpuDeviceState(2, "tpu0", "tpu1"),
			results: []resourceapi.DeviceRequestAllocationResult{
				result(DriverName, "tpu-pool", "tpu0", "tpus"),
				result(DriverName, "tpu-pool", "tpu1", "tpus"),
			},
			wantDevices: []string{"tpu0", "tpu1"},
		},
		{
			// Without the ownership filter the foreign entry pushes the result
			// count past tpuChipCount, so a complete TPU allocation is rejected
			// because the workload also asked for a NIC.
			name:  "full chip set alongside a device owned by another driver",
			state: tpuDeviceState(2, "tpu0", "tpu1"),
			results: []resourceapi.DeviceRequestAllocationResult{
				result(DriverName, "tpu-pool", "tpu0", "tpus"),
				result(DriverName, "tpu-pool", "tpu1", "tpus"),
				result(foreignDriverName, "nic-pool", "nic0", "nics"),
			},
			wantDevices: []string{"tpu0", "tpu1"},
		},
		{
			// Device names are only unique within a driver's pools. A foreign
			// device that happens to be called "tpu0" must not be resolved
			// against this driver's allocatable map or prepared as a TPU.
			name:  "another driver uses a device name that also exists here",
			state: tpuDeviceState(2, "tpu0", "tpu1"),
			results: []resourceapi.DeviceRequestAllocationResult{
				result(DriverName, "tpu-pool", "tpu0", "tpus"),
				result(DriverName, "tpu-pool", "tpu1", "tpus"),
				result(foreignDriverName, "nic-pool", "tpu0", "nics"),
			},
			wantDevices: []string{"tpu0", "tpu1"},
		},
		{
			// The partial-allocation error still has to fire. A foreign result
			// must not stand in for a missing TPU and make the count add up.
			name:  "partial TPU allocation padded by a foreign result",
			state: tpuDeviceState(2, "tpu0", "tpu1"),
			results: []resourceapi.DeviceRequestAllocationResult{
				result(DriverName, "tpu-pool", "tpu0", "tpus"),
				result(foreignDriverName, "nic-pool", "nic0", "nics"),
			},
			wantErr: true,
		},
		{
			name:  "only foreign results",
			state: tpuDeviceState(2, "tpu0", "tpu1"),
			results: []resourceapi.DeviceRequestAllocationResult{
				result(foreignDriverName, "nic-pool", "nic0", "nics"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepared, err := tt.state.prepareDevices(claimWithResults(tt.results...))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("prepareDevices() = %d devices, want an error", len(prepared))
				}
				return
			}
			if err != nil {
				t.Fatalf("prepareDevices() error = %v, want nil", err)
			}
			if len(prepared) != len(tt.wantDevices) {
				t.Fatalf("prepareDevices() prepared %d devices, want %d", len(prepared), len(tt.wantDevices))
			}
			got := make(map[string]bool, len(prepared))
			for _, device := range prepared {
				got[device.DeviceName] = true
			}
			for _, want := range tt.wantDevices {
				if !got[want] {
					t.Errorf("prepareDevices() did not prepare %q; prepared %v", want, got)
				}
			}
		})
	}
}

func claimSpecPath(cdiDir, claimUID string) string {
	specName := cdiapi.GenerateTransientSpecName(cdiVendor, cdiClass, claimUID)
	return filepath.Join(cdiDir, specName+".yaml")
}

func newTestDeviceStateWithDirs(t *testing.T, pluginDir, cdiDir string, chipCount int, deviceNames ...string) *DeviceState {
	t.Helper()
	state := tpuDeviceState(chipCount, deviceNames...)

	cdi, err := NewCDIHandler(&Config{flags: &Flags{cdiRoot: cdiDir}})
	if err != nil {
		t.Fatalf("NewCDIHandler: %v", err)
	}
	state.cdi = cdi

	cpm, err := checkpointmanager.NewCheckpointManager(pluginDir)
	if err != nil {
		t.Fatalf("NewCheckpointManager: %v", err)
	}
	state.checkpointManager = cpm

	checkpoints, err := cpm.ListCheckpoints()
	if err != nil {
		t.Fatalf("ListCheckpoints: %v", err)
	}
	if !slices.Contains(checkpoints, DriverPluginCheckpointFile) {
		if err := cpm.CreateCheckpoint(DriverPluginCheckpointFile, newCheckpoint()); err != nil {
			t.Fatalf("CreateCheckpoint: %v", err)
		}
	}

	return state
}

func assertDevicesEqual(t *testing.T, a, b []kubeletplugin.Device) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("device count mismatch: got %d, want %d", len(a), len(b))
	}
	for i := range a {
		if !slices.Equal(a[i].Requests, b[i].Requests) {
			t.Errorf("device[%d] requests = %v, want %v", i, a[i].Requests, b[i].Requests)
		}
		if a[i].DeviceName != b[i].DeviceName {
			t.Errorf("device[%d] name = %q, want %q", i, a[i].DeviceName, b[i].DeviceName)
		}
		if a[i].PoolName != b[i].PoolName {
			t.Errorf("device[%d] pool = %q, want %q", i, a[i].PoolName, b[i].PoolName)
		}
		if !slices.Equal(a[i].CDIDeviceIDs, b[i].CDIDeviceIDs) {
			t.Errorf("device[%d] CDI IDs = %v, want %v", i, a[i].CDIDeviceIDs, b[i].CDIDeviceIDs)
		}
	}
}

func assertClaimSpecResolvesPreparedDevices(t *testing.T, cdiDir string, claimUID string, prepared []kubeletplugin.Device) {
	t.Helper()
	specPath := claimSpecPath(cdiDir, claimUID)
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", specPath, err)
	}

	var spec cdispec.Spec
	if err := yaml.Unmarshal(specBytes, &spec); err != nil {
		t.Fatalf("Unmarshal CDI spec: %v", err)
	}
	if spec.Kind != cdiKind {
		t.Errorf("spec.Kind = %q, want %q", spec.Kind, cdiKind)
	}

	specDevices := make(map[string]cdispec.Device)
	for _, dev := range spec.Devices {
		specDevices[dev.Name] = dev
	}

	for _, dev := range prepared {
		expectedCDIName := fmt.Sprintf("%s-%s", claimUID, dev.DeviceName)
		if _, ok := specDevices[expectedCDIName]; !ok {
			t.Errorf("expected CDI device %q not found in CDI spec %s", expectedCDIName, specPath)
		}
	}
}

func TestPrepareRestoredClaimRecreatesMissingClaimSpec(t *testing.T) {
	pluginDir := t.TempDir()
	cdiDir := t.TempDir()
	state := newTestDeviceStateWithDirs(t, pluginDir, cdiDir, 2, "tpu0", "tpu1")

	claim := claimWithResults(
		result(DriverName, "tpu-pool", "tpu0", "tpus"),
		result(DriverName, "tpu-pool", "tpu1", "tpus"),
	)

	prepared, err := state.Prepare(context.Background(), claim)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(prepared) != 2 {
		t.Fatalf("prepared %d devices, want 2", len(prepared))
	}
	assertClaimSpecResolvesPreparedDevices(t, cdiDir, string(claim.UID), prepared)

	// Simulate node reboot or transient directory wipe by removing the CDI spec file.
	specPath := claimSpecPath(cdiDir, string(claim.UID))
	if err := os.Remove(specPath); err != nil {
		t.Fatalf("Remove(%s): %v", specPath, err)
	}

	// Create a new DeviceState instance sharing the checkpoint directory to simulate plugin restart.
	restartedState := newTestDeviceStateWithDirs(t, pluginDir, cdiDir, 2, "tpu0", "tpu1")
	restored, err := restartedState.Prepare(context.Background(), claim)
	if err != nil {
		t.Fatalf("Prepare on restarted state: %v", err)
	}

	assertDevicesEqual(t, prepared, restored)
	assertClaimSpecResolvesPreparedDevices(t, cdiDir, string(claim.UID), restored)
}

func TestPrepareRestoredClaimIsIdempotentWhenClaimSpecExists(t *testing.T) {
	pluginDir := t.TempDir()
	cdiDir := t.TempDir()
	state := newTestDeviceStateWithDirs(t, pluginDir, cdiDir, 2, "tpu0", "tpu1")

	claim := claimWithResults(
		result(DriverName, "tpu-pool", "tpu0", "tpus"),
		result(DriverName, "tpu-pool", "tpu1", "tpus"),
	)

	prepared, err := state.Prepare(context.Background(), claim)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(prepared) != 2 {
		t.Fatalf("prepared %d devices, want 2", len(prepared))
	}
	assertClaimSpecResolvesPreparedDevices(t, cdiDir, string(claim.UID), prepared)

	// Re-prepare when the CDI spec file already exists.
	restartedState := newTestDeviceStateWithDirs(t, pluginDir, cdiDir, 2, "tpu0", "tpu1")
	restored, err := restartedState.Prepare(context.Background(), claim)
	if err != nil {
		t.Fatalf("Prepare idempotent call: %v", err)
	}

	assertDevicesEqual(t, prepared, restored)
	assertClaimSpecResolvesPreparedDevices(t, cdiDir, string(claim.UID), restored)
}

func TestPrepareRestoredClaimFailsWhenClaimSpecCannotBeRecreated(t *testing.T) {
	pluginDir := t.TempDir()
	cdiDir := t.TempDir()
	state := newTestDeviceStateWithDirs(t, pluginDir, cdiDir, 2, "tpu0", "tpu1")

	claim := claimWithResults(
		result(DriverName, "tpu-pool", "tpu0", "tpus"),
		result(DriverName, "tpu-pool", "tpu1", "tpus"),
	)

	_, err := state.Prepare(context.Background(), claim)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	specPath := claimSpecPath(cdiDir, string(claim.UID))
	if err := os.Remove(specPath); err != nil {
		t.Fatalf("Remove(%s): %v", specPath, err)
	}

	// Create a directory at the spec file path so that recreating the spec file fails.
	if err := os.Mkdir(specPath, 0750); err != nil {
		t.Fatalf("Mkdir(%s): %v", specPath, err)
	}

	restartedState := newTestDeviceStateWithDirs(t, pluginDir, cdiDir, 2, "tpu0", "tpu1")
	restored, err := restartedState.Prepare(context.Background(), claim)
	if err == nil {
		t.Fatal("expected Prepare to fail when CDI spec cannot be recreated, got nil error")
	}
	if !strings.Contains(err.Error(), "unable to recreate CDI spec file for claim") {
		t.Errorf("error %q does not contain expected substring", err.Error())
	}
	if restored != nil {
		t.Errorf("expected nil restored devices, got %v", restored)
	}
}
