package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

// settingsTestApp builds an app with the widgets applySettings touches:
// the Pages stack and the status bar.
func settingsTestApp(t *testing.T) *app {
	t.Helper()
	return &app{
		pages:  tview.NewPages(),
		status: tview.NewTextView().SetDynamicColors(true),
		tapp:   tview.NewApplication(),
		info:   tview.NewTable(),
	}
}

func TestSettingsRowsCoverPanelToggles(t *testing.T) {
	rows := settingsTestApp(t).settingsRows()
	labels := make(map[string]bool, len(rows))
	for _, row := range rows {
		labels[row.label] = true
		if row.get == nil || row.set == nil {
			t.Errorf("settings row %q has nil accessors", row.label)
		}
	}
	for _, want := range []string{"Shortcut bar", "Column borders", "Auto update cache"} {
		if !labels[want] {
			t.Errorf("settings panel is missing row %q", want)
		}
	}
	sawTheme := false
	for label := range labels {
		if strings.HasPrefix(label, "Theme: ") {
			sawTheme = true
		}
	}
	if !sawTheme {
		t.Error("settings panel is missing the Theme pack row")
	}
	for _, gone := range []string{"Light theme", "Dark theme", "Theme follows terminal", "Follow terminal"} {
		if labels[gone] {
			t.Errorf("settings panel still carries removed row %q", gone)
		}
	}
}

// TestThemeCycleCoversCustomAndFollowTerminal pins the theme cycle: the
// row loops through the config packs, then the custom palette (color rows
// shown), then follow terminal (terminal palette paints), then wraps back
// to the packs; custom is seeded from the palette that painted before it.
func TestThemeCycleCoversCustomAndFollowTerminal(t *testing.T) {
	originalTheme := theme
	originalPacks := configuredThemePacks
	originalName := configuredThemeName
	originalPalette := configuredTheme
	originalFollow := configuredFollowTerminal
	t.Cleanup(func() {
		theme = originalTheme
		configuredThemePacks = originalPacks
		configuredThemeName = originalName
		configuredTheme = originalPalette
		configuredFollowTerminal = originalFollow
	})
	configuredThemePacks = map[string]providers.Theme{
		"lazycloud_dark": {Background: "black", Labels: "yellow"},
	}
	configuredThemeName = "lazycloud_dark"
	configuredTheme = configuredThemePacks["lazycloud_dark"]
	configuredFollowTerminal = false

	a := settingsTestApp(t)
	var themeRow settingsRow
	for _, row := range a.settingsRows() {
		if strings.HasPrefix(row.label, "Theme: ") {
			themeRow = row
		}
	}
	if themeRow.set == nil {
		t.Fatal("Theme row missing")
	}

	themeRow.set(true)
	if configuredThemeName != customThemeName {
		t.Fatalf("first cycle selected %q, want %q", configuredThemeName, customThemeName)
	}
	if configuredFollowTerminal {
		t.Error("custom selection must not turn follow terminal on")
	}
	if !themeColorsEditable() {
		t.Error("custom selection must show the color rows")
	}
	if got := configuredThemePacks[customThemeName]; got.Background != "black" || got.Labels != "yellow" {
		t.Errorf("custom pack seeded from %+v, want the previously painted palette", got)
	}

	themeRow.set(true)
	if !configuredFollowTerminal {
		t.Fatal("second cycle did not select follow terminal")
	}
	if themeColorsEditable() {
		t.Error("follow terminal must hide the color rows")
	}
	if theme.background != "default" || theme.labels != "yellow" {
		t.Errorf("follow palette = %q/%q, want the terminal-follow palette (default/yellow)", theme.background, theme.labels)
	}

	themeRow.set(true)
	if configuredFollowTerminal {
		t.Error("third cycle did not leave follow terminal")
	}
	if configuredThemeName != "lazycloud_dark" {
		t.Fatalf("third cycle selected %q, want the wrap back to lazycloud_dark", configuredThemeName)
	}
	if themeColorsEditable() {
		t.Error("a config pack must keep the color rows hidden")
	}
	if theme.labels != "yellow" {
		t.Errorf("pack palette labels = %q, want the lazycloud_dark yellow", theme.labels)
	}
}

