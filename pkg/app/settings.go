package app

import (
	"sort"
	"strings"

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
// first (theme pack, theme mode, shortcut bar), then behavior (auto update).
func (a *app) settingsRows() []settingsRow {
	return []settingsRow{
		{label: "Theme: " + configuredThemeName, get: func() bool { return false },
			set: func(on bool) {
				if on {
					a.cycleThemeName(1)
				}
			}},
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

// configuredThemes holds the theme packs of the loaded config plus the
// selected pack's two sections, so the settings panel can read and write
// them without touching the config file on disk until the user saves.
// configuredThemesDark and configuredThemesLight are the selected pack's
// sections; configuredTheme aliases the dark section for existing call
// sites that only care about the mode key living there.
var (
	configuredThemePacks  map[string]providers.ThemeSpec
	configuredThemeName   string
	configuredThemesDark  providers.Theme
	configuredThemesLight providers.Theme
)

var configuredTheme providers.Theme

// setConfiguredThemes resolves the config's selected pack into the
// configured sections and applies it. Run and reloadConfig hand the whole
// loaded file over; the pack map stays in sync so a later save keeps every
// pack, not just the active one.
func setConfiguredThemes(config providers.Config) {
	configuredThemePacks = config.Themes
	configuredThemeName = config.ThemeName
	spec, ok := config.Themes[config.ThemeName]
	if !ok {
		configuredThemeName = providers.DefaultThemeName
		spec = config.Themes[configuredThemeName]
	}
	configuredThemesDark = spec.Dark
	configuredThemesLight = spec.Light
	configuredTheme = configuredThemesDark
	applyConfigTheme(configuredThemesDark, configuredThemesLight)
}

// configuredThemeNames lists the pack names in the order the settings row
// cycles them: alphabetical, so a growing list stays predictable.
func configuredThemeNames() []string {
	names := make([]string, 0, len(configuredThemePacks))
	for name := range configuredThemePacks {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		names = append(names, providers.DefaultThemeName)
	}
	return names
}

// cycleThemeName switches the selected pack, snaps the panel's sections to
// the new pack, previews it live, and repaints the row so the label shows
// the selection. Unsaved edits to the current pack are stored back into the
// pack map first so switching away and back does not lose them; the new
// pack keeps its own mode, which the mode rows and color fields pick up on
// the next rebuild.
func (a *app) cycleThemeName(dir int) {
	if configuredThemePacks == nil {
		configuredThemePacks = map[string]providers.ThemeSpec{}
	}
	configuredThemePacks[configuredThemeName] = providers.ThemeSpec{Dark: configuredThemesDark, Light: configuredThemesLight}
	names := configuredThemeNames()
	idx := 0
	for i, name := range names {
		if name == configuredThemeName {
			idx = i
			break
		}
	}
	configuredThemeName = names[(idx+dir+len(names))%len(names)]
	spec := configuredThemePacks[configuredThemeName]
	configuredThemesDark = spec.Dark
	configuredThemesLight = spec.Light
	configuredTheme = configuredThemesDark
	a.previewThemeColors()
	a.rebuildSettingsForm()
}

func configuredThemeMode() providers.ThemeMode {
	return providers.ParseThemeMode(configuredTheme.Mode)
}

func setConfiguredThemeMode(mode providers.ThemeMode) {
	configuredTheme.Mode = string(mode)
	configuredThemesDark.Mode = string(mode)
}

// activeThemeSection returns the section the color fields should edit: the
// section the resolved mode paints with, so an edit always lands where the
// user sees it. System mode resolves against the given terminal background;
// on a light terminal that is the light section.
func activeThemeSection(background providers.BackgroundKind) *providers.Theme {
	if resolveTheme(configuredThemesDark, configuredThemesLight, background) == configuredThemesDark {
		return &configuredThemesDark
	}
	return &configuredThemesLight
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

	// The key summary spans the overlay width under the form, outside the
	// form itself so no label column eats its space. It is not focusable,
	// so tab never lands on it and the focus chain stays form, buttons,
	// then esc closes.
	hint := tview.NewTextView().SetDynamicColors(false).SetText(settingsUsageHint)
	hint.SetTextColor(theme.shortcutLabelColor())
	a.settingsHint = hint

	row := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(form, settingsFormHeight(a), 1, true).
		AddItem(hint, 2, 0, false).
		AddItem(nil, 0, 1, false)
	column := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(row, 56, 1, true).
		AddItem(nil, 0, 1, false)
	a.settingsRow = row
	a.settingsColumn = column
	a.pages.AddPage("settings", column, true, false)
}

// settingsUsageHint is the two-line key summary the settings overlay draws
// under the form. It sits outside the form because the form's vertical
// layout starts every item's text after the widest label column, which
// would leave the hint a third of the overlay width. Every key named here
// is one the panel actually handles: tab moves between rows, space toggles
// a checkbox (and on the Theme row cycles the pack), arrows cycle color
// values, and closing (esc or the Close button) persists the edits.
const settingsUsageHint = "tab moves · space toggles or cycles the theme\narrows cycle colors · esc saves and closes"

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
	// Keep the user's row through the refill: Clear resets focus to the
	// first item, and applySettings rebuilds after every toggle. Without
	// this, the caret would jump to the top after each change. Capture the
	// indices before Clear destroys the old widgets, and only restore when
	// this form's subtree (the form itself or any of its items) held focus.
	item, button := form.GetFocusedItemIndex()
	wasFocused := form.HasFocus()
	form.Clear(true)
	form.SetLabelColor(theme.shortcutLabelColor())
	form.SetFieldTextColor(theme.shortcutTextColor())
	form.SetFieldBackgroundColor(tcell.ColorDefault)
	form.SetButtonBackgroundColor(tcell.ColorDefault)
	form.SetButtonTextColor(theme.shortcutTextColor())
	// Focused buttons paint their whole box with the activated style, so
	// the theme accent makes the focused button readable at a glance.
	form.SetButtonActivatedStyle(tcell.StyleDefault.
		Background(theme.focusColor()).
		Foreground(readableOnColor(theme.focusColor())))
	for _, row := range a.settingsRows() {
		row := row
		checkbox := newFocusCheckbox()
		checkbox.SetLabel(row.label)
		checkbox.SetChecked(row.get())
		checkbox.SetChangedFunc(func(checked bool) {
			row.set(checked)
			a.applySettings()
		})
		form.AddFormItem(checkbox)
	}
	section := activeThemeSection(a.settingsBackgroundKind())
	for _, key := range themeColorKeys {
		key := key
		field := newColorPreviewField()
		form.AddFormItem(field)
		field.SetLabel(key.label)
		field.SetFieldWidth(12)
		field.SetText(key.get(*section))
		field.SetChangedFunc(func(text string) {
			key.set(section, text)
			a.previewThemeColors()
		})
		// Up and down cycle the curated color list so the palette is
		// reachable without typing; the field stays open for hex and
		// tcell names. Tab and shift-tab still move between fields.
		field.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyDown:
				cycleThemeColor(section, key, 1, field.InputField)
				return nil
			case tcell.KeyUp:
				cycleThemeColor(section, key, -1, field.InputField)
				return nil
			}
			return event
		})
	}
	path, _ := providers.ConfigFilePath()
	form.AddTextView("Config file", path, 38, 1, false, false)
	form.AddButton("Open in editor", func() { a.openConfigFile() })
	form.AddButton("Close", func() { a.closeSettings() })
	if wasFocused {
		if item >= 0 && item < form.GetFormItemCount() {
			form.SetFocus(item)
		} else if button >= 0 && button < form.GetButtonCount() {
			form.SetFocus(form.GetFormItemCount() + button)
		}
		// SetFocus only blurs and focuses when the index changes; after a
		// Clear nothing has focus, so for the restored row to actually
		// receive the focus callbacks (and key events) the app must hand
		// focus to the form again, which delegates to focusedElement.
		a.tapp.SetFocus(form)
	}
}

