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

// themeColorKey pairs a form label with the Theme field it edits. Order is
// the form order; changes here renumber nothing else.
type themeColorKey struct {
	label string
	get   func(providers.Theme) string
	set   func(*providers.Theme, string)
}

// themeColorKeys lists every color key of a theme section. The settings
// form renders one input per key for the active mode's section, so all
// colors are editable in the modal.
var themeColorKeys = []themeColorKey{
	{label: "Background", get: func(t providers.Theme) string { return t.Background },
		set: func(t *providers.Theme, v string) { t.Background = v }},
	{label: "Labels", get: func(t providers.Theme) string { return t.Labels },
		set: func(t *providers.Theme, v string) { t.Labels = v }},
	{label: "Text", get: func(t providers.Theme) string { return t.Text },
		set: func(t *providers.Theme, v string) { t.Text = v }},
	{label: "Borders", get: func(t providers.Theme) string { return t.Borders },
		set: func(t *providers.Theme, v string) { t.Borders = v }},
	{label: "Title", get: func(t providers.Theme) string { return t.Title },
		set: func(t *providers.Theme, v string) { t.Title = v }},
	{label: "Shortcut labels", get: func(t providers.Theme) string { return t.ShortcutLabels },
		set: func(t *providers.Theme, v string) { t.ShortcutLabels = v }},
	{label: "Shortcut text", get: func(t providers.Theme) string { return t.ShortcutText },
		set: func(t *providers.Theme, v string) { t.ShortcutText = v }},
	{label: "Focus", get: func(t providers.Theme) string { return t.Focus },
		set: func(t *providers.Theme, v string) { t.Focus = v }},
	{label: "Status labels", get: func(t providers.Theme) string { return t.StatusLabels },
		set: func(t *providers.Theme, v string) { t.StatusLabels = v }},
	{label: "Status text", get: func(t providers.Theme) string { return t.StatusText },
		set: func(t *providers.Theme, v string) { t.StatusText = v }},
	{label: "Summary labels", get: func(t providers.Theme) string { return t.SummaryLabels },
		set: func(t *providers.Theme, v string) { t.SummaryLabels = v }},
	{label: "Summary text", get: func(t providers.Theme) string { return t.SummaryText },
		set: func(t *providers.Theme, v string) { t.SummaryText = v }},
	{label: "Detail labels", get: func(t providers.Theme) string { return t.DetailLabels },
		set: func(t *providers.Theme, v string) { t.DetailLabels = v }},
	{label: "Detail text", get: func(t providers.Theme) string { return t.DetailText },
		set: func(t *providers.Theme, v string) { t.DetailText = v }},
	{label: "Active row text", get: func(t providers.Theme) string { return t.ActiveRowText },
		set: func(t *providers.Theme, v string) { t.ActiveRowText = v }},
	{label: "Active row background", get: func(t providers.Theme) string { return t.ActiveRowBackground },
		set: func(t *providers.Theme, v string) { t.ActiveRowBackground = v }},
}

// configuredThemes holds both theme sections of the loaded config so the
// settings panel can read and write them without touching the config file
// on disk until the user saves. configuredThemesDark is the dark section
// (the legacy "theme" key); configuredThemesLight is the light section.
var (
	configuredThemesDark  providers.Theme
	configuredThemesLight providers.Theme
)

// configuredTheme aliases the dark section for existing call sites that only
// care about the mode key living in the dark section.
var configuredTheme providers.Theme

func configuredThemeMode() providers.ThemeMode {
	return providers.ParseThemeMode(configuredTheme.Mode)
}

func setConfiguredThemeMode(mode providers.ThemeMode) {
	configuredTheme.Mode = string(mode)
	configuredThemesDark.Mode = string(mode)
}

// activeThemeSection returns the section the color fields should edit: the
// light section in light mode, the dark one otherwise. System mode edits
// the dark section; the terminal-resolved palette is not editable in place.
func activeThemeSection() *providers.Theme {
	if configuredThemeMode() == providers.ModeLight {
		return &configuredThemesLight
	}
	return &configuredThemesDark
}