// TestCycleThemeNameSwitchesAndSavesPacks pins the pack row: cycling moves
// the selection through the packs in name order, previews the new pack's
// palette, keeps edits made to a pack before switching away, and saving
// writes the selected pack plus the edits parked in the other packs.
func TestCycleThemeNameSwitchesAndSavesPacks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	snapshotSettingsGlobals(t)
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Background: "black", Labels: "yellow"}
	configuredFollowTerminal = false
	configuredThemePacks = map[string]providers.Theme{
		"lazycloud_dark": configuredTheme,
		"ocean":          {Background: "navy", Labels: "aqua"},
	}
	configuredThemeName = "lazycloud_dark"

	a.cycleThemeName(1)
	if configuredThemeName != "ocean" {
		t.Fatalf("cycleThemeName(1) selected %q, want ocean", configuredThemeName)
	}
	if configuredTheme.Labels != "aqua" {
		t.Fatalf("cycled labels = %q, want the ocean pack's aqua", configuredTheme.Labels)
	}
	if theme.labels != "aqua" {
		t.Fatalf("preview labels = %q, want the ocean pack painted live", theme.labels)
	}

	// Past the packs the cycle reaches custom, seeded from the palette
	// that painted before it, and then follow terminal.
	a.cycleThemeName(1)
	if configuredThemeName != customThemeName {
		t.Fatalf("second cycle selected %q, want custom", configuredThemeName)
	}
	if !themeColorsEditable() {
		t.Error("custom selection must show the color rows")
	}
	if got := configuredThemePacks[customThemeName]; got.Labels != "aqua" {
		t.Fatalf("custom pack = %+v, want seeded from the ocean palette", got)
	}
	a.cycleThemeName(1)
	if !configuredFollowTerminal {
		t.Fatal("third cycle did not select follow terminal")
	}

	// Past follow terminal the cycle wraps back to the first pack.
	a.cycleThemeName(1)
	if configuredFollowTerminal {
		t.Fatal("the wrap did not leave follow terminal")
	}
	if configuredThemeName != "lazycloud_dark" {
		t.Fatalf("wrap selected %q, want lazycloud_dark", configuredThemeName)
	}

	// Edits made to the lazycloud_dark pack before switching away survive
	// the switch and the save.
	configuredTheme.Labels = "teal"
	a.cycleThemeName(1) // lazycloud_dark -> ocean
	a.applySettings()
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig(), defaultInfoConfig())
	if err != nil {
		t.Fatal(err)
	}
	if config.ThemeName != "ocean" {
		t.Fatalf("saved themeName = %q, want ocean", config.ThemeName)
	}
	if config.Theme.Labels != "aqua" {
		t.Fatalf("saved mirror labels = %q, want the ocean pack mirrored", config.Theme.Labels)
	}
	if got := config.Themes["lazycloud_dark"].Labels; got != "teal" {
		t.Fatalf("saved lazycloud_dark labels = %q, want the parked teal edit", got)
	}
	if got := config.Themes["ocean"].Labels; got != "aqua" {
		t.Fatalf("saved ocean labels = %q, want aqua kept", got)
	}
}

func TestRebuildSettingsFormReflectsLiveValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	snapshotSettingsGlobals(t)
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Background: "black", Labels: "orange"}
	configuredFollowTerminal = false
	configuredThemeName = customThemeName
	configuredThemePacks = map[string]providers.Theme{customThemeName: configuredTheme}
	a.buildSettingsPage()
	if a.settingsForm == nil {
		t.Fatal("buildSettingsPage did not create the form")
	}
	infobarEnabled = false
	a.rebuildSettingsForm()
	want := len(a.settingsRows()) + len(themeColorKeys)
	if got := a.settingsForm.GetFormItemCount(); got != want {
		t.Errorf("form has %d items, want %d (toggles + color fields)", got, want)
	}
	path, err := providers.ConfigFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, filepath.Join(home, ".config", "lazycloud")) {
		t.Errorf("config path %q not under test home", path)
	}
}

