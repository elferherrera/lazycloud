package app

import (
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

// settingsRow describes one togglable settings entry rendered by the panel.
// themeRow marks the entry that renders as the theme cycle field instead
// of a checkbox.
type settingsRow struct {
	label    string
	get      func() bool
	set      func(on bool)
	themeRow bool
}

// settingsRows returns the toggleable entries in panel order: behavior
// first (auto update, column borders, shortcut bar), then the theme cycle
// (config packs, the custom palette, follow terminal); the color rows
// follow the toggles in rebuildSettingsForm.
func (a *app) settingsRows() []settingsRow {
	return []settingsRow{
		{label: "Auto update cache", get: func() bool { return a.staleAutoRefresh },
			set: func(on bool) { a.staleAutoRefresh = on }},
		{label: "Column borders", get: func() bool { return infoColumnBorders },
			set: func(on bool) { a.setInfoColumnBorders(on) }},
		{label: "Shortcut bar", get: func() bool { return infobarEnabled },
			set: func(on bool) { a.setInfobarEnabled(on) }},
		{label: themeRowLabel, get: func() bool { return !configuredFollowTerminal },
			// Space cycles on every press: the row's field shows the
			// selection itself, so the value the checkbox contract would
			// hand to set is unused and the option list advances instead.
			set:      func(_ bool) { a.cycleThemeName(1) },
			themeRow: true},
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

// customThemeName is the pseudo pack that owns the editable palette: while
// it is selected the color rows are shown, and saving writes the palette
// under this name into the config's pack map.
const customThemeName = "custom"

// followTerminalThemeName is the pseudo entry of the theme cycle that paints
// with the terminal's own colors, the single-palette equivalent of the old
// follow-terminal toggle.
const followTerminalThemeName = "follow terminal"

// themeSelectionLabel names the theme row's current selection: the follow
// pseudo entry while the terminal palette paints, the selected pack
// otherwise.
func themeSelectionLabel() string {
	if configuredFollowTerminal {
		return followTerminalThemeName
	}
	return configuredThemeName
}

// themeOptionNames lists the entries the theme row cycles: every pack of
// the config (alphabetically), the custom palette, and the follow-terminal
// entry last. The custom palette keeps its fixed slot after the packs even
// once seeded into the pack map, so cycling from it always reaches follow
// terminal instead of re-entering the pack list where "custom" would sort
// alphabetically.
func themeOptionNames() []string {
	packs := configuredThemeNames()
	names := make([]string, 0, len(packs)+2)
	for _, name := range packs {
		if name != customThemeName {
			names = append(names, name)
		}
	}
	names = append(names, customThemeName, followTerminalThemeName)
	return names
}

// themeColorsEditable reports whether the color rows edit a palette: only
// the custom selection carries editable colors, config packs stay pristine
// behind their hidden rows.
func themeColorsEditable() bool {
	return !configuredFollowTerminal && configuredThemeName == customThemeName
}

// selectThemeOption applies one entry of the theme cycle: follow terminal
// paints with the terminal's own colors, the custom selection seeds the
// editable palette from the pack that painted before it, and a config pack
// snaps the panel to its palette.
func (a *app) selectThemeOption(name string) {
	if name == followTerminalThemeName {
		if !configuredFollowTerminal {
			setConfiguredFollowTerminal(true)
			a.previewThemeColors()
			a.rebuildSettingsForm()
		}
		return
	}
	if configuredFollowTerminal {
		setConfiguredFollowTerminal(false)
	}
	if name == customThemeName {
		if _, ok := configuredThemePacks[customThemeName]; !ok {
			if configuredThemePacks == nil {
				configuredThemePacks = map[string]providers.Theme{}
			}
			configuredThemePacks[customThemeName] = configuredTheme
		}
	}
	configuredThemeName = name
	configuredTheme = configuredThemePacks[name]
	a.previewThemeColors()
	a.rebuildSettingsForm()
}

// cycleThemeName advances the theme row through the option list, snapping
// the panel's palette to the new selection and previewing it live.
// Unsaved edits to the current pack are stored back into the pack map
// first so switching away and back does not lose them.
func (a *app) cycleThemeName(dir int) {
	if configuredThemePacks == nil {
		configuredThemePacks = map[string]providers.Theme{}
	}
	if !configuredFollowTerminal {
		configuredThemePacks[configuredThemeName] = configuredTheme
	}
	names := themeOptionNames()
	idx := len(names) - 1
	if !configuredFollowTerminal {
		idx = -1
		for i, name := range names {
			if name == configuredThemeName {
				idx = i
				break
			}
		}
	}
	// A selection no longer in the list (pack deleted outside the panel)
	// restarts at the top, the one anchor that cannot fall off it.
	if idx < 0 {
		a.selectThemeOption(names[0])
		return
	}
	a.selectThemeOption(names[(idx+dir+len(names))%len(names)])
}

// setConfiguredFollowTerminal flips the terminal-follow selection: on, the
// app paints with the terminal's own colors and the color rows hide; off,
// the selected pack paints and its rows come back.
func setConfiguredFollowTerminal(on bool) {
	configuredFollowTerminal = on
	if on {
		applyTheme(providers.FollowTerminalTheme())
		return
	}
	applyActiveThemeFn()
}

// settingsPanelWidth is the window width, sized to the widest option row
// (the color-row labels plus the 12-column swatch field) with margin, so
// the "Open in editor" button and tview's blank line above it fit without
// the panel carrying dead space.
const settingsPanelWidth = 44

// buildSettingsPage creates the settings overlay: a bordered form with the
// toggleable rows, one color input per key of the active mode's section,
// and buttons. The page is added to the Pages stack once
// and rebuilt on every open.
func (a *app) buildSettingsPage() {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" Settings ")
	// Single-spaced rows: with 16 color inputs the padded default (one
	// blank line per item) would need ~47 rows and clip on a stock
	// terminal, so the form stays one line per item like the rest of the
	// overlays and still scrolls to the focused field on short screens.
	form.SetItemPadding(0)
	// The rows are read-only fields, so letters are free for vim
	// navigation: j/k step to the next/previous row (wrapping, buttons
	// included), gg/G jump to the first/last row, and the up/down arrows
	// do the same for keyboard layouts without vim habits. The capture
	// sits on the form, ahead of the per-item captures, so it works from
	// every row — except the color fields, which are real text inputs:
	// their runes must reach the field, or values like "orange" could
	// never be typed (the j/k/g/G interception would eat their letters).
	ggPending := false
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		total := form.GetFormItemCount() + form.GetButtonCount()
		if total == 0 {
			return event
		}
		item, button := form.GetFocusedItemIndex()
		if item >= 0 {
			if _, ok := form.GetFormItem(item).(*colorPreviewField); ok && event.Key() == tcell.KeyRune {
				ggPending = false
				return event
			}
		}
		current := item
		if button >= 0 {
			current = form.GetFormItemCount() + button
		}
		if current < 0 {
			current = 0
		}
		move := func(to int) {
			to = ((to % total) + total) % total
			form.SetFocus(to)
		}
		switch event.Key() {
		case tcell.KeyDown:
			ggPending = false
			move(current + 1)
			return nil
		case tcell.KeyUp:
			ggPending = false
			move(current - 1)
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				ggPending = false
				move(current + 1)
				return nil
			case 'k':
				ggPending = false
				move(current - 1)
				return nil
			case 'G':
				ggPending = false
				move(total - 1)
				return nil
			case 'g':
				if ggPending {
					ggPending = false
					move(0)
					return nil
				}
				ggPending = true
				return nil
			}
		}
		ggPending = false
		return event
	})
	a.settingsForm = form
	a.rebuildSettingsForm()

	// The spacers center the form on screen like a floating window.
	panel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, settingsFormHeight(a), 1, true)
	a.settingsPanel = panel
	row := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(panel, settingsFormHeight(a), 1, false).
		AddItem(nil, 0, 1, false)
	column := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(row, settingsPanelWidth, 1, true).
		AddItem(nil, 0, 1, false)
	a.settingsRow = row
	a.settingsColumn = column
	a.pages.AddPage("settings", column, true, false)
}

