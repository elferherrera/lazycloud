package providers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func testConfigDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return filepath.Join(home, ".config", "lazycloud")
}

func writeLegacyFiles(t *testing.T, dir string, theme *Theme, shortcuts map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if theme != nil {
		data, err := json.Marshal(theme)
		if err != nil {
			t.Fatalf("marshalling legacy theme: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "theme.json"), data, 0o644); err != nil {
			t.Fatalf("writing legacy theme file: %v", err)
		}
	}
	if shortcuts != nil {
		data, err := json.Marshal(shortcuts)
		if err != nil {
			t.Fatalf("marshalling legacy shortcuts: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "shortcuts.json"), data, 0o644); err != nil {
			t.Fatalf("writing legacy shortcuts file: %v", err)
		}
	}
}

func readConfigFile(t *testing.T, dir string) Config {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("unmarshalling config file: %v", err)
	}
	return config
}

func TestConfigDefaultsOnFirstRun(t *testing.T) {
	dir := testConfigDir(t)

	defaultTheme := Theme{Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", Focus: "yellow", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", SettingsLabels: "yellow", SettingsText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	defaultShortcuts := map[string]string{"1": "vms", "2": "buckets", "0": "firewalls"}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}
	defaultInfobar := Infobar{ShowShortcuts: true}

	got, err := LoadConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate, defaultInfobar)
	if err != nil {
		t.Fatalf("LoadConfig() on missing file error = %v", err)
	}
	if got.Theme != defaultTheme {
		t.Fatalf("LoadConfig() theme = %+v, want defaults %+v", got.Theme, defaultTheme)
	}
	if !reflect.DeepEqual(got.Shortcuts, defaultShortcuts) {
		t.Fatalf("LoadConfig() shortcuts = %v, want defaults %v", got.Shortcuts, defaultShortcuts)
	}
	if got.AutoUpdate != defaultAutoUpdate {
		t.Fatalf("LoadConfig() autoupdate = %+v, want defaults %+v", got.AutoUpdate, defaultAutoUpdate)
	}
	onDisk := readConfigFile(t, dir)
	if onDisk.Theme != defaultTheme {
		t.Fatalf("config file theme = %+v, want defaults %+v", onDisk.Theme, defaultTheme)
	}
	if !reflect.DeepEqual(onDisk.Shortcuts, defaultShortcuts) {
		t.Fatalf("config file shortcuts = %v, want defaults %v", onDisk.Shortcuts, defaultShortcuts)
	}
	if onDisk.AutoUpdate != defaultAutoUpdate {
		t.Fatalf("config file autoupdate = %+v, want defaults %+v", onDisk.AutoUpdate, defaultAutoUpdate)
	}
}

