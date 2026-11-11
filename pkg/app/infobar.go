package app

import (
	"reflect"
	"sort"
	"strings"

	"github.com/elferherrera/lazycloud/cmd/providers"
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
	"search all resources (Ctrl-P)":                 "search",
	"show projects":                                 "projects",
	"open log file in editor":                       "log file",
	"show dns records for zone":                     "dns records",
	"open config file in editor":                    "config",
	"open settings":                                 "settings",
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

type commandSuggestion struct {
	name string
	desc string
}

// infobarCommandDescs carries descriptions for commands the : help listing
// does not cover: the argument-taking commands and the view commands that
// never made it into the : entry's command rows.
var infobarCommandDescs = map[string]string{
	"sort":                    "sort rows by a column",
	"write":                   "write the view to a file",
	"search":                  "open the search panel",
	"peerings":                "show VPC peerings",
	"peering":                 "show VPC peerings",
	"psc":                     "show private service connect",
	"private-service-connect": "show private service connect",
}

// infobarCommands caches the suggestion list built once at startup: building
// it reads viewCommands, which refreshInfobar reaches through setView, so a
// lazy build would form an initialization cycle (the shortcuts table uses the
// same init trick).
var infobarCommands []commandSuggestion

func init() {
	infobarCommands = commandSuggestions()
}

// commandSuggestions returns every runnable :command sorted by name with the
// description shown in the help panel. Aliases inherit the description of the
// command they dispatch to, resolved by comparing the underlying func
// pointer, and anything still unmatched falls back to infobarCommandDescs.
func commandSuggestions() []commandSuggestion {
	names := commandNames()
	descs := make(map[string]string, len(names))
	for _, s := range shortcuts {
		for _, c := range s.commands {
			descs[strings.TrimPrefix(c.cmd, ":")] = c.desc
		}
	}
	pointers := make(map[uintptr]string, len(viewCommands))
	for name, run := range viewCommands {
		if descs[name] != "" {
			pointers[reflect.ValueOf(run).Pointer()] = descs[name]
		}
	}
	suggestions := make([]commandSuggestion, 0, len(names))
	for _, name := range names {
		desc := descs[name]
		if desc == "" {
			if run, ok := viewCommands[name]; ok {
				desc = pointers[reflect.ValueOf(run).Pointer()]
			}
		}
		if desc == "" {
			desc = infobarCommandDescs[name]
		}
		suggestions = append(suggestions, commandSuggestion{name: name, desc: desc})
	}
	return suggestions
}

// infobarCommandMatches filters the suggestions by a case-insensitive prefix
// of the typed text. Multi-token input (arguments already typed) matches
// nothing, since suggestions apply to the command name only.
func infobarCommandMatches(suggestions []commandSuggestion, text string) []commandSuggestion {
	text = strings.ToLower(strings.TrimSpace(text))
	if strings.Contains(text, " ") {
		return nil
	}
	matches := make([]commandSuggestion, 0, len(suggestions))
	for _, s := range suggestions {
		if strings.HasPrefix(strings.ToLower(s.name), text) {
			matches = append(matches, s)
		}
	}
	return matches
}

// infobarCommandLine renders the matched commands as :name desc pairs in the
// same style as the shortcuts line, fitted to the character budget with the
// overflow hinted as +N more.
func infobarCommandLine(matches []commandSuggestion, budget int) string {
	if len(matches) == 0 {
		return ""
	}
	parts := make([]string, 0, 2*len(matches)+1)
	width := 0
	dropped := 0
	for i, s := range matches {
		w := 1 + len(s.name) + 1 + len(s.desc) + 1
		if width+w > budget {
			dropped++
			continue
		}
		width += w
		if i > 0 {
			parts = append(parts, "  ")
		}
		parts = append(parts, "["+theme.shortcutLabels+"]:"+s.name+"[-]")
		if s.desc != "" {
			parts = append(parts, "["+theme.shortcutText+"]"+s.desc+"[-]")
		}
	}
	if dropped > 0 {
		parts = append(parts, "  ["+theme.shortcutLabels+"]+"+itoa(dropped)+" more[-]")
	}
	return strings.Join(parts, " ")
}

func populateInfobarCommands(view *tview.TextView, text string, screenWidth int) {
	view.SetText(infobarCommandLine(infobarCommandMatches(infobarCommands, text), infobarBudgetFor(screenWidth)))
}

// infobarShowsFilterPaths reports whether the infobar should list the view's
// detail-filter paths instead of the view shortcuts: filter mode, but only
// while the bottom input is the active filter editor or the search panel is
// closed. While global search is open, searchPreview/searchSelect drive the
// bottom input with the landed query, and the query's head token is not a
// path the user is typing, so a miss would blank the bar — show shortcuts.
func (a *app) infobarShowsFilterPaths() bool {
	if appState.CmdMode != modeFilter {
		return false
	}
	return a.cmdFocused || !a.searchOpen.Load()
}

// infobarFilterPathMatches filters the cached paths by a case-insensitive
// prefix of the head token — the text before the first space, the same token
// Tab completion rotates.
func infobarFilterPathMatches(paths []string, text string) []string {
	head, _, _ := strings.Cut(text, " ")
	head = strings.ToLower(strings.TrimSpace(head))
	matches := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.HasPrefix(strings.ToLower(p), head) {
			matches = append(matches, p)
		}
	}
	return matches
}

// infobarFilterPathsLine renders the matched paths as label-coloured tokens
// fitted to the character budget, overflow hinted as +N more.
func infobarFilterPathsLine(paths []string, budget int) string {
	if len(paths) == 0 {
		return ""
	}
	tokens := make([]string, 0, len(paths)+1)
	width := 0
	dropped := 0
	for i, p := range paths {
		w := len(p)
		if i > 0 {
			w += 2
		}
		if width+w > budget {
			dropped++
			continue
		}
		width += w
		tokens = append(tokens, "["+theme.shortcutLabels+"]"+p+"[-]")
	}
	if len(tokens) == 0 {
		return ""
	}
	if dropped > 0 {
		tokens = append(tokens, "["+theme.shortcutLabels+"]+"+itoa(dropped)+" more[-]")
	}
	return strings.Join(tokens, "  ")
}

func populateInfobarFilterPaths(view *tview.TextView, kind viewKind, text string, screenWidth int) {
	view.SetText(infobarFilterPathsLine(infobarFilterPathMatches(detailPaths(views[kind].samples), text), infobarBudgetFor(screenWidth)))
}

// infobarShowsCommands reports whether the infobar should list :command
// suggestions instead of the view shortcuts: command mode with the bottom
// input focused. The focus state is a plain flag kept by the input's
// Focus/Blur hooks, not tview's GetFocus, because the before-draw refresh
// runs while tview holds its own lock.
func (a *app) infobarShowsCommands() bool {
	return appState.CmdMode == modeCmd && a.cmdFocused
}

func populateInfobarPanel(view *tview.TextView, screenWidth int) {
	entries, dropped := infobarShortcuts(infobarBudgetFor(screenWidth), infobarSortOrder)
	view.SetText(infobarLine(entries, dropped))
}

func (a *app) refreshInfobar() {
	if !infobarEnabled || a.infobar == nil {
		return
	}
	a.infobarRenderedWidth = a.infobarWidth()
	if a.infobarShowsCommands() {
		populateInfobarCommands(a.infobar, a.cmd.GetText(), a.infobarRenderedWidth)
		return
	}
	if a.infobarShowsFilterPaths() {
		populateInfobarFilterPaths(a.infobar, appState.ViewKind, a.cmd.GetText(), a.infobarRenderedWidth)
		return
	}
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
