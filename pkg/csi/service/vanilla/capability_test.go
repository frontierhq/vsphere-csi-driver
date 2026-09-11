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

package vanilla

import (
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
)

// hasRPC returns true when the supplied slice of capability RPC types
// contains the given RPC constant. It is the building block for the
// snapshot / list-volume / EXPAND_VOLUME assertions below and makes
// the test failure messages useful ("rpc missing" rather than a
// raw len/slice equality dump).
func hasRPC(rpcs []csi.ControllerServiceCapability_RPC_Type, want csi.ControllerServiceCapability_RPC_Type) bool {
	for _, got := range rpcs {
		if got == want {
			return true
		}
	}
	return false
}

// TestControllerCapabilityRPCs_NoFlags asserts the structural
// floor: every controller RPC capability list must always advertise
// CREATE_DELETE_VOLUME, PUBLISH_UNPUBLISH_VOLUME and EXPAND_VOLUME,
// regardless of the snapshot and list-volume FSSs. EXPAND_VOLUME is
// not gated on the BlockVolumeSnapshot flag because offline expansion
// is supported on this lab.
func TestControllerCapabilityRPCs_NoFlags(t *testing.T) {
	got := controllerCapabilityRPCs(false, false)

	assert.True(t, hasRPC(got, csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME),
		"CREATE_DELETE_VOLUME must always be advertised")
	assert.True(t, hasRPC(got, csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME),
		"PUBLISH_UNPUBLISH_VOLUME must always be advertised")
	assert.True(t, hasRPC(got, csi.ControllerServiceCapability_RPC_EXPAND_VOLUME),
		"EXPAND_VOLUME must always be advertised (offline expansion is supported)")

	assert.False(t, hasRPC(got, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT),
		"CREATE_DELETE_SNAPSHOT must not appear when BlockVolumeSnapshot is disabled")
	assert.False(t, hasRPC(got, csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS),
		"LIST_SNAPSHOTS must not appear when BlockVolumeSnapshot is disabled")
	assert.False(t, hasRPC(got, csi.ControllerServiceCapability_RPC_LIST_VOLUMES),
		"LIST_VOLUMES must not appear when ListVolumes is disabled")
	assert.False(t, hasRPC(got, csi.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES),
		"LIST_VOLUMES_PUBLISHED_NODES must not appear when ListVolumes is disabled")
}

// TestControllerCapabilityRPCs_SnapshotOnly asserts that
// BlockVolumeSnapshot controls the two snapshot RPCs and nothing
// else. ListVolumes is independent.
func TestControllerCapabilityRPCs_SnapshotOnly(t *testing.T) {
	got := controllerCapabilityRPCs(true, false)

	assert.True(t, hasRPC(got, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT),
		"CREATE_DELETE_SNAPSHOT must appear when BlockVolumeSnapshot is enabled")
	assert.True(t, hasRPC(got, csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS),
		"LIST_SNAPSHOTS must appear when BlockVolumeSnapshot is enabled")
	assert.False(t, hasRPC(got, csi.ControllerServiceCapability_RPC_LIST_VOLUMES),
		"LIST_VOLUMES must remain absent when ListVolumes is disabled")
	assert.True(t, hasRPC(got, csi.ControllerServiceCapability_RPC_EXPAND_VOLUME),
		"EXPAND_VOLUME must remain present when only BlockVolumeSnapshot is enabled")
}

// TestControllerCapabilityRPCs_ListVolumesOnly asserts that
// ListVolumes controls the two list-volume RPCs and nothing else.
// BlockVolumeSnapshot is independent: enabling one does not enable
// the other.
func TestControllerCapabilityRPCs_ListVolumesOnly(t *testing.T) {
	got := controllerCapabilityRPCs(false, true)

	assert.True(t, hasRPC(got, csi.ControllerServiceCapability_RPC_LIST_VOLUMES),
		"LIST_VOLUMES must appear when ListVolumes is enabled")
	assert.True(t, hasRPC(got, csi.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES),
		"LIST_VOLUMES_PUBLISHED_NODES must appear when ListVolumes is enabled")
	assert.False(t, hasRPC(got, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT),
		"CREATE_DELETE_SNAPSHOT must remain absent when BlockVolumeSnapshot is disabled")
	assert.False(t, hasRPC(got, csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS),
		"LIST_SNAPSHOTS must remain absent when BlockVolumeSnapshot is disabled")
	assert.True(t, hasRPC(got, csi.ControllerServiceCapability_RPC_EXPAND_VOLUME),
		"EXPAND_VOLUME must remain present when only ListVolumes is enabled")
}

// TestControllerCapabilityRPCs_BothFlags asserts that with both
// feature-state switches enabled, every optional RPC is advertised
// alongside the always-on trio.
func TestControllerCapabilityRPCs_BothFlags(t *testing.T) {
	got := controllerCapabilityRPCs(true, true)

	for _, rpc := range []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES,
	} {
		assert.True(t, hasRPC(got, rpc),
			"expected capability %v to be advertised when both FSSs are enabled", rpc)
	}
}
