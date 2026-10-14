package main

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
	selectRow  func(a *app, row, col int)
	copyRow    func(a *app, row int) ([]byte, error)
	consoleURL func(a *app, row int) string
}

// views is populated in init rather than as a var initializer because the
// view definitions reference app methods (setView) that read this map, which
// would otherwise form an initialization cycle.
var views map[viewKind]viewDef

func init() {
	views = map[viewKind]viewDef{
		viewProjects:     projectsView,
		viewIPs:          ipsView,
		viewIPsAll:       ipsAllView,
		viewNetworks:     networksView,
		viewNetworksAll:  networksAllView,
		viewRoutes:       routesView,
		viewRoutesAll:    routesAllView,
		viewFirewalls:    firewallsView,
		viewFirewallsAll: firewallsAllView,
		viewAccounts:     accountsView,
		viewAccountsAll:  accountsAllView,
		viewVMs:          vmsView,
		viewVMsAll:       vmsAllView,
		viewBuckets:      bucketsView,
		viewBucketsAll:   bucketsAllView,
		viewLBs:          lbsView,
		viewLBsAll:       lbsAllView,
		viewClusters:     clustersView,
		viewClustersAll:  clustersAllView,
		viewWorkloads:    workloadsView,
	}
}