func TestConfigMigratesLegacyFiles(t *testing.T) {
	dir := testConfigDir(t)
	legacyTheme := Theme{Labels: "aqua", Text: "gray", Borders: "green", Title: "orange"}
	legacyShortcuts := map[string]string{"1": "clusters", "5": "ips"}
	writeLegacyFiles(t, dir, &legacyTheme, legacyShortcuts)

	defaultTheme := Theme{Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", Focus: "yellow", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", SettingsLabels: "yellow", SettingsText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	defaultShortcuts := map[string]string{"1": "vms", "2": "buckets", "0": "firewalls"}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}
	defaultInfobar := Infobar{ShowShortcuts: true}

	got, err := LoadConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate, defaultInfobar)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := Theme{Labels: "aqua", Text: "gray", Borders: "green", Title: "orange", ShortcutLabels: "aqua", ShortcutText: "gray", Focus: "aqua", StatusLabels: "yellow", StatusText: "gray", SummaryLabels: "aqua", SummaryText: "gray", DetailLabels: "yellow", DetailText: "gray", SettingsLabels: "yellow", SettingsText: "gray", ActiveRowText: "black", ActiveRowBackground: "gray"}
	if got.Theme != want {
		t.Fatalf("LoadConfig() theme = %+v, want migrated %+v", got.Theme, want)
	}
	if !reflect.DeepEqual(got.Shortcuts, legacyShortcuts) {
		t.Fatalf("LoadConfig() shortcuts = %v, want legacy %v", got.Shortcuts, legacyShortcuts)
	}
	if got.AutoUpdate != defaultAutoUpdate {
		t.Fatalf("LoadConfig() autoupdate = %+v, want defaults %+v", got.AutoUpdate, defaultAutoUpdate)
	}
	onDisk := readConfigFile(t, dir)
	if onDisk.Theme != want {
		t.Fatalf("config file theme = %+v, want migrated %+v", onDisk.Theme, want)
	}
	if !reflect.DeepEqual(onDisk.Shortcuts, legacyShortcuts) {
		t.Fatalf("config file shortcuts = %v, want legacy %v", onDisk.Shortcuts, legacyShortcuts)
	}
	if _, err := os.Stat(filepath.Join(dir, "theme.json")); err != nil {
		t.Fatalf("legacy theme file should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "shortcuts.json")); err != nil {
		t.Fatalf("legacy shortcuts file should remain: %v", err)
	}
}

func TestConfigRoundtrip(t *testing.T) {
	testConfigDir(t)

	custom := Config{
		Theme:      Theme{Labels: "aqua", Text: "gray", Borders: "green", Title: "orange", ShortcutLabels: "red", ShortcutText: "silver", Focus: "red", StatusLabels: "teal", StatusText: "olive", SummaryLabels: "maroon", SummaryText: "navy", DetailLabels: "purple", DetailText: "pink", SettingsText: "gray", ActiveRowText: "lime", ActiveRowBackground: "teal"},
		Shortcuts:  map[string]string{"1": "clusters", "5": "ips"},
		AutoUpdate: AutoUpdate{Enabled: false, ThresholdInMinutes: 30},
		Infobar:    Infobar{ShowShortcuts: false},
	}
	if err := SaveConfig(custom); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	got, err := LoadConfig(Theme{}, nil, AutoUpdate{}, Infobar{})
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got.Theme != custom.Theme {
		t.Fatalf("LoadConfig() theme = %+v, want saved %+v", got.Theme, custom.Theme)
	}
	if !reflect.DeepEqual(got.Shortcuts, custom.Shortcuts) {
		t.Fatalf("LoadConfig() shortcuts = %v, want saved %v", got.Shortcuts, custom.Shortcuts)
	}
	if got.AutoUpdate != custom.AutoUpdate {
		t.Fatalf("LoadConfig() autoupdate = %+v, want saved %+v", got.AutoUpdate, custom.AutoUpdate)
	}
	if got.Infobar != custom.Infobar {
		t.Fatalf("LoadConfig() infobar = %+v, want saved %+v", got.Infobar, custom.Infobar)
	}
}

func TestConfigMergesMissingThemeKeys(t *testing.T) {
	dir := testConfigDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	partial := "theme:\n  labels: aqua\n  borders: green\nshortcuts:\n  \"1\": vms\n  2: buckets\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(partial), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	defaultTheme := Theme{Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", Focus: "yellow", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", SettingsLabels: "yellow", SettingsText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	defaultShortcuts := map[string]string{"1": "vms", "2": "buckets", "0": "firewalls"}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}
	defaultInfobar := Infobar{ShowShortcuts: true}

	got, err := LoadConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate, defaultInfobar)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := Theme{Labels: "aqua", Text: "white", Borders: "green", Title: "white", ShortcutLabels: "aqua", ShortcutText: "white", Focus: "aqua", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "aqua", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", SettingsLabels: "yellow", SettingsText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	if got.Theme != want {
		t.Fatalf("LoadConfig() theme = %+v, want merged %+v", got.Theme, want)
	}
	wantShortcuts := map[string]string{"1": "vms", "2": "buckets"}
	if !reflect.DeepEqual(got.Shortcuts, wantShortcuts) {
		t.Fatalf("LoadConfig() shortcuts = %v, want %v", got.Shortcuts, wantShortcuts)
	}
	if got.AutoUpdate != defaultAutoUpdate {
		t.Fatalf("LoadConfig() autoupdate = %+v, want defaults %+v", got.AutoUpdate, defaultAutoUpdate)
	}
	onDisk := readConfigFile(t, dir)
	if onDisk.Theme != want {
		t.Fatalf("config file theme = %+v, want merged %+v", onDisk.Theme, want)
	}
	if onDisk.AutoUpdate != defaultAutoUpdate {
		t.Fatalf("config file autoupdate = %+v, want defaults %+v", onDisk.AutoUpdate, defaultAutoUpdate)
	}
}

