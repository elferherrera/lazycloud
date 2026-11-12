package app

import (
	"sort"
	"strings"

	"github.com/elferherrera/lazycloud/pkg/providers"
	"github.com/rivo/tview"
)

var infobarEnabled = true

// infobarSortOrder mirrors the config's infobar sort_order between config
// loads: "current" keeps the built-in order, "alphabetical" sorts by key.
var infobarSortOrder = providers.InfobarSortCurrent

var infobarGlobalOrder = []string{"u", "U", "y", "v", "g", "?", "q", ":", "/", "a", "space", "b", "F1"}

func newInfobarPanel() *tview.TextView {
	return tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false).
		SetTextColor(theme.shortcutTextColor())
}

func infobarGlobalRank(label string) int {
	for i, l := range infobarGlobalOrder {
		if l == label {
			return i
		}
	}
	return len(infobarGlobalOrder)
}

// infobarBudgetFor returns the character budget for the infobar line. The
// screen width is used when known so the bar fills the terminal instead of a
// hardcoded width; one column is left for the cursor-free right edge.
func infobarBudgetFor(screenWidth int) int {
	budget := screenWidth - 2
	if budget < 20 {
		budget = 20
	}
	return budget
}

// infobarShortcuts picks the shortcuts shown in the infobar: panel-specific
// keys first, then globals in a fixed order, deduped by label and fitted to
// the budget. Dropped entries are reported so the caller can hint at them.
// An alphabetical order flattens specific and global entries into one
// label-sorted list instead.
func infobarShortcuts(budget int, order string) (fitted []shortcut, dropped int) {
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
	entries := make([]shortcut, 0, len(specific)+len(global))
	if order == providers.InfobarSortAlpha {
		entries = append(entries, specific...)
		entries = append(entries, global...)
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i].label() < entries[j].label()
		})
	} else {
		sort.SliceStable(global, func(i, j int) bool {
			return infobarGlobalRank(global[i].label()) < infobarGlobalRank(global[j].label())
		})
		entries = append(entries, specific...)
		entries = append(entries, global...)
	}
	seen := make(map[string]bool, len(entries))
	unique := make([]shortcut, 0, len(entries))
	for _, s := range entries {
		if seen[s.label()] {
			continue
		}
		seen[s.label()] = true
		unique = append(unique, s)
	}
	fitted = make([]shortcut, 0, len(unique))
	width := 0
	for _, s := range unique {
		w := len(s.label()) + 1 + len(infobarDesc(s)) + 1
		if width+w > budget {
			dropped++
			continue
		}
		width += w
		fitted = append(fitted, s)
	}
	return fitted, dropped
}

func infobarDesc(s shortcut) string {
	if short, ok := infobarShortDesc[s.desc]; ok {
		return short
	}
	return s.desc
}

var infobarShortDesc = map[string]string{
	"search side panel (Tab toggle, Esc close)": "search",
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

// infobarLine renders the fitted shortcuts as one tview tag line. Keys are
// colored with the shortcut label color, descriptions in the text color, and
// the pairs are separated by two spaces so entries read as pairs.
func infobarLine(entries []shortcut, dropped int) string {
	if len(entries) == 0 {
		return ""
	}
	parts := make([]string, 0, len(entries)+1)
	for i, s := range entries {
		if i > 0 {
			parts = append(parts, "  ")
		}
		parts = append(parts, "["+theme.shortcutLabels+"]"+s.label()+"[-]", "["+theme.shortcutText+"]"+infobarDesc(s))
	}
	if dropped > 0 {
		parts = append(parts, "  ["+theme.shortcutLabels+"]+"+itoa(dropped)+" more in ?[-]")
	}
	return strings.Join(parts, " ")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func (a *app) infobarWidth() int {
	if a.screen == nil {
		return 0
	}
	w, _ := a.screen.Size()
	return w
}

func populateInfobarPanel(view *tview.TextView, screenWidth int) {
	entries, dropped := infobarShortcuts(infobarBudgetFor(screenWidth), infobarSortOrder)
	view.SetText(infobarLine(entries, dropped))
}

func (a *app) refreshInfobar() {
	if !infobarEnabled {
		return
	}
	a.infobarRenderedWidth = a.infobarWidth()
	populateInfobarPanel(a.infobar, a.infobarRenderedWidth)
}

func (a *app) setInfobarEnabled(enabled bool) {
	if enabled == infobarEnabled {
		return
	}
	infobarEnabled = enabled
	if a.rootView != nil {
		a.rootView.ResizeItem(a.infobar, infobarHeight(enabled), infobarProportion(enabled))
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