// focusMarker prefixes the focused row's label, noFocusMarker pads the
// unfocused rows so the label column never shifts sideways when focus
// moves between rows.
const (
	focusMarker   = "▸ "
	noFocusMarker = "  "
)

// focusCheckbox is a settings toggle that shows which row is focused. The
// marker is part of the stored label because Checkbox.Draw prints the
// stored label directly, but the prefix keeps a constant width (▸ and two
// spaces both measure 2 cells) so the label column never shifts sideways
// when focus moves. The accent colors survive the form's per-draw restyle
// by being re-applied in SetFormAttributes, which Form.Draw calls after
// its own styling — the same trick colorPreviewField uses for its swatch.
type focusCheckbox struct {
	*tview.Checkbox
	base string
}

func newFocusCheckbox() *focusCheckbox {
	c := &focusCheckbox{Checkbox: tview.NewCheckbox()}
	c.SetFocusFunc(func() { c.update(true) })
	c.SetBlurFunc(func() { c.update(false) })
	return c
}

// SetLabel records the bare row label and displays it behind the current
// focus prefix.
func (c *focusCheckbox) SetLabel(label string) *tview.Checkbox {
	c.base = label
	return c.Checkbox.SetLabel(c.labeled())
}

// labeled returns the row label with the focus prefix of the current state.
func (c *focusCheckbox) labeled() string {
	if c.HasFocus() {
		return focusMarker + c.base
	}
	return noFocusMarker + c.base
}

