package app

import (
	"errors"
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func TestFetchProjectVMsHappyPath(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()

	listProjectVMs = func(string) ([]providers.VM, error) {
		return []providers.VM{{Name: "vm-1", Zone: "z1"}}, nil
	}
	listProjectDisks = func(string) ([]providers.Disk, error) {
		return []providers.Disk{{Name: "disk-1"}}, nil
	}

	count, msg, err := fetchProjectVMs(a, "p1")
	if err != nil || count != 1 {
		t.Fatalf("fetchProjectVMs() = %d, %q, %v", count, msg, err)
	}
	if msg != "Fetched 1 VMs" {
		t.Fatalf("msg = %q", msg)
	}
}

func TestFetchProjectVMsError(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()

	listProjectVMs = func(string) ([]providers.VM, error) { return nil, errors.New("boom") }
	if _, _, err := fetchProjectVMs(a, "p1"); err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestFetchProjectVMsDisksFailSoft(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()

	listProjectVMs = func(string) ([]providers.VM, error) {
		return []providers.VM{{Name: "vm-1"}}, nil
	}
	listProjectDisks = func(string) ([]providers.Disk, error) { return nil, errors.New("disk boom") }
	count, _, err := fetchProjectVMs(a, "p1")
	if err != nil || count != 1 {
		t.Fatalf("fetchProjectVMs() with disk failure = %d, %v; disks are soft", count, err)
	}
}

func TestFetchProjectVMsCacheClosed(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()

	listProjectVMs = func(string) ([]providers.VM, error) { return []providers.VM{}, nil }
	a.cache.Close()
	if _, _, err := fetchProjectVMs(a, "p1"); err == nil {
		t.Fatal("expected cache save error to propagate")
	}
}

func newFetchTestApp(t *testing.T) *app {
	t.Helper()
	a := newFilterTestApp(t)
	logger, err := providers.NewLogger()
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	a.logger = logger
	t.Cleanup(func() { logger.Close() })
	return a
}

func restoreSeams(t *testing.T) func() {
	t.Helper()
	saved := struct {
		projects          func() ([]providers.Project, error)
		vms               func(string) ([]providers.VM, error)
		disks             func(string) ([]providers.Disk, error)
		buckets           func(string) ([]providers.Bucket, error)
		clusters          func(string) ([]providers.Cluster, error)
		workloads         func(string, string, string) ([]providers.Workload, error)
		nodes             func(string, string, string) ([]providers.ClusterNode, error)
		networks          func(string) ([]providers.Network, error)
		subnetworks       func(string) ([]providers.Subnetwork, error)
		peerings          func(string) ([]providers.NetworkPeering, error)
		routes            func(string) ([]providers.Route, error)
		firewalls         func(string) ([]providers.FirewallRule, error)
		ips               func(string) ([]providers.IPAddress, error)
		forwardingRules   func(string) ([]providers.ForwardingRule, error)
		targetProxies     func(string) ([]providers.TargetProxy, error)
		urlMaps           func(string) ([]providers.URLMap, error)
		backendServices   func(string) ([]providers.BackendService, error)
		snapshots         func(string) ([]providers.Snapshot, error)
		images            func(string) ([]providers.Image, error)
		instanceGroups    func(string) ([]providers.InstanceGroup, error)
		instanceGroupVMs  func(string, string, string) ([]providers.InstanceGroupMember, error)
		dnsZones          func(string) ([]providers.DNSZone, error)
		dnsRecords        func(string, string) ([]providers.DNSRecord, error)
		cloudRouters      func(string) ([]providers.CloudRouter, error)
		serviceAttachment func(string) ([]providers.ServiceAttachment, error)
		vpnGateways       func(string) ([]providers.VPNGateway, error)
		reservations      func(string) ([]providers.Reservation, error)
		instanceTemplates func(string) ([]providers.InstanceTemplate, error)
		serviceAccounts   func(string) ([]providers.ServiceAccount, error)
		iamPolicy         func(string) ([]providers.IAMBinding, error)
		customRoles       func(string) ([]providers.CustomRole, error)
		bucketIAM         func(string, string) ([]providers.BucketIAMBinding, error)
	}{
		listGcloudProjects, listProjectVMs, listProjectDisks, listProjectBuckets,
		listProjectClusters, listClusterWorkloads, listClusterNodes,
		listProjectNetworks, listProjectSubnetworks, listProjectPeerings,
		listProjectRoutes, listProjectFirewallRules, listProjectIPAddresses,
		listProjectForwardingRules, listProjectTargetProxies, listProjectURLMaps,
		listProjectBackendServices, listProjectSnapshots, listProjectImages,
		listProjectInstanceGroups, listInstanceGroupVMs, listProjectDNSZones,
		listZoneDNSRecords, listProjectCloudRouters, listProjectServiceAttach,
		listProjectVPNGateways, listProjectReservations, listProjectInstanceTpl,
		listProjectServiceAccounts, listProjectIAMPolicy, listProjectCustomRoles, listBucketIAMPolicy,
	}
	return func() {
		listGcloudProjects = saved.projects
		listProjectVMs = saved.vms
		listProjectDisks = saved.disks
		listProjectBuckets = saved.buckets
		listProjectClusters = saved.clusters
		listClusterWorkloads = saved.workloads
		listClusterNodes = saved.nodes
		listProjectNetworks = saved.networks
		listProjectSubnetworks = saved.subnetworks
		listProjectPeerings = saved.peerings
		listProjectRoutes = saved.routes
		listProjectFirewallRules = saved.firewalls
		listProjectIPAddresses = saved.ips
		listProjectForwardingRules = saved.forwardingRules
		listProjectTargetProxies = saved.targetProxies
		listProjectURLMaps = saved.urlMaps
		listProjectBackendServices = saved.backendServices
		listProjectSnapshots = saved.snapshots
		listProjectImages = saved.images
		listProjectInstanceGroups = saved.instanceGroups
		listInstanceGroupVMs = saved.instanceGroupVMs
		listProjectDNSZones = saved.dnsZones
		listZoneDNSRecords = saved.dnsRecords
		listProjectCloudRouters = saved.cloudRouters
		listProjectServiceAttach = saved.serviceAttachment
		listProjectVPNGateways = saved.vpnGateways
		listProjectReservations = saved.reservations
		listProjectInstanceTpl = saved.instanceTemplates
		listProjectServiceAccounts = saved.serviceAccounts
		listProjectIAMPolicy = saved.iamPolicy
		listProjectCustomRoles = saved.customRoles
		listBucketIAMPolicy = saved.bucketIAM
	}
}

// fetchCase drives one fetcher twice: once with a failing seam, once with a
// single-row seam, asserting the error propagates and the count comes back.
func fetchCase(t *testing.T, name string, fetch func(a *app, projectID string) (int, string, error)) {
	t.Helper()
	t.Run(name+"/error", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		failingSeams(t)
		if _, _, err := fetch(a, "p1"); err == nil {
			t.Fatalf("%s: expected seam error to propagate", name)
		}
	})
	t.Run(name+"/happy", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		emptySeams(t)
		count, msg, err := fetch(a, "p1")
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
		if count != 0 {
			t.Fatalf("%s: count = %d, want 0 from empty stubs", name, count)
		}
		if msg == "" {
			t.Fatalf("%s: empty success message", name)
		}
	})
}

