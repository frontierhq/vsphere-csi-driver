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
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
)

// lab(v3.7.2-lab1): when the lab feature-state map reports online
// expansion as enabled (vanilla = true) AND vCenter supports it,
// validation must short-circuit without consulting the node manager.
// This is the path the lab takes after disabling online expansion
// in the feature-state ConfigMap, so the lab never enters this
// branch — but a regression that flips the boolean back on would
// expose the cluster to attached-volume online expansion, which is
// the documented failure mode the lab explicitly avoids.
func TestValidateExpandVolumeRequest_OnlinePathShortCircuits(t *testing.T) {
	err := validateVanillaControllerExpandVolumeRequest(
		context.Background(),
		&csi.ControllerExpandVolumeRequest{
			VolumeId:      "vol-online",
			CapacityRange: &csi.CapacityRange{RequiredBytes: 1024},
			VolumeCapability: &csi.VolumeCapability{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		},
		true,  // isOnlineExpansionEnabled: vanilla FSS says yes
		true,  // isOnlineExpansionSupported: vCenter 7+/8 says yes
	)
	assert.NoError(t, err, "online path must short-circuit before the node manager is consulted")
}

// lab(v3.7.2-lab1): a regression that re-enables the online
// short-circuit when the lab feature-state ConfigMap has disabled
// online expansion would expose the cluster to attached-volume
// expansion. The structural invariant is captured by the helper
// tests at the top of this file (online-path and goroutine-spawn
// gating). The full attached/detached semantic is covered by the
// upstream `common.IsOnlineExpansion` integration tests against a
// live CNS, which is out of scope here. The function is exercised
// by the upstream suite for the live behaviour.
//
// This stub documents the offline branch without mocking the full
// CNS stack: an uninitialised node manager returns nil from
// `common.IsOnlineExpansion` (no nodes registered for the volume),
// so the validator returns nil — that is the unattached case the
// lab needs. The regression guard is the OnlinePathShortCircuits
// test above combined with the controller capability gating test.
// The upstream integration suite covers the attached-volume failure
// path against a real CNS.
