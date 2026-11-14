package nebius

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nebius/gosdk/proto/nebius/common/v1"
	commonv1 "github.com/nebius/gosdk/proto/nebius/common/v1"
	computev1 "github.com/nebius/gosdk/proto/nebius/compute/v1"
	iamv1 "github.com/nebius/gosdk/proto/nebius/iam/v1"
	iamv2 "github.com/nebius/gosdk/proto/nebius/iam/v2"
	mk8sv1 "github.com/nebius/gosdk/proto/nebius/mk8s/v1"
	storagev1 "github.com/nebius/gosdk/proto/nebius/storage/v1"
	vpcv1 "github.com/nebius/gosdk/proto/nebius/vpc/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fakeNebiusAPI implements nebiusAPI with canned data and call
// recording, so every fetcher path runs without the network.
type fakeNebiusAPI struct {
	tenants      []*iamv2.Tenant
	projects     map[string][]*iamv2.Project
	instances    map[string][]*computev1.Instance
	disks        map[string][]*computev1.Disk
	clusters     map[string][]*mk8sv1.Cluster
	nodeGroups   map[string][]*mk8sv1.NodeGroup
	networks     map[string][]*vpcv1.Network
	subnets      map[string][]*vpcv1.Subnet
	buckets      map[string][]*storagev1.Bucket
	serviceAccts map[string][]*iamv1.ServiceAccount
	groups       map[string][]*iamv1.Group
	memberships  map[string][]*iamv1.GroupMembership
	permits      map[string][]*iamv1.AccessPermit

	err   error
	calls []string
}

func (f *fakeNebiusAPI) ListTenants(ctx context.Context) ([]*iamv2.Tenant, error) {
	f.calls = append(f.calls, "tenants")
	return f.tenants, f.err
}

func (f *fakeNebiusAPI) ListProjects(ctx context.Context, parentID string) ([]*iamv2.Project, error) {
	f.calls = append(f.calls, "projects:"+parentID)
	return f.projects[parentID], f.err
}

func (f *fakeNebiusAPI) ListInstances(ctx context.Context, parentID string) ([]*computev1.Instance, error) {
	f.calls = append(f.calls, "instances:"+parentID)
	return f.instances[parentID], f.err
}

func (f *fakeNebiusAPI) ListDisks(ctx context.Context, parentID string) ([]*computev1.Disk, error) {
	f.calls = append(f.calls, "disks:"+parentID)
	return f.disks[parentID], f.err
}

func (f *fakeNebiusAPI) ListClusters(ctx context.Context, parentID string) ([]*mk8sv1.Cluster, error) {
	f.calls = append(f.calls, "clusters:"+parentID)
	return f.clusters[parentID], f.err
}

func (f *fakeNebiusAPI) ListNodeGroups(ctx context.Context, parentID string) ([]*mk8sv1.NodeGroup, error) {
	f.calls = append(f.calls, "nodegroups:"+parentID)
	return f.nodeGroups[parentID], f.err
}

func (f *fakeNebiusAPI) ListNetworks(ctx context.Context, parentID string) ([]*vpcv1.Network, error) {
	f.calls = append(f.calls, "networks:"+parentID)
	return f.networks[parentID], f.err
}

func (f *fakeNebiusAPI) ListSubnets(ctx context.Context, parentID string) ([]*vpcv1.Subnet, error) {
	f.calls = append(f.calls, "subnets:"+parentID)
	return f.subnets[parentID], f.err
}

func (f *fakeNebiusAPI) ListBuckets(ctx context.Context, parentID string) ([]*storagev1.Bucket, error) {
	f.calls = append(f.calls, "buckets:"+parentID)
	return f.buckets[parentID], f.err
}

func (f *fakeNebiusAPI) ListServiceAccounts(ctx context.Context, parentID string) ([]*iamv1.ServiceAccount, error) {
	f.calls = append(f.calls, "serviceaccounts:"+parentID)
	return f.serviceAccts[parentID], f.err
}

