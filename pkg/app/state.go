package app

import "github.com/rivo/tview"

type viewKind string

const (
	viewProjects     viewKind = "projects"
	viewIPs          viewKind = "ips"
	viewIPsAll       viewKind = "ips-all"
	viewNetworks     viewKind = "networks"
	viewNetworksAll  viewKind = "networks-all"
	viewRoutes       viewKind = "routes"
	viewRoutesAll    viewKind = "routes-all"
	viewFirewalls    viewKind = "firewalls"
	viewFirewallsAll viewKind = "firewalls-all"
	viewAccounts     viewKind = "accounts"
	viewAccountsAll  viewKind = "accounts-all"
	viewVMs          viewKind = "vms"
	viewVMsAll       viewKind = "vms-all"
	viewBuckets      viewKind = "buckets"
	viewBucketsAll   viewKind = "buckets-all"
	viewLBs          viewKind = "lbs"
	viewLBsAll       viewKind = "lbs-all"
	viewClusters     viewKind = "clusters"
	viewClustersAll  viewKind = "clusters-all"
	viewWorkloads    viewKind = "workloads"
	viewNodePools    viewKind = "nodepools"
	viewDNSZones     viewKind = "dns-zones"
	viewDNSZonesAll  viewKind = "dns-zones-all"
	viewDNSRecords   viewKind = "dns-records"
)

type cmdMode string

const (
	modeCmd    cmdMode = "cmd"
	modeFilter cmdMode = "filter"
)

type State struct {
	Name           string
	ProjectID      string
	ProjectNumber  string
	CmdMode        cmdMode
	ViewKind       viewKind
	PrevViewKind   viewKind
	AllProjects    bool
	Cluster        string
	ClusterProject string
	Zone           string
	ZoneProject    string
}

var appState State

func renderSummary(summary *tview.TextView) {
	if appState.AllProjects {
		summary.SetText("[yellow]Projects: [-]all")
		return
	}
	summary.SetText(
		"[yellow]Name: [-]" + appState.Name +
			"\n[yellow]Project ID: [-]" + appState.ProjectID +
			"\n[yellow]Project Number: [-]" + appState.ProjectNumber)
}

func ipsViewKind() viewKind {
	if appState.AllProjects {
		return viewIPsAll
	}
	return viewIPs
}

func networksViewKind() viewKind {
	if appState.AllProjects {
		return viewNetworksAll
	}
	return viewNetworks
}

func routesViewKind() viewKind {
	if appState.AllProjects {
		return viewRoutesAll
	}
	return viewRoutes
}

func firewallsViewKind() viewKind {
	if appState.AllProjects {
		return viewFirewallsAll
	}
	return viewFirewalls
}

func accountsViewKind() viewKind {
	if appState.AllProjects {
		return viewAccountsAll
	}
	return viewAccounts
}

func vmsViewKind() viewKind {
	if appState.AllProjects {
		return viewVMsAll
	}
	return viewVMs
}

func bucketsViewKind() viewKind {
	if appState.AllProjects {
		return viewBucketsAll
	}
	return viewBuckets
}

func lbsViewKind() viewKind {
	if appState.AllProjects {
		return viewLBsAll
	}
	return viewLBs
}

func clustersViewKind() viewKind {
	if appState.AllProjects {
		return viewClustersAll
	}
	return viewClusters
}

func dnsZonesViewKind() viewKind {
	if appState.AllProjects {
		return viewDNSZonesAll
	}
	return viewDNSZones
}
