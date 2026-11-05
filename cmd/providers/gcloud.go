package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	computev1 "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/storage/v1"
)

type Project struct {
	ProjectID     string
	Name          string
	ProjectNumber string
}

type Network struct {
	ProjectID             string
	Name                  string
	Description           string
	RoutingMode           string
	AutoCreateSubnetworks bool
	MTU                   int32
	UpdatedAt             string
}

type NetworkPeering struct {
	ProjectID            string
	Network              string
	Name                 string
	PeerNetwork          string
	PeerProject          string
	State                string
	StateDetails         string
	AutoCreateRoutes     bool
	ExchangeSubnetRoutes bool
	ExportCustomRoutes   bool
	ImportCustomRoutes   bool
	UpdatedAt            string
}

type Subnetwork struct {
	ProjectID             string
	Name                  string
	Region                string
	Network               string
	IPRange               string
	SecondaryRanges       []string
	PrivateIPGoogleAccess bool
	Purpose               string
	Description           string
	Status                string
	UpdatedAt             string
}

type IPAddress struct {
	ProjectID   string
	Name        string
	Address     string
	AddressType string
	Status      string
	Region      string
	Subnetwork  string
	Network     string
	UpdatedAt   string
}

type VMInterface struct {
	Network    string
	Subnetwork string
	InternalIP string
	ExternalIP string
}

type VMDisk struct {
	Name       string
	DeviceName string
	Mode       string
	AutoDelete bool
	SizeGB     int64
}

type VMServiceAccount struct {
	Email  string
	Scopes []string
}

type VM struct {
	ProjectID       string
	Name            string
	InstanceID      string
	Zone            string
	MachineType     string
	Status          string
	Labels          map[string]string
	NetworkTags     []string
	ServiceAccounts []VMServiceAccount
	InstanceTemplate string
	Interfaces      []VMInterface
	BootDisk        *VMDisk
	DataDisks       []VMDisk
	UpdatedAt       string
}

type Disk struct {
	ProjectID   string
	Name        string
	Zone        string
	SizeGB      int64
	Type        string
	Status      string
	SourceImage string
	Users       []string
	UpdatedAt   string
}

func ListGcloudProjects() ([]Project, error) {
	ctx := context.Background()
	client, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var projects []Project
	it := client.SearchProjects(ctx, &resourcemanagerpb.SearchProjectsRequest{})
	for {
		p, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		projectNumber := p.GetName()
		if parts := strings.Split(projectNumber, "/"); len(parts) > 1 {
			projectNumber = parts[len(parts)-1]
		}
		projects = append(projects, Project{
			ProjectID:     p.GetProjectId(),
			Name:          p.GetDisplayName(),
			ProjectNumber: projectNumber,
		})
	}
	return projects, nil
}

func resourceShortName(resourceURL, fallback string) string {
	if resourceURL == "" {
		return fallback
	}
	if parts := strings.Split(resourceURL, "/"); len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return resourceURL
}

