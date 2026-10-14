package main

import (
	"fmt"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"lazycloud/cmd/providers"
)

func main() {
	tview.Borders.HorizontalFocus = tview.Borders.Horizontal
	tview.Borders.VerticalFocus = tview.Borders.Vertical
	tview.Borders.TopLeftFocus = tview.Borders.TopLeft
	tview.Borders.TopRightFocus = tview.Borders.TopRight
	tview.Borders.BottomLeftFocus = tview.Borders.BottomLeft
	tview.Borders.BottomRightFocus = tview.Borders.BottomRight

	appState.CmdMode = modeCmd
	appState.ViewKind = viewProjects

	cache, err := providers.NewCache()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error opening cache:", err)
		os.Exit(1)
	}
	defer cache.Close()

	logger, err := providers.NewLogger()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error opening log file:", err)
		os.Exit(1)
	}
	defer logger.Close()

	lastProject, err := providers.LoadLastProject()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading last project:", err)
		os.Exit(1)
	}
	if lastProject.ProjectID != "" {
		appState.ProjectID = lastProject.ProjectID
		appState.Name = lastProject.Name
		appState.ProjectNumber = lastProject.ProjectNumber
	}

	tapp := tview.NewApplication()

	a := &app{
		tapp:    tapp,
		cmd:     tview.NewInputField(),
		info:    newInfoPanel(),
		summary: newSummaryPanel(),
		status:  tview.NewTextView().SetDynamicColors(true),
		pages:   tview.NewPages(),
		cache:   cache,
		logger:  logger,
	}

	a.info.SetBorder(true)

	shortcutsTable := newShortcutsPanel()
	helpOverlay, helpRow := newHelpOverlay(shortcutsTable)
	a.shortcuts = shortcutsTable
	a.helpRow = helpRow

	shortcutsSummary := newShortcutsSummaryPanel()
	summary := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(a.summary, 0, 1, false).
		AddItem(shortcutsSummary, 0, 1, false)

	mainView := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(summary, 4, 1, false).
		AddItem(a.cmd, 0, 0, false).
		AddItem(a.info, 0, 1, false).
		AddItem(a.status, 1, 1, false)

	a.pages.AddPage("main", mainView, true, true)
	a.pages.AddPage("help", helpOverlay, true, false)

	a.cmd.SetDoneFunc(a.done)
	a.cmd.SetChangedFunc(a.changed)
	a.cmd.SetFieldStyle(tcell.StyleDefault.Foreground(tview.Styles.PrimaryTextColor))
	a.cmd.SetFocusFunc(func() {
		mainView.ResizeItem(a.cmd, 3, 1)
	})
	a.cmd.SetBlurFunc(func() {
		mainView.ResizeItem(a.cmd, 0, 0)
	})

	a.info.SetSelectedFunc(a.selected)

	renderSummary(a.summary)
	a.setView(viewProjects)
	a.statusText("help ?")

	tapp.SetRoot(a.pages, true)
	tapp.SetInputCapture(a.inputCapture)
	tapp.SetFocus(a.info)

	if err := tapp.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running application:", err)
		os.Exit(1)
	}
}
