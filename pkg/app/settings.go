package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

// settingsRow describes one togglable settings entry rendered by the panel.
type settingsRow struct {
	label string
	get   func() bool
	set   func(on bool)
}

// settingsRows returns the toggleable entries in panel order: appearance
// first (theme mode, shortcut bar), then behavior (auto update).
func (a *app) settingsRows() []settingsRow {
	return []settingsRow{
		{label: "Light theme", get: func() bool { return configuredThemeMode() == providers.ModeLight },
			set: func(on bool) {
				mode := providers.ModeDark
				if on {
					mode = providers.ModeLight
				}
				setConfiguredThemeMode(mode)
			}},
		{label: "Dark theme", get: func() bool { return configuredThemeMode() == providers.ModeDark },
			set: func(on bool) {
				mode := providers.ModeSystem
				if on {
					mode = providers.ModeDark
				}
				setConfiguredThemeMode(mode)
			}},
		{label: "Theme follows terminal", get: func() bool { return configuredThemeMode() == providers.ModeSystem },
			set: func(on bool) {
				if on {
					setConfiguredThemeMode(providers.ModeSystem)
				}
			}},
		{label: "Shortcut bar", get: func() bool { return infobarEnabled },
			set: func(on bool) { infobarEnabled = on }},
		{label: "Auto update cache", get: func() bool { return a.staleAutoRefresh },
			set: func(on bool) { a.staleAutoRefresh = on }},
	}
}

// configuredTheme holds the theme section of the loaded config so the settings
// panel can read and write the mode without touching the config file on disk
// until the user toggles.
var configuredTheme providers.Theme

func configuredThemeMode() providers.ThemeMode {
	return providers.ParseThemeMode(configuredTheme.Mode)
}

func setConfiguredThemeMode(mode providers.ThemeMode) {
	configuredTheme.Mode = string(mode)
}

// buildSettingsPage creates the settings overlay: a bordered form with one
// checkbox per settings row, the config file path, and buttons. The page is
// added to the Pages stack once and rebuilt on every open.
func (a *app) buildSettingsPage() {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" Settings ")
	a.settingsForm = form
	a.rebuildSettingsForm()

	row := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(form, len(a.settingsRows())*2+6, 1, true).
		AddItem(nil, 0, 1, false)
	column := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(row, 56, 1, true).
		AddItem(nil, 0, 1, false)
	a.settingsRow = row
	a.settingsColumn = column
	a.pages.AddPage("settings", column, true, false)
}

// rebuildSettingsForm refills the form from the live values so the checkboxes
// match the current config every time the panel opens or a toggle saves.
func (a *app) rebuildSettingsForm() {
	if a.settingsForm == nil {
		return
	}
	form := a.settingsForm
	form.Clear(true)
	form.SetLabelColor(theme.shortcutLabelColor())
	form.SetFieldTextColor(theme.shortcutTextColor())
	form.SetFieldBackgroundColor(tcell.ColorDefault)
	form.SetButtonBackgroundColor(tcell.ColorDefault)
	form.SetButtonTextColor(theme.shortcutTextColor())
	for _, row := range a.settingsRows() {
		row := row
		form.AddCheckbox(row.label, row.get(), func(checked bool) {
			row.set(checked)
			a.applySettings()
		})
	}
	path, _ := providers.ConfigFilePath()
	form.AddTextView("Config file", path, 38, 1, false, false)
	form.AddButton("Open in editor", func() { a.openConfigFile() })
	form.AddButton("Close", func() { a.closeSettings() })
}

// applySettings persists the toggled values and applies them live, then
// resyncs the checkboxes with what was written.
func (a *app) applySettings() {
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig())
	if err != nil {
		a.statusText("[red]Failed to read config: [-]" + err.Error())
		return
	}
	config.Theme.Mode = configuredTheme.Mode
	config.Infobar.ShowShortcuts = infobarEnabled
	config.AutoUpdate.Enabled = a.staleAutoRefresh
	if err := providers.SaveConfig(config); err != nil {
		a.statusText("[red]Failed to save config: [-]" + err.Error())
		return
	}
	applyConfigTheme(config.Theme)
	a.applyThemeToWidgets()
	a.setInfobarEnabled(infobarEnabled)
	a.rebuildSettingsForm()
	a.statusText("[green]Settings saved[-]")
}

func (a *app) openSettings() {
	if a.settingsForm == nil {
		a.buildSettingsPage()
	}
	a.rebuildSettingsForm()
	a.resizeSettingsOverlay()
	a.pages.ShowPage("settings")
	a.tapp.SetFocus(a.settingsForm)
	a.statusInfo("space toggles, tab moves, enter runs a button, esc closes")
}

func (a *app) closeSettings() {
	a.pages.HidePage("settings")
	a.tapp.SetFocus(a.info)
}

func (a *app) resizeSettingsOverlay() {
	if a.settingsColumn == nil || a.screen == nil {
		return
	}
	w, _ := a.screen.Size()
	if w <= 0 {
		return
	}
	if width := overlayWidth(w); width > 56 {
		a.settingsColumn.ResizeItem(a.settingsRow, width, 1)
	}
}
