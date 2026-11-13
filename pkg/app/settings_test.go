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
	for _, want := range []string{"Light theme", "Dark theme", "Theme follows terminal", "Shortcut bar", "Auto update cache"} {
		if !labels[want] {
			t.Errorf("settings panel is missing row %q", want)
		}
	}
}

func TestThemeModeTogglesAreExclusive(t *testing.T) {
	// The theme pack row previews its pack live, which repaints the global
	// palette; restore it so later theme tests see the palette they expect.
	originalTheme := theme
	t.Cleanup(func() { theme = originalTheme })
	setConfiguredThemeMode(providers.ModeLight)
	if configuredThemeMode() != providers.ModeLight {
		t.Fatal("light mode not stored")
	}
	rows := settingsTestApp(t).settingsRows()
	var dark, system bool
	for _, row := range rows {
		row.set(true)
		switch row.label {
		case "Dark theme":
			dark = configuredThemeMode() == providers.ModeDark
		case "Theme follows terminal":
			system = configuredThemeMode() == providers.ModeSystem
		}
	}
	if !dark || !system {
		t.Errorf("toggles did not set modes: dark=%v system=%v", dark, system)
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
	t.Cleanup(func() { theme = originalTheme })
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Mode: string(providers.ModeDark), Labels: "yellow"}
	configuredThemesDark = configuredTheme
	configuredThemesLight = providers.DefaultLightTheme()
	configuredThemePacks = map[string]providers.ThemeSpec{
		"default": {Dark: configuredTheme, Light: configuredThemesLight},
		"ocean":   {Dark: providers.Theme{Mode: string(providers.ModeDark), Labels: "aqua"}, Light: providers.DefaultLightTheme()},
	}
	configuredThemeName = "default"

	a.cycleThemeName(1)
	if configuredThemeName != "ocean" {
		t.Fatalf("cycleThemeName(1) selected %q, want ocean", configuredThemeName)
	}
	if configuredThemesDark.Labels != "aqua" {
		t.Fatalf("cycled dark labels = %q, want the ocean pack's aqua", configuredThemesDark.Labels)
	}
	if theme.labels != "aqua" {
		t.Fatalf("preview labels = %q, want the ocean pack painted live", theme.labels)
	}

	// Wrapping past the end returns to the first pack.
	a.cycleThemeName(1)
	if configuredThemeName != "default" {
		t.Fatalf("cycleThemeName(1) wrap selected %q, want default", configuredThemeName)
	}

	// Edits made to the default pack before switching away survive the
	// switch and the save.
	configuredThemesDark.Labels = "teal"
	a.cycleThemeName(1) // default -> ocean
	a.applySettings()
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig())
	if err != nil {
		t.Fatal(err)
	}
	if config.ThemeName != "ocean" {
		t.Fatalf("saved themeName = %q, want ocean", config.ThemeName)
	}
	if config.Theme.Labels != "aqua" {
		t.Fatalf("saved legacy labels = %q, want the ocean pack mirrored", config.Theme.Labels)
	}
	if got := config.Themes["default"].Dark.Labels; got != "teal" {
		t.Fatalf("saved default pack dark labels = %q, want the parked teal edit", got)
	}
	if got := config.Themes["ocean"].Dark.Labels; got != "aqua" {
		t.Fatalf("saved ocean pack dark labels = %q, want aqua kept", got)
	}
}

func TestRebuildSettingsFormReflectsLiveValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := settingsTestApp(t)
	a.buildSettingsPage()
	if a.settingsForm == nil {
		t.Fatal("buildSettingsPage did not create the form")
	}
	infobarEnabled = false
	configuredThemesDark = providers.Theme{Mode: "dark", Labels: "orange"}
	configuredThemesLight = providers.DefaultLightTheme()
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
	// applySettings swaps the global palette through applyConfigTheme;
	// restore it so later theme tests see the palette they expect.
	originalTheme := theme
	t.Cleanup(func() { theme = originalTheme })
	a := settingsTestApp(t)
	a.staleAutoRefresh = true
	infobarEnabled = false
	configuredTheme = providers.Theme{Mode: string(providers.ModeLight)}
	configuredThemesDark = providers.Theme{Mode: string(providers.ModeLight)}
	configuredThemesLight = providers.DefaultLightTheme()
	a.applySettings()
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig())
	if err != nil {
		t.Fatal(err)
	}
	if config.Theme.Mode != string(providers.ModeLight) {
		t.Errorf("theme mode = %q, want light", config.Theme.Mode)
	}
	if config.Infobar.ShowShortcuts {
		t.Error("infobar toggle not persisted as disabled")
	}
	if !config.AutoUpdate.Enabled {
		t.Error("auto update toggle not persisted as enabled")
	}
}

func TestActiveThemeSectionFollowsMode(t *testing.T) {
	configuredTheme = providers.Theme{Mode: string(providers.ModeDark)}
	configuredThemesDark = configuredTheme
	configuredThemesLight = providers.DefaultLightTheme()
	if activeThemeSection(providers.BackgroundDark) != &configuredThemesDark {
		t.Error("activeThemeSection(dark) did not return the dark section")
	}
	configuredTheme.Mode = string(providers.ModeLight)
	configuredThemesDark.Mode = string(providers.ModeLight)
	if activeThemeSection(providers.BackgroundDark) != &configuredThemesLight {
		t.Error("activeThemeSection(light) did not return the light section")
	}
	// System mode follows the terminal: dark terminal edits the dark
	// section, light terminal edits the light one the UI actually paints.
	configuredTheme.Mode = string(providers.ModeSystem)
	configuredThemesDark.Mode = string(providers.ModeSystem)
	if activeThemeSection(providers.BackgroundDark) != &configuredThemesDark {
		t.Error("activeThemeSection(system, dark terminal) did not return the dark section")
	}
	if activeThemeSection(providers.BackgroundLight) != &configuredThemesLight {
		t.Error("activeThemeSection(system, light terminal) did not return the light section")
	}
}

func TestSettingsColorFieldsEditActiveSection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// applySettings repaints the global palette through applyConfigTheme;
	// restore it so later theme tests see the palette they expect.
	originalTheme := theme
	t.Cleanup(func() { theme = originalTheme })
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Mode: string(providers.ModeLight)}
	configuredThemesDark = configuredTheme
	configuredThemesLight = providers.DefaultLightTheme()
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
	if configuredThemesLight.Labels != "orange" {
		t.Errorf("light section labels = %q, want orange after editing the field", configuredThemesLight.Labels)
	}
	if configuredThemesDark.Labels == "orange" {
		t.Error("dark section labels changed, want it untouched while editing light mode")
	}

	// Saving persists the edited section.
	a.applySettings()
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig())
	if err != nil {
		t.Fatal(err)
	}
	if config.ThemeLight.Labels != "orange" {
		t.Errorf("saved light labels = %q, want orange", config.ThemeLight.Labels)
	}
}

// TestCloseSettingsPersistsColorEdits pins the Close path: color edits live
// in memory while the panel is open, so closing it (Close button or Esc) must
// persist them like a checkbox toggle does.
func TestCloseSettingsPersistsColorEdits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// closeSettings repaints the global palette through applyConfigTheme;
	// restore it so later theme tests see the palette they expect.
	originalTheme := theme
	t.Cleanup(func() { theme = originalTheme })
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Mode: string(providers.ModeDark)}
	configuredThemesDark = configuredTheme
	configuredThemesLight = providers.DefaultLightTheme()
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
	if config.Theme.Labels != "teal" {
		t.Errorf("saved dark labels = %q, want teal after closing the panel", config.Theme.Labels)
	}
}
