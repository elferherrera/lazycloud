package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"lazycloud/cmd/providers"
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
		if s.key >= '1' && s.key <= '9' {
			numeric = append(numeric, s)
		}
	}
	half := (len(numeric) + 1) / 2
	table := tview.NewTable().SetBorders(false).SetSelectable(false, false)
	for i, s := range numeric {
		row, col := i, 0
		if i >= half {
			row, col = i-half, 2
		}
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
	query := strings.ToLower(a.cmd.GetText())
	filtered := make([][]string, 0, len(a.filterBase))
	for _, r := range a.filterBase {
		for _, val := range r {
			if strings.Contains(strings.ToLower(val), query) {
				filtered = append(filtered, r)
				break
			}
		}
	}
	a.visibleRows = filtered
	populateInfoPanel(a.info, a.viewHeaders, filtered)
	a.updateInfoTitle()
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
	a.statusText("[yellow]filter mode[-]")
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
