package app

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type shortcut struct {
	key      rune
	desc     string
	visible  func() bool
	enabled  func(a *app) bool
	run      func(a *app)
	commands []viewCommand
}

type viewCommand struct {
	cmd  string
	desc string
	view viewKind
}

func inProjectsView() bool {
	return appState.ViewKind == viewProjects
}

func inVMView() bool {
	return appState.ViewKind == viewVMs || appState.ViewKind == viewVMsAll
}

func inClustersView() bool {
	return appState.ViewKind == viewClusters || appState.ViewKind == viewClustersAll
}

func inNodePoolsView() bool {
	return appState.ViewKind == viewNodePools
}

func inDNSZonesView() bool {
	return appState.ViewKind == viewDNSZones || appState.ViewKind == viewDNSZonesAll
}

func inInstanceGroupsView() bool {
	return appState.ViewKind == viewInstanceGroups || appState.ViewKind == viewInstanceGroupsAll
}

func inRoutesView() bool {
	return appState.ViewKind == viewRoutes || appState.ViewKind == viewRoutesAll
}

func inLogsView() bool {
	return inVMView() || inClustersView() || inNodePoolsView() || appState.ViewKind == viewWorkloads
}

// The shortcuts table is the single source of truth for key bindings: the
// global input capture dispatches on it and the help overlay renders it.
// Shortcuts with a visible predicate are rendered only when it holds.
// It is populated in init rather than as a var initializer because the
// definitions reference app methods (setView) that read this slice, which
// would otherwise form an initialization cycle.
var shortcuts []shortcut