// buildSettingsPage creates the settings overlay: a bordered form with the
// toggleable rows, one color input per key of the active mode's section, the
// config file path, and buttons. The page is added to the Pages stack once
// and rebuilt on every open.
func (a *app) buildSettingsPage() {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" Settings ")
	// Single-spaced rows: with 16 color inputs the padded default (one
	// blank line per item) would need ~47 rows and clip on a stock
	// terminal, so the form stays one line per item like the rest of the
	// overlays and still scrolls to the focused field on short screens.
	form.SetItemPadding(0)
	a.settingsForm = form
	a.rebuildSettingsForm()

	row := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(form, settingsFormHeight(a), 1, true).
		AddItem(nil, 0, 1, false)
	column := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(row, 56, 1, true).
		AddItem(nil, 0, 1, false)
	a.settingsRow = row
	a.settingsColumn = column
	a.pages.AddPage("settings", column, true, false)
}

// settingsFormHeight returns the flex height the form needs so tview draws
// the buttons instead of clipping them: one line per form item (toggles,
// color inputs, config path), the leading blank line inside the border, a
// blank line before and after the button row, and the border itself.
// On very short screens the flex hands the form a smaller height and tview
// scrolls it to the focused item, so every field stays reachable.
func settingsFormHeight(a *app) int {
	return len(a.settingsRows()) + len(themeColorKeys) + 7
}

// themeColorCycle is the curated list the color fields cycle through with
// the arrow keys: the 16-color set plus the names terminals render distinctly.
// Hex values and any other tcell name still type directly into the field.
var themeColorCycle = []string{
	"black", "white", "gray", "silver",
	"red", "green", "yellow", "blue", "magenta", "cyan",
	"olive", "orange", "teal", "maroon", "navy",
}

// rebuildSettingsForm refills the form from the live values so the fields
// match the current config every time the panel opens or a value changes.
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
	section := activeThemeSection()
	for _, key := range themeColorKeys {
		key := key
		form.AddInputField(key.label, key.get(*section), 12, nil,
			func(text string) {
				key.set(section, text)
				a.previewThemeColors()
		})
		idx := form.GetFormItemCount() - 1
		field, ok := form.GetFormItem(idx).(*tview.InputField)
		if !ok {
			continue
		}
		// Up and down cycle the curated color list so the palette is
		// reachable without typing; the field stays open for hex and
		// tcell names. Tab and shift-tab still move between fields.
		field.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyDown:
				cycleThemeColor(section, key, 1, field)
				return nil
			case tcell.KeyUp:
				cycleThemeColor(section, key, -1, field)
				return nil
			}
			return event
		})
	}
	path, _ := providers.ConfigFilePath()
	form.AddTextView("Config file", path, 38, 1, false, false)
	form.AddButton("Open in editor", func() { a.openConfigFile() })
	form.AddButton("Close", func() { a.closeSettings() })
}

// cycleThemeColor moves the section value for one key through the curated
// list and writes it into the field, which re-fires the changed callback so
// the preview repaints. Values outside the list enter the cycle at the
// first entry.
func cycleThemeColor(section *providers.Theme, key themeColorKey, dir int, field *tview.InputField) {
	idx := 0
	for i, c := range themeColorCycle {
		if c == key.get(*section) {
			idx = (i + dir + len(themeColorCycle)) % len(themeColorCycle)
			break
		}
	}
	field.SetText(themeColorCycle[idx])
}

// previewThemeColors applies the edited colors to the running UI without
// writing the config, so the user sees each keystroke take effect.
func (a *app) previewThemeColors() {
	applyTheme(resolveTheme(configuredThemesDark, configuredThemesLight, providers.BackgroundDark))
	a.applyThemeToWidgets()
}

// applySettings persists the toggled values and applies them live, then
// resyncs the form with what was written.
func (a *app) applySettings() {
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig())
	if err != nil {
		a.statusText("[red]Failed to read config: [-]" + err.Error())
		return
	}
	config.Theme = configuredThemesDark
	config.ThemeLight = configuredThemesLight
	config.Infobar.ShowShortcuts = infobarEnabled
	config.AutoUpdate.Enabled = a.staleAutoRefresh
	config.Theme.Mode = configuredTheme.Mode
	if err := providers.SaveConfig(config); err != nil {
		a.statusText("[red]Failed to save config: [-]" + err.Error())
		return
	}
	applyConfigTheme(config.Theme, config.ThemeLight)
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
	a.statusInfo("space toggles, arrows cycle colors, tab moves, enter runs a button, esc closes")
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