func TestConfigPreservesExplicitAutoUpdate(t *testing.T) {
	dir := testConfigDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	existing := "theme:\n  labels: aqua\nshortcuts:\n  1: vms\nautoupdate:\n  enabled: false\n  threshold_in_minutes: 15\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(existing), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	defaultTheme := Theme{}
	defaultShortcuts := map[string]string{}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}
	defaultInfobar := Infobar{ShowShortcuts: true}

	got, err := LoadConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate, defaultInfobar)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := AutoUpdate{Enabled: false, ThresholdInMinutes: 15}
	if got.AutoUpdate != want {
		t.Fatalf("LoadConfig() autoupdate = %+v, want preserved %+v", got.AutoUpdate, want)
	}
	onDisk := readConfigFile(t, dir)
	if onDisk.AutoUpdate != want {
		t.Fatalf("config file autoupdate = %+v, want preserved %+v", onDisk.AutoUpdate, want)
	}
}

func TestConfigThemeSectionKeepsValues(t *testing.T) {
	testConfigDir(t)
	custom := Config{Theme: Theme{Labels: "aqua"}}
	if err := SaveConfig(custom); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	got, err := LoadConfig(Theme{}, nil, AutoUpdate{}, Infobar{})
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got.Theme.Labels != "aqua" {
		t.Fatalf("LoadConfig() labels = %q, want aqua kept", got.Theme.Labels)
	}
}

func TestConfigThemeMirrorFollowsSelection(t *testing.T) {
	dir := testConfigDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("theme:\n  labels: aqua\n"), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}
	got, err := LoadConfig(Theme{Background: "black", Labels: "yellow"}, nil, AutoUpdate{}, Infobar{})
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got.Theme.Labels != "aqua" {
		t.Fatalf("LoadConfig() labels = %q, want aqua kept", got.Theme.Labels)
	}
	pack, ok := got.Themes[got.ThemeName]
	if !ok {
		t.Fatalf("LoadConfig() selected pack %q missing", got.ThemeName)
	}
	if pack.Labels != "aqua" {
		t.Fatalf("selected pack labels = %q, want aqua from the theme section", pack.Labels)
	}
}

func TestMergeThemeFillsEveryEmptyKey(t *testing.T) {
	defaults := Theme{Background: "black", Labels: "yellow", Text: "white"}
	merged := MergeTheme(Theme{}, defaults)
	if merged.Labels != "yellow" {
		t.Errorf("MergeTheme() labels = %q, want default yellow", merged.Labels)
	}
	if merged.Background != "black" {
		t.Errorf("MergeTheme() background = %q, want default black", merged.Background)
	}
	kept := MergeTheme(Theme{Background: "white"}, defaults)
	if kept.Background != "white" {
		t.Errorf("MergeTheme() background = %q, want configured white kept", kept.Background)
	}
}

func TestThemePathRejectsMissingHome(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := themePath(); err == nil {
		t.Error("themePath() with no home = nil error, want an error")
	}
}