func init() {
	shortcuts = []shortcut{
		{key: ':', desc: "focus command", run: (*app).focusCmd, commands: []viewCommand{
			{":projects", "show projects", viewProjects},
			{":ips", "show IP addresses", viewIPs},
			{":networks", "show VPCs and subnets", viewNetworks},
			{":accounts", "show service accounts", viewAccounts},
			{":vms", "show VMs", viewVMs},
			{":buckets", "show buckets", viewBuckets},
			{":lbs", "show load balancers", viewLBs},
			{":clusters", "show kubernetes clusters", viewClusters},
			{":routes", "show routes", viewRoutes},
			{":firewalls", "show firewall rules", viewFirewalls},
			{":dns", "show DNS zones", viewDNSZones},
			{":routers", "show cloud routers", viewCloudRouters},
			{":vpns", "show VPN gateways", viewVPNs},
			{":igs", "show instance groups", viewInstanceGroups},
		}},
		{key: '/', desc: "filter list ('- x' excludes, Tab details)", run: (*app).startFilter},
		{key: 'a', desc: "toggle all projects", run: (*app).toggleAllProjects},
		{key: '1', desc: "show projects", run: func(a *app) { a.setView(viewProjects) }},
		{key: '2', desc: "show VMs", run: func(a *app) { showVMs(a) }},
		{key: '3', desc: "show buckets", run: func(a *app) { showBuckets(a) }},
		{key: '4', desc: "show service accounts", run: func(a *app) { showAccounts(a) }},
		{key: '5', desc: "show IP addresses", run: (*app).showIPs},
		{key: '6', desc: "show VPCs and subnets", run: (*app).showNetworks},
		{key: '7', desc: "show load balancers", run: showLBs},
		{key: '8', desc: "show kubernetes clusters", run: showClusters},
		{key: '9', desc: "show routes", run: showRoutes},
		{key: '0', desc: "show firewall rules", run: showFirewalls},
		{
			key:     'w',
			desc:    "show workloads for cluster",
			visible: inClustersView,
			enabled: func(a *app) bool { return inClustersView() },
			run:     (*app).showClusterWorkloads,
		},
		{
			key:     'n',
			desc:    "show node pools for cluster",
			visible: inClustersView,
			enabled: func(a *app) bool { return inClustersView() },
			run:     (*app).showClusterNodePools,
		},
		{
			key:     'm',
			desc:    "show VMs for node pool",
			visible: inNodePoolsView,
			enabled: func(a *app) bool { return inNodePoolsView() },
			run:     (*app).showNodePoolVMs,
		},
		{
			key:     'm',
			desc:    "show VMs for instance group",
			visible: inInstanceGroupsView,
			enabled: func(a *app) bool { return inInstanceGroupsView() },
			run:     (*app).showInstanceGroupVMs,
		},
		{
			key:     'd',
			desc:    "show dns records for zone",
			visible: inDNSZonesView,
			enabled: func(a *app) bool { return inDNSZonesView() },
			run:     (*app).showZoneDNSRecords,
		},
		{
			key:     'b',
			desc:    "back to previous view",
			enabled: func(a *app) bool { return appState.PrevViewKind != "" },
			run:     func(a *app) { a.setView(appState.PrevViewKind) },
		},
		{
			key:     'e',
			desc:    "open workload events in GCP console",
			visible: func() bool { return appState.ViewKind == viewWorkloads },
			enabled: func(a *app) bool { return appState.ViewKind == viewWorkloads },
			run:     (*app).openWorkloadEvents,
		},
		{
			key:     'c',
			desc:    "fetch cluster credentials",
			visible: inClustersView,
			enabled: func(a *app) bool { return inClustersView() },
			run:     (*app).getClusterCredentials,
		},
		{
			key:     'c',
			desc:    "show next hop chart for route",
			visible: inRoutesView,
			enabled: func(a *app) bool { return inRoutesView() },
			run:     (*app).showRouteHopChart,
		},
		{
			key:     'c',
			desc:    "open shell with kubectl context set",
			visible: func() bool { return appState.ViewKind == viewWorkloads },
			enabled: func(a *app) bool { return appState.ViewKind == viewWorkloads },
			run:     (*app).openWorkloadShell,
		},
		{
			key:     'r',
			desc:    "restore last deleted project",
			visible: inProjectsView,
			enabled: func(a *app) bool { return inProjectsView() },
			run:     (*app).restoreProject,
		},
		{
			key:     'x',
			desc:    "delete project from cache",
			visible: inProjectsView,
			enabled: func(a *app) bool { return inProjectsView() },
			run:     (*app).deleteProject,
		},
		{key: 'u', desc: "update cache", run: (*app).refreshView},
		{key: 'y', desc: "copy row to clipboard", run: (*app).copyRow},
		{key: 'v', desc: "show row details", run: (*app).showRowDetails},
		{key: 'g', desc: "open in GCP console", run: (*app).openConsole},
		{
			key:     'l',
			desc:    "open logs in GCP console",
			visible: inLogsView,
			enabled: func(a *app) bool { return inLogsView() },
			run:     (*app).openLogs,
		},
		{
			key:     's',
			desc:    "SSH to VM via IAP",
			visible: inVMView,
			enabled: func(a *app) bool { return inVMView() },
			run:     (*app).sshVM,
		},
		{key: '?', desc: "toggle help", run: (*app).toggleHelp},
		{
			key:  'q',
			desc: "quit",
			enabled: func(a *app) bool {
				return a.tapp.GetFocus() == a.info
			},
			run: (*app).quit,
		},
	}
}

func visibleShortcuts() []shortcut {
	visible := make([]shortcut, 0, len(shortcuts))
	for _, s := range shortcuts {
		if s.key >= '0' && s.key <= '9' {
			continue
		}
		if s.visible == nil || s.visible() {
			visible = append(visible, s)
		}
	}
	return visible
}

func newShortcutsPanel() *tview.Table {
	table := tview.NewTable().SetBorders(false).SetSelectable(false, false)
	populateShortcutsPanel(table)
	return table
}

func shortcutsRowCount() int {
	count := 0
	for _, s := range visibleShortcuts() {
		count += 1 + len(s.commands)
	}
	return count
}

func populateShortcutsPanel(table *tview.Table) int {
	table.Clear()
	row := 0
	for _, s := range visibleShortcuts() {
		table.SetCell(row, 0, tview.NewTableCell(string(s.key)).
			SetTextColor(tview.Styles.SecondaryTextColor).
			SetAttributes(tcell.AttrBold))
		table.SetCell(row, 1, tview.NewTableCell(s.desc).SetExpansion(1))
		row++
		for _, c := range s.commands {
			desc := c.desc
			if c.view != "" {
				desc += " (" + string(c.view) + ")"
			}
			table.SetCell(row, 0, tview.NewTableCell(c.cmd).
				SetTextColor(tview.Styles.SecondaryTextColor))
			table.SetCell(row, 1, tview.NewTableCell(desc).SetExpansion(1))
			row++
		}
	}
	return row
}