// themeRowLabel is the Theme row's bare label: the current selection
// shows in the row's field (space cycles it), so the label stays constant
// while the value changes.
const themeRowLabel = "Theme"

// settingsFormHeight returns the flex height the form needs so tview draws
// the buttons instead of clipping them: one line per form item (toggles,
// color inputs), the leading blank line inside the border, tview's blank
// line between the options and the buttons, one blank after the buttons,
// and the border itself. Follow-terminal mode and a config pack drop the
// color rows, so the panel shrinks with them. On very short screens the
// flex hands the form a smaller height and tview scrolls it to the focused
// item, so every field stays reachable.
func settingsFormHeight(a *app) int {
	// rows: toggles + color inputs, plus the leading blank line inside
	// the border, the blank line between the options and the buttons,
	// one blank line after the buttons, and the border itself.
	return settingsFormRows(a) + 6
}

// settingsFormRows counts the item rows the rebuild adds, so the height
// follows the actual form contents instead of a fragile constant.
func settingsFormRows(a *app) int {
	rows := len(a.settingsRows())
	if !configuredFollowTerminal && themeColorsEditable() {
		rows += len(themeColorKeys)
	}
	return rows
}

func (a *app) resizeSettingsOverlay() {
	if a.settingsColumn == nil || a.screen == nil {
		return
	}
	_, h := a.screen.Size()
	// The panel scrolls to the focused row when the flex gives it less
	// than it asks for, but only when it actually asks for more than the
	// screen: clamp the requested height so a small terminal still shows
	// the panel top-aligned with every row reachable by focus, instead
	// of the flex centering a 30-row form in 22 rows and clipping the
	// Follow terminal and Theme rows off the top. The clamp tracks the
	// terminal both ways: once a taller terminal can fit the form, the
	// request goes back to its natural size so the panel floats centered.
	height := settingsFormHeight(a)
	if height > h-2 {
		height = h - 2
	}
	// The height lives at two flex levels: the row centers the panel at
	// it, and the panel itself holds the form at it. Resizing only the
	// outer item keeps the drawn form at its build-time height, so the
	// bordered window never visibly grows into the new space.
	a.settingsRow.ResizeItem(a.settingsPanel, height, 1)
	if panel, ok := a.settingsPanel.(*tview.Flex); ok {
		panel.ResizeItem(a.settingsForm, height, 1)
	}
}

