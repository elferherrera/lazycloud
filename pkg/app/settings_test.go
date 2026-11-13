package app

import (
	"path/filepath"
	"strings"
	"testing"

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
	for _, want := range []string{"Follow terminal", "Shortcut bar", "Auto update cache"} {
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
	for _, gone := range []string{"Light theme", "Dark theme", "Theme follows terminal"} {
		if labels[gone] {
			t.Errorf("settings panel still carries removed row %q", gone)
		}
	}
}

// TestFollowTerminalToggleDrivesPalette pins the toggle: on paints with the
// terminal palette, off returns to the selected pack.
func TestFollowTerminalToggleDrivesPalette(t *testing.T) {
	originalTheme := theme
	originalFollow := configuredFollowTerminal
	t.Cleanup(func() {
		theme = originalTheme
		configuredFollowTerminal = originalFollow
	})
	configuredTheme = providers.Theme{Background: "#272822", Labels: "#a6e22e"}
	configuredFollowTerminal = false

	rows := settingsTestApp(t).settingsRows()
	var followRow settingsRow
	for _, row := range rows {
		if row.label == "Follow terminal" {
			followRow = row
		}
	}
	if followRow.set == nil {
		t.Fatal("Follow terminal row missing")
	}

	followRow.set(true)
	if !configuredFollowTerminal {
		t.Error("toggle did not store follow-terminal mode")
	}
	if theme.background != "default" || theme.labels != "yellow" {
		t.Errorf("toggle-on palette = %q/%q, want the terminal-follow palette (default/yellow)", theme.background, theme.labels)
	}

	followRow.set(false)
	if configuredFollowTerminal {
		t.Error("toggle did not clear follow-terminal mode")
	}
	if theme.background != "#272822" || theme.labels != "#a6e22e" {
		t.Errorf("toggle-off palette = %q/%q, want the selected pack back", theme.background, theme.labels)
	}
}

// TestCycleThemeNameSwitchesAndSavesPacks pins the pack row: cycling moves
// the selection through the packs in name order, previews the new pack's
// palette, keeps edits made to a pack before switching away, and saving
// writes the selected pack plus the edits parked in the other packs.
func TestCycleThemeNameSwitchesAndSavesPacks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	originalTheme := theme
	originalFollow := configuredFollowTerminal
	t.Cleanup(func() {
		theme = originalTheme
		configuredFollowTerminal = originalFollow
	})
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

	// Wrapping past the end returns to the first pack.
	a.cycleThemeName(1)
	if configuredThemeName != "lazycloud_dark" {
		t.Fatalf("cycleThemeName(1) wrap selected %q, want lazycloud_dark", configuredThemeName)
	}

	// Edits made to the lazycloud_dark pack before switching away survive
	// the switch and the save.
	configuredTheme.Labels = "teal"
	a.cycleThemeName(1) // lazycloud_dark -> ocean
	a.applySettings()
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig())
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
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Background: "black", Labels: "orange"}
	configuredFollowTerminal = false
	t.Cleanup(func() { configuredFollowTerminal = false })
	a.buildSettingsPage()
	if a.settingsForm == nil {
		t.Fatal("buildSettingsPage did not create the form")
	}
	infobarEnabled = false
	a.rebuildSettingsForm()
	want := len(a.settingsRows()) + len(themeColorKeys) + 2
	if got := a.settingsForm.GetFormItemCount(); got != want {
		t.Errorf("form has %d items, want %d (toggles + save-as + color fields + config path)", got, want)
	}
	path, err := providers.ConfigFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, filepath.Join(home, ".config", "lazycloud")) {
		t.Errorf("config path %q not under test home", path)
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
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig())
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
	originalTheme := theme
	originalFollow := configuredFollowTerminal
	t.Cleanup(func() {
		theme = originalTheme
		configuredFollowTerminal = originalFollow
	})
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Background: "black"}
	configuredFollowTerminal = false
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
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig())
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
	originalTheme := theme
	originalFollow := configuredFollowTerminal
	t.Cleanup(func() {
		theme = originalTheme
		configuredFollowTerminal = originalFollow
	})
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Background: "black"}
	configuredFollowTerminal = false
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
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig())
	if err != nil {
		t.Fatal(err)
	}
	if got := config.Themes[config.ThemeName].Labels; got != "teal" {
		t.Errorf("saved pack labels = %q, want teal after closing the panel", got)
	}
}
