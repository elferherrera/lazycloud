package nebius

import (
	"context"
	"errors"
	"fmt"
	"github.com/elferherrera/lazycloud/pkg/providers"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/nebius/gosdk"
	computev1 "github.com/nebius/gosdk/proto/nebius/compute/v1"
	iamv1 "github.com/nebius/gosdk/proto/nebius/iam/v1"
	iamv2 "github.com/nebius/gosdk/proto/nebius/iam/v2"
	mk8sv1 "github.com/nebius/gosdk/proto/nebius/mk8s/v1"
	storagev1 "github.com/nebius/gosdk/proto/nebius/storage/v1"
	vpcv1 "github.com/nebius/gosdk/proto/nebius/vpc/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// nebiusTokenSource produces the IAM token for gRPC calls. The
// default reads NEBIUS_IAM_TOKEN, falling back to the nebius CLI's
// get-access-token, so users with a configured CLI profile need
// no extra setup.
type nebiusTokenSource func() (string, error)

func defaultNebiusTokenSource() nebiusTokenSource {
	return func() (string, error) {
		if token := os.Getenv("NEBIUS_IAM_TOKEN"); token != "" {
			return token, nil
		}
		out, err := exec.Command("nebius", "iam", "get-access-token").Output()
		if err != nil {
			return "", fmt.Errorf("getting nebius token (install the nebius CLI and run `nebius profile create`, or set NEBIUS_IAM_TOKEN): %w", err)
		}
		token := strings.TrimSpace(string(out))
		if token == "" {
			return "", errors.New("nebius CLI returned an empty token")
		}
		return token, nil
	}
}

// nebiusAPI is the narrow seam over the gosdk services the Nebius
// fetchers use. Production wires gosdk services; tests inject
// fakes, so no test touches the network.
type nebiusAPI interface {
	ListTenants(ctx context.Context) ([]*iamv2.Tenant, error)
	ListProjects(ctx context.Context, parentID string) ([]*iamv2.Project, error)
	ListInstances(ctx context.Context, parentID string) ([]*computev1.Instance, error)
	ListDisks(ctx context.Context, parentID string) ([]*computev1.Disk, error)
	ListClusters(ctx context.Context, parentID string) ([]*mk8sv1.Cluster, error)
	ListNodeGroups(ctx context.Context, parentID string) ([]*mk8sv1.NodeGroup, error)
	ListNetworks(ctx context.Context, parentID string) ([]*vpcv1.Network, error)
	ListSubnets(ctx context.Context, parentID string) ([]*vpcv1.Subnet, error)
	ListBuckets(ctx context.Context, parentID string) ([]*storagev1.Bucket, error)
	ListServiceAccounts(ctx context.Context, parentID string) ([]*iamv1.ServiceAccount, error)
	ListGroups(ctx context.Context, parentID string) ([]*iamv1.Group, error)
	ListGroupMembers(ctx context.Context, parentID string) ([]*iamv1.GroupMembership, error)
	ListAccessPermits(ctx context.Context, parentID string) ([]*iamv1.AccessPermit, error)
}

// nebiusClient adapts a gosdk SDK to nebiusAPI, walking pagination
// on every list. The page size is 100 because the compute service
// rejects 1000 with InvalidArgument (live-verified: 50 and 100
// accepted), and an unset size would use a small server default;
// with maxPages=100 a list caps at 10k items, matching the GCP
// fetchers.
type nebiusClient struct {
	sdk      *gosdk.SDK
	tokenSrc nebiusTokenSource
	pageSize int64
	maxPages int
}

// listPages walks one paginated list call. newReq builds the request
// carrying one page token, fetch runs it; the walker feeds each
// response's token forward until a response has none, or the page cap
// trips, which guards against a server looping on the same token.
func listPages[T any, R any](c *nebiusClient, newReq func(pageToken string) R, fetch func(R) (items []T, nextToken string, err error)) ([]T, error) {
	var out []T
	var token string
	for page := 0; page < c.maxPages; page++ {
		req := newReq(token)
		items, nextToken, err := fetch(req)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if nextToken == "" {
			return out, nil
		}
		token = nextToken
	}
	return nil, fmt.Errorf("nebius: list exceeded %d pages", c.maxPages)
}

func newNebiusClient(tokenSrc nebiusTokenSource) (*nebiusClient, error) {
	token, err := tokenSrc()
	if err != nil {
		return nil, err
	}
	// One-shot CLI process: the token is captured once here and never
	// refreshed. If this client is ever reused long-lived, the token
	// source needs a rotating credential instead.
	sdk, err := gosdk.New(context.Background(), gosdk.WithCredentials(gosdk.IAMToken(token)))
	if err != nil {
		return nil, fmt.Errorf("creating nebius sdk: %w", err)
	}
	return &nebiusClient{sdk: sdk, tokenSrc: tokenSrc, pageSize: 100, maxPages: 100}, nil
}

// newNebiusAPIClient builds the production nebiusAPI.
func newNebiusAPIClient() (nebiusAPI, error) {
	return newNebiusClient(defaultNebiusTokenSource())
}

func (c *nebiusClient) ListTenants(ctx context.Context) ([]*iamv2.Tenant, error) {
	svc := c.sdk.Services().IAM().V2().Tenant()
	return listPages(c,
		func(pageToken string) *iamv2.ListTenantsRequest {
			return &iamv2.ListTenantsRequest{PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *iamv2.ListTenantsRequest) ([]*iamv2.Tenant, string, error) {
			resp, err := svc.List(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetItems(), resp.GetNextPageToken(), nil
		})
}

func (c *nebiusClient) ListProjects(ctx context.Context, parentID string) ([]*iamv2.Project, error) {
	svc := c.sdk.Services().IAM().V2().Project()
	return listPages(c,
		func(pageToken string) *iamv2.ListProjectsRequest {
			return &iamv2.ListProjectsRequest{ParentId: parentID, PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *iamv2.ListProjectsRequest) ([]*iamv2.Project, string, error) {
			resp, err := svc.List(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetItems(), resp.GetNextPageToken(), nil
		})
}

func (c *nebiusClient) ListInstances(ctx context.Context, parentID string) ([]*computev1.Instance, error) {
	svc := c.sdk.Services().Compute().V1().Instance()
	return listPages(c,
		func(pageToken string) *computev1.ListInstancesRequest {
			return &computev1.ListInstancesRequest{ParentId: parentID, PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *computev1.ListInstancesRequest) ([]*computev1.Instance, string, error) {
			resp, err := svc.List(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetItems(), resp.GetNextPageToken(), nil
		})
}

func (c *nebiusClient) ListDisks(ctx context.Context, parentID string) ([]*computev1.Disk, error) {
	svc := c.sdk.Services().Compute().V1().Disk()
	return listPages(c,
		func(pageToken string) *computev1.ListDisksRequest {
			return &computev1.ListDisksRequest{ParentId: parentID, PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *computev1.ListDisksRequest) ([]*computev1.Disk, string, error) {
			resp, err := svc.List(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetItems(), resp.GetNextPageToken(), nil
		})
}

func (c *nebiusClient) ListClusters(ctx context.Context, parentID string) ([]*mk8sv1.Cluster, error) {
	svc := c.sdk.Services().MK8S().V1().Cluster()
	return listPages(c,
		func(pageToken string) *mk8sv1.ListClustersRequest {
			return &mk8sv1.ListClustersRequest{ParentId: parentID, PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *mk8sv1.ListClustersRequest) ([]*mk8sv1.Cluster, string, error) {
			resp, err := svc.List(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetItems(), resp.GetNextPageToken(), nil
		})
}

func (c *nebiusClient) ListNodeGroups(ctx context.Context, parentID string) ([]*mk8sv1.NodeGroup, error) {
	svc := c.sdk.Services().MK8S().V1().NodeGroup()
	return listPages(c,
		func(pageToken string) *mk8sv1.ListNodeGroupsRequest {
			return &mk8sv1.ListNodeGroupsRequest{ParentId: parentID, PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *mk8sv1.ListNodeGroupsRequest) ([]*mk8sv1.NodeGroup, string, error) {
			resp, err := svc.List(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetItems(), resp.GetNextPageToken(), nil
		})
}

func (c *nebiusClient) ListNetworks(ctx context.Context, parentID string) ([]*vpcv1.Network, error) {
	svc := c.sdk.Services().VPC().V1().Network()
	return listPages(c,
		func(pageToken string) *vpcv1.ListNetworksRequest {
			return &vpcv1.ListNetworksRequest{ParentId: parentID, PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *vpcv1.ListNetworksRequest) ([]*vpcv1.Network, string, error) {
			resp, err := svc.List(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetItems(), resp.GetNextPageToken(), nil
		})
}

func (c *nebiusClient) ListSubnets(ctx context.Context, parentID string) ([]*vpcv1.Subnet, error) {
	svc := c.sdk.Services().VPC().V1().Subnet()
	return listPages(c,
		func(pageToken string) *vpcv1.ListSubnetsRequest {
			return &vpcv1.ListSubnetsRequest{ParentId: parentID, PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *vpcv1.ListSubnetsRequest) ([]*vpcv1.Subnet, string, error) {
			resp, err := svc.List(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetItems(), resp.GetNextPageToken(), nil
		})
}

func (c *nebiusClient) ListBuckets(ctx context.Context, parentID string) ([]*storagev1.Bucket, error) {
	svc := c.sdk.Services().Storage().V1().Bucket()
	return listPages(c,
		func(pageToken string) *storagev1.ListBucketsRequest {
			return &storagev1.ListBucketsRequest{ParentId: parentID, PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *storagev1.ListBucketsRequest) ([]*storagev1.Bucket, string, error) {
			resp, err := svc.List(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetItems(), resp.GetNextPageToken(), nil
		})
}

func (c *nebiusClient) ListServiceAccounts(ctx context.Context, parentID string) ([]*iamv1.ServiceAccount, error) {
	svc := c.sdk.Services().IAM().V1().ServiceAccount()
	return listPages(c,
		func(pageToken string) *iamv1.ListServiceAccountRequest {
			return &iamv1.ListServiceAccountRequest{ParentId: parentID, PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *iamv1.ListServiceAccountRequest) ([]*iamv1.ServiceAccount, string, error) {
			resp, err := svc.List(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetItems(), resp.GetNextPageToken(), nil
		})
}

func (c *nebiusClient) ListGroups(ctx context.Context, parentID string) ([]*iamv1.Group, error) {
	svc := c.sdk.Services().IAM().V1().Group()
	return listPages(c,
		func(pageToken string) *iamv1.ListGroupsRequest {
			return &iamv1.ListGroupsRequest{ParentId: parentID, PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *iamv1.ListGroupsRequest) ([]*iamv1.Group, string, error) {
			resp, err := svc.List(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetItems(), resp.GetNextPageToken(), nil
		})
}

func (c *nebiusClient) ListGroupMembers(ctx context.Context, parentID string) ([]*iamv1.GroupMembership, error) {
	svc := c.sdk.Services().IAM().V1().GroupMembership()
	return listPages(c,
		func(pageToken string) *iamv1.ListGroupMembershipsRequest {
			return &iamv1.ListGroupMembershipsRequest{ParentId: parentID, PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *iamv1.ListGroupMembershipsRequest) ([]*iamv1.GroupMembership, string, error) {
			resp, err := svc.ListMembers(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetMemberships(), resp.GetNextPageToken(), nil
		})
}

func (c *nebiusClient) ListAccessPermits(ctx context.Context, parentID string) ([]*iamv1.AccessPermit, error) {
	svc := c.sdk.Services().IAM().V1().AccessPermit()
	return listPages(c,
		func(pageToken string) *iamv1.ListAccessPermitRequest {
			return &iamv1.ListAccessPermitRequest{ParentId: parentID, PageSize: c.pageSize, PageToken: pageToken}
		},
		func(req *iamv1.ListAccessPermitRequest) ([]*iamv1.AccessPermit, string, error) {
			resp, err := svc.List(ctx, req)
			if err != nil {
				return nil, "", err
			}
			return resp.GetItems(), resp.GetNextPageToken(), nil
		})
}

// nebiusFetchers holds the openable API behind package-level
// functions with the same signatures as their GCP counterparts, so
// the CLI registry and the cache flow stay untouched.
var nebiusFetchers = struct {
	open func() (nebiusAPI, error)
}{open: newNebiusAPIClient}

func nebiusOpen() (nebiusAPI, error) {
	return nebiusFetchers.open()
}

// fmtTime renders protobuf timestamps the way the cache stores
// them: RFC3339 UTC, empty for nil.
func fmtTime(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return ""
	}
	return ts.AsTime().UTC().Format(time.RFC3339)
}

// ListProjects walks every tenant the token can see and
// returns all projects. The Nebius project id occupies the same
// slot a GCP project id occupies everywhere else in lazycloud.
func ListProjects() ([]providers.Project, error) {
	ctx := context.Background()
	api, err := nebiusOpen()
	if err != nil {
		return nil, err
	}
	tenants, err := api.ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	var projects []providers.Project
	for _, tenant := range tenants {
		items, err := api.ListProjects(ctx, tenant.GetMetadata().GetId())
		if err != nil {
			return nil, err
		}
		for _, p := range items {
			// ProjectNumber carries the Nebius region. The projects table
			// reuses the column GCP fills with its numeric project number;
			// Nebius has no such number, so no value is fabricated.
			projects = append(projects, providers.Project{
				ProjectID:     p.GetMetadata().GetId(),
				Name:          p.GetMetadata().GetName(),
				ProjectNumber: p.GetSpec().GetRegion(),
			})
		}
	}
	return projects, nil
}

// ListVMs maps compute instances onto the VM struct the CLI
// and cache already know. Nebius has no zones, so Zone stays empty
// rather than carrying a fabricated region; the column tolerates it.
func ListVMs(projectID string) ([]providers.VM, error) {
	ctx := context.Background()
	api, err := nebiusOpen()
	if err != nil {
		return nil, err
	}
	instances, err := api.ListInstances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	vms := make([]providers.VM, 0, len(instances))
	for _, inst := range instances {
		vm := providers.VM{
			ProjectID:  projectID,
			Name:       inst.GetMetadata().GetName(),
			InstanceID: inst.GetMetadata().GetId(),
			Status:     inst.GetStatus().GetState().String(),
			Labels:     inst.GetMetadata().GetLabels(),
			UpdatedAt:  fmtTime(inst.GetMetadata().GetUpdatedAt()),
		}
		if res := inst.GetSpec().GetResources(); res != nil {
			vm.MachineType = res.GetPlatform() + "/" + res.GetPreset()
		}
		for _, iface := range inst.GetStatus().GetNetworkInterfaces() {
			vmi := providers.VMInterface{
				InternalIP: stripCIDR(iface.GetIpAddress().GetAddress()),
				ExternalIP: stripCIDR(iface.GetPublicIpAddress().GetAddress()),
			}
			vm.Interfaces = append(vm.Interfaces, vmi)
		}
		if sa := inst.GetSpec().GetServiceAccountId(); sa != "" {
			vm.ServiceAccounts = append(vm.ServiceAccounts, providers.VMServiceAccount{Email: sa})
		}
		vms = append(vms, vm)
	}
	return vms, nil
}

// stripCIDR turns "10.0.0.1/32" into "10.0.0.1"; Nebius reports
// interface addresses in CIDR form.
func stripCIDR(addr string) string {
	if i := strings.Index(addr, "/"); i >= 0 {
		return addr[:i]
	}
	return addr
}

// ListDisks maps compute disks onto the Disk struct.
func ListDisks(projectID string) ([]providers.Disk, error) {
	ctx := context.Background()
	api, err := nebiusOpen()
	if err != nil {
		return nil, err
	}
	disks, err := api.ListDisks(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]providers.Disk, 0, len(disks))
	for _, d := range disks {
		disk := providers.Disk{
			ProjectID: projectID,
			Name:      d.GetMetadata().GetName(),
			Status:    d.GetStatus().GetState().String(),
			UpdatedAt: fmtTime(d.GetMetadata().GetUpdatedAt()),
		}
		// Live disks carry size_bytes (status mirrors spec); the
		// spec size is a oneof across four units, so read the byte
		// total and convert, instead of guessing one arm.
		if bytes := d.GetStatus().GetSizeBytes(); bytes > 0 {
			disk.SizeGB = bytes / (1 << 30)
		}
		disk.SourceImage = d.GetSpec().GetSourceImageId()
		disk.Type = d.GetSpec().GetType().String()
		if rw := d.GetStatus().GetReadWriteAttachment(); rw != "" {
			disk.Users = append(disk.Users, rw)
		}
		disk.Users = append(disk.Users, d.GetStatus().GetReadOnlyAttachments()...)
		out = append(out, disk)
	}
	return out, nil
}

// ListClusters maps mk8s clusters onto the Cluster struct.
// NodeGroups are listed per cluster so the CLI column keeps
// working; Nebius node groups live under the cluster, not the
// project.
func ListClusters(projectID string) ([]providers.Cluster, error) {
	ctx := context.Background()
	api, err := nebiusOpen()
	if err != nil {
		return nil, err
	}
	clusters, err := api.ListClusters(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]providers.Cluster, 0, len(clusters))
	for _, cl := range clusters {
		cluster := providers.Cluster{
			ProjectID: projectID,
			Name:      cl.GetMetadata().GetName(),
			Status:    cl.GetStatus().GetState().String(),
			Version:   cl.GetSpec().GetControlPlane().GetVersion(),
			Labels:    cl.GetMetadata().GetLabels(),
			CreatedAt: fmtTime(cl.GetMetadata().GetCreatedAt()),
			UpdatedAt: fmtTime(cl.GetMetadata().GetUpdatedAt()),
		}
		ngs, err := api.ListNodeGroups(ctx, cl.GetMetadata().GetId())
		if err != nil {
			return nil, err
		}
		for _, ng := range ngs {
			cluster.NodePools = append(cluster.NodePools, providers.ClusterNodePool{
				Name:    ng.GetMetadata().GetName(),
				Version: ng.GetSpec().GetVersion(),
				// A node group is either fixed or autoscaling (oneof size);
				// adding both reads 0 + 0 for the unset side, and for an
				// autoscaling group the max is the capacity worth showing.
				NodeCount: ng.GetSpec().GetFixedNodeCount() + ng.GetSpec().GetAutoscaling().GetMaxNodeCount(),
			})
		}
		out = append(out, cluster)
	}
	return out, nil
}

// ListNetworks fetches one project's VPC networks.
func ListNetworks(projectID string) ([]providers.Network, error) {
	ctx := context.Background()
	api, err := nebiusOpen()
	if err != nil {
		return nil, err
	}
	nets, err := api.ListNetworks(ctx, projectID)
	if err != nil {
		return nil, err
	}
	networks := make([]providers.Network, 0, len(nets))
	for _, n := range nets {
		networks = append(networks, providers.Network{
			ProjectID: projectID,
			Name:      n.GetMetadata().GetName(),
			UpdatedAt: fmtTime(n.GetMetadata().GetUpdatedAt()),
		})
	}
	return networks, nil
}

// subnetCIDRs collects the subnet's IPv4 private CIDRs. The
// pools field replaces the deprecated ipv4_private_cidrs; keep
// the old field as fallback for subnets served by older API
// responses that still populate it.
func subnetCIDRs(s *vpcv1.Subnet) []string {
	var cidrs []string
	for _, pool := range s.GetStatus().GetIpv4PrivatePools() {
		cidrs = append(cidrs, pool.GetCidrs()...)
	}
	if len(cidrs) == 0 {
		cidrs = s.GetStatus().GetIpv4PrivateCidrs()
	}
	return cidrs
}

// ListSubnetworks fetches one project's VPC subnets. The CIDR
// pools flatten into the IPRange field, comma-joined, so the
// networks view keeps one row per subnet like GCP does.
func ListSubnetworks(projectID string) ([]providers.Subnetwork, error) {
	ctx := context.Background()
	api, err := nebiusOpen()
	if err != nil {
		return nil, err
	}
	subs, err := api.ListSubnets(ctx, projectID)
	if err != nil {
		return nil, err
	}
	subnets := make([]providers.Subnetwork, 0, len(subs))
	for _, s := range subs {
		subnets = append(subnets, providers.Subnetwork{
			ProjectID: projectID,
			Name:      s.GetMetadata().GetName(),
			Network:   s.GetSpec().GetNetworkId(),
			IPRange:   strings.Join(subnetCIDRs(s), ","),
			Status:    s.GetStatus().GetState().String(),
			UpdatedAt: fmtTime(s.GetMetadata().GetUpdatedAt()),
		})
	}
	return subnets, nil
}

// ListBuckets maps storage buckets onto the Bucket struct.
func ListBuckets(projectID string) ([]providers.Bucket, error) {
	ctx := context.Background()
	api, err := nebiusOpen()
	if err != nil {
		return nil, err
	}
	buckets, err := api.ListBuckets(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]providers.Bucket, 0, len(buckets))
	for _, b := range buckets {
		bucket := providers.Bucket{
			ProjectID: projectID,
			Name:      b.GetMetadata().GetName(),
			Location:  b.GetStatus().GetRegion(),
			CreatedAt: fmtTime(b.GetMetadata().GetCreatedAt()),
			UpdatedAt: fmtTime(b.GetMetadata().GetUpdatedAt()),
		}
		if b.GetSpec().GetVersioningPolicy() == storagev1.VersioningPolicy_ENABLED {
			bucket.Versioning = true
		}
		bucket.StorageClass = b.GetSpec().GetDefaultStorageClass().String()
		out = append(out, bucket)
	}
	return out, nil
}

// ListServiceAccounts maps iam service accounts onto the
// ServiceAccount struct.
func ListServiceAccounts(projectID string) ([]providers.ServiceAccount, error) {
	ctx := context.Background()
	api, err := nebiusOpen()
	if err != nil {
		return nil, err
	}
	accounts, err := api.ListServiceAccounts(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]providers.ServiceAccount, 0, len(accounts))
	for _, sa := range accounts {
		out = append(out, providers.ServiceAccount{
			ProjectID:   projectID,
			Email:       sa.GetMetadata().GetId(),
			DisplayName: sa.GetMetadata().GetName(),
			UniqueID:    sa.GetMetadata().GetId(),
			Description: sa.GetSpec().GetDescription(),
			Disabled:    !sa.GetStatus().GetActive(),
			UpdatedAt:   fmtTime(sa.GetMetadata().GetUpdatedAt()),
		})
	}
	return out, nil
}

// ListIAM maps the Nebius access model onto the IAMBinding
// rows: one row per (member, permit) pair, where the permit's
// group is the binding context. Condition carries the group name.
func ListIAM(projectID string) ([]providers.IAMBinding, error) {
	ctx := context.Background()
	api, err := nebiusOpen()
	if err != nil {
		return nil, err
	}
	groups, err := api.ListGroups(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var bindings []providers.IAMBinding
	for _, g := range groups {
		groupID := g.GetMetadata().GetId()
		permits, err := api.ListAccessPermits(ctx, groupID)
		if err != nil {
			return nil, err
		}
		if len(permits) == 0 {
			continue
		}
		memberships, err := api.ListGroupMembers(ctx, groupID)
		if err != nil {
			return nil, err
		}
		for _, permit := range permits {
			role := permit.GetSpec().GetRole()
			resource := permit.GetSpec().GetResourceId()
			for _, m := range memberships {
				bindings = append(bindings, providers.IAMBinding{
					ProjectID: projectID,
					Member:    m.GetSpec().GetMemberId(),
					Role:      role,
					Condition: fmt.Sprintf("group:%s resource:%s", g.GetMetadata().GetName(), resource),
					UpdatedAt: fmtTime(permit.GetMetadata().GetCreatedAt()),
				})
			}
		}
	}
	return bindings, nil
}