func TestConfigPreservesExplicitInfobar(t *testing.T) {
	dir := testConfigDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	existing := "theme:\n  labels: aqua\nshortcuts:\n  1: vms\ninfobar:\n  show_shortcuts: false\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(existing), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	defaultTheme := Theme{}
	defaultShortcuts := map[string]string{}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}
	defaultInfobar := Infobar{ShowShortcuts: true, SortOrder: InfobarSortCurrent}

	got, err := LoadConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate, defaultInfobar)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := Infobar{ShowShortcuts: false, SortOrder: InfobarSortCurrent}
	if got.Infobar != want {
		t.Fatalf("LoadConfig() infobar = %+v, want preserved %+v", got.Infobar, want)
	}
	onDisk := readConfigFile(t, dir)
	if onDisk.Infobar != want {
		t.Fatalf("config file infobar = %+v, want preserved %+v", onDisk.Infobar, want)
	}
}

func TestConfigPreservesExplicitInfobarSortOrder(t *testing.T) {
	dir := testConfigDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	existing := "theme:\n  labels: aqua\nshortcuts:\n  1: vms\ninfobar:\n  show_shortcuts: true\n  sort_order: alphabetical\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(existing), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	defaultTheme := Theme{}
	defaultShortcuts := map[string]string{}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}
	defaultInfobar := Infobar{ShowShortcuts: true, SortOrder: InfobarSortCurrent}

	got, err := LoadConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate, defaultInfobar)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := Infobar{ShowShortcuts: true, SortOrder: InfobarSortAlpha}
	if got.Infobar != want {
		t.Fatalf("LoadConfig() infobar = %+v, want preserved %+v", got.Infobar, want)
	}
	onDisk := readConfigFile(t, dir)
	if onDisk.Infobar != want {
		t.Fatalf("config file infobar = %+v, want preserved %+v", onDisk.Infobar, want)
	}
}

// TestConfigThemeLightSectionBecomesPack pins the legacy themeLight
// migration: an old light-mode config painted with themeLight, so that
// section becomes the palette of the house pack.
func TestConfigThemeLightSectionBecomesPack(t *testing.T) {
	dir := testConfigDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	content := "theme:\n  mode: light\n  labels: blue\nthemeLight:\n  labels: orange\n  background: white\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}
	got, err := LoadConfig(Theme{Background: "black"}, nil, AutoUpdate{}, Infobar{})
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	pack, ok := got.Themes[DefaultThemeName]
	if !ok {
		t.Fatalf("LoadConfig() themes missing the %q pack: %+v", DefaultThemeName, got.Themes)
	}
	if pack.Labels != "orange" || pack.Background != "white" {
		t.Fatalf("default pack = %+v, want the themeLight section (orange on white)", pack)
	}
	if got.FollowTerminal {
		t.Fatal("LoadConfig() followTerminal = true, want false for mode light")
	}
	onDisk := readConfigFile(t, dir)
	if onDisk.Themes[DefaultThemeName].Labels != "orange" {
		t.Fatalf("config file pack labels = %q, want orange persisted", onDisk.Themes[DefaultThemeName].Labels)
	}
	if onDisk.FollowTerminal {
		t.Fatal("config file followTerminal = true, want false")
	}
}

// TestConfigThemeModeSystemMigratesToFollow pins the followTerminal
// migration: theme.mode system asked the app to follow the terminal, so
// the load keeps that intent instead of pinning a palette.
func TestConfigThemeModeSystemMigratesToFollow(t *testing.T) {
	dir := testConfigDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	content := "theme:\n  mode: system\n  labels: aqua\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}
	got, err := LoadConfig(Theme{Background: "black", Labels: "yellow"}, nil, AutoUpdate{}, Infobar{})
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !got.FollowTerminal {
		t.Fatal("LoadConfig() followTerminal = false, want true from theme.mode system")
	}
	if got.Theme.Labels != "aqua" {
		t.Fatalf("LoadConfig() labels = %q, want aqua kept", got.Theme.Labels)
	}
	onDisk := readConfigFile(t, dir)
	if !onDisk.FollowTerminal {
		t.Fatal("config file followTerminal = false, want the migration persisted")
	}
}

