package app

// viewDef describes a single view of the information panel: how its rows are
// loaded from the cache, how it is refreshed from Google, and which actions
// are available on a focused row. Every view is declared in its own file and
// registered in the views map below.
type viewDef struct {
	title      string
	headers    []string
	load       func(a *app) ([][]string, error)
	status     func(a *app, count int) string
	refresh    func(a *app)
	fetch      func(a *app, projectID string) (string, error)
	selectRow  func(a *app, row, col int)
	resource   func(a *app, row []string) (any, error)
	samples    []any
	consoleURL func(a *app, row int) string
}

// views is populated in init rather than as a var initializer because the
// view definitions reference app methods (setView) that read this map, which
// would otherwise form an initialization cycle.
var views map[viewKind]viewDef

func init() {
	views = map[viewKind]viewDef{
		viewProjects:          projectsView,
		viewIPs:               ipsView,
		viewIPsAll:            ipsAllView,
		viewNetworks:          networksView,
		viewNetworksAll:       networksAllView,
		viewRoutes:            routesView,
		viewRoutesAll:         routesAllView,
		viewFirewalls:         firewallsView,
		viewFirewallsAll:      firewallsAllView,
		viewAccounts:          accountsView,
		viewAccountsAll:       accountsAllView,
		viewVMs:               vmsView,
		viewVMsAll:            vmsAllView,
		viewBuckets:           bucketsView,
		viewBucketsAll:        bucketsAllView,
		viewLBs:               lbsView,
		viewLBsAll:            lbsAllView,
		viewClusters:          clustersView,
		viewClustersAll:       clustersAllView,
		viewWorkloads:         workloadsView,
		viewNodePools:         nodePoolsView,
		viewDNSZones:          dnsZonesView,
		viewDNSZonesAll:       dnsZonesAllView,
		viewDNSRecords:        dnsRecordsView,
		viewCloudRouters:      cloudRoutersView,
		viewCloudRoutersAll:   cloudRoutersAllView,
		viewVPNs:              vpnsView,
		viewVPNsAll:           vpnsAllView,
		viewInstanceGroups:    instanceGroupsView,
		viewInstanceGroupsAll: instanceGroupsAllView,
	}
}

// projectViewKinds lists the per-project views the U shortcut refreshes, in fetch order.
var projectViewKinds = []viewKind{
	viewVMs,
	viewIPs,
	viewNetworks,
	viewAccounts,
	viewBuckets,
	viewLBs,
	viewClusters,
	viewRoutes,
	viewFirewalls,
	viewDNSZones,
	viewCloudRouters,
	viewVPNs,
	viewInstanceGroups,
}

// projectRefresh builds the refresh action shared by the per-project views:
// it fetches and caches the view's resources in the background, then reloads
// the view and reports the status text returned by fetch.
func projectRefresh(kind viewKind, label string, fetch func(a *app, projectID string) (string, error)) func(a *app) {
	return func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusInfo("Fetching " + label + "...")
		go func() {
			projectID := appState.ProjectID
			msg, err := fetch(a, projectID)
			if err != nil {
				a.reportError(string(kind), projectID, "Failed to fetch "+label, err)
				return
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(kind)
				a.statusText("[green]" + msg + "[-]")
			})
		}()
	}
}
