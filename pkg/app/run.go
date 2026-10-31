package app

import (
	"errors"
	"fmt"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func checkNestedExecution() error {
	if os.Getenv("LAZYCLOUD_CONTEXT") != "true" {
		return nil
	}
	return errors.New("Detected nested lazycloud execution. Exit this context to start lazycloud")
}

func Run() error {
	if err := checkNestedExecution(); err != nil {
		return err
	}
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
		return fmt.Errorf("opening cache: %w", err)
	}
	defer cache.Close()

	logger, err := providers.NewLogger()
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer logger.Close()

	shortcutConfig, err := providers.LoadShortcuts(defaultShortcutConfig())
	if err != nil {
		return fmt.Errorf("loading shortcuts config: %w", err)
	}
	applyShortcutConfig(shortcutConfig)

	themeConfig, err := providers.LoadTheme(defaultThemeConfig())
	if err != nil {
		return fmt.Errorf("loading theme config: %w", err)
	}
	applyTheme(themeConfig)

	lastProject, err := providers.LoadLastProject()
	if err != nil {
		return fmt.Errorf("loading last project: %w", err)
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
		status:  tview.NewTextView().SetDynamicColors(true).SetTextColor(theme.statusTextColor()),
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
	tapp.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		a.screen = screen
		return false
	})

	return tapp.Run()
}
