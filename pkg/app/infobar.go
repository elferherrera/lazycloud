package app

import (
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var infobarEnabled = true

var infobarBudget = 160

var infobarGlobalOrder = []string{"u", "U", "y", "v", "g", "?", "q", ":", "/", "a", "space", "b", "F1"}

func newInfobarPanel() *tview.Table {
	table := tview.NewTable().SetBorders(false).SetSelectable(false, false)
	populateInfobarPanel(table)
	return table
}

func infobarGlobalRank(label string) int {
	for i, l := range infobarGlobalOrder {
		if l == label {
			return i
		}
	}
	return len(infobarGlobalOrder)
}

func infobarShortcuts() []shortcut {
	var specific, global []shortcut
	for _, s := range shortcuts {
		if s.key >= '0' && s.key <= '9' {
			continue
		}
		if s.visible != nil && !s.visible() {
			continue
		}
		if s.visible == nil {
			global = append(global, s)
		} else {
			specific = append(specific, s)
		}
	}
	sort.SliceStable(global, func(i, j int) bool {
		return infobarGlobalRank(global[i].label()) < infobarGlobalRank(global[j].label())
	})
	entries := make([]shortcut, 0, len(specific)+len(global))
	entries = append(entries, specific...)
	entries = append(entries, global...)
	seen := make(map[string]bool, len(entries))
	unique := make([]shortcut, 0, len(entries))
	for _, s := range entries {
		if seen[s.label()] {
			continue
		}
		seen[s.label()] = true
		unique = append(unique, s)
	}
	fitted := make([]shortcut, 0, len(unique))
	width := 0
	for _, s := range unique {
		w := len(s.label()) + 1 + len(infobarDesc(s)) + 1
		if width+w > infobarBudget {
			continue
		}
		width += w
		fitted = append(fitted, s)
	}
	return fitted
}

func infobarDesc(s shortcut) string {
	if short, ok := infobarShortDesc[s.desc]; ok {
		return short
	}
	return s.desc
}

var infobarShortDesc = map[string]string{
	"focus command": "commands",
	"filter list ('- x' excludes, Tab details)":     "filter",
	"toggle all projects":                           "all projects",
	"show projects":                                 "projects",
	"open log file in editor":                       "log file",
	"show dns records for zone":                     "dns records",
	"open config file in editor":                    "config",
	"back to previous view":                         "back",
	"update cache":                                  "update",
	"update cache for all views":                    "update all",
	"copy row to clipboard":                         "copy",
	"show row details":                              "details",
	"open in GCP console":                           "console",
	"open logs in GCP console":                      "logs",
	"SSH to VM via IAP":                             "ssh",
	"show workloads for cluster":                    "workloads",
	"show node pools for cluster":                   "node pools",
	"show VMs for node pool":                        "member VMs",
	"show VMs for instance group":                   "member VMs",
	"show VMs for instance template":                "member VMs",
	"show iam policy for bucket":                    "bucket iam",
	"open cluster autoscaler logs in GCP console":   "autoscaler",
	"open workload events in GCP console":           "events",
	"open cluster create audit logs in GCP console": "audit logs",
	"fetch cluster credentials":                     "credentials",
	"show next hop chart for route":                 "chart",
	"open shell with kubectl context set":           "shell",
	"restore last deleted project":                  "restore",
	"delete project from cache":                     "delete",
	"toggle help":                                   "help",
	"quit":                                          "quit",
}

func populateInfobarPanel(table *tview.Table) {
	table.Clear()
	for i, s := range infobarShortcuts() {
		table.SetCell(0, i*2, tview.NewTableCell(s.label()).
			SetTextColor(theme.shortcutLabelColor()).
			SetAttributes(tcell.AttrBold))
		table.SetCell(0, i*2+1, tview.NewTableCell(infobarDesc(s)).
			SetTextColor(theme.shortcutTextColor()).
			SetExpansion(1))
	}
}

func (a *app) refreshInfobar() {
	if !infobarEnabled {
		return
	}
	populateInfobarPanel(a.infobar)
}

func (a *app) setInfobarEnabled(enabled bool) {
	if enabled == infobarEnabled {
		return
	}
	infobarEnabled = enabled
	if a.mainView != nil {
		a.mainView.ResizeItem(a.infobar, infobarHeight(enabled), infobarProportion(enabled))
	}
	if enabled {
		a.refreshInfobar()
	}
}

func infobarHeight(enabled bool) int {
	if enabled {
		return 1
	}
	return 0
}

func infobarProportion(enabled bool) int {
	if enabled {
		return 1
	}
	return 0
}