// TestConfigThemesMapMigration pins the pack migration: a file with only
// the legacy sections gets them wrapped into a default pack, old
// two-section packs collapse to the palette their mode selected, and a
// selection pointing at a missing pack falls back to the default pack.
func TestConfigThemesMapMigration(t *testing.T) {
	defaultTheme := Theme{Background: "black", Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", Focus: "yellow", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", SettingsLabels: "yellow", SettingsText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}
	defaultInfobar := Infobar{ShowShortcuts: true}

	t.Run("legacy sections become the default pack", func(t *testing.T) {
		dir := testConfigDir(t)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating config dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("theme:\n  labels: aqua\n"), 0o644); err != nil {
			t.Fatalf("writing config file: %v", err)
		}
		got, err := LoadConfig(defaultTheme, nil, defaultAutoUpdate, defaultInfobar)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		pack, ok := got.Themes[DefaultThemeName]
		if !ok {
			t.Fatalf("LoadConfig() themes missing the %q pack: %+v", DefaultThemeName, got.Themes)
		}
		if len(got.Themes) != len(BundledThemes()) {
			t.Fatalf("LoadConfig() themes has %d packs, want the house palette under its bundle name plus the other %d bundles", len(got.Themes), len(BundledThemes())-1)
		}
		if pack.Labels != "aqua" || pack.Text != defaultTheme.Text {
			t.Fatalf("default pack = %+v, want explicit labels with filled text", pack)
		}
		if got.ThemeName != DefaultThemeName {
			t.Fatalf("LoadConfig() themeName = %q, want %q", got.ThemeName, DefaultThemeName)
		}
		onDisk := readConfigFile(t, dir)
		if onDisk.Themes[DefaultThemeName].Labels != "aqua" {
			t.Fatalf("config file default pack = %+v, want aqua persisted", onDisk.Themes[DefaultThemeName])
		}
	})

	t.Run("two-section packs collapse to the mode's palette", func(t *testing.T) {
		dir := testConfigDir(t)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating config dir: %v", err)
		}
		content := "themeName: ocean\nthemes:\n  ocean:\n    dark:\n      mode: light\n      labels: aqua\n    light:\n      labels: teal\n  forest:\n    dark:\n      labels: olive\n  emptypack: {}\n"
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
			t.Fatalf("writing config file: %v", err)
		}
		got, err := LoadConfig(defaultTheme, nil, defaultAutoUpdate, defaultInfobar)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if len(got.Themes) != 2+len(BundledThemes()) {
			t.Fatalf("LoadConfig() themes has %d packs, want ocean, forest and the %d bundles", len(got.Themes), len(BundledThemes()))
		}
		ocean := got.Themes["ocean"]
		if ocean.Labels != "teal" || ocean.Text != defaultTheme.Text {
			t.Fatalf("ocean = %+v, want the light section (teal) because mode was light, with filled text", ocean)
		}
		forest := got.Themes["forest"]
		if forest.Labels != "olive" {
			t.Fatalf("forest = %+v, want the dark section (olive) kept", forest)
		}
		if _, ok := got.Themes["emptypack"]; ok {
			t.Fatal("LoadConfig() kept an empty legacy pack, want it dropped")
		}
		if got.ThemeName != "ocean" {
			t.Fatalf("LoadConfig() themeName = %q, want ocean kept", got.ThemeName)
		}
		// The collapse must persist: a partial pack on disk comes back
		// complete on the next load without rewriting it again.
		onDisk := readConfigFile(t, dir)
		if onDisk.Themes["ocean"].Text != defaultTheme.Text {
			t.Fatalf("config file ocean text = %q, want filled-in value persisted", onDisk.Themes["ocean"].Text)
		}
	})

	t.Run("flat packs keep their palette", func(t *testing.T) {
		dir := testConfigDir(t)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating config dir: %v", err)
		}
		content := "themeName: ocean\nthemes:\n  ocean:\n    labels: aqua\n"
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
			t.Fatalf("writing config file: %v", err)
		}
		got, err := LoadConfig(defaultTheme, nil, defaultAutoUpdate, defaultInfobar)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		ocean := got.Themes["ocean"]
		if ocean.Labels != "aqua" || ocean.Text != defaultTheme.Text {
			t.Fatalf("ocean = %+v, want the flat palette with filled text", ocean)
		}
	})

	t.Run("missing selection falls back to the default pack", func(t *testing.T) {
		dir := testConfigDir(t)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating config dir: %v", err)
		}
		content := "themeName: no-such-pack\nthemes:\n  ocean:\n    dark:\n      labels: aqua\n"
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
			t.Fatalf("writing config file: %v", err)
		}
		got, err := LoadConfig(defaultTheme, nil, defaultAutoUpdate, defaultInfobar)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if got.ThemeName != DefaultThemeName {
			t.Fatalf("LoadConfig() themeName = %q, want fallback to %q", got.ThemeName, DefaultThemeName)
		}
	})
}

