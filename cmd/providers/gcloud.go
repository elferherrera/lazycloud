package providers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	computev1 "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
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

type VM struct {
	ProjectID   string
	Name        string
	InstanceID  string
	Zone        string
	MachineType string
	Status      string
	Interfaces  []VMInterface
	BootDisk    *VMDisk
	DataDisks   []VMDisk
	UpdatedAt   string
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

func targetRef(targetURL string) (string, string) {
	if targetURL == "" {
		return "", ""
	}
	parts := strings.Split(targetURL, "/")
	for i, p := range parts {
		switch p {
		case "targetHttpProxies", "targetHttpsProxies", "targetTcpProxies", "targetSslProxies", "targetGrpcProxies", "backendServices":
			if i+1 < len(parts) {
				return p, parts[i+1]
			}
		}
	}
	return "", parts[len(parts)-1]
}

func forwardingRuleFrom(projectID, region string, r *computepb.ForwardingRule) ForwardingRule {
	kind, target := targetRef(r.GetTarget())
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