// failingSeams points every seam at an error.
func failingSeams(t *testing.T) {
	t.Helper()
	boom := errors.New("boom")
	listGcloudProjects = func() ([]providers.Project, error) { return nil, boom }
	listProjectVMs = func(string) ([]providers.VM, error) { return nil, boom }
	listProjectDisks = func(string) ([]providers.Disk, error) { return nil, boom }
	listProjectBuckets = func(string) ([]providers.Bucket, error) { return nil, boom }
	listProjectClusters = func(string) ([]providers.Cluster, error) { return nil, boom }
	listClusterWorkloads = func(string, string, string) ([]providers.Workload, error) { return nil, boom }
	listClusterNodes = func(string, string, string) ([]providers.ClusterNode, error) { return nil, boom }
	listProjectNetworks = func(string) ([]providers.Network, error) { return nil, boom }
	listProjectSubnetworks = func(string) ([]providers.Subnetwork, error) { return nil, boom }
	listProjectPeerings = func(string) ([]providers.NetworkPeering, error) { return nil, boom }
	listProjectRoutes = func(string) ([]providers.Route, error) { return nil, boom }
	listProjectFirewallRules = func(string) ([]providers.FirewallRule, error) { return nil, boom }
	listProjectIPAddresses = func(string) ([]providers.IPAddress, error) { return nil, boom }
	listProjectForwardingRules = func(string) ([]providers.ForwardingRule, error) { return nil, boom }
	listProjectTargetProxies = func(string) ([]providers.TargetProxy, error) { return nil, boom }
	listProjectURLMaps = func(string) ([]providers.URLMap, error) { return nil, boom }
	listProjectBackendServices = func(string) ([]providers.BackendService, error) { return nil, boom }
	listProjectSnapshots = func(string) ([]providers.Snapshot, error) { return nil, boom }
	listProjectImages = func(string) ([]providers.Image, error) { return nil, boom }
	listProjectInstanceGroups = func(string) ([]providers.InstanceGroup, error) { return nil, boom }
	listInstanceGroupVMs = func(string, string, string) ([]providers.InstanceGroupMember, error) { return nil, boom }
	listProjectDNSZones = func(string) ([]providers.DNSZone, error) { return nil, boom }
	listZoneDNSRecords = func(string, string) ([]providers.DNSRecord, error) { return nil, boom }
	listProjectCloudRouters = func(string) ([]providers.CloudRouter, error) { return nil, boom }
	listProjectServiceAttach = func(string) ([]providers.ServiceAttachment, error) { return nil, boom }
	listProjectVPNGateways = func(string) ([]providers.VPNGateway, error) { return nil, boom }
	listProjectReservations = func(string) ([]providers.Reservation, error) { return nil, boom }
	listProjectInstanceTpl = func(string) ([]providers.InstanceTemplate, error) { return nil, boom }
	listProjectServiceAccounts = func(string) ([]providers.ServiceAccount, error) { return nil, boom }
	listProjectIAMPolicy = func(string) ([]providers.IAMBinding, error) { return nil, boom }
	listProjectCustomRoles = func(string) ([]providers.CustomRole, error) { return nil, boom }
	listBucketIAMPolicy = func(string, string) ([]providers.BucketIAMBinding, error) { return nil, boom }
}