// newHelpOverlay returns a centered floating window showing the shortcuts
// table, meant to be stacked on top of the main view via Pages.
func newHelpOverlay(help *tview.Table) (overlay, row *tview.Flex) {
	help.SetBorder(true).SetTitle(" Help ")
	row = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(help, shortcutsRowCount()+2, 1, true).
		AddItem(nil, 0, 1, false)
	overlay = tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(row, 60, 1, true).
		AddItem(nil, 0, 1, false)
	return overlay, row
}

func (a *app) inputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlC:
		a.quit()
		return nil
	case tcell.KeyEsc:
		a.escapePressed()
		return nil
	case tcell.KeyTab, tcell.KeyBacktab:
		if appState.CmdMode == modeFilter && (a.tapp.GetFocus() == a.cmd || a.tapp.GetFocus() == a.info) {
			a.completeFilterDetail(event.Key() == tcell.KeyTab)
			return nil
		}
	}
	if a.tapp.GetFocus() == a.cmd {
		return event
	}
	for _, s := range shortcuts {
		if event.Rune() != s.key {
			continue
		}
		enabled := s.enabled
		if enabled == nil {
			enabled = func(a *app) bool { return true }
		}
		if !enabled(a) {
			continue
		}
		s.run(a)
		return nil
	}
	return event
}

func (a *app) focusCmd() {
	if appState.CmdMode == modeFilter {
		a.exitFilterMode()
		a.applyFilter()
	}
	appState.CmdMode = modeCmd
	a.cmd.SetBorder(true)
	a.cmd.SetLabel(": ")
	a.tapp.SetFocus(a.cmd)
}

func (a *app) startFilter() {
	if appState.CmdMode == modeFilter && a.tapp.GetFocus() == a.info {
		a.setFilterBase(a.visibleRows)
		a.cmd.SetText("")
	} else {
		if appState.CmdMode == modeFilter {
			a.exitFilterMode()
		}
		appState.CmdMode = modeFilter
		a.setFilterBase(a.allRows)
		a.cmd.SetBorder(true)
		a.cmd.SetLabel(" / ")
		a.cmd.SetText("")
	}
	a.enterFilterMode()
	a.tapp.SetFocus(a.cmd)
}

func (a *app) completeFilterDetail(forward bool) {
	paths := detailPaths(views[appState.ViewKind].samples)
	if len(paths) == 0 {
		return
	}
	text := a.cmd.GetText()
	head, rest, _ := strings.Cut(text, " ")
	rest = strings.TrimSpace(rest)
	idx := -1
	for i, p := range paths {
		if strings.EqualFold(head, p) {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = detailPathPrefixIndex(paths, head, forward)
		if idx < 0 {
			if forward {
				a.cmd.SetText(paths[0] + " " + text)
			} else {
				a.cmd.SetText(paths[len(paths)-1] + " " + text)
			}
			return
		}
		a.cmd.SetText(paths[idx] + " " + rest)
		return
	}
	if (forward && idx == len(paths)-1) || (!forward && idx == 0) {
		a.cmd.SetText(rest)
		return
	}
	if forward {
		a.cmd.SetText(paths[idx+1] + " " + rest)
	} else {
		a.cmd.SetText(paths[idx-1] + " " + rest)
	}
}

func detailPathPrefixIndex(paths []string, head string, forward bool) int {
	if forward {
		for i, p := range paths {
			if detailPathHasPrefix(p, head) {
				return i
			}
		}
		return -1
	}
	for i := len(paths) - 1; i >= 0; i-- {
		if detailPathHasPrefix(paths[i], head) {
			return i
		}
	}
	return -1
}

func detailPathHasPrefix(p, head string) bool {
	return len(head) <= len(p) && strings.EqualFold(p[:len(head)], head)
}

func (a *app) toggleAllProjects() {
	appState.AllProjects = !appState.AllProjects
	renderSummary(a.summary)
	switch appState.ViewKind {
	case viewProjects, viewWorkloads, viewDNSRecords:
	case viewIPs, viewIPsAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(ipsViewKind())
		}
	case viewNetworks, viewNetworksAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(networksViewKind())
		}
	case viewRoutes, viewRoutesAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(routesViewKind())
		}
	case viewFirewalls, viewFirewallsAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(firewallsViewKind())
		}
	case viewAccounts, viewAccountsAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(accountsViewKind())
		}
	case viewVMs, viewVMsAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(vmsViewKind())
		}
	case viewBuckets, viewBucketsAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(bucketsViewKind())
		}
	case viewLBs, viewLBsAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(lbsViewKind())
		}
	case viewClusters, viewClustersAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(clustersViewKind())
		}
	case viewDNSZones, viewDNSZonesAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(dnsZonesViewKind())
		}
	case viewCloudRouters, viewCloudRoutersAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(cloudRoutersViewKind())
		}
	case viewVPNs, viewVPNsAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(vpnsViewKind())
		}
	case viewInstanceGroups, viewInstanceGroupsAll:
		if !appState.AllProjects && appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
		} else {
			a.setView(instanceGroupsViewKind())
		}
	}
}