func ResourceProjectName(resourceURL string) string {
	parts := strings.Split(resourceURL, "/")
	for i, part := range parts {
		if part == "projects" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func ListProjectNetworks(projectID string) ([]Network, error) {
	ctx := context.Background()

	client, err := computev1.NewNetworksRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var networks []Network
	it := client.List(ctx, &computepb.ListNetworksRequest{Project: projectID})
	for {
		n, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		networks = append(networks, Network{
			ProjectID:             projectID,
			Name:                  n.GetName(),
			Description:           n.GetDescription(),
			RoutingMode:           n.GetRoutingConfig().GetRoutingMode(),
			AutoCreateSubnetworks: n.GetAutoCreateSubnetworks(),
			MTU:                   n.GetMtu(),
		})
	}
	return networks, nil
}

func ListProjectPeerings(projectID string) ([]NetworkPeering, error) {
	ctx := context.Background()

	client, err := computev1.NewNetworksRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var peerings []NetworkPeering
	it := client.List(ctx, &computepb.ListNetworksRequest{Project: projectID})
	for {
		n, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, p := range n.GetPeerings() {
			peerings = append(peerings, NetworkPeering{
				ProjectID:            projectID,
				Network:              n.GetName(),
				Name:                 p.GetName(),
				PeerNetwork:          resourceShortName(p.GetNetwork(), ""),
				PeerProject:          ResourceProjectName(p.GetNetwork()),
				State:                p.GetState(),
				StateDetails:         p.GetStateDetails(),
				AutoCreateRoutes:     p.GetAutoCreateRoutes(),
				ExchangeSubnetRoutes: p.GetExchangeSubnetRoutes(),
				ExportCustomRoutes:   p.GetExportCustomRoutes(),
				ImportCustomRoutes:   p.GetImportCustomRoutes(),
			})
		}
	}
	return peerings, nil
}

func ListProjectSubnetworks(projectID string) ([]Subnetwork, error) {
	ctx := context.Background()

	client, err := computev1.NewSubnetworksRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var subnetworks []Subnetwork
	it := client.AggregatedList(ctx, &computepb.AggregatedListSubnetworksRequest{Project: projectID})
	for {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, sn := range entry.Value.GetSubnetworks() {
			var secondary []string
			for _, r := range sn.GetSecondaryIpRanges() {
				secondary = append(secondary, fmt.Sprintf("%s=%s", r.GetRangeName(), r.GetIpCidrRange()))
			}
			subnetworks = append(subnetworks, Subnetwork{
				ProjectID:             projectID,
				Name:                  sn.GetName(),
				Region:                resourceShortName(sn.GetRegion(), ""),
				Network:               resourceShortName(sn.GetNetwork(), ""),
				IPRange:               sn.GetIpCidrRange(),
				SecondaryRanges:       secondary,
				PrivateIPGoogleAccess: sn.GetPrivateIpGoogleAccess(),
				Purpose:               sn.GetPurpose(),
				Description:           sn.GetDescription(),
				Status:                sn.GetState(),
			})
		}
	}
	return subnetworks, nil
}

type Route struct {
	ProjectID   string
	Name        string
	Network     string
	DestRange   string
	Priority    int64
	NextHopKind string
	NextHop     string
	Description string
	UpdatedAt   string
}

func routeNextHop(r *computepb.Route) (kind, hop string) {
	switch {
	case r.GetNextHopGateway() != "":
		return "gateway", resourceShortName(r.GetNextHopGateway(), "")
	case r.GetNextHopInstance() != "":
		return "instance", resourceShortName(r.GetNextHopInstance(), "")
	case r.GetNextHopIp() != "":
		return "ip", r.GetNextHopIp()
	case r.GetNextHopVpnTunnel() != "":
		return "vpn-tunnel", resourceShortName(r.GetNextHopVpnTunnel(), "")
	case r.GetNextHopIlb() != "":
		return "ilb", resourceShortName(r.GetNextHopIlb(), "")
	case r.GetNextHopPeering() != "":
		return "peering", r.GetNextHopPeering()
	}
	return "", ""
}

func ListProjectRoutes(projectID string) ([]Route, error) {
	ctx := context.Background()

	client, err := computev1.NewRoutesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var routes []Route
	it := client.List(ctx, &computepb.ListRoutesRequest{Project: projectID})
	for {
		r, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		kind, hop := routeNextHop(r)
		routes = append(routes, Route{
			ProjectID:   projectID,
			Name:        r.GetName(),
			Network:     resourceShortName(r.GetNetwork(), ""),
			DestRange:   r.GetDestRange(),
			Priority:    int64(r.GetPriority()),
			NextHopKind: kind,
			NextHop:     hop,
			Description: r.GetDescription(),
		})
	}
	return routes, nil
}

type FirewallRule struct {
	ProjectID   string
	Name        string
	Network     string
	Direction   string
	Priority    int64
	Action      string
	Source      string
	Targets     string
	Disabled    bool
	Description string
	UpdatedAt   string
}

func firewallAction(f *computepb.Firewall) string {
	var entries []string
	appendAllowed := func(prefix string, rules []*computepb.Allowed) {
		for _, e := range rules {
			entry := prefix + " " + e.GetIPProtocol()
			if ports := e.GetPorts(); len(ports) > 0 {
				entry += ":" + strings.Join(ports, ",")
			}
			entries = append(entries, entry)
		}
	}
	appendAllowed("allow", f.GetAllowed())
	for _, e := range f.GetDenied() {
		entry := "deny " + e.GetIPProtocol()
		if ports := e.GetPorts(); len(ports) > 0 {
			entry += ":" + strings.Join(ports, ",")
		}
		entries = append(entries, entry)
	}
	return strings.Join(entries, ",")
}

func firewallSource(f *computepb.Firewall) string {
	var sources []string
	sources = append(sources, f.GetSourceRanges()...)
	sources = append(sources, f.GetSourceTags()...)
	sources = append(sources, f.GetSourceServiceAccounts()...)
	return strings.Join(sources, ",")
}

func ListProjectFirewallRules(projectID string) ([]FirewallRule, error) {
	ctx := context.Background()

	client, err := computev1.NewFirewallsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var rules []FirewallRule
	it := client.List(ctx, &computepb.ListFirewallsRequest{Project: projectID})
	for {
		f, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		rules = append(rules, FirewallRule{
			ProjectID:   projectID,
			Name:        f.GetName(),
			Network:     resourceShortName(f.GetNetwork(), ""),
			Direction:   f.GetDirection(),
			Priority:    int64(f.GetPriority()),
			Action:      firewallAction(f),
			Source:      firewallSource(f),
			Targets:     strings.Join(f.GetTargetTags(), ","),
			Disabled:    f.GetDisabled(),
			Description: f.GetDescription(),
		})
	}
	return rules, nil
}

type ServiceAccount struct {
	ProjectID   string
	Email       string
	DisplayName string
	UniqueID    string
	Description string
	Disabled    bool
	UpdatedAt   string
}

func ListProjectServiceAccounts(projectID string) ([]ServiceAccount, error) {
	ctx := context.Background()

	svc, err := iam.NewService(ctx)
	if err != nil {
		return nil, err
	}

	var accounts []ServiceAccount
	err = svc.Projects.ServiceAccounts.List("projects/"+projectID).Pages(ctx, func(page *iam.ListServiceAccountsResponse) error {
		for _, sa := range page.Accounts {
			accounts = append(accounts, ServiceAccount{
				ProjectID:   projectID,
				Email:       sa.Email,
				DisplayName: sa.DisplayName,
				UniqueID:    sa.UniqueId,
				Description: sa.Description,
				Disabled:    sa.Disabled,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

func vmDisk(d *computepb.AttachedDisk) *VMDisk {
	return &VMDisk{
		Name:       resourceShortName(d.GetSource(), d.GetDeviceName()),
		DeviceName: d.GetDeviceName(),
		Mode:       d.GetMode(),
		AutoDelete: d.GetAutoDelete(),
		SizeGB:     d.GetDiskSizeGb(),
	}
}

func ListProjectVMs(projectID string) ([]VM, error) {
	ctx := context.Background()

	client, err := computev1.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var vms []VM
	it := client.AggregatedList(ctx, &computepb.AggregatedListInstancesRequest{Project: projectID})
	for {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, inst := range entry.Value.GetInstances() {
			if inst.GetZone() == "" {
				continue
			}
vm := VM{
			ProjectID:   projectID,
			Name:        inst.GetName(),
			InstanceID:  strconv.FormatUint(inst.GetId(), 10),
			Zone:        resourceShortName(inst.GetZone(), ""),
			MachineType: resourceShortName(inst.GetMachineType(), ""),
			Status:      inst.GetStatus(),
			Labels:      inst.GetLabels(),
			NetworkTags: inst.GetTags().GetItems(),
		}
		for _, sa := range inst.GetServiceAccounts() {
			vm.ServiceAccounts = append(vm.ServiceAccounts, VMServiceAccount{Email: sa.GetEmail(), Scopes: sa.GetScopes()})
		}
		for _, m := range inst.GetMetadata().GetItems() {
			if m.GetKey() == "instance-template" {
				vm.InstanceTemplate = resourceShortName(m.GetValue(), "")
				break
			}
		}
			for _, iface := range inst.GetNetworkInterfaces() {
				vmi := VMInterface{
					Network:    resourceShortName(iface.GetNetwork(), ""),
					Subnetwork: resourceShortName(iface.GetSubnetwork(), ""),
					InternalIP: iface.GetNetworkIP(),
				}
				for _, ac := range iface.GetAccessConfigs() {
					if natIP := ac.GetNatIP(); natIP != "" {
						vmi.ExternalIP = natIP
						break
					}
				}
				vm.Interfaces = append(vm.Interfaces, vmi)
			}
			for _, d := range inst.GetDisks() {
				disk := vmDisk(d)
				if d.GetBoot() {
					vm.BootDisk = disk
				} else {
					vm.DataDisks = append(vm.DataDisks, *disk)
				}
			}
			vms = append(vms, vm)
		}
	}
	return vms, nil
}

func ListProjectDisks(projectID string) ([]Disk, error) {
	ctx := context.Background()

	client, err := computev1.NewDisksRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var disks []Disk
	it := client.AggregatedList(ctx, &computepb.AggregatedListDisksRequest{Project: projectID})
	for {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, d := range entry.Value.GetDisks() {
			if d.GetZone() == "" {
				continue
			}
			users := make([]string, 0, len(d.GetUsers()))
			for _, u := range d.GetUsers() {
				users = append(users, resourceShortName(u, ""))
			}
			disks = append(disks, Disk{
				ProjectID:   projectID,
				Name:        d.GetName(),
				Zone:        resourceShortName(d.GetZone(), ""),
				SizeGB:      d.GetSizeGb(),
				Type:        resourceShortName(d.GetType(), ""),
				Status:      d.GetStatus(),
				SourceImage: resourceShortName(d.GetSourceImage(), ""),
				Users:       users,
			})
		}
	}
	return disks, nil
}

type Bucket struct {
	ProjectID     string
	Name          string
	Location      string
	StorageClass  string
	Versioning    bool
	UniformAccess bool
	CreatedAt     string
	UpdatedAt     string
}

func ListProjectBuckets(projectID string) ([]Bucket, error) {
	ctx := context.Background()

	svc, err := storage.NewService(ctx)
	if err != nil {
		return nil, err
	}

	var buckets []Bucket
	err = svc.Buckets.List(projectID).Pages(ctx, func(page *storage.Buckets) error {
		for _, b := range page.Items {
			versioning := b.Versioning != nil && b.Versioning.Enabled
			uniform := b.IamConfiguration != nil &&
				b.IamConfiguration.UniformBucketLevelAccess != nil &&
				b.IamConfiguration.UniformBucketLevelAccess.Enabled
			buckets = append(buckets, Bucket{
				ProjectID:     projectID,
				Name:          b.Name,
				Location:      b.Location,
				StorageClass:  b.StorageClass,
				Versioning:    versioning,
				UniformAccess: uniform,
				CreatedAt:     b.TimeCreated,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return buckets, nil
}

type ForwardingRule struct {
	ProjectID  string
	Name       string
	Region     string
	IPAddress  string
	PortRange  string
	Protocol   string
	Scheme     string
	TargetKind string
	Target     string
	TargetURL  string
	UpdatedAt  string
}

type TargetProxy struct {
	ProjectID      string
	Kind           string
	Name           string
	URLMap         string
	BackendService string
	UpdatedAt      string
}

type PathRule struct {
	Paths   []string
	Service string
}

type URLMap struct {
	ProjectID      string
	Name           string
	DefaultService string
	PathRules      []PathRule
	UpdatedAt      string
}

type Backend struct {
	Group         string
	BalancingMode string
}

type BackendService struct {
	ProjectID    string
	Name         string
	Region       string
	Protocol     string
	PortName     string
	Backends     []Backend
	HealthChecks []string
	UpdatedAt    string
}

type PSCConnection struct {
	Endpoint string
	Project  string
	Network  string
	Status   string
}

type ServiceAttachment struct {
	ProjectID     string
	Name          string
	Region        string
	TargetService string
	Preference    string
	Description   string
	Connections   []PSCConnection
	UpdatedAt     string
}

func targetRef(targetURL string) (string, string) {
	if targetURL == "" {
		return "", ""
	}
	parts := strings.Split(targetURL, "/")
	for i, p := range parts {
		switch p {
		case "targetHttpProxies", "targetHttpsProxies", "targetTcpProxies", "targetSslProxies", "targetGrpcProxies", "backendServices", "serviceAttachments":
			if i+1 < len(parts) {
				return p, parts[i+1]
			}
		}
	}
	return "", parts[len(parts)-1]
}

func ResourcePath(resourceURL string) string {
	if i := strings.Index(resourceURL, "projects/"); i >= 0 {
		return resourceURL[i:]
	}
	return resourceURL
}

func forwardingRuleFrom(projectID, region string, r *computepb.ForwardingRule) ForwardingRule {
	targetURL := r.GetTarget()
	if targetURL == "" {
		targetURL = r.GetBackendService()
	}
	kind, target := targetRef(targetURL)
	return ForwardingRule{
		ProjectID:  projectID,
		Name:       r.GetName(),
		Region:     region,
		IPAddress:  r.GetIPAddress(),
		PortRange:  r.GetPortRange(),
		Protocol:   r.GetIPProtocol(),
		Scheme:     r.GetLoadBalancingScheme(),
		TargetKind: kind,
		Target:     target,
		TargetURL:  targetURL,
	}
}

func ListProjectForwardingRules(projectID string) ([]ForwardingRule, error) {
	ctx := context.Background()

	globalClient, err := computev1.NewGlobalForwardingRulesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer globalClient.Close()

	regionalClient, err := computev1.NewForwardingRulesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer regionalClient.Close()

	var rules []ForwardingRule
	globalIt := globalClient.List(ctx, &computepb.ListGlobalForwardingRulesRequest{Project: projectID})
	for {
		r, err := globalIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		rules = append(rules, forwardingRuleFrom(projectID, "global", r))
	}

	regionalIt := regionalClient.AggregatedList(ctx, &computepb.AggregatedListForwardingRulesRequest{Project: projectID})
	for {
		entry, err := regionalIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, r := range entry.Value.GetForwardingRules() {
			if r.GetRegion() == "" {
				continue
			}
			rules = append(rules, forwardingRuleFrom(projectID, resourceShortName(r.GetRegion(), entry.Key), r))
		}
	}
	return rules, nil
}

func ListProjectTargetProxies(projectID string) ([]TargetProxy, error) {
	ctx := context.Background()

	httpClient, err := computev1.NewTargetHttpProxiesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer httpClient.Close()

	httpsClient, err := computev1.NewTargetHttpsProxiesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer httpsClient.Close()

	tcpClient, err := computev1.NewTargetTcpProxiesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer tcpClient.Close()

	sslClient, err := computev1.NewTargetSslProxiesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer sslClient.Close()

	grpcClient, err := computev1.NewTargetGrpcProxiesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer grpcClient.Close()

	var proxies []TargetProxy
	httpIt := httpClient.List(ctx, &computepb.ListTargetHttpProxiesRequest{Project: projectID})
	for {
		p, err := httpIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		proxies = append(proxies, TargetProxy{ProjectID: projectID, Kind: "targetHttpProxies", Name: p.GetName(), URLMap: resourceShortName(p.GetUrlMap(), "")})
	}
	httpsIt := httpsClient.List(ctx, &computepb.ListTargetHttpsProxiesRequest{Project: projectID})
	for {
		p, err := httpsIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		proxies = append(proxies, TargetProxy{ProjectID: projectID, Kind: "targetHttpsProxies", Name: p.GetName(), URLMap: resourceShortName(p.GetUrlMap(), "")})
	}
	tcpIt := tcpClient.List(ctx, &computepb.ListTargetTcpProxiesRequest{Project: projectID})
	for {
		p, err := tcpIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		proxies = append(proxies, TargetProxy{ProjectID: projectID, Kind: "targetTcpProxies", Name: p.GetName(), BackendService: resourceShortName(p.GetService(), "")})
	}
	sslIt := sslClient.List(ctx, &computepb.ListTargetSslProxiesRequest{Project: projectID})
	for {
		p, err := sslIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		proxies = append(proxies, TargetProxy{ProjectID: projectID, Kind: "targetSslProxies", Name: p.GetName(), BackendService: resourceShortName(p.GetService(), "")})
	}
	grpcIt := grpcClient.List(ctx, &computepb.ListTargetGrpcProxiesRequest{Project: projectID})
	for {
		p, err := grpcIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		proxies = append(proxies, TargetProxy{ProjectID: projectID, Kind: "targetGrpcProxies", Name: p.GetName(), URLMap: resourceShortName(p.GetUrlMap(), "")})
	}
	return proxies, nil
}

func ListProjectURLMaps(projectID string) ([]URLMap, error) {
	ctx := context.Background()

	client, err := computev1.NewUrlMapsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var maps []URLMap
	it := client.List(ctx, &computepb.ListUrlMapsRequest{Project: projectID})
	for {
		m, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		um := URLMap{
			ProjectID:      projectID,
			Name:           m.GetName(),
			DefaultService: resourceShortName(m.GetDefaultService(), ""),
		}
		for _, pm := range m.GetPathMatchers() {
			for _, pr := range pm.GetPathRules() {
				um.PathRules = append(um.PathRules, PathRule{Paths: pr.GetPaths(), Service: resourceShortName(pr.GetService(), "")})
			}
		}
		maps = append(maps, um)
	}
	return maps, nil
}

func backendServiceFrom(projectID, region string, b *computepb.BackendService) BackendService {
	svc := BackendService{
		ProjectID: projectID,
		Name:      b.GetName(),
		Region:    region,
		Protocol:  b.GetProtocol(),
		PortName:  b.GetPortName(),
	}
	for _, be := range b.GetBackends() {
		svc.Backends = append(svc.Backends, Backend{Group: resourceShortName(be.GetGroup(), ""), BalancingMode: be.GetBalancingMode()})
	}
	for _, hc := range b.GetHealthChecks() {
		svc.HealthChecks = append(svc.HealthChecks, resourceShortName(hc, ""))
	}
	return svc
}

func ListProjectBackendServices(projectID string) ([]BackendService, error) {
	ctx := context.Background()

	client, err := computev1.NewBackendServicesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var services []BackendService
	it := client.List(ctx, &computepb.ListBackendServicesRequest{Project: projectID})
	for {
		s, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		services = append(services, backendServiceFrom(projectID, "global", s))
	}

	aggIt := client.AggregatedList(ctx, &computepb.AggregatedListBackendServicesRequest{Project: projectID})
	for {
		entry, err := aggIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, s := range entry.Value.GetBackendServices() {
			if s.GetRegion() == "" {
				continue
			}
			services = append(services, backendServiceFrom(projectID, resourceShortName(s.GetRegion(), entry.Key), s))
		}
	}
	return services, nil
}

type ClusterNodePool struct {
	Name      string
	Version   string
	NodeCount int64
}

type Cluster struct {
	ProjectID   string
	Name        string
	Description string
	Location    string
	Status      string
	Version     string
	Endpoint    string
	NodePools   []ClusterNodePool
	CreatedAt   string
	UpdatedAt   string
}

func ListProjectClusters(projectID string) ([]Cluster, error) {
	ctx := context.Background()

	svc, err := container.NewService(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := svc.Projects.Locations.Clusters.List("projects/" + projectID + "/locations/-").Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	var clusters []Cluster
	for _, c := range resp.Clusters {
		cluster := Cluster{
			ProjectID:   projectID,
			Name:        c.Name,
			Description: c.Description,
			Location:    c.Location,
			Status:      c.Status,
			Version:     c.CurrentMasterVersion,
			Endpoint:    c.Endpoint,
			CreatedAt:   c.CreateTime,
		}
		for _, np := range c.NodePools {
			cluster.NodePools = append(cluster.NodePools, ClusterNodePool{Name: np.Name, Version: np.Version, NodeCount: np.InitialNodeCount})
		}
		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

type Workload struct {
	ProjectID string
	Cluster   string
	Kind      string
	Namespace string
	Name      string
	Replicas  string
	Images    []string
	UpdatedAt string
}

type k8sWorkload struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Replicas *int32 `json:"replicas"`
		Template struct {
			Spec struct {
				Containers []struct {
					Image string `json:"image"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		ReadyReplicas          int32 `json:"readyReplicas"`
		DesiredNumberScheduled int32 `json:"desiredNumberScheduled"`
		NumberReady            int32 `json:"numberReady"`
	} `json:"status"`
}

type k8sWorkloadList struct {
	Items []k8sWorkload `json:"items"`
}

func ListClusterWorkloads(projectID, location, name string) ([]Workload, error) {
	scopeFlag := "--region"
	if strings.Count(location, "-") > 1 {
		scopeFlag = "--zone"
	}
	dir, err := os.MkdirTemp("", "lazycloud-kube-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	kubeconfig := filepath.Join(dir, "config")

	creds := exec.Command("gcloud", "container", "clusters", "get-credentials", name,
		scopeFlag, location, "--project", projectID)
	creds.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	if out, err := creds.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("gcloud get-credentials: %w: %s", err, strings.TrimSpace(string(out)))
	}

	get := exec.Command("kubectl", "get", "deployments,statefulsets,daemonsets", "--all-namespaces", "--output", "json")
	get.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	out, err := get.Output()
	if err != nil {
		message := err.Error()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			message = string(exitErr.Stderr)
		}
		return nil, fmt.Errorf("kubectl get: %s", strings.TrimSpace(message))
	}

	var list k8sWorkloadList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, err
	}

	var workloads []Workload
	for _, item := range list.Items {
		w := Workload{
			ProjectID: projectID,
			Cluster:   name,
			Kind:      item.Kind,
			Namespace: item.Metadata.Namespace,
			Name:      item.Metadata.Name,
		}
		for _, c := range item.Spec.Template.Spec.Containers {
			if c.Image != "" {
				w.Images = append(w.Images, c.Image)
			}
		}
		if item.Kind == "DaemonSet" {
			w.Replicas = fmt.Sprintf("%d/%d", item.Status.NumberReady, item.Status.DesiredNumberScheduled)
		} else {
			total := int32(0)
			if item.Spec.Replicas != nil {
				total = *item.Spec.Replicas
			}
			w.Replicas = fmt.Sprintf("%d/%d", item.Status.ReadyReplicas, total)
		}
		workloads = append(workloads, w)
	}
	return workloads, nil
}

type ClusterNode struct {
	ProjectID  string
	Cluster    string
	Name       string
	NodePool   string
	Status     string
	InternalIP string
	UpdatedAt  string
}

type k8sNode struct {
	Metadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
	Status struct {
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
		Addresses []struct {
			Type    string `json:"type"`
			Address string `json:"address"`
		} `json:"addresses"`
	} `json:"status"`
}

type k8sNodeList struct {
	Items []k8sNode `json:"items"`
}

func ListClusterNodes(projectID, location, name string) ([]ClusterNode, error) {
	scopeFlag := "--region"
	if strings.Count(location, "-") > 1 {
		scopeFlag = "--zone"
	}
	dir, err := os.MkdirTemp("", "lazycloud-kube-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	kubeconfig := filepath.Join(dir, "config")

	creds := exec.Command("gcloud", "container", "clusters", "get-credentials", name,
		scopeFlag, location, "--project", projectID)
	creds.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	if out, err := creds.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("gcloud get-credentials: %w: %s", err, strings.TrimSpace(string(out)))
	}

	get := exec.Command("kubectl", "get", "nodes", "--output", "json")
	get.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	out, err := get.Output()
	if err != nil {
		message := err.Error()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			message = string(exitErr.Stderr)
		}
		return nil, fmt.Errorf("kubectl get: %s", strings.TrimSpace(message))
	}

	var list k8sNodeList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, err
	}

	nodes := make([]ClusterNode, 0, len(list.Items))
	for _, item := range list.Items {
		node := ClusterNode{
			ProjectID: projectID,
			Cluster:    name,
			Name:       item.Metadata.Name,
			NodePool:   item.Metadata.Labels["cloud.google.com/gke-nodepool"],
			Status:     "NotReady",
		}
		for _, c := range item.Status.Conditions {
			if c.Type == "Ready" {
				if c.Status == "True" {
					node.Status = "Ready"
				}
				break
			}
		}
		for _, addr := range item.Status.Addresses {
			if addr.Type == "InternalIP" {
				node.InternalIP = addr.Address
				break
			}
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

type DNSZone struct {
	ProjectID   string
	Name        string
	DNSName     string
	Visibility  string
	Description string
	UpdatedAt   string
}

type DNSRecord struct {
	ProjectID string
	Zone      string
	Name      string
	Type      string
	TTL       int64
	Data      []string
	UpdatedAt string
}

func ListProjectDNSZones(projectID string) ([]DNSZone, error) {
	ctx := context.Background()

	svc, err := dns.NewService(ctx)
	if err != nil {
		return nil, err
	}

	var zones []DNSZone
	err = svc.ManagedZones.List(projectID).Pages(ctx, func(page *dns.ManagedZonesListResponse) error {
		for _, z := range page.ManagedZones {
			zones = append(zones, DNSZone{
				ProjectID:   projectID,
				Name:        z.Name,
				DNSName:     z.DnsName,
				Visibility:  z.Visibility,
				Description: z.Description,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return zones, nil
}

func ListZoneDNSRecords(projectID, zone string) ([]DNSRecord, error) {
	ctx := context.Background()

	svc, err := dns.NewService(ctx)
	if err != nil {
		return nil, err
	}

	var records []DNSRecord
	err = svc.ResourceRecordSets.List(projectID, zone).Pages(ctx, func(page *dns.ResourceRecordSetsListResponse) error {
		for _, r := range page.Rrsets {
			records = append(records, DNSRecord{
				ProjectID: projectID,
				Zone:      zone,
				Name:      r.Name,
				Type:      r.Type,
				TTL:       r.Ttl,
				Data:      r.Rrdatas,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func ListProjectIPAddresses(projectID string) ([]IPAddress, error) {
	ctx := context.Background()

	addressesClient, err := computev1.NewAddressesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer addressesClient.Close()

	globalClient, err := computev1.NewGlobalAddressesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer globalClient.Close()

	var addresses []IPAddress

	globalIt := globalClient.List(ctx, &computepb.ListGlobalAddressesRequest{Project: projectID})
	for {
		addr, err := globalIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, IPAddress{
			Name:        addr.GetName(),
			Address:     addr.GetAddress(),
			AddressType: addr.GetAddressType(),
			Status:      addr.GetStatus(),
			Region:      "global",
			Subnetwork:  resourceShortName(addr.GetSubnetwork(), ""),
			Network:     resourceShortName(addr.GetNetwork(), ""),
		})
	}

	regionalIt := addressesClient.AggregatedList(ctx, &computepb.AggregatedListAddressesRequest{Project: projectID})
	for {
		entry, err := regionalIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, addr := range entry.Value.GetAddresses() {
			if addr.GetRegion() == "" {
				continue
			}
			region := resourceShortName(addr.GetRegion(), "global")
			addresses = append(addresses, IPAddress{
				Name:        addr.GetName(),
				Address:     addr.GetAddress(),
				AddressType: addr.GetAddressType(),
				Status:      addr.GetStatus(),
				Region:      region,
				Subnetwork:  resourceShortName(addr.GetSubnetwork(), ""),
				Network:     resourceShortName(addr.GetNetwork(), ""),
			})
		}
	}
	return addresses, nil
}

type CloudRouterPeer struct {
	Name      string
	Interface string
	IPAddress string
	PeerIP    string
	PeerASN   uint32
	Priority  uint32
}

type CloudRouter struct {
	ProjectID   string
	Name        string
	Region      string
	Network     string
	Description string
	BGPASN      uint32
	Peers       []CloudRouterPeer
	NATs        []string
	CreatedAt   string
	UpdatedAt   string
}

func ListProjectCloudRouters(projectID string) ([]CloudRouter, error) {
	ctx := context.Background()

	client, err := computev1.NewRoutersRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var routers []CloudRouter
	it := client.AggregatedList(ctx, &computepb.AggregatedListRoutersRequest{Project: projectID})
	for {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, r := range entry.Value.GetRouters() {
			if r.GetRegion() == "" {
				continue
			}
			peers := make([]CloudRouterPeer, 0, len(r.GetBgpPeers()))
			for _, p := range r.GetBgpPeers() {
				peers = append(peers, CloudRouterPeer{
					Name:      p.GetName(),
					Interface: p.GetInterfaceName(),
					IPAddress: p.GetIpAddress(),
					PeerIP:    p.GetPeerIpAddress(),
					PeerASN:   p.GetPeerAsn(),
					Priority:  p.GetAdvertisedRoutePriority(),
				})
			}
			nats := make([]string, 0, len(r.GetNats()))
			for _, n := range r.GetNats() {
				nats = append(nats, n.GetName())
			}
			routers = append(routers, CloudRouter{
				ProjectID:   projectID,
				Name:        r.GetName(),
				Region:      resourceShortName(r.GetRegion(), ""),
				Network:     resourceShortName(r.GetNetwork(), ""),
				Description: r.GetDescription(),
				BGPASN:      r.GetBgp().GetAsn(),
				Peers:       peers,
				NATs:        nats,
				CreatedAt:   r.GetCreationTimestamp(),
			})
		}
	}
	return routers, nil
}

type VPNTunnel struct {
	Name      string
	Interface int32
	PeerIP    string
	IKE       int32
	Status    string
	Router    string
}

type VPNGateway struct {
	ProjectID   string
	Name        string
	Type        string
	Region      string
	Network     string
	IPs         []string
	Description string
	Tunnels     []VPNTunnel
	CreatedAt   string
	UpdatedAt   string
}

func ListProjectServiceAttachments(projectID string) ([]ServiceAttachment, error) {
	ctx := context.Background()

	client, err := computev1.NewServiceAttachmentsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var attachments []ServiceAttachment
	it := client.AggregatedList(ctx, &computepb.AggregatedListServiceAttachmentsRequest{Project: projectID})
	for {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, sa := range entry.Value.GetServiceAttachments() {
			if sa.GetRegion() == "" {
				continue
			}
			conns := make([]PSCConnection, 0, len(sa.GetConnectedEndpoints()))
			for _, e := range sa.GetConnectedEndpoints() {
				conns = append(conns, PSCConnection{
					Endpoint: resourceShortName(e.GetEndpoint(), ""),
					Project:  ResourceProjectName(e.GetEndpoint()),
					Network:  resourceShortName(e.GetConsumerNetwork(), ""),
					Status:   e.GetStatus(),
				})
			}
			attachments = append(attachments, ServiceAttachment{
				ProjectID:     projectID,
				Name:          sa.GetName(),
				Region:        resourceShortName(sa.GetRegion(), ""),
				TargetService: resourceShortName(sa.GetTargetService(), ""),
				Preference:    sa.GetConnectionPreference(),
				Description:   sa.GetDescription(),
				Connections:   conns,
			})
		}
	}
	return attachments, nil
}

func ListProjectVPNGateways(projectID string) ([]VPNGateway, error) {
	ctx := context.Background()

	haClient, err := computev1.NewVpnGatewaysRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer haClient.Close()

	classicClient, err := computev1.NewTargetVpnGatewaysRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer classicClient.Close()

	tunnelsClient, err := computev1.NewVpnTunnelsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer tunnelsClient.Close()

	tunnelsByGateway := map[string][]VPNTunnel{}
	tunnelsIt := tunnelsClient.AggregatedList(ctx, &computepb.AggregatedListVpnTunnelsRequest{Project: projectID})
	for {
		entry, err := tunnelsIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, t := range entry.Value.GetVpnTunnels() {
			gateway := t.GetVpnGateway()
			if gateway == "" {
				gateway = t.GetTargetVpnGateway()
			}
			tunnelsByGateway[gateway] = append(tunnelsByGateway[gateway], VPNTunnel{
				Name:      t.GetName(),
				Interface: t.GetVpnGatewayInterface(),
				PeerIP:    t.GetPeerIp(),
				IKE:       t.GetIkeVersion(),
				Status:    t.GetStatus(),
				Router:    resourceShortName(t.GetRouter(), ""),
			})
		}
	}

	var gateways []VPNGateway
	haIt := haClient.AggregatedList(ctx, &computepb.AggregatedListVpnGatewaysRequest{Project: projectID})
	for {
		entry, err := haIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, g := range entry.Value.GetVpnGateways() {
			if g.GetRegion() == "" {
				continue
			}
			ips := make([]string, 0, len(g.GetVpnInterfaces()))
			for _, i := range g.GetVpnInterfaces() {
				ips = append(ips, i.GetIpAddress())
			}
			gateways = append(gateways, VPNGateway{
				ProjectID:   projectID,
				Name:        g.GetName(),
				Type:        "ha",
				Region:      resourceShortName(g.GetRegion(), ""),
				Network:     resourceShortName(g.GetNetwork(), ""),
				IPs:         ips,
				Description: g.GetDescription(),
				Tunnels:     tunnelsByGateway[g.GetSelfLink()],
				CreatedAt:   g.GetCreationTimestamp(),
			})
		}
	}
	classicIt := classicClient.AggregatedList(ctx, &computepb.AggregatedListTargetVpnGatewaysRequest{Project: projectID})
	for {
		entry, err := classicIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, g := range entry.Value.GetTargetVpnGateways() {
			if g.GetRegion() == "" {
				continue
			}
			gateways = append(gateways, VPNGateway{
				ProjectID:   projectID,
				Name:        g.GetName(),
				Type:        "classic",
				Region:      resourceShortName(g.GetRegion(), ""),
				Network:     resourceShortName(g.GetNetwork(), ""),
				Description: g.GetDescription(),
				Tunnels:     tunnelsByGateway[g.GetSelfLink()],
				CreatedAt:   g.GetCreationTimestamp(),
			})
		}
	}
	return gateways, nil
}

type InstanceGroup struct {
	ProjectID   string
	Name        string
	Zone        string
	Type        string
	Size        int32
	Network     string
	Subnetwork  string
	Template    string
	Description string
	UpdatedAt   string
}

func ListProjectInstanceGroups(projectID string) ([]InstanceGroup, error) {
	ctx := context.Background()

	groupsClient, err := computev1.NewInstanceGroupsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer groupsClient.Close()

	managersClient, err := computev1.NewInstanceGroupManagersRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer managersClient.Close()

	unmanaged := make(map[string]InstanceGroup)
	groupsIt := groupsClient.AggregatedList(ctx, &computepb.AggregatedListInstanceGroupsRequest{Project: projectID})
	for {
		entry, err := groupsIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, g := range entry.Value.GetInstanceGroups() {
			if g.GetZone() == "" && g.GetRegion() == "" {
				continue
			}
			location := resourceShortName(g.GetZone(), "")
			if location == "" {
				location = resourceShortName(g.GetRegion(), "")
			}
			unmanaged[location+"/"+g.GetName()] = InstanceGroup{
				ProjectID:   projectID,
				Name:        g.GetName(),
				Zone:        location,
				Type:        "unmanaged",
				Size:        g.GetSize(),
				Network:     resourceShortName(g.GetNetwork(), ""),
				Subnetwork:  resourceShortName(g.GetSubnetwork(), ""),
				Description: g.GetDescription(),
			}
		}
	}

	var groups []InstanceGroup
	managed := make(map[string]bool)
	managersIt := managersClient.AggregatedList(ctx, &computepb.AggregatedListInstanceGroupManagersRequest{Project: projectID})
	for {
		entry, err := managersIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, m := range entry.Value.GetInstanceGroupManagers() {
			if m.GetZone() == "" && m.GetRegion() == "" {
				continue
			}
			location := resourceShortName(m.GetZone(), "")
			if location == "" {
				location = resourceShortName(m.GetRegion(), "")
			}
			key := location + "/" + m.GetName()
			managed[key] = true
			group := InstanceGroup{
				ProjectID:   projectID,
				Name:        m.GetName(),
				Zone:        location,
				Type:        "managed",
				Size:        m.GetTargetSize(),
				Template:    resourceShortName(m.GetInstanceTemplate(), ""),
				Description: m.GetDescription(),
			}
			if g, ok := unmanaged[key]; ok {
				group.Network = g.Network
				group.Subnetwork = g.Subnetwork
			}
			groups = append(groups, group)
		}
	}
	for _, g := range unmanaged {
		if managed[g.Zone+"/"+g.Name] {
			continue
		}
		groups = append(groups, g)
	}
	return groups, nil
}

type InstanceGroupMember struct {
	Zone string
	Name string
}

func instanceRefFromLink(link string) (string, string) {
	parts := strings.Split(link, "/")
	zone := ""
	name := ""
	for i, part := range parts {
		if (part == "zones" || part == "regions") && i+1 < len(parts) {
			zone = parts[i+1]
		}
		if part == "instances" && i+1 < len(parts) {
			name = parts[i+1]
		}
	}
	return zone, name
}

func instanceGroupMembers(it *computev1.InstanceWithNamedPortsIterator) ([]InstanceGroupMember, error) {
	var members []InstanceGroupMember
	for {
		item, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		zone, name := instanceRefFromLink(item.GetInstance())
		if zone == "" || name == "" {
			continue
		}
		members = append(members, InstanceGroupMember{Zone: zone, Name: name})
	}
	return members, nil
}

func ListInstanceGroupVMs(projectID, location, name string) ([]InstanceGroupMember, error) {
	ctx := context.Background()
	if strings.Count(location, "-") > 1 {
		client, err := computev1.NewInstanceGroupsRESTClient(ctx)
		if err != nil {
			return nil, err
		}
		defer client.Close()
		return instanceGroupMembers(client.ListInstances(ctx, &computepb.ListInstancesInstanceGroupsRequest{
			Project:       projectID,
			Zone:          location,
			InstanceGroup: name,
		}))
	}
	client, err := computev1.NewRegionInstanceGroupsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	return instanceGroupMembers(client.ListInstances(ctx, &computepb.ListInstancesRegionInstanceGroupsRequest{
		Project:       projectID,
		Region:        location,
		InstanceGroup: name,
	}))
}

type Reservation struct {
	ProjectID   string
	Name        string
	Zone        string
	MachineType string
	Count       int64
	InUseCount  int64
	ShareType   string
	Required    bool
	Status      string
	Description string
	Commitment  string
	UpdatedAt   string
}

func ListProjectReservations(projectID string) ([]Reservation, error) {
	ctx := context.Background()

	client, err := computev1.NewReservationsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var reservations []Reservation
	it := client.AggregatedList(ctx, &computepb.AggregatedListReservationsRequest{Project: projectID})
	for {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, r := range entry.Value.GetReservations() {
			if r.GetZone() == "" {
				continue
			}
			reservations = append(reservations, Reservation{
				ProjectID:   projectID,
				Name:        r.GetName(),
				Zone:        resourceShortName(r.GetZone(), ""),
				MachineType: resourceShortName(r.GetSpecificReservation().GetInstanceProperties().GetMachineType(), ""),
				Count:       r.GetSpecificReservation().GetCount(),
				InUseCount:  r.GetSpecificReservation().GetInUseCount(),
				ShareType:   r.GetShareSettings().GetShareType(),
				Required:    r.GetSpecificReservationRequired(),
				Status:      r.GetStatus(),
				Description: r.GetDescription(),
				Commitment:  resourceShortName(r.GetCommitment(), ""),
			})
		}
	}
	return reservations, nil
}

type Image struct {
	ProjectID       string
	Name            string
	Family          string
	Status          string
	SizeGB          int64
	StorageLocation string
	Source          string
	CreatedAt       string
	Description     string
	Deprecated      string
	UpdatedAt       string
}

func imageSource(img *computepb.Image) string {
	for _, source := range []string{img.GetSourceDisk(), img.GetSourceImage(), img.GetSourceSnapshot()} {
		if source != "" {
			return resourceShortName(source, "")
		}
	}
	return ""
}

func ListProjectImages(projectID string) ([]Image, error) {
	ctx := context.Background()

	client, err := computev1.NewImagesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var images []Image
	it := client.List(ctx, &computepb.ListImagesRequest{Project: projectID})
	for {
		img, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		images = append(images, Image{
			ProjectID:       projectID,
			Name:            img.GetName(),
			Family:          img.GetFamily(),
			Status:          img.GetStatus(),
			SizeGB:          img.GetDiskSizeGb(),
			StorageLocation: strings.Join(img.GetStorageLocations(), ","),
			Source:          imageSource(img),
			CreatedAt:       img.GetCreationTimestamp(),
			Description:     img.GetDescription(),
			Deprecated:      img.GetDeprecated().GetState(),
		})
	}
	return images, nil
}

var PublicImageProjects = []string{
	"almalinux-cloud",
	"centos-cloud",
	"cos-cloud",
	"debian-cloud",
	"opensuse-cloud",
	"oracle-linux-cloud",
	"rhel-cloud",
	"rocky-linux-cloud",
	"suse-cloud",
	"suse-sap-cloud",
	"ubuntu-os-cloud",
	"windows-cloud",
	"windows-sql-cloud",
}

type InstanceTemplateDisk struct {
	DeviceName  string
	SourceImage string
	DiskSizeGB  int64
	DiskType    string
	AutoDelete  bool
	Boot        bool
}

type InstanceTemplateInterface struct {
	Network    string
	Subnetwork string
	NetworkIP  string
	ExternalIP string
}

type InstanceTemplate struct {
	ProjectID   string
	Name        string
	Location    string
	MachineType string
	Disks       []InstanceTemplateDisk
	Interfaces  []InstanceTemplateInterface
	Labels      map[string]string
	Description string
	CreatedAt   string
	UpdatedAt   string
}

func ListProjectInstanceTemplates(projectID string) ([]InstanceTemplate, error) {
	ctx := context.Background()

	client, err := computev1.NewInstanceTemplatesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var templates []InstanceTemplate
	it := client.AggregatedList(ctx, &computepb.AggregatedListInstanceTemplatesRequest{Project: projectID})
	for {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, t := range entry.Value.GetInstanceTemplates() {
			location := resourceShortName(t.GetRegion(), "")
			if location == "" {
				location = "global"
			}
			template := InstanceTemplate{
				ProjectID:   projectID,
				Name:        t.GetName(),
				Location:    location,
				MachineType: resourceShortName(t.GetProperties().GetMachineType(), ""),
				Labels:      t.GetProperties().GetLabels(),
				Description: t.GetDescription(),
				CreatedAt:   t.GetCreationTimestamp(),
			}
			for _, d := range t.GetProperties().GetDisks() {
				template.Disks = append(template.Disks, InstanceTemplateDisk{
					DeviceName:  d.GetDeviceName(),
					SourceImage: resourceShortName(d.GetInitializeParams().GetSourceImage(), ""),
					DiskSizeGB:  d.GetInitializeParams().GetDiskSizeGb(),
					DiskType:    resourceShortName(d.GetInitializeParams().GetDiskType(), ""),
					AutoDelete:  d.GetAutoDelete(),
					Boot:        d.GetBoot(),
				})
			}
			for _, iface := range t.GetProperties().GetNetworkInterfaces() {
				ti := InstanceTemplateInterface{
					Network:    resourceShortName(iface.GetNetwork(), ""),
					Subnetwork: resourceShortName(iface.GetSubnetwork(), ""),
					NetworkIP:  iface.GetNetworkIP(),
				}
				for _, ac := range iface.GetAccessConfigs() {
					if natIP := ac.GetNatIP(); natIP != "" {
						ti.ExternalIP = natIP
						break
					}
				}
				template.Interfaces = append(template.Interfaces, ti)
			}
			templates = append(templates, template)
		}
	}
	return templates, nil
}
