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
	"errors"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cnsvsphere "sigs.k8s.io/vsphere-csi-driver/v3/pkg/common/cns-lib/vsphere"
)

// expandRequestFixture returns a minimal valid expansion request.
func expandRequestFixture() *csi.ControllerExpandVolumeRequest {
	return &csi.ControllerExpandVolumeRequest{
		VolumeId:      "vol-online",
		CapacityRange: &csi.CapacityRange{RequiredBytes: 1024},
		VolumeCapability: &csi.VolumeCapability{
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}
}

// fakeNodeProvider records whether the offline validation path queried nodes.
type fakeNodeProvider struct {
	nodes      []*cnsvsphere.VirtualMachine
	err        error
	calls      int
	gotContext context.Context
}

func (f *fakeNodeProvider) GetAllNodes(ctx context.Context) ([]*cnsvsphere.VirtualMachine, error) {
	f.calls++
	f.gotContext = ctx
	return f.nodes, f.err
}

// TestValidateExpandVolumeRequest_OnlinePathShortCircuits asserts
// that when online expansion is enabled and vCenter supports it, the
// validator returns nil without consulting the node provider. The
// lab disables online expansion at the feature-state level, so this
// path is not taken in production — but a regression that flips the
// boolean back on would expose the cluster to attached-volume
// expansion, which is the documented failure mode the lab avoids.
func TestValidateExpandVolumeRequest_OnlinePathShortCircuits(t *testing.T) {
	fp := &fakeNodeProvider{}
	err := validateVanillaControllerExpandVolumeRequest(
		context.Background(),
		expandRequestFixture(),
		true, // isOnlineExpansionEnabled
		true, // isOnlineExpansionSupported
		fp,
	)
	assert.NoError(t, err, "online path must short-circuit before the node provider is consulted")
	assert.Equal(t, 0, fp.calls, "node provider must not be consulted on the online short-circuit")
}

// TestValidateExpandVolumeRequest_OfflineBranchConsultsNodeProvider
// asserts that when online expansion is not both enabled and
// supported, the validator calls the node provider. The lab ships
// with online expansion disabled at the feature-state level so this
// is the production code path.
func TestValidateExpandVolumeRequest_OfflineBranchConsultsNodeProvider(t *testing.T) {
	fp := &fakeNodeProvider{nodes: nil}
	err := validateVanillaControllerExpandVolumeRequest(
		context.Background(),
		expandRequestFixture(),
		false, // isOnlineExpansionEnabled
		true,  // isOnlineExpansionSupported
		fp,
	)
	// An empty node list represents the detached/offline case; the
	// common.IsOnlineExpansion helper treats an unattached volume as
	// a successful offline expansion. The structural invariant under
	// test is that the node provider was consulted.
	assert.NoError(t, err)
	assert.Equal(t, 1, fp.calls, "node provider must be consulted when the online short-circuit is not taken")
}

// TestValidateExpandVolumeRequest_OfflineBranchSucceedsWithNoNodes
// asserts the unattached/offline semantic explicitly: an empty node
// list from the provider represents the offline expansion case and
// must succeed. This is the production behaviour the lab relies on
// — expanding a detached volume is supported; expanding an attached
// one is not.
func TestValidateExpandVolumeRequest_OfflineBranchSucceedsWithNoNodes(t *testing.T) {
	fp := &fakeNodeProvider{nodes: nil}
	err := validateVanillaControllerExpandVolumeRequest(
		context.Background(),
		expandRequestFixture(),
		false, true, fp,
	)
	assert.NoError(t, err, "empty node list must represent the detached/offline case and succeed")
	assert.Equal(t, 1, fp.calls)
}

// TestValidateExpandVolumeRequest_NodeProviderErrorBecomesInternal
// asserts that an error returned by the node provider is surfaced as
// a gRPC Internal validation failure, matching the upstream
// behaviour the lab preserves.
func TestValidateExpandVolumeRequest_NodeProviderErrorBecomesInternal(t *testing.T) {
	fp := &fakeNodeProvider{err: errors.New("kaboom")}
	err := validateVanillaControllerExpandVolumeRequest(
		context.Background(),
		expandRequestFixture(),
		false, true, fp,
	)
	assert.Error(t, err)
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok, "error must be a gRPC status")
	assert.Equal(t, codes.Internal, grpcStatus.Code(), "node-provider failures must surface as Internal")
	assert.Equal(t, 1, fp.calls)
}

// TestValidateExpandVolumeRequest_NeitherFlagEnabled consults the
// node provider. The single-flag false / supported=true case above
// is the most common production path; this test pins the inverse
// combination (enabled=true, supported=false) so a regression that
// only checks one of the booleans is caught.
func TestValidateExpandVolumeRequest_DisabledByVCenterStillConsults(t *testing.T) {
	fp := &fakeNodeProvider{nodes: nil}
	err := validateVanillaControllerExpandVolumeRequest(
		context.Background(),
		expandRequestFixture(),
		true,  // isOnlineExpansionEnabled
		false, // isOnlineExpansionSupported: vCenter is too old
		fp,
	)
	assert.NoError(t, err)
	assert.Equal(t, 1, fp.calls, "node provider must be consulted when only one online flag is true")
}