func (a *app) restoreProject() {
	p, err := a.cache.RestoreProject()
	if err != nil {
		a.statusText("[red]Failed to restore project: [-]" + err.Error())
		return
	}
	if p == nil {
		a.statusText("[red]No deleted projects to restore[-]")
		return
	}
	a.setView(viewProjects)
	a.statusText(fmt.Sprintf("[green]Restored %s[-]", p.ProjectID))
}

func (a *app) openLogs() {
	if inClustersView() {
		a.openClusterNodeEvents()
		return
	}
	if inNodePoolsView() {
		a.openNodePoolEvents()
		return
	}
	if appState.ViewKind == viewWorkloads {
		a.openWorkloadLogs()
		return
	}
	a.openVMLogging()
}

func (a *app) showIPs() {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(ipsViewKind())
}

func (a *app) showNetworks() {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(networksViewKind())
}

func (a *app) deleteProject() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	p, err := a.cache.DeleteProject(a.info.GetCell(row, 0).Text)
	if err != nil {
		a.statusText("[red]Failed to delete project: [-]" + err.Error())
		return
	}
	if p == nil {
		a.statusText("[red]Project not found in cache[-]")
		return
	}
	if appState.ProjectID == p.ProjectID {
		appState.ProjectID = ""
		appState.Name = ""
		appState.ProjectNumber = ""
		renderSummary(a.summary)
	}
	a.setView(viewProjects)
	a.statusText(fmt.Sprintf("[green]Removed %s from cache[-]", p.ProjectID))
}

func (a *app) refreshView() {
	views[appState.ViewKind].refresh(a)
}

func (a *app) tableRowCells(row int) []string {
	cells := make([]string, a.info.GetColumnCount())
	for c := range cells {
		cells[c] = a.info.GetCell(row, c).Text
	}
	return cells
}

func (a *app) copyRow() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	def := views[appState.ViewKind]
	if def.resource == nil {
		return
	}
	res, err := def.resource(a, a.tableRowCells(row))
	if err != nil {
		a.statusText("[red]Failed to copy: [-]" + err.Error())
		return
	}
	jsonBytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		a.statusText("[red]Failed to copy: [-]" + err.Error())
		return
	}
	if err := copyToClipboard(string(jsonBytes)); err != nil {
		a.statusText("[red]Failed to copy: [-]" + err.Error())
		return
	}
	a.statusText("[green]Copied JSON to clipboard[-]")
}