func (f *fakeNebiusAPI) ListGroups(ctx context.Context, parentID string) ([]*iamv1.Group, error) {
	f.calls = append(f.calls, "groups:"+parentID)
	return f.groups[parentID], f.err
}

func (f *fakeNebiusAPI) ListGroupMembers(ctx context.Context, parentID string) ([]*iamv1.GroupMembership, error) {
	f.calls = append(f.calls, "memberships:"+parentID)
	return f.memberships[parentID], f.err
}

func (f *fakeNebiusAPI) ListAccessPermits(ctx context.Context, parentID string) ([]*iamv1.AccessPermit, error) {
	f.calls = append(f.calls, "permits:"+parentID)
	return f.permits[parentID], f.err
}

// withFakeAPI swaps the package open seam for one test and restores
// it after.
func withFakeAPI(t *testing.T, api nebiusAPI) {
	t.Helper()
	old := nebiusFetchers.open
	nebiusFetchers.open = func() (nebiusAPI, error) { return api, nil }
	t.Cleanup(func() { nebiusFetchers.open = old })
}

func TestNebiusTokenSourceEnv(t *testing.T) {
	t.Setenv("NEBIUS_IAM_TOKEN", "tok-1")
	token, err := defaultNebiusTokenSource()()
	if err != nil || token != "tok-1" {
		t.Fatalf("env token = %q, %v", token, err)
	}
}

func TestNebiusTokenSourceCLITrim(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "nebius")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho '  tok-2  '\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEBIUS_IAM_TOKEN", "")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	token, err := defaultNebiusTokenSource()()
	if err != nil || token != "tok-2" {
		t.Fatalf("cli token = %q, %v", token, err)
	}
}

func TestNebiusTokenSourceCLIMissing(t *testing.T) {
	t.Setenv("NEBIUS_IAM_TOKEN", "")
	t.Setenv("PATH", t.TempDir())
	_, err := defaultNebiusTokenSource()()
	if err == nil || !strings.Contains(err.Error(), "nebius CLI") {
		t.Fatalf("missing cli error = %v", err)
	}
}

func TestNebiusTokenSourceCLIEmpty(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "nebius")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEBIUS_IAM_TOKEN", "")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := defaultNebiusTokenSource()()
	if err == nil || !strings.Contains(err.Error(), "empty token") {
		t.Fatalf("empty token error = %v", err)
	}
}