// themeColorCycle is the curated list the color fields step through with
// the space bar: the 16-color set plus the names terminals render
// distinctly. The fields also take typed input (hex values and any other
// tcell name), so this list is the keyboard shortcut, not the boundary.
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
		if row.themeRow {
			form.AddFormItem(newThemeSelectionField(a))
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
	for _, key := range themeColorKeys {
		// The color rows edit the custom palette; config packs stay
		// pristine behind their hidden rows, and follow-terminal mode
		// paints from the terminal, so there is nothing to edit.
		if !themeColorsEditable() {
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
			a.previewThemeColors()
		})
		// The color fields are real text inputs: typed runes edit the value
		// (hex, tcell names), space steps the curated color list, and the
		// swatch preview follows every change.
		field.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
				cycleThemeColor(&configuredTheme, key, 1, field.InputField)
				return nil
			}
			return event
		})
	}
	form.AddButton("Open in editor", func() { a.openConfigFile() })
	if wasFocused {
		a.restoreSettingsFocus(form, restoreLabel, item, button)
		// SetFocus only blurs and focuses when the index changes; after a
		// Clear nothing has focus, so for the restored row to actually
		// receive the focus callbacks (and key events) the app must hand
		// focus to the form again, which delegates to focusedElement.
		a.tapp.SetFocus(form)
	}
	// The window follows the form contents: cycling the Theme row into
	// custom adds the color fields (the panel must grow to fit them),
	// cycling back to a pack or follow terminal drops them (the panel
	// must shrink), so every refill re-derives the flex height.
	a.resizeSettingsOverlay()
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
// for buttons and rows whose label legitimately changed.
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

// themeSelectionField is the Theme row's cycle control: a read-only field
// whose text is the current selection (config pack, custom, or follow
// terminal), with space and enter walking the option list. Every other
// key is swallowed in the capture — the acceptance func cannot stop
// Backspace or Delete, which tview's text area would happily honour —
// except tab and backtab, which drive the form's own focus movement. It
// carries the same focus marker and accent styles as the checkboxes, and
// re-applies them in SetFormAttributes because Form.Draw restyles every
// item on each draw.
type themeSelectionField struct {
	*tview.InputField
	baseLabel string
}