// update repaints the row for the focused state: accent-colored label and
// a filled focus style for the checkbox square, so the focused row reads
// at a glance instead of white-on-white.
func (c *focusCheckbox) update(focused bool) {
	c.Checkbox.SetLabel(c.labeled())
	if focused {
		c.SetLabelStyle(tcell.StyleDefault.Foreground(theme.focusColor()))
		c.SetActivatedStyle(theme.focusStyle())
	} else {
		c.SetLabelStyle(tcell.StyleDefault.Foreground(theme.shortcutLabelColor()))
	}
}

// SetFormAttributes re-applies the focused state after the form's per-draw
// restyle.
func (c *focusCheckbox) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	c.Checkbox.SetFormAttributes(labelWidth, labelColor, bgColor, fieldTextColor, fieldBgColor)
	c.update(c.HasFocus())
	return c
}

// cycleThemeColor moves the section value for one key through the curated
// list and writes it into the field, which re-fires the changed callback so
// the preview repaints. Values outside the list enter at the first entry in
// the move direction, so an unknown value always cycles somewhere sensible.
func cycleThemeColor(section *providers.Theme, key themeColorKey, dir int, field *tview.InputField) {
	idx := 0
	if dir > 0 {
		idx = len(themeColorCycle) - 1
	}
	for i, c := range themeColorCycle {
		if c == key.get(*section) {
			idx = (i + dir + len(themeColorCycle)) % len(themeColorCycle)
			break
		}
	}
	field.SetText(themeColorCycle[idx])
}

// colorPreviewField is a color input that paints its background with the
// configured color and keeps the value text readable over it (black or
// white by luma), like a color picker swatch. Form.Draw restyles every item
// on each draw, so the preview re-applies itself in SetFormAttributes,
// which the form calls right after its own styling.
type colorPreviewField struct {
	*tview.InputField
	userChanged func(text string)
	baseLabel   string
}

func newColorPreviewField() *colorPreviewField {
	f := &colorPreviewField{InputField: tview.NewInputField()}
	f.InputField.SetChangedFunc(func(text string) {
		if f.userChanged != nil {
			f.userChanged(text)
		}
		f.applyPreview()
	})
	f.SetFocusFunc(f.applyFocusStyle)
	f.SetBlurFunc(f.applyFocusStyle)
	return f
}

// SetLabel records the bare row label and displays it with the current
// focus prefix; the swatch field needs the same marker the checkboxes
// carry so every row shows focus the same way.
func (f *colorPreviewField) SetLabel(label string) *tview.InputField {
	f.baseLabel = label
	return f.InputField.SetLabel(f.labeled())
}

// labeled returns the row label with the focus prefix of the current state.
func (f *colorPreviewField) labeled() string {
	if f.HasFocus() {
		return focusMarker + f.baseLabel
	}
	return noFocusMarker + f.baseLabel
}

// applyFocusStyle repaints the label style for the focused state; the
// accent is re-applied here because Form.Draw's per-draw restyle flattens
// styles set outside SetFormAttributes.
func (f *colorPreviewField) applyFocusStyle() {
	f.InputField.SetLabel(f.labeled())
	if f.HasFocus() {
		f.SetLabelStyle(tcell.StyleDefault.Foreground(theme.focusColor()))
	} else {
		f.SetLabelStyle(tcell.StyleDefault.Foreground(theme.shortcutLabelColor()))
	}
}

// SetChangedFunc chains the caller's handler with the swatch repaint, so
// both the config value and the preview update on every edit. The return
// value stays the embedded field because form code chains tview setters.
func (f *colorPreviewField) SetChangedFunc(handler func(text string)) *tview.InputField {
	f.userChanged = handler
	return f.InputField
}

// SetFormAttributes delegates to the embedded field, then repaints the
// swatch: this runs on every draw, after the form has overwritten the
// field style, so the preview survives redraws.
func (f *colorPreviewField) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	f.InputField.SetFormAttributes(labelWidth, labelColor, bgColor, fieldTextColor, fieldBgColor)
	f.applyFocusStyle()
	f.applyPreview()
	return f
}

// applyPreview paints the field background with the current value and picks
// black or white text so the value stays legible on any color.
func (f *colorPreviewField) applyPreview() {
	color := tcell.GetColor(strings.TrimSpace(f.GetText()))
	if color == tcell.ColorDefault {
		// Unparseable text (mid-typing): keep the form style so the
		// half-typed value stays visible instead of hiding the field.
		return
	}
	text := tcell.ColorWhite
	if color == tcell.ColorBlack || isLightColor(color) {
		text = tcell.ColorBlack
	}
	f.SetFieldStyle(tcell.StyleDefault.Background(color).Foreground(text))
}