func (a *app) showRowDetails() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	def := views[appState.ViewKind]
	if def.resource == nil {
		return
	}
	res, err := def.resource(a, a.tableRowCells(row))
	if err != nil {
		a.statusText("[red]Failed to load row details: [-]" + err.Error())
		return
	}
	jsonBytes, err := json.Marshal(res)
	if err != nil {
		a.statusText("[red]Failed to parse row details: [-]" + err.Error())
		return
	}
	var data any
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		a.statusText("[red]Failed to parse row details: [-]" + err.Error())
		return
	}
	var sb strings.Builder
	appendRowDetails(&sb, data, "")
	overlay, text := a.newOverlay(fmt.Sprintf(" %s details ", views[appState.ViewKind].title), sb.String())
	a.pages.RemovePage("row")
	a.pages.AddPage("row", overlay, true, true)
	a.tapp.SetFocus(text)
}

func appendRowDetails(sb *strings.Builder, data any, indent string) {
	switch v := data.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			switch v[k].(type) {
			case map[string]any, []any:
				fmt.Fprintf(sb, "%s[yellow]%s:[-]\n", indent, k)
				appendRowDetails(sb, v[k], indent+"  ")
			default:
				fmt.Fprintf(sb, "%s[yellow]%s: [-]%v\n", indent, k, v[k])
			}
		}
	case []any:
		for i, item := range v {
			switch item.(type) {
			case map[string]any, []any:
				fmt.Fprintf(sb, "%s[yellow]%d:[-]\n", indent, i)
				appendRowDetails(sb, item, indent+"  ")
			default:
				fmt.Fprintf(sb, "%s%v\n", indent, item)
			}
		}
	}
}

func (a *app) openConsole() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	url := views[appState.ViewKind].consoleURL(a, row)
	if err := openBrowser(url); err != nil {
		a.statusText("[red]Failed to open browser: [-]" + err.Error())
		return
	}
	a.statusText("[green]Opened GCP console in browser[-]")
}

func (a *app) toggleHelp() {
	if name, _ := a.pages.GetFrontPage(); name == "help" {
		a.pages.HidePage("help")
	} else {
		a.pages.ShowPage("help")
	}
}

func (a *app) quit() {
	a.tapp.Stop()
}

func (a *app) escapePressed() {
	if name, _ := a.pages.GetFrontPage(); name == "help" {
		a.toggleHelp()
		return
	}
	if name, _ := a.pages.GetFrontPage(); name == "vm" || name == "ip" || name == "lb" || name == "dns" || name == "cluster" || name == "row" || name == "route" {
		a.pages.RemovePage(name)
		a.tapp.SetFocus(a.info)
		return
	}
	a.cmd.SetBorder(false)
	if appState.CmdMode == modeFilter {
		a.exitFilterMode()
		a.cmd.SetText("")
		a.applyFilter()
	}
	a.tapp.SetFocus(a.info)
}

func (a *app) done(key tcell.Key) {
	switch key {
	case tcell.KeyEnter:
		if appState.CmdMode == modeFilter {
			a.filterEnter()
		} else {
			a.commandEnter()
		}
	case tcell.KeyEscape:
		a.tapp.SetFocus(a.info)
	default:
		a.tapp.SetFocus(a.cmd)
	}
}

func (a *app) filterEnter() {
	a.tapp.SetFocus(a.info)
}

func (a *app) commandEnter() {
	text := strings.TrimSpace(a.cmd.GetText())
	switch {
	case text == "ips" || net.ParseIP(text) != nil:
		a.showIPs()
	case text == "networks" || text == "vpcs":
		a.showNetworks()
	case text == "routes":
		showRoutes(a)
	case text == "firewalls":
		showFirewalls(a)
	case text == "accounts" || text == "sa":
		showAccounts(a)
	case text == "vms":
		showVMs(a)
	case text == "buckets":
		showBuckets(a)
	case text == "lbs" || text == "loadbalancers":
		showLBs(a)
	case text == "clusters" || text == "gke":
		showClusters(a)
	case text == "dns" || text == "zones":
		showDNSZones(a)
	case text == "routers" || text == "cloud-routers":
		showCloudRouters(a)
	case text == "vpns" || text == "vpn":
		showVPNs(a)
	case text == "igs" || text == "instance-groups":
		showInstanceGroups(a)
	case text == "projects":
		a.setView(viewProjects)
	default:
		a.cmd.SetText("")
	}
	a.tapp.SetFocus(a.info)
}
