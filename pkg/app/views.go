package app

// viewDef describes a single view of the information panel: how its rows are
// loaded from the cache, how it is refreshed from Google, and which actions
// are available on a focused row. Every view is declared in its own file and
// registered in the views map below.
type viewDef struct {
	title      string
	headers    []string
	load       func(a *app) ([][]string, error)
	refresh    func(a *app)
	fetch      func(a *app, projectID string) (int, string, error)
	selectRow  func(a *app, row, col int)
	resource   func(a *app, row []string) (any, error)
	details    func(res any) string
	samples    []any
	consoleURL func(a *app, row int) string
}

// views is populated in init rather than as a var initializer because the
// view definitions reference app methods (setView) that read this map, which
// would otherwise form an initialization cycle.
var views map[viewKind]viewDef

func init() {
	views = map[viewKind]viewDef{
		viewProjects:             projectsView,
		viewIPs:                  ipsView,
		viewIPsAll:               ipsAllView,
		viewNetworks:             networksView,
		viewNetworksAll:          networksAllView,
		viewPeerings:             peeringsView,
		viewPeeringsAll:          peeringsAllView,
		viewRoutes:               routesView,
		viewRoutesAll:            routesAllView,
		viewFirewalls:            firewallsView,
		viewFirewallsAll:         firewallsAllView,
		viewAccounts:             accountsView,
		viewAccountsAll:          accountsAllView,
		viewVMs:                  vmsView,
		viewVMsAll:               vmsAllView,
		viewBuckets:              bucketsView,
		viewBucketsAll:           bucketsAllView,
		viewLBs:                  lbsView,
		viewLBsAll:               lbsAllView,
		viewClusters:             clustersView,
		viewClustersAll:          clustersAllView,
		viewWorkloads:            workloadsView,
		viewNodePools:            nodePoolsView,
		viewDNSZones:             dnsZonesView,
		viewDNSZonesAll:          dnsZonesAllView,
		viewDNSRecords:           dnsRecordsView,
		viewCloudRouters:         cloudRoutersView,
		viewCloudRoutersAll:      cloudRoutersAllView,
		viewVPNs:                 vpnsView,
		viewVPNsAll:              vpnsAllView,
		viewPSC:                  pscView,
		viewPSCAll:               pscAllView,
		viewInstanceGroups:       instanceGroupsView,
		viewInstanceGroupsAll:    instanceGroupsAllView,
		viewReservations:         reservationsView,
		viewReservationsAll:      reservationsAllView,
		viewImages:               imagesView,
		viewImagesAll:            imagesAllView,
		viewPublicImages:         publicImagesView,
		viewInstanceTemplates:    instanceTemplatesView,
		viewInstanceTemplatesAll: instanceTemplatesAllView,
		viewDisks:                disksView,
		viewDisksAll:             disksAllView,
		viewSnapshots:            snapshotsView,
		viewSnapshotsAll:         snapshotsAllView,
		viewIAM:                  iamView,
		viewIAMAll:               iamAllView,
		viewBucketIAM:            bucketIAMView,
		viewRoles:                rolesView,
		viewRolesAll:             rolesAllView,
	}
}

// projectViewKinds lists the per-project views the U shortcut refreshes, in fetch order.
var projectViewKinds = []viewKind{
	viewVMs,
	viewIPs,
	viewNetworks,
	viewPeerings,
	viewAccounts,
	viewBuckets,
	viewLBs,
	viewClusters,
	viewRoutes,
	viewFirewalls,
	viewDNSZones,
	viewCloudRouters,
	viewVPNs,
	viewPSC,
	viewInstanceGroups,
	viewReservations,
	viewImages,
	viewInstanceTemplates,
	viewDisks,
	viewSnapshots,
	viewIAM,
	viewRoles,
}

// projectRefresh builds the refresh action shared by the per-project views:
// it fetches and caches the view's resources in the background, then reloads
// the view and reports the status text returned by fetch.
func projectRefresh(kind viewKind, label string, fetch func(a *app, projectID string) (int, string, error)) func(a *app) {
	return func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusProgress("Fetching " + label + "...")
		a.goBackground(func() {
			projectID := appState.ProjectID
			_, msg, err := fetch(a, projectID)
			if err != nil {
				a.reportError(string(kind), projectID, "Failed to fetch "+label, err)
				return
			}
			a.markViewUpdated(kind, projectID)
			a.queueUpdateDraw(func() {
				a.setView(kind)
				a.statusText("[green]" + msg + "[-]")
			})
		})
	}
}

func viewKindNames() []string {
	names := make([]string, 0, len(views))
	for kind := range views {
		names = append(names, string(kind))
	}
	return names
}