// isLightColor reports whether a color reads as light, using the same
// BT.601 luma test the terminal background detection uses.
func isLightColor(color tcell.Color) bool {
	r, g, b := color.TrueColor().RGB()
	return (299*int(r)+587*int(g)+114*int(b))/1000 > 127
}

// previewThemeColors applies the edited colors to the running UI without
// writing the config, so the user sees each keystroke take effect. It
// resolves the mode against the cached terminal background (queried once
// per panel open, not per keystroke), so in system mode on a light terminal
// the preview paints the light section that closing will save.
func (a *app) previewThemeColors() {
	applyTheme(resolveTheme(configuredThemesDark, configuredThemesLight, a.settingsBackgroundKind()))
	a.applyThemeToWidgets()
	a.applyThemeChrome()
}

// settingsBackgroundKind returns the terminal background for the settings
// panel, querying it on first use and caching it for the panel session.
func (a *app) settingsBackgroundKind() providers.BackgroundKind {
	if a.settingsBackground == "" {
		a.settingsBackground = detectBackground()
	}
	return a.settingsBackground
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
	// themeLight.mode round-trips as a copy of theme.mode so the file never
	// carries a value that looks like it selects the mode: only theme.mode
	// does, and a hand-edited themeLight.mode is ignored on load.
	config.ThemeLight.Mode = configuredTheme.Mode
	config.Infobar.ShowShortcuts = infobarEnabled
	config.AutoUpdate.Enabled = a.staleAutoRefresh
	config.Theme.Mode = configuredTheme.Mode
	// Store the edited sections back and write the whole pack map: edits
	// made to other packs during this panel session live in the map (kept
	// there by cycleThemeName), and dropping them on save would lose work.
	// A nil map (no config file yet) gets a fresh one-pack map.
	if configuredThemePacks == nil {
		configuredThemePacks = map[string]providers.ThemeSpec{}
	}
	configuredThemePacks[configuredThemeName] = providers.ThemeSpec{Dark: configuredThemesDark, Light: configuredThemesLight}
	config.Themes = configuredThemePacks
	config.ThemeName = configuredThemeName
	if err := providers.SaveConfig(config); err != nil {
		a.statusText("[red]Failed to save config: [-]" + err.Error())
		return
	}
	applyConfigTheme(config.Theme, config.ThemeLight)
	a.applyThemeToWidgets()
	a.applyThemeChrome()
	a.setInfobarEnabled(infobarEnabled)
	a.rebuildSettingsForm()
	a.statusText("[green]Settings saved[-]")
}

func (a *app) openSettings() {
	// One terminal query per panel session; previewThemeColors caches it.
	// Reset before the first build so the initial rebuild does not grow a
	// second query.
	a.settingsBackground = ""
	if a.settingsForm == nil {
		a.buildSettingsPage()
	}
	a.rebuildSettingsForm()
	a.resizeSettingsOverlay()
	a.pages.ShowPage("settings")
	a.tapp.SetFocus(a.settingsForm)
	a.updateSettingsCursorStyle()
	a.statusInfo("space toggles, arrows cycle colors, tab moves, enter runs a button, esc closes")
}

func (a *app) closeSettings() {
	// Color edits only live in memory until a checkbox save runs, so closing
	// the panel persists them: Esc and the Close button both land here.
	a.applySettings()
	a.pages.HidePage("settings")
	a.tapp.SetFocus(a.info)
	a.restoreCursorStyle()
}

// updateSettingsCursorStyle switches the terminal cursor to a steady block
// in the theme accent color while the settings form has focus, so even rows
// without a painted box (text views, fields painted by a swatch) carry a
// visible focus signal. InputField.ShowCursor keeps the caret visible on the
// fields themselves.
func (a *app) updateSettingsCursorStyle() {
	if a.screen == nil {
		return
	}
	a.screen.SetCursorStyle(tcell.CursorStyleSteadyBlock, theme.focusColor())
}

// restoreCursorStyle hands the cursor back to the terminal default when the
// settings page loses focus. The explicit ColorReset matters: SetCursorStyle
// without a color passes ColorNone, which tcell treats as "leave the color
// alone", so the theme accent would stick to the cursor after close.
// tcell also resets the cursor style and color on shutdown, so a crash
// cannot leave the terminal with a styled cursor either.
func (a *app) restoreCursorStyle() {
	if a.screen == nil {
		return
	}
	a.screen.SetCursorStyle(tcell.CursorStyleDefault, tcell.ColorReset)
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
