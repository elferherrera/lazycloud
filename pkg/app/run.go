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

// newTApp builds the tview application. A package var so tests can hand
// run() an application already wired to a simulation screen.
var newTApp = tview.NewApplication

func Run() error {
	if err := checkNestedExecution(); err != nil {
		return err
	}
	// The terminal background decides which palette system mode paints
	// with, so it must be read once here: after the tview screen starts,
	// tcell owns stdin and consumes terminal replies, so a live OSC 11
	// query can only time out and fall back to COLORFGBG (which dark
	// terminals often report as light). One cached detection at boot, before
	// tcell exists, stays correct for the whole session.
	cachedBackground = detectBackground()

	tview.Borders.HorizontalFocus = tview.Borders.Horizontal
	tview.Borders.VerticalFocus = tview.Borders.Vertical
	tview.Borders.TopLeftFocus = tview.Borders.TopLeft
	tview.Borders.TopRightFocus = tview.Borders.TopRight
	tview.Borders.BottomLeftFocus = tview.Borders.BottomLeft
	tview.Borders.BottomRightFocus = tview.Borders.BottomRight

	appState.CmdMode = modeCmd
	appState.ViewKind = viewProjects

	a, cleanup, err := bootApp()
	if err != nil {
		return err
	}
	defer cleanup()

	return a.runUI()
}

// bootApp builds the app: cache, logger, config, widgets and layout. The
// returned cleanup closes the cache and log; the caller runs it after the UI
// loop ends. Split from Run so tests can boot the same app and drive the
// loop against a simulation screen.
func bootApp() (*app, func(), error) {
	cache, err := providers.NewCache()
	if err != nil {
		return nil, nil, fmt.Errorf("opening cache: %w", err)
	}
	if err := cache.SeedViewUpdates(viewKindNames()); err != nil {
		cache.Close()
		return nil, nil, fmt.Errorf("seeding view updates: %w", err)
	}

	logger, err := providers.NewLogger()
	if err != nil {
		cache.Close()
		return nil, nil, fmt.Errorf("opening log file: %w", err)
	}

	cleanup := func() {
		cache.Close()
		logger.Close()
	}

	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig())
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}
	applyShortcutConfig(config.Shortcuts)
	setConfiguredThemes(config)
	if cachedBackground == "" {
		// Tests boot the app directly, skipping Run's detection. Query
		// here rather than inside applyConfigTheme so a bad query cost is
		// paid once per boot, not per section application.
		cachedBackground = detectBackground()
	}
	applyAutoUpdateConfig(config.AutoUpdate)
	infobarEnabled = config.Infobar.ShowShortcuts
	infobarSortOrder = config.Infobar.SortOrder

	lastProject, err := providers.LoadLastProject()
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("loading last project: %w", err)
	}
	if lastProject.ProjectID != "" {
		appState.ProjectID = lastProject.ProjectID
		appState.Name = lastProject.Name
		appState.ProjectNumber = lastProject.ProjectNumber
	}

	tapp := newTApp()

	a := &app{
		tapp:             tapp,
		cmd:              tview.NewInputField(),
		info:             newInfoPanel(),
		summary:          newSummaryPanel(),
		status:           tview.NewTextView().SetDynamicColors(true).SetTextColor(theme.statusTextColor()),
		pages:            tview.NewPages(),
		infobar:          newInfobarPanel(),
		cache:            cache,
		logger:           logger,
		autoFetched:      map[string]bool{},
		staleAutoRefresh: config.AutoUpdate.Enabled,
	}

	a.info.SetBorder(true)

	helpTextView := newHelpTextView()
	helpOverlay, helpRow := newHelpOverlay(helpTextView)
	a.helpText = helpTextView
	a.helpRow = helpRow
	a.helpColumn = helpOverlay

	a.shortcutsSummary = newShortcutsSummaryPanel()

	a.buildMainLayout(helpOverlay)

	a.cmd.SetDoneFunc(a.done)
	a.cmd.SetChangedFunc(a.changed)
	a.cmd.SetFieldStyle(tcell.StyleDefault.Foreground(tview.Styles.PrimaryTextColor))
	a.cmd.SetFocusFunc(func() {
		a.cmdFocused = true
		a.mainView.ResizeItem(a.cmd, 3, 1)
	})
	a.cmd.SetBlurFunc(func() {
		a.cmdFocused = false
		a.mainView.ResizeItem(a.cmd, 0, 0)
	})

	a.info.SetSelectedFunc(a.selected)

	renderSummary(a.summary)
	a.setView(viewProjects)

	tapp.SetRoot(a.pages, true)
	tapp.SetInputCapture(a.inputCapture)
	tapp.SetFocus(a.info)

	return a, cleanup, nil
}

// runUI marks the app live and enters the event loop. The before-draw hook
// records the screen so panel sizing can read terminal dimensions, and
// refills the infobar whenever the width it was fitted for changes — the
// startup render runs before the loop, with the screen still unknown, and
// terminal resizes change the fitting budget.
func (a *app) runUI() error {
	a.tapp.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		a.uiMu.Lock()
		a.screen = screen
		a.uiMu.Unlock()
		if w, _ := screen.Size(); w != a.infobarRenderedWidth {
			a.refreshInfobar()
		}
		return false
	})
	// The first draw is the earliest point where the real terminal size is
	// known; the infobar renders with a placeholder width before that, so
	// refit it once the actual width exists.
	a.tapp.SetAfterDrawFunc(func(screen tcell.Screen) {
		a.refreshInfobar()
		a.tapp.SetAfterDrawFunc(nil)
	})

	a.uiRunning.Store(true)
	err := a.tapp.Run()
	// Back to inline mode once the loop is gone, so a late queued callback
	// cannot block forever on a dead event loop.
	a.uiRunning.Store(false)
	return err
}