// TestShortcutBarToggleResizesInfobar pins the panel wiring, not just the
// flag: the Shortcut bar checkbox must run setInfobarEnabled so the status
// row resizes live — assigning the var alone leaves the layout untouched,
// and the later applySettings save becomes a no-op because the flag is
// already equal.
func TestShortcutBarToggleResizesInfobar(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	originalInfobar := infobarEnabled
	t.Cleanup(func() { infobarEnabled = originalInfobar })
	a := settingsTestApp(t)
	a.cmd = tview.NewInputField()
	originalMode := appState.CmdMode
	appState.CmdMode = modeCmd
	t.Cleanup(func() { appState.CmdMode = originalMode })
	a.infobar = tview.NewTextView()
	a.rootView = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tview.NewBox(), 9, 0, false).
		AddItem(a.infobar, 1, 0, false)
	infobarEnabled = true

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 10)
	a.rootView.SetRect(0, 0, 80, 10)
	drawInfobar := func() int {
		a.rootView.Draw(screen)
		_, _, _, height := a.infobar.GetRect()
		return height
	}

	var shortcutBar settingsRow
	for _, row := range a.settingsRows() {
		if row.label == "Shortcut bar" {
			shortcutBar = row
		}
	}
	if shortcutBar.set == nil {
		t.Fatal("settings panel is missing the Shortcut bar row")
	}

	shortcutBar.set(false)
	if infobarEnabled {
		t.Error("toggle did not clear infobarEnabled")
	}
	if height := drawInfobar(); height != 0 {
		t.Errorf("infobar height after disabling = %d, want 0", height)
	}

	shortcutBar.set(true)
	if !infobarEnabled {
		t.Error("toggle did not set infobarEnabled")
	}
	if height := drawInfobar(); height != 1 {
		t.Errorf("infobar height after enabling = %d, want 1", height)
	}
}

func TestApplySettingsPersistsToggles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// applySettings repaints the global palette; restore it so later
	// theme tests see the palette they expect.
	originalTheme := theme
	originalFollow := configuredFollowTerminal
	t.Cleanup(func() {
		theme = originalTheme
		configuredFollowTerminal = originalFollow
	})
	a := settingsTestApp(t)
	a.staleAutoRefresh = true
	infobarEnabled = false
	configuredFollowTerminal = true
	configuredTheme = providers.Theme{Background: "black"}
	a.applySettings()
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig(), defaultInfoConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !config.FollowTerminal {
		t.Error("follow terminal toggle not persisted as enabled")
	}
	if config.Infobar.ShowShortcuts {
		t.Error("infobar toggle not persisted as disabled")
	}
	if !config.AutoUpdate.Enabled {
		t.Error("auto update toggle not persisted as enabled")
	}
}

func TestSettingsColorFieldsEditPackPalette(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// applySettings repaints the global palette; restore it so later
	// theme tests see the palette they expect.
	snapshotSettingsGlobals(t)
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Background: "black"}
	configuredFollowTerminal = false
	configuredThemeName = customThemeName
	configuredThemePacks = map[string]providers.Theme{
		configuredThemeName: configuredTheme,
	}
	a.buildSettingsPage()

	// Find the Labels input field and type into it the way the form does.
	// Labels carry a focus prefix, so match the recorded base label.
	var field *tview.InputField
	for i := 0; i < a.settingsForm.GetFormItemCount(); i++ {
		item := a.settingsForm.GetFormItem(i)
		if pf, ok := item.(*colorPreviewField); ok && pf.baseLabel == "Labels" {
			field = pf.InputField
			break
		}
	}
	if field == nil {
		t.Fatal("settings form has no Labels color field")
	}
	field.SetText("orange")
	if configuredTheme.Labels != "orange" {
		t.Errorf("palette labels = %q, want orange after editing the field", configuredTheme.Labels)
	}

	// Saving persists the edited palette into the pack.
	a.applySettings()
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig(), defaultInfoConfig())
	if err != nil {
		t.Fatal(err)
	}
	if got := config.Themes[configuredThemeName].Labels; got != "orange" {
		t.Errorf("saved pack labels = %q, want orange", got)
	}
}

// TestCloseSettingsPersistsColorEdits pins the Close path: color edits live
// in memory while the panel is open, so closing it (Close button or Esc) must
// persist them like a checkbox toggle does.
func TestCloseSettingsPersistsColorEdits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// closeSettings repaints the global palette; restore it so later
	// theme tests see the palette they expect.
	snapshotSettingsGlobals(t)
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Background: "black"}
	configuredFollowTerminal = false
	configuredThemeName = customThemeName
	configuredThemePacks = map[string]providers.Theme{
		configuredThemeName: configuredTheme,
	}
	a.buildSettingsPage()

	// Edit a color field without toggling any checkbox.
	var field *tview.InputField
	for i := 0; i < a.settingsForm.GetFormItemCount(); i++ {
		if pf, ok := a.settingsForm.GetFormItem(i).(*colorPreviewField); ok && pf.baseLabel == "Labels" {
			field = pf.InputField
			break
		}
	}
	if field == nil {
		t.Fatal("settings form has no Labels color field")
	}
	field.SetText("teal")

	a.closeSettings()
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig(), defaultInfoConfig())
	if err != nil {
		t.Fatal(err)
	}
	if got := config.Themes[config.ThemeName].Labels; got != "teal" {
		t.Errorf("saved pack labels = %q, want teal after closing the panel", got)
	}
}