// TestConfigSeedsBundledThemesOnce pins the bundle seeding: a fresh or
// upgraded config gains the shipped palettes plus the bundledThemes flag,
// a user pack under a bundle name keeps its keys, and the flag stops the
// seeding from running again so a deleted bundle stays deleted.
func TestConfigSeedsBundledThemesOnce(t *testing.T) {
	defaultTheme := Theme{Background: "black", Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", Focus: "yellow", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", SettingsLabels: "yellow", SettingsText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}
	defaultInfobar := Infobar{ShowShortcuts: true}

	t.Run("upgraded config gains bundles and the flag", func(t *testing.T) {
		dir := testConfigDir(t)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating config dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("theme:\n  labels: aqua\n"), 0o644); err != nil {
			t.Fatalf("writing config file: %v", err)
		}
		got, err := LoadConfig(defaultTheme, nil, defaultAutoUpdate, defaultInfobar)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if !got.BundledThemes {
			t.Fatal("LoadConfig() did not set the bundledThemes flag")
		}
		monokai, ok := got.Themes["monokai"]
		if !ok {
			t.Fatalf("LoadConfig() themes missing the monokai bundle: %+v", got.Themes)
		}
		if monokai.Labels != "#a6e22e" || monokai.Background != "#272822" {
			t.Fatalf("monokai = %+v, want the bundled palette", monokai)
		}
		// The house packs seed alongside the scheme packs.
		if got.Themes["lazycloud_dark"].Background != "black" {
			t.Fatalf("lazycloud_dark background = %q, want the shipped dark palette", got.Themes["lazycloud_dark"].Background)
		}
		if got.Themes["lazycloud_light"].Background != "white" {
			t.Fatalf("lazycloud_light background = %q, want the shipped light palette", got.Themes["lazycloud_light"].Background)
		}
		// Gruvbox, solarized and one-dark ship their canonical dark
		// variants as single palettes now.
		if got.Themes["gruvbox"].Background != "#282828" {
			t.Fatalf("gruvbox background = %q, want the bundled dark variant", got.Themes["gruvbox"].Background)
		}
		if got.Themes["solarized"].Background != "#002b36" {
			t.Fatalf("solarized background = %q, want the bundled dark variant", got.Themes["solarized"].Background)
		}
		if got.Themes["one-dark"].Background != "#282c34" {
			t.Fatalf("one-dark background = %q, want the bundled dark variant", got.Themes["one-dark"].Background)
		}
		// A second load must not rewrite the file: the flag already says
		// the bundles are there.
		before, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := LoadConfig(defaultTheme, nil, defaultAutoUpdate, defaultInfobar); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatal("LoadConfig() rewrote the config on the second load despite the bundledThemes flag")
		}
	})

	t.Run("deleted bundle stays deleted once flagged", func(t *testing.T) {
		dir := testConfigDir(t)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating config dir: %v", err)
		}
		// Seed once, then hand-delete a bundle from the file.
		if _, err := LoadConfig(defaultTheme, nil, defaultAutoUpdate, defaultInfobar); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		pruned := strings.Replace(string(data), "  monokai:\n", "  monokai-gone:\n", 1)
		if pruned == string(data) {
			t.Fatal("test could not find the monokai pack to rename")
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(pruned), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := LoadConfig(defaultTheme, nil, defaultAutoUpdate, defaultInfobar)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got.Themes["monokai"]; ok {
			t.Fatal("LoadConfig() re-seeded the deleted monokai bundle despite the flag")
		}
		if _, ok := got.Themes["monokai-gone"]; !ok {
			t.Fatal("LoadConfig() lost the renamed pack")
		}
	})

	t.Run("user pack under a bundle name wins", func(t *testing.T) {
		dir := testConfigDir(t)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating config dir: %v", err)
		}
		content := "themes:\n  nord:\n    dark:\n      labels: lime\n"
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
			t.Fatalf("writing config file: %v", err)
		}
		got, err := LoadConfig(defaultTheme, nil, defaultAutoUpdate, defaultInfobar)
		if err != nil {
			t.Fatal(err)
		}
		nord := got.Themes["nord"]
		if nord.Labels != "lime" {
			t.Fatalf("nord labels = %q, want the user's lime kept over the bundle", nord.Labels)
		}
	})
}

