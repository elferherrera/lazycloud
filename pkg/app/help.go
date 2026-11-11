package app

import (
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// helpSections names the groups shown in the help overlay and their order.
// Panel-specific keys (drill-downs like workloads, project actions like
// restore/delete) come first so the top of the overlay answers "what can I
// do here", then the always-on keys, then the colon commands.
var helpSections = []string{"This view", "General", "Commands"}

// sectionFor sorts a visible shortcut into a help section by what it does,
// not by which key it binds: panel-specific entries depend on appState,
// everything else is always available.
func sectionFor(s shortcut) string {
	if s.visible != nil {
		return "This view"
	}
	if len(s.commands) > 0 {
		return "Commands"
	}
	return "General"
}

// sectionedShortcuts groups the visible shortcuts into the help sections,
// keeping the key-sorted order inside each section so the overlay stays
// predictable to scan.
func sectionedShortcuts() map[string][]shortcut {
	grouped := make(map[string][]shortcut, len(helpSections))
	for _, s := range visibleShortcuts() {
		grouped[sectionFor(s)] = append(grouped[sectionFor(s)], s)
	}
	return grouped
}

// helpKeyWidth is the padded column the key labels are aligned into before
// their descriptions, so every description starts at the same offset. Long
// colon commands (:public-images) fit indented without pushing their
// descriptions against the key.
const helpKeyWidth = 18

func helpKeyColumn(key string) string {
	if len(key) >= helpKeyWidth {
		return key
	}
	return key + strings.Repeat(" ", helpKeyWidth-len(key))
}

// helpText renders the help overlay content as colored lines grouped by
// section. A TextView (not a table) keeps the whole list scrollable, so a
// small terminal never clips the first sections away.
func helpText() string {
	var sb strings.Builder
	grouped := sectionedShortcuts()
	first := true
	for _, section := range helpSections {
		entries := grouped[section]
		if len(entries) == 0 {
			continue
		}
		if !first {
			sb.WriteString("\n")
		}
		first = false
		sb.WriteString("[" + theme.shortcutLabels + "]" + strings.ToUpper(section[:1]) + strings.ToUpper(section[1:]) + "[-]\n")
		for _, s := range entries {
			sb.WriteString("[" + theme.shortcutLabels + "]" + helpKeyColumn(s.label()) + "[-][" + theme.shortcutText + "]" + s.desc + "[-]\n")
			for _, c := range sortedCommands(s.commands) {
				desc := c.desc
				if c.view != "" {
					desc += " (" + string(c.view) + ")"
				}
				sb.WriteString("[" + theme.shortcutLabels + "]" + helpKeyColumn("  "+c.cmd) + "[-][" + theme.shortcutText + "]" + desc + "[-]\n")
			}
		}
	}
	return sb.String()
}

// helpLineCount is the content height the overlay needs.
func helpLineCount() int {
	return strings.Count(helpText(), "\n")
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
	sort.SliceStable(visible, func(i, j int) bool {
		return strings.ToLower(visible[i].label()) < strings.ToLower(visible[j].label())
	})
	return visible
}

func shortcutKeyLabel(key rune) string {
	if key == ' ' {
		return "space"
	}
	return string(key)
}

func (s shortcut) label() string {
	if s.specialKey != 0 {
		if s.specialKey == tcell.KeyCtrlUnderscore {
			return "Ctrl-/"
		}
		return tcell.KeyNames[s.specialKey]
	}
	return shortcutKeyLabel(s.key)
}

func (s shortcut) matches(event *tcell.EventKey) bool {
	if s.specialKey != 0 {
		return event.Key() == s.specialKey
	}
	return event.Rune() == s.key
}

func sortedCommands(commands []viewCommand) []viewCommand {
	sorted := append([]viewCommand(nil), commands...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].cmd < sorted[j].cmd
	})
	return sorted
}

// newHelpTextView builds the scrollable help body: a bordered TextView so
// long lists scroll instead of clipping, and dynamic colors for the key
// column. Screen-fitted sizing happens in resizeHelpOverlay on open.
func newHelpTextView() *tview.TextView {
	text := tview.NewTextView().SetDynamicColors(true).SetTextColor(theme.shortcutTextColor())
	text.SetBorder(true).SetTitle(" Help ")
	text.SetTitleColor(theme.titleColor())
	text.SetBorderColor(theme.borderColor())
	return text
}

// newHelpOverlay returns a centered floating window around the help text,
// stacked on top of the main view via Pages. The width starts at a
// comfortable default and is refit to the terminal on every open via
// resizeHelpOverlay.
func newHelpOverlay(text *tview.TextView) (overlay, row *tview.Flex) {
	row = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(text, overlayHeight(helpLineCount(), 0), 1, true).
		AddItem(nil, 0, 1, false)
	overlay = tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(row, overlayWidth(0), 1, true).
		AddItem(nil, 0, 1, false)
	return overlay, row
}

// resizeHelpOverlay fits the help window to the current terminal so long
// entries like ":templates" are not clipped on wide terminals and short
// terminals scroll the list instead of hiding its top.
func (a *app) resizeHelpOverlay() {
	if a.helpColumn == nil || a.screen == nil {
		return
	}
	w, h := a.screen.Size()
	if w <= 0 || h <= 0 {
		return
	}
	a.helpRow.ResizeItem(a.helpText, overlayHeight(helpLineCount(), h), 1)
	a.helpColumn.ResizeItem(a.helpRow, overlayWidth(w), 1)
}
