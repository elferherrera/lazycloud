package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

type app struct {
	tapp      *tview.Application
	cmd       *tview.InputField
	info      *tview.Table
	summary   *tview.TextView
	status    *tview.TextView
	pages     *tview.Pages
	cache     *providers.Cache
	logger    *providers.Logger
	shortcuts *tview.Table
	helpRow   *tview.Flex
	screen    tcell.Screen

	viewHeaders []string
	allRows     [][]string
	filterBase  [][]string
	visibleRows [][]string
}

func newSummaryPanel() *tview.TextView {
	return tview.NewTextView().
		SetDynamicColors(true).
		SetText("[yellow]No selection[-]")
}

func newShortcutsSummaryPanel() *tview.Table {
	var numeric []shortcut
	for _, s := range shortcuts {
		if s.key >= '0' && s.key <= '9' {
			numeric = append(numeric, s)
		}
	}
	rows := (len(numeric) + 2) / 3
	table := tview.NewTable().SetBorders(false).SetSelectable(false, false)
	for i, s := range numeric {
		row, col := i%rows, (i/rows)*2
		table.SetCell(row, col, tview.NewTableCell(string(s.key)).
			SetTextColor(tview.Styles.SecondaryTextColor).
			SetAttributes(tcell.AttrBold))
		table.SetCell(row, col+1, tview.NewTableCell(strings.TrimPrefix(s.desc, "show ")).
			SetExpansion(1))
	}
	return table
}

func newInfoPanel() *tview.Table {
	return tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)
}

func populateInfoPanel(table *tview.Table, headers []string, rows [][]string) {
	table.Clear()
	for col, header := range headers {
		table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tview.Styles.SecondaryTextColor).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}
	for row, r := range rows {
		for col, val := range r {
			table.SetCell(row+1, col, tview.NewTableCell(val))
		}
	}
	table.Select(1, 0).SetFixed(1, 0)
}

func (a *app) statusText(text string) {
	a.status.SetText(text)
}

func (a *app) reportError(view, projectID, message string, err error) {
	a.logger.Error(view, projectID, message+": "+err.Error())
	a.tapp.QueueUpdateDraw(func() {
		a.summary.SetText("[red]" + message + ": [-]" + err.Error())
		a.statusText("[red]" + message + "[-]")
	})
}

func (a *app) updateInfoTitle() {
	a.info.SetTitle(fmt.Sprintf(" %s (%d) ", views[appState.ViewKind].title, len(a.visibleRows)))
}

func (a *app) applyFilter() {
	include, excludes := splitFilterQuery(strings.ToLower(a.cmd.GetText()))
	filtered := make([][]string, 0, len(a.filterBase))
	for _, r := range a.filterBase {
		if rowMatchesFilter(r, include, excludes) {
			filtered = append(filtered, r)
		}
	}
	a.visibleRows = filtered
	populateInfoPanel(a.info, a.viewHeaders, filtered)
	a.updateInfoTitle()
}

func splitFilterQuery(query string) (string, []string) {
	if !strings.Contains(query, "- ") {
		return query, nil
	}
	parts := strings.Split(query, "- ")
	include := strings.TrimSpace(parts[0])
	excludes := make([]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		if term := strings.TrimSpace(part); term != "" {
			excludes = append(excludes, term)
		}
	}
	return include, excludes
}

func rowMatchesFilter(row []string, include string, excludes []string) bool {
	matched := false
	for _, val := range row {
		if strings.Contains(strings.ToLower(val), include) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	for _, exclude := range excludes {
		for _, val := range row {
			if strings.Contains(strings.ToLower(val), exclude) {
				return false
			}
		}
	}
	return true
}

func (a *app) refreshShortcuts() {
	count := populateShortcutsPanel(a.shortcuts)
	a.helpRow.ResizeItem(a.shortcuts, count+2, 1)
}

func (a *app) setView(kind viewKind) {
	def := views[kind]
	rows, err := def.load(a)
	if err != nil {
		a.statusText("[red]Failed to load view: [-]" + err.Error())
		a.cmd.SetText("")
		return
	}
	if kind != appState.ViewKind {
		appState.PrevViewKind = appState.ViewKind
	}
	appState.ViewKind = kind
	a.refreshShortcuts()
	a.viewHeaders = def.headers
	a.allRows = rows
	a.filterBase = rows
	a.cmd.SetText("")
	a.applyFilter()
	if def.status != nil {
		a.statusText(def.status(a, len(rows)))
	}
}

func (a *app) enterFilterMode() {
	a.statusText("[yellow]filter mode ('- x' excludes x)[-]")
}

func (a *app) exitFilterMode() {
	appState.CmdMode = modeCmd
	a.filterBase = a.allRows
	a.statusText("")
}

func (a *app) changed(text string) {
	if appState.CmdMode == modeFilter {
		a.applyFilter()
	}
}

func overlayHeight(lines, screenHeight int) int {
	height := lines + 2
	if screenHeight > 0 && height > screenHeight-2 {
		height = screenHeight - 2
	}
	if height < 3 {
		height = 3
	}
	return height
}

func overlayWidth(screenWidth int) int {
	width := 80
	if screenWidth > 0 && width > screenWidth-2 {
		width = screenWidth - 2
	}
	if width < 10 {
		width = 10
	}
	return width
}

func (a *app) newOverlay(title, content string) (*tview.Flex, *tview.TextView) {
	text := tview.NewTextView().SetDynamicColors(true).SetText(content)
	text.SetBorder(true).SetTitle(title)
	lines := strings.Count(text.GetText(true), "\n")
	screenWidth, screenHeight := 0, 0
	if a.screen != nil {
		screenWidth, screenHeight = a.screen.Size()
	}
	height := overlayHeight(lines, screenHeight)
	width := overlayWidth(screenWidth)
	overlay := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(text, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
	return overlay, text
}

func (a *app) selected(row, col int) {
	if def := views[appState.ViewKind]; def.selectRow != nil {
		def.selectRow(a, row, col)
	}
}

func copyToClipboard(text string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			c = exec.Command("wl-copy")
		} else {
			c = exec.Command("xclip", "-selection", "clipboard")
		}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	c.Stdin = strings.NewReader(text)
	return c.Run()
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