func TestConfigRenamesPristineDefaultPack(t *testing.T) {
	defaultTheme := Theme{Background: "black", Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", Focus: "yellow", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", SettingsLabels: "yellow", SettingsText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}
	defaultInfobar := Infobar{ShowShortcuts: true}

	t.Run("pristine default pack renames to lazycloud_dark", func(t *testing.T) {
		dir := testConfigDir(t)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating config dir: %v", err)
		}
		pack := lazycloudDarkPalette()
		data, err := yaml.Marshal(map[string]map[string]Theme{
			"themes": {legacyDefaultPackName: pack},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := LoadConfig(defaultTheme, nil, defaultAutoUpdate, defaultInfobar)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got.Themes[legacyDefaultPackName]; ok {
			t.Fatal("LoadConfig() kept the legacy default pack name")
		}
		if _, ok := got.Themes[DefaultThemeName]; !ok {
			t.Fatalf("LoadConfig() lost the pack under %q", DefaultThemeName)
		}
		if got.ThemeName != DefaultThemeName {
			t.Fatalf("ThemeName = %q, want rewired to %q", got.ThemeName, DefaultThemeName)
		}
	})

	t.Run("user-edited lazycloud_dark keeps the legacy default pack", func(t *testing.T) {
		dir := testConfigDir(t)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating config dir: %v", err)
		}
		edited := lazycloudDarkPalette()
		edited.Labels = "lime"
		data, err := yaml.Marshal(map[string]map[string]Theme{
			"themes": {legacyDefaultPackName: lazycloudDarkPalette(), DefaultThemeName: edited},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := LoadConfig(defaultTheme, nil, defaultAutoUpdate, defaultInfobar)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got.Themes[legacyDefaultPackName]; !ok {
			t.Fatal("LoadConfig() deleted the legacy default pack although lazycloud_dark was user-edited")
		}
		if got.Themes[DefaultThemeName].Labels != "lime" {
			t.Fatalf("lazycloud_dark labels = %q, want the user's lime kept", got.Themes[DefaultThemeName].Labels)
		}
		if got.ThemeName != DefaultThemeName {
			t.Fatalf("ThemeName = %q, want lazycloud_dark kept as the selection", got.ThemeName)
		}
	})
}