// emptySeams points every seam at empty success, enough for the save paths.
func emptySeams(t *testing.T) {
	t.Helper()
	listGcloudProjects = func() ([]providers.Project, error) { return nil, nil }
	listProjectVMs = func(string) ([]providers.VM, error) { return nil, nil }
	listProjectDisks = func(string) ([]providers.Disk, error) { return nil, nil }
	listProjectBuckets = func(string) ([]providers.Bucket, error) { return nil, nil }
	listProjectClusters = func(string) ([]providers.Cluster, error) { return nil, nil }
	listClusterWorkloads = func(string, string, string) ([]providers.Workload, error) { return nil, nil }
	listClusterNodes = func(string, string, string) ([]providers.ClusterNode, error) { return nil, nil }
	listProjectNetworks = func(string) ([]providers.Network, error) { return nil, nil }
	listProjectSubnetworks = func(string) ([]providers.Subnetwork, error) { return nil, nil }
	listProjectPeerings = func(string) ([]providers.NetworkPeering, error) { return nil, nil }
	listProjectRoutes = func(string) ([]providers.Route, error) { return nil, nil }
	listProjectFirewallRules = func(string) ([]providers.FirewallRule, error) { return nil, nil }
	listProjectIPAddresses = func(string) ([]providers.IPAddress, error) { return nil, nil }
	listProjectForwardingRules = func(string) ([]providers.ForwardingRule, error) { return nil, nil }
	listProjectTargetProxies = func(string) ([]providers.TargetProxy, error) { return nil, nil }
	listProjectURLMaps = func(string) ([]providers.URLMap, error) { return nil, nil }
	listProjectBackendServices = func(string) ([]providers.BackendService, error) { return nil, nil }
	listProjectSnapshots = func(string) ([]providers.Snapshot, error) { return nil, nil }
	listProjectImages = func(string) ([]providers.Image, error) { return nil, nil }
	listProjectInstanceGroups = func(string) ([]providers.InstanceGroup, error) { return nil, nil }
	listInstanceGroupVMs = func(string, string, string) ([]providers.InstanceGroupMember, error) { return nil, nil }
	listProjectDNSZones = func(string) ([]providers.DNSZone, error) { return nil, nil }
	listZoneDNSRecords = func(string, string) ([]providers.DNSRecord, error) { return nil, nil }
	listProjectCloudRouters = func(string) ([]providers.CloudRouter, error) { return nil, nil }
	listProjectServiceAttach = func(string) ([]providers.ServiceAttachment, error) { return nil, nil }
	listProjectVPNGateways = func(string) ([]providers.VPNGateway, error) { return nil, nil }
	listProjectReservations = func(string) ([]providers.Reservation, error) { return nil, nil }
	listProjectInstanceTpl = func(string) ([]providers.InstanceTemplate, error) { return nil, nil }
	listProjectServiceAccounts = func(string) ([]providers.ServiceAccount, error) { return nil, nil }
	listProjectIAMPolicy = func(string) ([]providers.IAMBinding, error) { return nil, nil }
	listProjectCustomRoles = func(string) ([]providers.CustomRole, error) { return nil, nil }
	listBucketIAMPolicy = func(string, string) ([]providers.BucketIAMBinding, error) { return nil, nil }
}

func TestAllProjectFetchers(t *testing.T) {
	fetchCase(t, "buckets", fetchProjectBuckets)
	fetchCase(t, "accounts", fetchProjectAccounts)
	fetchCase(t, "cloudRouters", fetchProjectCloudRouters)
	fetchCase(t, "clusters", fetchProjectClusters)
	fetchCase(t, "disks", fetchProjectDisks)
	fetchCase(t, "dnsZones", fetchProjectDNSZones)
	fetchCase(t, "firewalls", fetchProjectFirewalls)
	fetchCase(t, "iam", fetchProjectIAM)
	fetchCase(t, "images", fetchProjectImages)
	fetchCase(t, "instanceGroups", fetchProjectInstanceGroups)
	fetchCase(t, "instanceTemplates", fetchProjectInstanceTemplates)
	fetchCase(t, "ips", fetchProjectIPs)
	fetchCase(t, "lbs", fetchProjectLBs)
	fetchCase(t, "networks", fetchProjectNetworks)
	fetchCase(t, "peerings", fetchProjectPeerings)
	fetchCase(t, "psc", fetchProjectPSC)
	fetchCase(t, "reservations", fetchProjectReservations)
	fetchCase(t, "routes", fetchProjectRoutes)
	fetchCase(t, "snapshots", fetchProjectSnapshots)
	fetchCase(t, "vms", fetchProjectVMs)
	fetchCase(t, "vpns", fetchProjectVPNs)
}
