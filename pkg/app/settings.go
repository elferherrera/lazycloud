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
// first (follow-terminal or theme pack and colors, shortcut bar), then
// behavior (auto update).
func (a *app) settingsRows() []settingsRow {
	return []settingsRow{
		{label: "Follow terminal", get: func() bool { return configuredFollowTerminal },
			set: func(on bool) {
				setConfiguredFollowTerminal(on)
				if on {
					// The pack and any unsaved edits stay in memory; say
					// where the rows went so the toggle does not look
					// like it destroyed work.
					a.statusInfo("following the terminal palette; theme rows hidden, uncheck to edit " + configuredThemeName)
				}
			}},
		{label: "Theme: " + configuredThemeName, get: func() bool { return !configuredFollowTerminal },
			// Space cycles on every press: the checkbox visual is the
			// row's focus affordance (checked whenever a pack paints the
			// UI), not a state to flip, so the value tview hands to set
			// is ignored and the pack list advances instead.
			set: func(_ bool) { a.cycleThemeName(1) }},
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
	{label: "Settings labels", get: func(t providers.Theme) string { return t.SettingsLabels },
		set: func(t *providers.Theme, v string) { t.SettingsLabels = v }},
	{label: "Settings text", get: func(t providers.Theme) string { return t.SettingsText },
		set: func(t *providers.Theme, v string) { t.SettingsText = v }},
	{label: "Active row text", get: func(t providers.Theme) string { return t.ActiveRowText },
		set: func(t *providers.Theme, v string) { t.ActiveRowText = v }},
	{label: "Active row background", get: func(t providers.Theme) string { return t.ActiveRowBackground },
		set: func(t *providers.Theme, v string) { t.ActiveRowBackground = v }},
}

// configuredThemes holds the theme packs of the loaded config plus the
// selected pack's palette, so the settings panel can read and write them
// without touching the config file on disk until the user saves.
// configuredTheme is the selected pack's palette; configuredFollowTerminal
// mirrors the config flag and switches the whole app to the terminal's
// own colors.
var (
	configuredThemePacks     map[string]providers.Theme
	configuredThemeName      string
	configuredTheme          providers.Theme
	configuredFollowTerminal bool
)

// setConfiguredThemes resolves the config's selected pack into the
// configured palette and applies the palette the follow flag selects. Run
// and reloadConfig hand the whole loaded file over; the pack map stays in
// sync so a later save keeps every pack, not just the active one.
func setConfiguredThemes(config providers.Config) {
	configuredThemePacks = config.Themes
	configuredThemeName = config.ThemeName
	configuredFollowTerminal = config.FollowTerminal
	pack, ok := config.Themes[config.ThemeName]
	if !ok {
		configuredThemeName = providers.DefaultThemeName
		pack = config.Themes[configuredThemeName]
	}
	configuredTheme = pack
	applyActiveThemeFn()
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

// cycleThemeName switches the selected pack, snaps the panel's palette to
// the new pack, previews it live, and repaints the row so the label shows
// the selection. Unsaved edits to the current pack are stored back into the
// pack map first so switching away and back does not lose them.
func (a *app) cycleThemeName(dir int) {
	if configuredThemePacks == nil {
		configuredThemePacks = map[string]providers.Theme{}
	}
	configuredThemePacks[configuredThemeName] = configuredTheme
	names := configuredThemeNames()
	// A name no longer in the map (pack deleted outside the panel)
	// must not silently skip the first pack: start before the list so
	// any direction lands on a real pack.
	idx := -1
	for i, name := range names {
		if name == configuredThemeName {
			idx = i
			break
		}
	}
	if idx < 0 {
		// The selected name is gone; either direction restarts at the
		// top of the list, the one anchor that cannot fall off it.
		configuredThemeName = names[0]
		configuredTheme = configuredThemePacks[configuredThemeName]
		if a.settingsSaveAs != nil {
			a.settingsSaveAsSuggestion = suggestedThemeName()
			a.settingsSaveAs.SetText(a.settingsSaveAsSuggestion)
		}
		a.previewThemeColors()
		a.rebuildSettingsForm()
		return
	}
	configuredThemeName = names[(idx+dir+len(names))%len(names)]
	configuredTheme = configuredThemePacks[configuredThemeName]
	// The save-as name follows the pack switch: a name typed for the
	// previous pack would silently rename this one, so the field goes
	// back to the suggestion.
	if a.settingsSaveAs != nil {
		a.settingsSaveAsSuggestion = suggestedThemeName()
		a.settingsSaveAs.SetText(a.settingsSaveAsSuggestion)
	}
	a.previewThemeColors()
	a.rebuildSettingsForm()
}

// setConfiguredFollowTerminal flips the terminal-follow toggle: on, the
// app paints with the terminal's own colors and the theme and color rows
// hide; off, the selected pack paints and its rows come back.
func setConfiguredFollowTerminal(on bool) {
	configuredFollowTerminal = on
	if on {
		applyTheme(providers.FollowTerminalTheme())
		return
	}
	applyActiveThemeFn()
}

// suggestedThemeName seeds the save-as field: a bundled pack offers its
// fork name (monokai-custom), a custom pack offers its own name so typing
// something else creates a copy under that name.
func suggestedThemeName() string {
	if providers.IsBundledThemeName(configuredThemeName) {
		return providers.CustomThemeForkName(configuredThemeName, configuredThemeNameSet())
	}
	return configuredThemeName
}

// configuredThemeNameSet collects the pack names the fork-name picker must
// dodge, as the set CustomThemeForkName expects.
func configuredThemeNameSet() map[string]bool {
	set := make(map[string]bool, len(configuredThemePacks))
	for name := range configuredThemePacks {
		set[name] = true
	}
	return set
}

// forkBundledTheme moves a color edit off a bundled seed: the selected pack
// renames to the fork name and the seed returns to its shipped palette in
// the pack map, so saving writes the fork while monokai stays monokai.
// Runs right after a color edit lands in the section (the changed callback
// calls it after key.set), so the fork snapshot carries the edit, and does
// nothing once the pack is already custom.
func (a *app) forkBundledTheme() {
	if !providers.IsBundledThemeName(configuredThemeName) {
		return
	}
	seedName := configuredThemeName
	forkName := providers.CustomThemeForkName(seedName, configuredThemeNameSet())
	// The seed slot goes back to the pristine palette the bundle shipped;
	// the fork carries the seed palette plus the edit in flight.
	configuredThemePacks[seedName] = providers.BundledThemes()[seedName]
	configuredThemePacks[forkName] = configuredTheme
	configuredThemeName = forkName
	if a.sessionForks == nil {
		a.sessionForks = map[string]bool{}
	}
	a.sessionForks[forkName] = true
	// The Theme row and the save-as field follow the rename unless the
	// user typed a save-as name already.
	a.relabelThemeRow(seedName)
	a.syncSaveAsField()
	a.statusInfo(seedName + " edits forked as " + forkName)
}

// syncSaveAsField keeps the save-as name in step with the selected pack
// while the user has not typed over it: the field offers the fork name of
// a bundled seed or the pack's own name. It runs after renames and pack
// switches, and respects whatever the user typed on top.
func (a *app) syncSaveAsField() {
	if a.settingsSaveAs == nil {
		return
	}
	if a.settingsSaveAs.GetText() == a.settingsSaveAsSuggestion {
		newSuggestion := suggestedThemeName()
		a.settingsSaveAs.SetText(newSuggestion)
		a.settingsSaveAsSuggestion = newSuggestion
	}
}

// relabelThemeRow repaints the Theme row label after a rename without a
// full rebuild, so the caret stays in the color field being edited. The
// next rebuild refreshes the rest of the form.
func (a *app) relabelThemeRow(seedName string) {
	if a.settingsForm == nil {
		return
	}
	for i := 0; i < a.settingsForm.GetFormItemCount(); i++ {
		item := a.settingsForm.GetFormItem(i)
		if cb, ok := item.(*focusCheckbox); ok && cb.base == "Theme: "+seedName {
			cb.base = "Theme: " + configuredThemeName
			cb.SetLabel(cb.base)
		}
	}
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
	// then esc saves and closes.
	hint := tview.NewTextView().SetDynamicColors(false).SetText(settingsUsageHint)
	hint.SetTextColor(theme.settingsLabelColor())
	a.settingsHint = hint

	// The form and its hint line form the panel body; the shadow wrapper
	// adds the one-cell depth strip around it, and the spacers center the
	// whole thing on screen like a floating window.
	panel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, settingsFormHeight(a), 1, true).
		AddItem(hint, 2, 0, false)
	shadow := newShadowedOverlay(panel)
	a.settingsPanel = shadow
	row := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(shadow, settingsFormHeight(a)+2, 1, false).
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

// settingsUsageHintFollow is the follow-terminal variant: the theme and
// color rows are hidden there, so the hint names only the keys that still
// do something.
const settingsUsageHintFollow = "tab moves · space toggles\nenter runs a button · esc saves and closes"

// settingsFormHeight returns the flex height the form needs so tview draws
// the buttons instead of clipping them: one line per form item (toggles,
// theme name input, color inputs, config path), the leading blank line
// inside the border, a blank line before and after the button row, and the
// border itself. Follow-terminal mode drops the theme, save-as and color
// rows, so the panel shrinks with them. On very short screens the flex
// hands the form a smaller height and tview scrolls it to the focused
// item, so every field stays reachable.
func settingsFormHeight(a *app) int {
	// rows: toggles + theme row + save-as + color inputs + config path,
	// plus the leading blank line inside the border, a blank line before
	// and after the button row, and the border itself.
	if configuredFollowTerminal {
		return settingsFormRows(a) + 6
	}
	return settingsFormRows(a) + 8
}

// settingsFormRows counts the item rows the rebuild adds, so the height
// follows the actual form contents instead of a fragile constant.
func settingsFormRows(a *app) int {
	rows := len(a.settingsRows())
	if configuredFollowTerminal {
		rows-- // the Theme row hides
	} else {
		rows++ // the save-as input
		rows += len(themeColorKeys)
	}
	rows++ // the Config file path
	return rows
}

func (a *app) resizeSettingsOverlay() {
	if a.settingsColumn == nil || a.screen == nil {
		return
	}
	w, h := a.screen.Size()
	if w <= 0 {
		return
	}
	if width := overlayWidth(w); width > 56 {
		a.settingsColumn.ResizeItem(a.settingsRow, width, 1)
	}
	// The panel scrolls to the focused row when the flex gives it less
	// than it asks for, but only when it actually asks for more than the
	// screen: clamp the requested height so a small terminal still shows
	// the panel top-aligned with every row reachable by focus, instead
	// of the flex centering a 30-row form in 22 rows and clipping the
	// Follow terminal and Theme rows off the top.
	if want := settingsFormHeight(a) + 2; want > h-2 {
		a.settingsRow.ResizeItem(a.settingsPanel, h-2, 1)
	}
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
	// The key summary follows the mode: follow-terminal hides the theme
	// and color rows, so naming their keys would describe controls that
	// are not on screen.
	if a.settingsHint != nil {
		if configuredFollowTerminal {
			a.settingsHint.SetText(settingsUsageHintFollow)
		} else {
			a.settingsHint.SetText(settingsUsageHint)
		}
	}
	form := a.settingsForm
	// Keep the user's row through the refill: Clear resets focus to the
	// first item, and applySettings rebuilds after every toggle. The row
	// is remembered by label, not index: toggling Follow terminal adds
	// and removes rows, so a saved index can point at a different row or
	// past the end. Only restore when this form's subtree held focus.
	item, button := form.GetFocusedItemIndex()
	restoreLabel := ""
	if wasFocusedLabel, ok := a.settingsFocusLabel(); ok {
		restoreLabel = wasFocusedLabel
	}
	wasFocused := form.HasFocus()
	form.Clear(true)
	form.SetLabelColor(theme.settingsLabelColor())
	form.SetFieldTextColor(theme.settingsTextColor())
	form.SetFieldBackgroundColor(tcell.ColorDefault)
	form.SetButtonBackgroundColor(tcell.ColorDefault)
	form.SetButtonTextColor(theme.settingsTextColor())
	// Focused buttons paint their whole box with the activated style, so
	// the theme accent makes the focused button readable at a glance.
	form.SetButtonActivatedStyle(tcell.StyleDefault.
		Background(theme.focusColor()).
		Foreground(readableOnColor(theme.focusColor())))
	for _, row := range a.settingsRows() {
		row := row
		// Theme rows govern pack colors; in follow-terminal mode the
		// terminal owns every color, so they do not apply.
		if configuredFollowTerminal && isThemeRow(row.label) {
			continue
		}
		checkbox := newFocusCheckbox()
		checkbox.SetLabel(row.label)
		checkbox.SetChecked(row.get())
		checkbox.SetChangedFunc(func(checked bool) {
			row.set(checked)
			a.applySettings()
		})
		form.AddFormItem(checkbox)
	}
	// Save-as row: the name the current pack saves under. A bundled pack
	// forks into the suggested name on its first color edit, so the seed
	// stays pristine; typing a different name saves the colors as a new
	// pack instead. A typed name survives the refill. Follow-terminal
	// mode owns every color, so no pack gets saved or renamed.
	if configuredFollowTerminal {
		// The typed name outlives the toggle: stash it so turning
		// follow-terminal off gives the user back what they typed
		// instead of silently resetting to the suggestion.
		if a.settingsSaveAs != nil {
			if typed := a.settingsSaveAs.GetText(); typed != "" && typed != a.settingsSaveAsSuggestion {
				a.settingsTypedName = typed
			}
		}
		a.settingsSaveAs = nil
	} else {
		saveAs := tview.NewInputField()
		saveAs.SetFieldWidth(16)
		suggestion := suggestedThemeName()
		text := suggestion
		if a.settingsTypedName != "" {
			text = a.settingsTypedName
		} else if a.settingsSaveAs != nil {
			if typed := a.settingsSaveAs.GetText(); typed != "" && typed != a.settingsSaveAsSuggestion {
				text = typed
			}
		}
		a.settingsTypedName = ""
		saveAs.SetText(text)
		a.settingsSaveAsSuggestion = suggestion
		a.settingsSaveAs = saveAs
		// Same focus marker as the checkbox and color rows, so every focusable
		// row shows which one holds the caret.
		markSaveAs := func() {
			marker := noFocusMarker
			if saveAs.HasFocus() {
				marker = focusMarker
			}
			saveAs.SetLabel(marker + "Save theme as")
		}
		saveAs.SetFocusFunc(markSaveAs)
		saveAs.SetBlurFunc(markSaveAs)
		markSaveAs()
		form.AddFormItem(saveAs)
	}
	for _, key := range themeColorKeys {
		// The color rows edit the pack palette; follow-terminal mode
		// paints from the terminal, so there is nothing to edit.
		if configuredFollowTerminal {
			break
		}
		key := key
		field := newColorPreviewField()
		form.AddFormItem(field)
		field.SetLabel(key.label)
		field.SetFieldWidth(12)
		field.SetText(key.get(configuredTheme))
		field.SetChangedFunc(func(text string) {
			key.set(&configuredTheme, text)
			a.forkBundledTheme()
			a.previewThemeColors()
		})
		// Up and down cycle the curated color list so the palette is
		// reachable without typing; the field stays open for hex and
		// tcell names. Tab and shift-tab still move between fields.
		field.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyDown:
				cycleThemeColor(&configuredTheme, key, 1, field.InputField)
				return nil
			case tcell.KeyUp:
				cycleThemeColor(&configuredTheme, key, -1, field.InputField)
				return nil
			}
			return event
		})
	}
	path, _ := providers.ConfigFilePath()
	form.AddTextView("Config file", path, 38, 1, false, false)
	form.AddButton("Open in editor", func() { a.openConfigFile() })
	if configuredFollowTerminal {
		// Deleting or renaming packs only applies while a pack paints
		// the UI; in follow-terminal mode both buttons would be no-ops.
		form.AddButton("Close", func() { a.closeSettings() })
		if wasFocused {
			a.restoreSettingsFocus(form, restoreLabel, item, button)
			a.tapp.SetFocus(form)
		}
		return
	}
	// Delete stays honest about what it can do: bundled packs are seeds
	// that would come back on the next seeding anyway, and deleting the
	// selected pack needs a fallback, so the button only acts on custom
	// packs and otherwise explains itself in the status line.
	form.AddButton("Delete theme", func() { a.deleteTheme() })
	form.AddButton("Close", func() { a.closeSettings() })
	if wasFocused {
		a.restoreSettingsFocus(form, restoreLabel, item, button)
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

// settingsFocusLabel returns the bare label of the form row that holds
// focus, so a rebuild can put the caret back on the same row after
// rows appear or vanish. Labels carry a focus-marker prefix while
// focused, so the marker is stripped first.
func (a *app) settingsFocusLabel() (string, bool) {
	item, _ := a.settingsForm.GetFocusedItemIndex()
	if item < 0 || item >= a.settingsForm.GetFormItemCount() {
		return "", false
	}
	label := a.settingsForm.GetFormItem(item).GetLabel()
	label = strings.TrimPrefix(label, focusMarker)
	label = strings.TrimPrefix(label, noFocusMarker)
	return label, true
}

// restoreSettingsFocus hands the caret back to the row the user was on:
// by label when the row survived the rebuild (the row count changes
// when Follow terminal toggles), falling back to the pre-Clear indices
// for buttons and rows whose label legitimately changed (the Theme row
// renames itself on pack switch).
func (a *app) restoreSettingsFocus(form *tview.Form, label string, item, button int) {
	if label != "" {
		for i := 0; i < form.GetFormItemCount(); i++ {
			row := form.GetFormItem(i).GetLabel()
			row = strings.TrimPrefix(row, focusMarker)
			row = strings.TrimPrefix(row, noFocusMarker)
			if row == label {
				form.SetFocus(i)
				return
			}
		}
	}
	if item >= 0 && item < form.GetFormItemCount() {
		form.SetFocus(item)
	} else if button >= 0 && button < form.GetButtonCount() {
		form.SetFocus(form.GetFormItemCount() + button)
	}
}

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
		c.SetLabelStyle(tcell.StyleDefault.Foreground(theme.settingsLabelColor()))
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
		f.SetLabelStyle(tcell.StyleDefault.Foreground(theme.settingsLabelColor()))
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

// isLightColor reports whether a color reads as light, using the
// BT.601 luma test.
func isLightColor(color tcell.Color) bool {
	r, g, b := color.TrueColor().RGB()
	return (299*int(r)+587*int(g)+114*int(b))/1000 > 127
}

// isThemeRow reports whether a settings row governs pack colors; the
// follow-terminal toggle hides these rows because no pack paints the UI
// while it is on.
func isThemeRow(label string) bool {
	return strings.HasPrefix(label, "Theme: ")
}

// previewThemeColors applies the edited palette to the running UI without
// writing the config, so the user sees each keystroke take effect.
func (a *app) previewThemeColors() {
	applyActiveTheme()
	a.applyThemeToWidgets()
	a.applyThemeChrome()
}

// applySettings persists the toggled values and applies them live, then
// resyncs the form with what was written.
func (a *app) applySettings() {
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig())
	if err != nil {
		a.statusText("[red]Failed to read config: [-]" + err.Error())
		return
	}
	config.FollowTerminal = configuredFollowTerminal
	config.Infobar.ShowShortcuts = infobarEnabled
	config.AutoUpdate.Enabled = a.staleAutoRefresh
	// The freshly loaded file is the source of truth for packs the
	// panel never touched: a pack hand-edited (or added) while the
	// panel was open must not be clobbered by the in-memory copy from
	// before that edit. The in-memory map wins for the packs it still
	// carries, and names the panel deleted stay deleted instead of
	// being resurrected from the file.
	for name := range a.settingsDeletedPacks {
		delete(config.Themes, name)
	}
	for name, pack := range configuredThemePacks {
		config.Themes[name] = pack
	}
	// Store the edited palette back and write the whole pack map: edits
	// made to other packs during this panel session live in the map (kept
	// there by cycleThemeName), and dropping them on save would lose work.
	// A nil map (no config file yet) gets a fresh one-pack map.
	if configuredThemePacks == nil {
		configuredThemePacks = map[string]providers.Theme{}
	}
	configuredThemePacks[configuredThemeName] = configuredTheme
	config.Themes[configuredThemeName] = configuredTheme
	configuredThemePacks = config.Themes
	config.ThemeName = configuredThemeName
	if err := providers.SaveConfig(config); err != nil {
		a.statusText("[red]Failed to save config: [-]" + err.Error())
		return
	}
	config.Theme = configuredTheme
	applyActiveTheme()
	a.applyThemeToWidgets()
	a.applyThemeChrome()
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
	a.updateSettingsCursorStyle()
	if configuredFollowTerminal {
		a.statusInfo("space toggles, tab moves, enter runs a button, esc saves and closes")
	} else {
		a.statusInfo("space toggles, arrows cycle colors, tab moves, enter runs a button, esc saves and closes")
	}
}