func newThemeSelectionField(a *app) *themeSelectionField {
	f := &themeSelectionField{InputField: tview.NewInputField()}
	f.SetLabel(themeRowLabel)
	f.SetText(themeSelectionLabel())
	f.SetFieldWidth(themeSelectionFieldWidth())
	f.SetAcceptanceFunc(func(_ string, _ rune) bool { return false })
	f.SetFocusFunc(func() { f.update(true) })
	f.SetBlurFunc(func() { f.update(false) })
	f.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyBacktab:
			return event
		case tcell.KeyEnter, tcell.KeyRune:
			if event.Key() == tcell.KeyEnter || event.Rune() == ' ' {
				a.cycleThemeName(1)
			}
		}
		return nil
	})
	return f
}

// themeSelectionFieldWidth sizes the field to the longest option so every
// selection name draws whole; the option list only grows with pack names.
func themeSelectionFieldWidth() int {
	w := 0
	for _, name := range themeOptionNames() {
		if len(name) > w {
			w = len(name)
		}
	}
	if w == 0 {
		w = 1
	}
	return w
}

// SetLabel records the bare row label and displays it behind the current
// focus prefix.
func (f *themeSelectionField) SetLabel(label string) *tview.InputField {
	f.baseLabel = label
	return f.InputField.SetLabel(f.labeled())
}

// labeled returns the row label with the focus prefix of the current state.
func (f *themeSelectionField) labeled() string {
	if f.HasFocus() {
		return focusMarker + f.baseLabel
	}
	return noFocusMarker + f.baseLabel
}

// update repaints the row for the focused state: accent-colored label when
// focused, the settings label color otherwise.
func (f *themeSelectionField) update(focused bool) {
	f.InputField.SetLabel(f.labeled())
	if focused {
		f.SetLabelStyle(tcell.StyleDefault.Foreground(theme.focusColor()))
	} else {
		f.SetLabelStyle(tcell.StyleDefault.Foreground(theme.settingsLabelColor()))
	}
}

// SetFormAttributes re-applies the focused state after the form's per-draw
// restyle.
func (f *themeSelectionField) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	f.InputField.SetFormAttributes(labelWidth, labelColor, bgColor, fieldTextColor, fieldBgColor)
	f.update(f.HasFocus())
	return f
}

// Draw paints the field and then hides the cursor: the embedded text area
// calls ShowCursor whenever the field holds focus, printing a terminal
// cursor on the read-only value.
func (f *themeSelectionField) Draw(screen tcell.Screen) {
	f.InputField.Draw(screen)
	screen.HideCursor()
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

// colorPreviewField is a color text input that paints its background with
// the configured color and keeps the value text readable over it (black or
// white by luma), like a color picker swatch: typed runes edit the value,
// space steps the curated list, and the preview follows every change.
// Form.Draw restyles every item on each draw, so the preview re-applies
// itself in SetFormAttributes, which the form calls right after its own
// styling.
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

// labeled returns the row label with the focus prefix of the current state
// and a leading space, so the color rows sit one indent level under the
// toggles: they edit the palette the Theme row above selected.
func (f *colorPreviewField) labeled() string {
	if f.HasFocus() {
		return focusMarker + " " + f.baseLabel
	}
	return noFocusMarker + " " + f.baseLabel
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
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig(), defaultInfoConfig())
	if err != nil {
		a.statusText("[red]Failed to read config: [-]" + err.Error())
		return
	}
	config.FollowTerminal = configuredFollowTerminal
	config.Infobar.ShowShortcuts = infobarEnabled
	config.Info.ColumnBorders = infoColumnBorders
	config.AutoUpdate.Enabled = a.staleAutoRefresh
	// The freshly loaded file is the source of truth for packs the
	// panel never touched: a pack hand-edited (or added) while the
	// panel was open must not be clobbered by the in-memory copy from
	// before that edit. The in-memory map wins for the packs it still
	// carries.
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
	config.ThemeName = configuredThemeName
	configuredThemePacks = config.Themes
	if err := providers.SaveConfig(config); err != nil {
		a.statusText("[red]Failed to save config: [-]" + err.Error())
		return
	}
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
}

func (a *app) closeSettings() {
	// Color edits only live in memory until a checkbox save runs, so
	// closing the panel (Esc) persists them.
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