func TestFmtTime(t *testing.T) {
	if got := fmtTime(nil); got != "" {
		t.Fatalf("fmtTime(nil) = %q", got)
	}
	if got := fmtTime(&timestamppb.Timestamp{}); got != "1970-01-01T00:00:00Z" {
		t.Fatalf("fmtTime(zero) = %q", got)
	}
	ts := timestamppb.New(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	if got := fmtTime(ts); got != "2026-01-02T03:04:05Z" {
		t.Fatalf("fmtTime = %q", got)
	}
}

// pagedServer simulates one paginated nebius list: it answers with a
// canned token sequence and records the page tokens it saw.
type pagedServer struct {
	tokens    []string // response token per call
	items     int      // items per page
	requested []string
}

func (s *pagedServer) page(pageToken string) ([]*iamv2.Tenant, string, error) {
	s.requested = append(s.requested, pageToken)
	i := len(s.requested) - 1
	if i >= len(s.tokens) {
		return nil, "", nil
	}
	return []*iamv2.Tenant{{Metadata: &commonv1.ResourceMetadata{Id: fmt.Sprintf("t%d", i)}}}, s.tokens[i], nil
}

func TestListPagesWalksTokens(t *testing.T) {
	c := &nebiusClient{maxPages: 5}
	server := &pagedServer{tokens: []string{"tok-a", "tok-b", ""}}
	out, err := listPages(c,
		func(pageToken string) *iamv2.ListTenantsRequest {
			return &iamv2.ListTenantsRequest{PageToken: pageToken}
		},
		func(req *iamv2.ListTenantsRequest) ([]*iamv2.Tenant, string, error) {
			return server.page(req.GetPageToken())
		})
	if err != nil {
		t.Fatalf("listPages: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("items = %d, want 3", len(out))
	}
	if want := []string{"", "tok-a", "tok-b"}; !reflect.DeepEqual(server.requested, want) {
		t.Fatalf("page tokens = %v, want %v", server.requested, want)
	}
}

func TestListPagesPropagatesError(t *testing.T) {
	c := &nebiusClient{maxPages: 5}
	_, err := listPages(c,
		func(pageToken string) *iamv2.ListTenantsRequest {
			return &iamv2.ListTenantsRequest{}
		},
		func(req *iamv2.ListTenantsRequest) ([]*iamv2.Tenant, string, error) {
			return nil, "", errors.New("unavailable")
		})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("err = %v, want unavailable", err)
	}
}

func TestListPagesPageCap(t *testing.T) {
	// A server that always returns another token must trip the cap,
	// not loop forever.
	c := &nebiusClient{maxPages: 3}
	_, err := listPages(c,
		func(pageToken string) *iamv2.ListTenantsRequest {
			return &iamv2.ListTenantsRequest{PageToken: pageToken}
		},
		func(req *iamv2.ListTenantsRequest) ([]*iamv2.Tenant, string, error) {
			return []*iamv2.Tenant{}, "same-token", nil
		})
	if err == nil || !strings.Contains(err.Error(), "exceeded 3 pages") {
		t.Fatalf("err = %v, want page cap error", err)
	}
}

func TestStripCIDR(t *testing.T) {
	if got := stripCIDR("10.0.0.1/32"); got != "10.0.0.1" {
		t.Fatalf("stripCIDR(cidr) = %q", got)
	}
	if got := stripCIDR("10.0.0.1"); got != "10.0.0.1" {
		t.Fatalf("stripCIDR(plain) = %q", got)
	}
}

func TestListProjects(t *testing.T) {
	api := &fakeNebiusAPI{
		tenants: []*iamv2.Tenant{{Metadata: &v1.ResourceMetadata{Id: "tenant-1"}}},
		projects: map[string][]*iamv2.Project{
			"tenant-1": {
				{Metadata: &v1.ResourceMetadata{Id: "project-1", Name: "one"}, Spec: &iamv2.ProjectSpec{Region: "eu-north1"}},
			},
		},
	}
	withFakeAPI(t, api)
	projects, err := ListProjects()
	if err != nil {
		t.Fatalf("ListNebiusProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("projects = %d, want 1", len(projects))
	}
	p := projects[0]
	if p.ProjectID != "project-1" || p.Name != "one" || p.ProjectNumber != "eu-north1" {
		t.Fatalf("project = %+v", p)
	}
}

func TestListNebiusProjectsOpenError(t *testing.T) {
	old := nebiusFetchers.open
	nebiusFetchers.open = func() (nebiusAPI, error) { return nil, errors.New("no token") }
	t.Cleanup(func() { nebiusFetchers.open = old })
	if _, err := ListProjects(); err == nil {
		t.Fatal("open error must surface")
	}
}

func TestListNebiusProjectsTenantError(t *testing.T) {
	api := &fakeNebiusAPI{err: errors.New("boom")}
	withFakeAPI(t, api)
	if _, err := ListProjects(); err == nil {
		t.Fatal("tenant list error must surface")
	}
}

func TestListNebiusProjectsPerTenantError(t *testing.T) {
	api := &fakeNebiusAPI{
		tenants: []*iamv2.Tenant{{Metadata: &v1.ResourceMetadata{Id: "tenant-1"}}},
		err:     errors.New("boom"),
	}
	withFakeAPI(t, api)
	if _, err := ListProjects(); err == nil {
		t.Fatal("per-tenant error must surface")
	}
}

func TestListVMs(t *testing.T) {
	api := &fakeNebiusAPI{
		instances: map[string][]*computev1.Instance{
			"project-1": {{
				Metadata: &v1.ResourceMetadata{Id: "inst-1", Name: "vm-1", Labels: map[string]string{"team": "core"}},
				Spec: &computev1.InstanceSpec{
					Resources:        &computev1.ResourcesSpec{Platform: "intel", Size: &computev1.ResourcesSpec_Preset{Preset: "standard"}},
					ServiceAccountId: "svc-1",
				},
				Status: &computev1.InstanceStatus{
					State: computev1.InstanceStatus_CREATING,
					NetworkInterfaces: []*computev1.NetworkInterfaceStatus{{
						IpAddress:       &computev1.IPAddressStatus{Address: "10.0.0.5"},
						PublicIpAddress: &computev1.PublicIPAddressStatus{Address: "1.2.3.4"},
					}},
				},
			}},
		},
	}
	withFakeAPI(t, api)
	vms, err := ListVMs("project-1")
	if err != nil {
		t.Fatalf("ListNebiusVMs: %v", err)
	}
	if len(vms) != 1 {
		t.Fatalf("vms = %d, want 1", len(vms))
	}
	vm := vms[0]
	if vm.Name != "vm-1" || vm.InstanceID != "inst-1" || vm.MachineType != "intel/standard" {
		t.Fatalf("vm = %+v", vm)
	}
	if len(vm.Interfaces) != 1 || vm.Interfaces[0].InternalIP != "10.0.0.5" || vm.Interfaces[0].ExternalIP != "1.2.3.4" {
		t.Fatalf("interfaces = %+v", vm.Interfaces)
	}
	if len(vm.ServiceAccounts) != 1 || vm.ServiceAccounts[0].Email != "svc-1" {
		t.Fatalf("service accounts = %+v", vm.ServiceAccounts)
	}
	if vm.Labels["team"] != "core" {
		t.Fatalf("labels = %+v", vm.Labels)
	}
}

func TestListNebiusVMsError(t *testing.T) {
	api := &fakeNebiusAPI{err: errors.New("boom")}
	withFakeAPI(t, api)
	if _, err := ListVMs("p"); err == nil {
		t.Fatal("instance list error must surface")
	}
}

func TestListDisks(t *testing.T) {
	api := &fakeNebiusAPI{
		disks: map[string][]*computev1.Disk{
			"project-1": {{
				Metadata: &v1.ResourceMetadata{Id: "disk-1", Name: "d-1"},
				Spec: &computev1.DiskSpec{
					// Live disks carry size_bytes, not the GiB arm; the
					// mapper must read bytes from status.
					Source: &computev1.DiskSpec_SourceImageId{SourceImageId: "image-9"},
					Type:   computev1.DiskSpec_NETWORK_SSD,
				},
				Status: &computev1.DiskStatus{
					State:               computev1.DiskStatus_READY,
					SizeBytes:           21 * (1 << 30),
					ReadWriteAttachment: "inst-1",
					ReadOnlyAttachments: []string{"inst-2", "inst-3"},
				},
			}},
		},
	}
	withFakeAPI(t, api)
	disks, err := ListDisks("project-1")
	if err != nil {
		t.Fatalf("ListNebiusDisks: %v", err)
	}
	if len(disks) != 1 {
		t.Fatalf("disks = %d, want 1", len(disks))
	}
	d := disks[0]
	if d.Name != "d-1" || d.SizeGB != 21 || d.SourceImage != "image-9" {
		t.Fatalf("disk = %+v", d)
	}
	if len(d.Users) != 3 || d.Users[0] != "inst-1" {
		t.Fatalf("users = %+v", d.Users)
	}
}

func TestListNebiusDisksError(t *testing.T) {
	api := &fakeNebiusAPI{err: errors.New("boom")}
	withFakeAPI(t, api)
	if _, err := ListDisks("p"); err == nil {
		t.Fatal("disk list error must surface")
	}
}

func TestListClusters(t *testing.T) {
	api := &fakeNebiusAPI{
		clusters: map[string][]*mk8sv1.Cluster{
			"project-1": {{
				Metadata: &v1.ResourceMetadata{Id: "cluster-1", Name: "k-1"},
				Spec: &mk8sv1.ClusterSpec{
					ControlPlane: &mk8sv1.ControlPlaneSpec{Version: "1.31"},
				},
				Status: &mk8sv1.ClusterStatus{State: mk8sv1.ClusterStatus_RUNNING},
			}},
		},
		nodeGroups: map[string][]*mk8sv1.NodeGroup{
			"cluster-1": {
				{Metadata: &v1.ResourceMetadata{Id: "ng-1", Name: "pool-1"}, Spec: &mk8sv1.NodeGroupSpec{Version: "1.31", Size: &mk8sv1.NodeGroupSpec_FixedNodeCount{FixedNodeCount: 3}}},
			},
		},
	}
	withFakeAPI(t, api)
	clusters, err := ListClusters("project-1")
	if err != nil {
		t.Fatalf("ListNebiusClusters: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("clusters = %d, want 1", len(clusters))
	}
	c := clusters[0]
	if c.Name != "k-1" || c.Version != "1.31" {
		t.Fatalf("cluster = %+v", c)
	}
	if len(c.NodePools) != 1 || c.NodePools[0].Name != "pool-1" || c.NodePools[0].NodeCount != 3 {
		t.Fatalf("node pools = %+v", c.NodePools)
	}
}

func TestListNebiusClustersNodeGroupError(t *testing.T) {
	api := &fakeNebiusAPI{
		clusters: map[string][]*mk8sv1.Cluster{
			"project-1": {{Metadata: &v1.ResourceMetadata{Id: "cluster-1", Name: "k-1"}}},
		},
		err: errors.New("boom"),
	}
	withFakeAPI(t, api)
	if _, err := ListClusters("project-1"); err == nil {
		t.Fatal("node group list error must surface")
	}
}

func TestListNebiusClustersError(t *testing.T) {
	api := &fakeNebiusAPI{err: errors.New("boom")}
	withFakeAPI(t, api)
	if _, err := ListClusters("p"); err == nil {
		t.Fatal("cluster list error must surface")
	}
}

func TestListNetworks(t *testing.T) {
	api := &fakeNebiusAPI{
		networks: map[string][]*vpcv1.Network{
			"project-1": {{Metadata: &v1.ResourceMetadata{Id: "net-1", Name: "nw-1"}}},
		},
	}
	withFakeAPI(t, api)
	nets, err := ListNetworks("project-1")
	if err != nil {
		t.Fatalf("ListNebiusNetworks: %v", err)
	}
	if len(nets) != 1 || nets[0].Name != "nw-1" || nets[0].ProjectID != "project-1" {
		t.Fatalf("networks = %+v", nets)
	}
}

func TestListNebiusNetworksError(t *testing.T) {
	api := &fakeNebiusAPI{err: errors.New("boom")}
	withFakeAPI(t, api)
	if _, err := ListNetworks("p"); err == nil {
		t.Fatal("network list error must surface")
	}
}

func TestListSubnetworks(t *testing.T) {
	api := &fakeNebiusAPI{
		subnets: map[string][]*vpcv1.Subnet{
			"project-1": {{
				Metadata: &v1.ResourceMetadata{Id: "sub-1", Name: "sub-1"},
				Spec:     &vpcv1.SubnetSpec{NetworkId: "net-1"},
				Status: &vpcv1.SubnetStatus{
					State: vpcv1.SubnetStatus_READY,
					// Pools are the current field; the deprecated flat list
					// is only the fallback, covered by its own test below.
					Ipv4PrivatePools: []*vpcv1.SubnetStatusPool{{Cidrs: []string{"10.0.0.0/24", "10.1.0.0/24"}}},
				},
			}},
		},
	}
	withFakeAPI(t, api)
	subs, err := ListSubnetworks("project-1")
	if err != nil {
		t.Fatalf("ListNebiusSubnetworks: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("subnets = %d, want 1", len(subs))
	}
	s := subs[0]
	if s.Name != "sub-1" || s.Network != "net-1" || s.IPRange != "10.0.0.0/24,10.1.0.0/24" {
		t.Fatalf("subnet = %+v", s)
	}
	if s.Status != vpcv1.SubnetStatus_READY.String() {
		t.Fatalf("status = %q", s.Status)
	}
}

// TestListNebiusSubnetworksDeprecatedFallback covers the old flat
// CIDR list: a subnet with no pools falls back to it.
func TestListNebiusSubnetworksDeprecatedFallback(t *testing.T) {
	api := &fakeNebiusAPI{
		subnets: map[string][]*vpcv1.Subnet{
			"project-1": {{
				Metadata: &v1.ResourceMetadata{Id: "sub-2", Name: "sub-2"},
				Status: &vpcv1.SubnetStatus{
					State:            vpcv1.SubnetStatus_READY,
					Ipv4PrivateCidrs: []string{"192.168.0.0/16"},
				},
			}},
		},
	}
	withFakeAPI(t, api)
	subs, err := ListSubnetworks("project-1")
	if err != nil {
		t.Fatalf("ListNebiusSubnetworks: %v", err)
	}
	if len(subs) != 1 || subs[0].IPRange != "192.168.0.0/16" {
		t.Fatalf("subnets = %+v", subs)
	}
}

func TestListNebiusSubnetworksError(t *testing.T) {
	api := &fakeNebiusAPI{err: errors.New("boom")}
	withFakeAPI(t, api)
	if _, err := ListSubnetworks("p"); err == nil {
		t.Fatal("subnet list error must surface")
	}
}

func TestListBuckets(t *testing.T) {
	api := &fakeNebiusAPI{
		buckets: map[string][]*storagev1.Bucket{
			"project-1": {{
				Metadata: &v1.ResourceMetadata{Id: "bucket-1", Name: "b-1"},
				Spec: &storagev1.BucketSpec{
					VersioningPolicy:    storagev1.VersioningPolicy_ENABLED,
					DefaultStorageClass: storagev1.StorageClass_STANDARD,
				},
				Status: &storagev1.BucketStatus{Region: "eu-north1"},
			}},
		},
	}
	withFakeAPI(t, api)
	buckets, err := ListBuckets("project-1")
	if err != nil {
		t.Fatalf("ListNebiusBuckets: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("buckets = %d, want 1", len(buckets))
	}
	b := buckets[0]
	if b.Name != "b-1" || b.Location != "eu-north1" || !b.Versioning {
		t.Fatalf("bucket = %+v", b)
	}
}

func TestListNebiusBucketsError(t *testing.T) {
	api := &fakeNebiusAPI{err: errors.New("boom")}
	withFakeAPI(t, api)
	if _, err := ListBuckets("p"); err == nil {
		t.Fatal("bucket list error must surface")
	}
}

func TestListServiceAccounts(t *testing.T) {
	api := &fakeNebiusAPI{
		serviceAccts: map[string][]*iamv1.ServiceAccount{
			"project-1": {{
				Metadata: &v1.ResourceMetadata{Id: "sa-1", Name: "svc-one"},
				Spec:     &iamv1.ServiceAccountSpec{Description: "the one"},
				Status:   &iamv1.ServiceAccountStatus{Active: true},
			}},
		},
	}
	withFakeAPI(t, api)
	accounts, err := ListServiceAccounts("project-1")
	if err != nil {
		t.Fatalf("ListNebiusServiceAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("accounts = %d, want 1", len(accounts))
	}
	a := accounts[0]
	if a.Email != "sa-1" || a.DisplayName != "svc-one" || a.Disabled {
		t.Fatalf("account = %+v", a)
	}
}

func TestListNebiusServiceAccountsError(t *testing.T) {
	api := &fakeNebiusAPI{err: errors.New("boom")}
	withFakeAPI(t, api)
	if _, err := ListServiceAccounts("p"); err == nil {
		t.Fatal("service account list error must surface")
	}
}

func TestListIAM(t *testing.T) {
	api := &fakeNebiusAPI{
		groups: map[string][]*iamv1.Group{
			"project-1": {{Metadata: &v1.ResourceMetadata{Id: "group-1", Name: "devs"}}},
		},
		permits: map[string][]*iamv1.AccessPermit{
			"group-1": {{
				Metadata: &v1.ResourceMetadata{Id: "permit-1"},
				Spec:     &iamv1.AccessPermitSpec{Role: "editor", ResourceId: "project-1"},
			}},
		},
		memberships: map[string][]*iamv1.GroupMembership{
			"group-1": {{Spec: &iamv1.GroupMembershipSpec{MemberId: "user-alice"}}},
		},
	}
	withFakeAPI(t, api)
	bindings, err := ListIAM("project-1")
	if err != nil {
		t.Fatalf("ListNebiusIAM: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(bindings))
	}
	b := bindings[0]
	if b.Member != "user-alice" || b.Role != "editor" {
		t.Fatalf("binding = %+v", b)
	}
	if !strings.Contains(b.Condition, "group:devs") {
		t.Fatalf("condition = %q", b.Condition)
	}
}

func TestListNebiusIAMNoPermits(t *testing.T) {
	api := &fakeNebiusAPI{
		groups: map[string][]*iamv1.Group{
			"project-1": {{Metadata: &v1.ResourceMetadata{Id: "group-1", Name: "empty"}}},
		},
		permits: map[string][]*iamv1.AccessPermit{"group-1": nil},
	}
	withFakeAPI(t, api)
	bindings, err := ListIAM("project-1")
	if err != nil {
		t.Fatalf("ListNebiusIAM: %v", err)
	}
	if len(bindings) != 0 {
		t.Fatalf("bindings = %d, want 0", len(bindings))
	}
	for _, call := range api.calls {
		if strings.HasPrefix(call, "memberships:") {
			t.Fatalf("memberships listed for a group with no permits: %v", api.calls)
		}
	}
}

func TestListNebiusIAMPermitError(t *testing.T) {
	api := &fakeNebiusAPI{
		groups: map[string][]*iamv1.Group{
			"project-1": {{Metadata: &v1.ResourceMetadata{Id: "group-1", Name: "devs"}}},
		},
		err: errors.New("boom"),
	}
	withFakeAPI(t, api)
	if _, err := ListIAM("project-1"); err == nil {
		t.Fatal("permit list error must surface")
	}
}

func TestListNebiusIAMMembershipError(t *testing.T) {
	api := &fakeNebiusAPI{
		groups: map[string][]*iamv1.Group{
			"project-1": {{Metadata: &v1.ResourceMetadata{Id: "group-1", Name: "devs"}}},
		},
		permits: map[string][]*iamv1.AccessPermit{
			"group-1": {{Metadata: &v1.ResourceMetadata{Id: "permit-1"}, Spec: &iamv1.AccessPermitSpec{Role: "editor", ResourceId: "project-1"}}},
		},
		err: errors.New("boom"),
	}
	withFakeAPI(t, api)
	if _, err := ListIAM("project-1"); err == nil {
		t.Fatal("membership list error must surface")
	}
}

func mustHome(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	return home
}