// applyTypedThemeName consumes the save-as name at close time: a name typed
// over the suggestion renames the current pack (an auto-fork from this
// session is absorbed, leaving one pack behind), and a name that collides
// with an existing pack or a bundled seed is refused so nothing is silently
// overwritten. Reports whether the panel can close.
func (a *app) applyTypedThemeName() bool {
	if a.settingsSaveAs == nil {
		return true
	}
	typed := strings.TrimSpace(a.settingsSaveAs.GetText())
	// The untouched suggestion never renames anything: on a bundled pack
	// it is only the fork preview, so closing without edits must not
	// create a stray -custom copy.
	if typed == "" || typed == configuredThemeName || typed == a.settingsSaveAsSuggestion {
		return true
	}
	if _, exists := configuredThemePacks[typed]; exists || providers.IsBundledThemeName(typed) {
		reason := "already in use"
		if providers.IsBundledThemeName(typed) {
			reason = "a bundled theme name"
		}
		a.statusText("[red]theme name " + typed + " is " + reason + "; edit the Save theme as row to pick another[-]")
		// Keep the panel open with the caret on the offending row so the
		// name is fixed or cleared instead of silently dropped.
		a.settingsForm.SetFocus(len(a.settingsRows()))
		a.tapp.SetFocus(a.settingsForm)
		return false
	}
	if a.sessionForks[configuredThemeName] {
		// The auto-fork was only a landing spot for the edits; the typed
		// name replaces it, so no near-identical leftover stays behind.
		delete(configuredThemePacks, configuredThemeName)
		if a.settingsDeletedPacks == nil {
			a.settingsDeletedPacks = map[string]bool{}
		}
		a.settingsDeletedPacks[configuredThemeName] = true
	}
	configuredThemeName = typed
	return true
}

