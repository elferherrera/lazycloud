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
}

func inProjectsView() bool {
	return appState.ViewKind == viewProjects
}

func inVMView() bool {
	return appState.ViewKind == viewVMs || appState.ViewKind == viewVMsAll
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
			{":projects", "show projects"},
			{":ips", "show IP addresses"},
			{":networks", "show VPCs and subnets"},
			{":accounts", "show service accounts"},
			{":vms", "show VMs"},
		}},
		{key: '/', desc: "filter list", run: (*app).startFilter},
		{key: 'a', desc: "toggle all projects", run: (*app).toggleAllProjects},
		{key: '1', desc: "show projects", run: func(a *app) { a.setView(viewProjects) }},
		{key: '2', desc: "show VMs", run: func(a *app) { showVMs(a) }},
		{key: '3', desc: "show service accounts", run: func(a *app) { showAccounts(a) }},
		{key: '4', desc: "show IP addresses", run: (*app).showIPs},
		{key: '5', desc: "show VPCs and subnets", run: (*app).showNetworks},
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
			desc:    "open VM logs in GCP console",
			visible: inVMView,
			enabled: func(a *app) bool { return inVMView() },
			run:     (*app).openVMLogging,
		},
		{key: '?', desc: "toggle shortcuts", run: (*app).toggleHelp},
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
		if s.key >= '1' && s.key <= '9' {
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
			table.SetCell(row, 0, tview.NewTableCell(c.cmd).
				SetTextColor(tview.Styles.SecondaryTextColor))
			table.SetCell(row, 1, tview.NewTableCell(c.desc).SetExpansion(1))
			row++
		}
	}
	return row
}

// newHelpOverlay returns a centered floating window showing the shortcuts
// table, meant to be stacked on top of the main view via Pages.
func newHelpOverlay(help *tview.Table) (overlay, row *tview.Flex) {
	help.SetBorder(true).SetTitle(" Shortcuts ")
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
	case viewProjects:
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
	if name, _ := a.pages.GetFrontPage(); name == "vm" {
		a.pages.RemovePage("vm")
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
	def := views[appState.ViewKind]
	if def.selectRow != nil && a.info.GetRowCount() == 2 {
		row, _ := a.info.GetSelection()
		def.selectRow(a, row, 0)
		a.exitFilterMode()
		a.cmd.SetText("")
		a.applyFilter()
	}
	a.tapp.SetFocus(a.info)
}

func (a *app) commandEnter() {
	text := strings.TrimSpace(a.cmd.GetText())
	switch {
	case text == "ips" || net.ParseIP(text) != nil:
		a.showIPs()
	case text == "networks" || text == "vpcs":
		a.showNetworks()
	case text == "accounts" || text == "sa":
		showAccounts(a)
	case text == "vms":
		showVMs(a)
	case text == "projects":
		a.setView(viewProjects)
	default:
		a.cmd.SetText("")
	}
	a.tapp.SetFocus(a.info)
}
