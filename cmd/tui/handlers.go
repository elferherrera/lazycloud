package main

import (
	"fmt"
	"net"
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

func inDNSZonesView() bool {
	return appState.ViewKind == viewDNSZones || appState.ViewKind == viewDNSZonesAll
}

func inNestedView() bool {
	return appState.ViewKind == viewWorkloads || appState.ViewKind == viewDNSRecords
}

func (a *app) backToParentView() {
	switch appState.ViewKind {
	case viewWorkloads:
		a.setView(clustersViewKind())
	case viewDNSRecords:
		a.setView(dnsZonesViewKind())
	}
}

func inLogsView() bool {
	return inVMView() || appState.ViewKind == viewWorkloads
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
		}},
		{key: '/', desc: "filter list", run: (*app).startFilter},
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
			key:     'd',
			desc:    "show dns records for zone",
			visible: inDNSZonesView,
			enabled: func(a *app) bool { return inDNSZonesView() },
			run:     (*app).showZoneDNSRecords,
		},
		{
			key:     'b',
			desc:    "back to previous view",
			visible: inNestedView,
			enabled: func(a *app) bool { return inNestedView() },
			run:     (*app).backToParentView,
		},
		{
			key:     'c',
			desc:    "fetch cluster credentials",
			visible: inClustersView,
			enabled: func(a *app) bool { return inClustersView() },
			run:     (*app).getClusterCredentials,
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
		a.filterBase = a.visibleRows
		a.cmd.SetText("")
	} else {
		if appState.CmdMode == modeFilter {
			a.exitFilterMode()
		}
		appState.CmdMode = modeFilter
		a.filterBase = a.allRows
		a.cmd.SetBorder(true)
		a.cmd.SetLabel(" / ")
		a.cmd.SetText("")
	}
	a.enterFilterMode()
	a.tapp.SetFocus(a.cmd)
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

func (a *app) copyRow() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	jsonBytes, err := views[appState.ViewKind].copyRow(a, row)
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
	if name, _ := a.pages.GetFrontPage(); name == "vm" || name == "ip" || name == "lb" || name == "dns" || name == "cluster" {
		a.pages.RemovePage(name)
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
	case text == "projects":
		a.setView(viewProjects)
	default:
		a.cmd.SetText("")
	}
	a.tapp.SetFocus(a.info)
}