// deleteTheme removes the selected pack from the config. Bundled seeds
// and the default pack always come back (the seed on the next config seed,
// the default as the fallback), so deleting them would be a lie; the
// button refuses both and says so. Deleting a custom pack persists the
// loss, snaps the panel to the default pack, and previews it.
func (a *app) deleteTheme() {
	if providers.IsBundledThemeName(configuredThemeName) {
		a.statusText("[red]" + configuredThemeName + " is a bundled theme and cannot be deleted[-]")
		return
	}
	if configuredThemeName == providers.DefaultThemeName {
		a.statusText("[red]" + configuredThemeName + " is the fallback theme and cannot be deleted[-]")
		return
	}
	delete(configuredThemePacks, configuredThemeName)
	if configuredThemePacks == nil {
		configuredThemePacks = map[string]providers.Theme{}
	}
	// The default pack is the fallback selection, so it must exist to be
	// selectable after a delete.
	if _, ok := configuredThemePacks[providers.DefaultThemeName]; !ok {
		configuredThemePacks[providers.DefaultThemeName] = defaultThemeConfig()
	}
	deleted := configuredThemeName
	if a.settingsDeletedPacks == nil {
		a.settingsDeletedPacks = map[string]bool{}
	}
	a.settingsDeletedPacks[deleted] = true
	configuredThemeName = providers.DefaultThemeName
	configuredTheme = configuredThemePacks[providers.DefaultThemeName]
	// A name typed for the deleted pack must not rename the fallback
	// pack on close: the field falls back to the suggestion on rebuild.
	if a.settingsSaveAs != nil {
		a.settingsSaveAs.SetText("")
	}
	a.previewThemeColors()
	a.rebuildSettingsForm()
	a.applySettings()
	a.statusInfo("deleted " + deleted)
}

func (a *app) closeSettings() {
	// Color edits only live in memory until a checkbox save runs, so closing
	// the panel persists them: Esc and the Close button both land here. A
	// typed save-as name renames the pack as part of the same save; a name
	// collision keeps the panel open.
	if !a.applyTypedThemeName() {
		return
	}
	a.sessionForks = nil
	a.settingsDeletedPacks = nil
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
