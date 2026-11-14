package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elferherrera/lazycloud/pkg/providers"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	computev1 "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	iampb "cloud.google.com/go/iam/apiv1/iampb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/storage/v1"
)

func ListGcloudProjects() ([]providers.Project, error) {
	ctx := context.Background()
	client, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var projects []providers.Project
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
		projects = append(projects, providers.Project{
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

func ListProjectNetworks(projectID string) ([]providers.Network, error) {
	ctx := context.Background()

	client, err := computev1.NewNetworksRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var networks []providers.Network
	it := client.List(ctx, &computepb.ListNetworksRequest{Project: projectID})
	for {
		n, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		networks = append(networks, providers.Network{
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

func ListProjectPeerings(projectID string) ([]providers.NetworkPeering, error) {
	ctx := context.Background()

	client, err := computev1.NewNetworksRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var peerings []providers.NetworkPeering
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
			peerings = append(peerings, providers.NetworkPeering{
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

func ListProjectSubnetworks(projectID string) ([]providers.Subnetwork, error) {
	ctx := context.Background()

	client, err := computev1.NewSubnetworksRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var subnetworks []providers.Subnetwork
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
			subnetworks = append(subnetworks, providers.Subnetwork{
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

func ListProjectRoutes(projectID string) ([]providers.Route, error) {
	ctx := context.Background()

	client, err := computev1.NewRoutesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var routes []providers.Route
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
		routes = append(routes, providers.Route{
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

func ListProjectFirewallRules(projectID string) ([]providers.FirewallRule, error) {
	ctx := context.Background()

	client, err := computev1.NewFirewallsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var rules []providers.FirewallRule
	it := client.List(ctx, &computepb.ListFirewallsRequest{Project: projectID})
	for {
		f, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		rules = append(rules, providers.FirewallRule{
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

func ListProjectServiceAccounts(projectID string) ([]providers.ServiceAccount, error) {
	ctx := context.Background()

	svc, err := iam.NewService(ctx)
	if err != nil {
		return nil, err
	}

	var accounts []providers.ServiceAccount
	err = svc.Projects.ServiceAccounts.List("projects/"+projectID).Pages(ctx, func(page *iam.ListServiceAccountsResponse) error {
		for _, sa := range page.Accounts {
			accounts = append(accounts, providers.ServiceAccount{
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

func vmDisk(d *computepb.AttachedDisk) *providers.VMDisk {
	return &providers.VMDisk{
		Name:       resourceShortName(d.GetSource(), d.GetDeviceName()),
		DeviceName: d.GetDeviceName(),
		Mode:       d.GetMode(),
		AutoDelete: d.GetAutoDelete(),
		SizeGB:     d.GetDiskSizeGb(),
	}
}

func ListProjectVMs(projectID string) ([]providers.VM, error) {
	ctx := context.Background()

	client, err := computev1.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	it := client.AggregatedList(ctx, &computepb.AggregatedListInstancesRequest{Project: projectID})
	return collectZoneVMs(projectID, it)
}

// zoneEntryIterator is the slice of the aggregated-list iterator the VM
// collection needs: entries of zone to instances. Keeping it an interface
// lets the collector run against a canned iterator in tests.
type zoneEntryIterator interface {
	Next() (computev1.InstancesScopedListPair, error)
}

// collectZoneVMs drains it, mapping each zone entry through zoneVMs. The
// iterator ends with iterator.Done; any other error stops the walk.
func collectZoneVMs(projectID string, it zoneEntryIterator) ([]providers.VM, error) {
	var vms []providers.VM
	for {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		vms = append(vms, zoneVMs(projectID, entry.Value.GetInstances())...)
	}
	return vms, nil
}

// zoneVMs maps the instances of one aggregated-list zone entry, skipping
// instances without a zone.
func zoneVMs(projectID string, instances []*computepb.Instance) []providers.VM {
	var vms []providers.VM
	for _, inst := range instances {
		if inst.GetZone() == "" {
			continue
		}
		vms = append(vms, instanceToVM(projectID, inst))
	}
	return vms
}

// instanceToVM maps one compute instance API object to the cache VM row.
// Metadata carries the instance template name under a fixed key.
func instanceToVM(projectID string, inst *computepb.Instance) providers.VM {
	vm := providers.VM{
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
		vm.ServiceAccounts = append(vm.ServiceAccounts, providers.VMServiceAccount{Email: sa.GetEmail(), Scopes: sa.GetScopes()})
	}
	for _, m := range inst.GetMetadata().GetItems() {
		if m.GetKey() == "instance-template" {
			vm.InstanceTemplate = resourceShortName(m.GetValue(), "")
			break
		}
	}
	for _, iface := range inst.GetNetworkInterfaces() {
		vmi := providers.VMInterface{
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
	return vm
}

func ListProjectDisks(projectID string) ([]providers.Disk, error) {
	ctx := context.Background()

	client, err := computev1.NewDisksRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var disks []providers.Disk
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
			disks = append(disks, providers.Disk{
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

func ListProjectSnapshots(projectID string) ([]providers.Snapshot, error) {
	ctx := context.Background()

	client, err := computev1.NewSnapshotsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var snapshots []providers.Snapshot
	it := client.List(ctx, &computepb.ListSnapshotsRequest{Project: projectID})
	for {
		s, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, providers.Snapshot{
			ProjectID:       projectID,
			Name:            s.GetName(),
			Description:     s.GetDescription(),
			Status:          s.GetStatus(),
			SourceDisk:      resourceShortName(s.GetSourceDisk(), ""),
			SizeGB:          s.GetDiskSizeGb(),
			StorageBytes:    s.GetStorageBytes(),
			CreatedAt:       s.GetCreationTimestamp(),
			StorageLocation: strings.Join(s.GetStorageLocations(), ","),
			Labels:          s.GetLabels(),
		})
	}
	return snapshots, nil
}

func ListProjectBuckets(projectID string) ([]providers.Bucket, error) {
	ctx := context.Background()

	svc, err := storage.NewService(ctx)
	if err != nil {
		return nil, err
	}

	var buckets []providers.Bucket
	err = svc.Buckets.List(projectID).Pages(ctx, func(page *storage.Buckets) error {
		for _, b := range page.Items {
			versioning := b.Versioning != nil && b.Versioning.Enabled
			uniform := b.IamConfiguration != nil &&
				b.IamConfiguration.UniformBucketLevelAccess != nil &&
				b.IamConfiguration.UniformBucketLevelAccess.Enabled
			buckets = append(buckets, providers.Bucket{
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

func forwardingRuleFrom(projectID, region string, r *computepb.ForwardingRule) providers.ForwardingRule {
	targetURL := r.GetTarget()
	if targetURL == "" {
		targetURL = r.GetBackendService()
	}
	kind, target := targetRef(targetURL)
	return providers.ForwardingRule{
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

func ListProjectForwardingRules(projectID string) ([]providers.ForwardingRule, error) {
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

	var rules []providers.ForwardingRule
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

func ListProjectTargetProxies(projectID string) ([]providers.TargetProxy, error) {
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

	var proxies []providers.TargetProxy
	httpIt := httpClient.List(ctx, &computepb.ListTargetHttpProxiesRequest{Project: projectID})
	for {
		p, err := httpIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		proxies = append(proxies, providers.TargetProxy{ProjectID: projectID, Kind: "targetHttpProxies", Name: p.GetName(), URLMap: resourceShortName(p.GetUrlMap(), "")})
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
		proxies = append(proxies, providers.TargetProxy{ProjectID: projectID, Kind: "targetHttpsProxies", Name: p.GetName(), URLMap: resourceShortName(p.GetUrlMap(), "")})
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
		proxies = append(proxies, providers.TargetProxy{ProjectID: projectID, Kind: "targetTcpProxies", Name: p.GetName(), BackendService: resourceShortName(p.GetService(), "")})
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
		proxies = append(proxies, providers.TargetProxy{ProjectID: projectID, Kind: "targetSslProxies", Name: p.GetName(), BackendService: resourceShortName(p.GetService(), "")})
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
		proxies = append(proxies, providers.TargetProxy{ProjectID: projectID, Kind: "targetGrpcProxies", Name: p.GetName(), URLMap: resourceShortName(p.GetUrlMap(), "")})
	}
	return proxies, nil
}

func ListProjectURLMaps(projectID string) ([]providers.URLMap, error) {
	ctx := context.Background()

	client, err := computev1.NewUrlMapsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var maps []providers.URLMap
	it := client.List(ctx, &computepb.ListUrlMapsRequest{Project: projectID})
	for {
		m, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		um := providers.URLMap{
			ProjectID:      projectID,
			Name:           m.GetName(),
			DefaultService: resourceShortName(m.GetDefaultService(), ""),
		}
		for _, pm := range m.GetPathMatchers() {
			for _, pr := range pm.GetPathRules() {
				um.PathRules = append(um.PathRules, providers.PathRule{Paths: pr.GetPaths(), Service: resourceShortName(pr.GetService(), "")})
			}
		}
		maps = append(maps, um)
	}
	return maps, nil
}

func backendServiceFrom(projectID, region string, b *computepb.BackendService) providers.BackendService {
	svc := providers.BackendService{
		ProjectID: projectID,
		Name:      b.GetName(),
		Region:    region,
		Protocol:  b.GetProtocol(),
		PortName:  b.GetPortName(),
	}
	for _, be := range b.GetBackends() {
		svc.Backends = append(svc.Backends, providers.Backend{Group: resourceShortName(be.GetGroup(), ""), BalancingMode: be.GetBalancingMode()})
	}
	for _, hc := range b.GetHealthChecks() {
		svc.HealthChecks = append(svc.HealthChecks, resourceShortName(hc, ""))
	}
	return svc
}

func ListProjectBackendServices(projectID string) ([]providers.BackendService, error) {
	ctx := context.Background()

	client, err := computev1.NewBackendServicesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var services []providers.BackendService
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

func ListProjectClusters(projectID string) ([]providers.Cluster, error) {
	ctx := context.Background()

	svc, err := container.NewService(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := svc.Projects.Locations.Clusters.List("projects/" + projectID + "/locations/-").Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	var clusters []providers.Cluster
	for _, c := range resp.Clusters {
		cluster := providers.Cluster{
			ProjectID:   projectID,
			Name:        c.Name,
			Description: c.Description,
			Location:    c.Location,
			Status:      c.Status,
			Version:     c.CurrentMasterVersion,
			Endpoint:    c.Endpoint,
			Labels:      c.ResourceLabels,
			CreatedAt:   c.CreateTime,
		}
		for _, np := range c.NodePools {
			cluster.NodePools = append(cluster.NodePools, providers.ClusterNodePool{Name: np.Name, Version: np.Version, NodeCount: np.InitialNodeCount})
		}
		clusters = append(clusters, cluster)
	}
	return clusters, nil
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

func ListClusterWorkloads(projectID, location, name string) ([]providers.Workload, error) {
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

	var workloads []providers.Workload
	for _, item := range list.Items {
		w := providers.Workload{
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

func ListClusterNodes(projectID, location, name string) ([]providers.ClusterNode, error) {
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

	return parseClusterNodeList(out, projectID, name)
}

// parseClusterNodeList decodes kubectl node output into cache rows. Split
// from ListClusterNodes so the decode path is testable without kubectl.
func parseClusterNodeList(out []byte, projectID, name string) ([]providers.ClusterNode, error) {
	var list k8sNodeList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, err
	}
	return parseClusterNodes(projectID, name, list.Items), nil
}

// parseClusterNodes maps kubectl node objects to cache rows. Nodes start
// NotReady until a Ready condition says otherwise.
func parseClusterNodes(projectID, name string, items []k8sNode) []providers.ClusterNode {
	nodes := make([]providers.ClusterNode, 0, len(items))
	for _, item := range items {
		node := providers.ClusterNode{
			ProjectID: projectID,
			Cluster:   name,
			Name:      item.Metadata.Name,
			NodePool:  item.Metadata.Labels["cloud.google.com/gke-nodepool"],
			Status:    "NotReady",
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
	return nodes
}

func ListProjectDNSZones(projectID string) ([]providers.DNSZone, error) {
	ctx := context.Background()

	svc, err := dns.NewService(ctx)
	if err != nil {
		return nil, err
	}

	var zones []providers.DNSZone
	err = svc.ManagedZones.List(projectID).Pages(ctx, func(page *dns.ManagedZonesListResponse) error {
		for _, z := range page.ManagedZones {
			zones = append(zones, providers.DNSZone{
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

func ListZoneDNSRecords(projectID, zone string) ([]providers.DNSRecord, error) {
	ctx := context.Background()

	svc, err := dns.NewService(ctx)
	if err != nil {
		return nil, err
	}

	var records []providers.DNSRecord
	err = svc.ResourceRecordSets.List(projectID, zone).Pages(ctx, func(page *dns.ResourceRecordSetsListResponse) error {
		for _, r := range page.Rrsets {
			records = append(records, providers.DNSRecord{
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

func ListProjectIPAddresses(projectID string) ([]providers.IPAddress, error) {
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

	var addresses []providers.IPAddress

	globalIt := globalClient.List(ctx, &computepb.ListGlobalAddressesRequest{Project: projectID})
	for {
		addr, err := globalIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, providers.IPAddress{
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
			addresses = append(addresses, providers.IPAddress{
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

func ListProjectCloudRouters(projectID string) ([]providers.CloudRouter, error) {
	ctx := context.Background()

	client, err := computev1.NewRoutersRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var routers []providers.CloudRouter
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
			peers := make([]providers.CloudRouterPeer, 0, len(r.GetBgpPeers()))
			for _, p := range r.GetBgpPeers() {
				peers = append(peers, providers.CloudRouterPeer{
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
			routers = append(routers, providers.CloudRouter{
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

func ListProjectServiceAttachments(projectID string) ([]providers.ServiceAttachment, error) {
	ctx := context.Background()

	client, err := computev1.NewServiceAttachmentsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var attachments []providers.ServiceAttachment
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
			conns := make([]providers.PSCConnection, 0, len(sa.GetConnectedEndpoints()))
			for _, e := range sa.GetConnectedEndpoints() {
				conns = append(conns, providers.PSCConnection{
					Endpoint: resourceShortName(e.GetEndpoint(), ""),
					Project:  ResourceProjectName(e.GetEndpoint()),
					Network:  resourceShortName(e.GetConsumerNetwork(), ""),
					Status:   e.GetStatus(),
				})
			}
			attachments = append(attachments, providers.ServiceAttachment{
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

func ListProjectVPNGateways(projectID string) ([]providers.VPNGateway, error) {
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

	tunnelsByGateway := map[string][]providers.VPNTunnel{}
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
			tunnelsByGateway[gateway] = append(tunnelsByGateway[gateway], providers.VPNTunnel{
				Name:      t.GetName(),
				Interface: t.GetVpnGatewayInterface(),
				PeerIP:    t.GetPeerIp(),
				IKE:       t.GetIkeVersion(),
				Status:    t.GetStatus(),
				Router:    resourceShortName(t.GetRouter(), ""),
			})
		}
	}

	var gateways []providers.VPNGateway
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
			gateways = append(gateways, providers.VPNGateway{
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
			gateways = append(gateways, providers.VPNGateway{
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

func ListProjectInstanceGroups(projectID string) ([]providers.InstanceGroup, error) {
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

	unmanaged := make(map[string]providers.InstanceGroup)
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
			unmanaged[location+"/"+g.GetName()] = providers.InstanceGroup{
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

	var groups []providers.InstanceGroup
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
			group := providers.InstanceGroup{
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

func instanceGroupMembers(it *computev1.InstanceWithNamedPortsIterator) ([]providers.InstanceGroupMember, error) {
	var members []providers.InstanceGroupMember
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
		members = append(members, providers.InstanceGroupMember{Zone: zone, Name: name})
	}
	return members, nil
}

func ListInstanceGroupVMs(projectID, location, name string) ([]providers.InstanceGroupMember, error) {
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

func ListProjectReservations(projectID string) ([]providers.Reservation, error) {
	ctx := context.Background()

	client, err := computev1.NewReservationsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var reservations []providers.Reservation
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
			reservations = append(reservations, providers.Reservation{
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

func imageSource(img *computepb.Image) string {
	for _, source := range []string{img.GetSourceDisk(), img.GetSourceImage(), img.GetSourceSnapshot()} {
		if source != "" {
			return resourceShortName(source, "")
		}
	}
	return ""
}

func ListProjectImages(projectID string) ([]providers.Image, error) {
	ctx := context.Background()

	client, err := computev1.NewImagesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var images []providers.Image
	it := client.List(ctx, &computepb.ListImagesRequest{Project: projectID})
	for {
		img, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		images = append(images, providers.Image{
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

func ListProjectInstanceTemplates(projectID string) ([]providers.InstanceTemplate, error) {
	ctx := context.Background()

	client, err := computev1.NewInstanceTemplatesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var templates []providers.InstanceTemplate
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
			template := providers.InstanceTemplate{
				ProjectID:   projectID,
				Name:        t.GetName(),
				Location:    location,
				MachineType: resourceShortName(t.GetProperties().GetMachineType(), ""),
				Labels:      t.GetProperties().GetLabels(),
				Description: t.GetDescription(),
				CreatedAt:   t.GetCreationTimestamp(),
			}
			for _, d := range t.GetProperties().GetDisks() {
				template.Disks = append(template.Disks, providers.InstanceTemplateDisk{
					DeviceName:  d.GetDeviceName(),
					SourceImage: resourceShortName(d.GetInitializeParams().GetSourceImage(), ""),
					DiskSizeGB:  d.GetInitializeParams().GetDiskSizeGb(),
					DiskType:    resourceShortName(d.GetInitializeParams().GetDiskType(), ""),
					AutoDelete:  d.GetAutoDelete(),
					Boot:        d.GetBoot(),
				})
			}
			for _, iface := range t.GetProperties().GetNetworkInterfaces() {
				ti := providers.InstanceTemplateInterface{
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

func ListProjectIAMPolicy(projectID string) ([]providers.IAMBinding, error) {
	ctx := context.Background()

	client, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	policy, err := client.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
		Resource: "projects/" + projectID,
		Options:  &iampb.GetPolicyOptions{RequestedPolicyVersion: 3},
	})
	if err != nil {
		return nil, err
	}

	var bindings []providers.IAMBinding
	for _, b := range policy.GetBindings() {
		condition := b.GetCondition().GetTitle()
		if condition == "" {
			condition = b.GetCondition().GetExpression()
		}
		for _, member := range b.GetMembers() {
			bindings = append(bindings, providers.IAMBinding{
				ProjectID: projectID,
				Member:    member,
				Role:      b.GetRole(),
				Condition: condition,
			})
		}
	}
	return bindings, nil
}

func ListProjectCustomRoles(projectID string) ([]providers.CustomRole, error) {
	ctx := context.Background()

	svc, err := iam.NewService(ctx)
	if err != nil {
		return nil, err
	}

	var roles []providers.CustomRole
	err = svc.Projects.Roles.List("projects/"+projectID).View("FULL").Pages(ctx, func(page *iam.ListRolesResponse) error {
		for _, r := range page.Roles {
			roles = append(roles, providers.CustomRole{
				ProjectID:   projectID,
				Name:        resourceShortName(r.Name, ""),
				Title:       r.Title,
				Description: r.Description,
				Stage:       r.Stage,
				Permissions: r.IncludedPermissions,
				Deleted:     r.Deleted,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return roles, nil
}

func ListBucketIAMPolicy(projectID, bucket string) ([]providers.BucketIAMBinding, error) {
	ctx := context.Background()

	svc, err := storage.NewService(ctx)
	if err != nil {
		return nil, err
	}

	policy, err := svc.Buckets.GetIamPolicy(bucket).OptionsRequestedPolicyVersion(3).Do()
	if err != nil {
		return nil, err
	}

	var bindings []providers.BucketIAMBinding
	for _, b := range policy.Bindings {
		condition := ""
		if b.Condition != nil {
			condition = b.Condition.Title
			if condition == "" {
				condition = b.Condition.Expression
			}
		}
		for _, member := range b.Members {
			bindings = append(bindings, providers.BucketIAMBinding{
				ProjectID: projectID,
				Bucket:    bucket,
				Member:    member,
				Role:      b.Role,
				Condition: condition,
			})
		}
	}
	return bindings, nil
}
