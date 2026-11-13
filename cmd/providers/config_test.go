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

	defaultTheme := Theme{Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", Focus: "yellow", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
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

	defaultTheme := Theme{Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", Focus: "yellow", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	defaultShortcuts := map[string]string{"1": "vms", "2": "buckets", "0": "firewalls"}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}
	defaultInfobar := Infobar{ShowShortcuts: true}

	got, err := LoadConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate, defaultInfobar)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := Theme{Labels: "aqua", Text: "gray", Borders: "green", Title: "orange", ShortcutLabels: "aqua", ShortcutText: "gray", Focus: "aqua", StatusLabels: "yellow", StatusText: "gray", SummaryLabels: "aqua", SummaryText: "gray", DetailLabels: "yellow", DetailText: "gray", ActiveRowText: "black", ActiveRowBackground: "gray"}
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
		Theme:      Theme{Labels: "aqua", Text: "gray", Borders: "green", Title: "orange", ShortcutLabels: "red", ShortcutText: "silver", Focus: "red", StatusLabels: "teal", StatusText: "olive", SummaryLabels: "maroon", SummaryText: "navy", DetailLabels: "purple", DetailText: "pink", ActiveRowText: "lime", ActiveRowBackground: "teal"},
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

	defaultTheme := Theme{Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", Focus: "yellow", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	defaultShortcuts := map[string]string{"1": "vms", "2": "buckets", "0": "firewalls"}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}
	defaultInfobar := Infobar{ShowShortcuts: true}

	got, err := LoadConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate, defaultInfobar)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := Theme{Labels: "aqua", Text: "white", Borders: "green", Title: "white", ShortcutLabels: "aqua", ShortcutText: "white", Focus: "aqua", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "aqua", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
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

func TestConfigThemeModeRoundtrip(t *testing.T) {
	testConfigDir(t)
	custom := Config{Theme: Theme{Mode: "light", Labels: "aqua"}}
	if err := SaveConfig(custom); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	got, err := LoadConfig(Theme{}, nil, AutoUpdate{}, Infobar{})
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got.Theme.Mode != "light" {
		t.Fatalf("LoadConfig() theme mode = %q, want light", got.Theme.Mode)
	}
}

func TestConfigThemeModeDefaultsToSystem(t *testing.T) {
	dir := testConfigDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("theme:\n  labels: aqua\n"), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}
	got, err := LoadConfig(Theme{Mode: "system"}, nil, AutoUpdate{}, Infobar{})
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got.Theme.Mode != "system" {
		t.Fatalf("LoadConfig() theme mode = %q, want system default", got.Theme.Mode)
	}
	if got.Theme.Labels != "aqua" {
		t.Fatalf("LoadConfig() labels = %q, want aqua kept", got.Theme.Labels)
	}
}

func TestMergeThemeFillsEveryEmptyKey(t *testing.T) {
	defaults := Theme{Mode: "system", Background: "black", Labels: "yellow", Text: "white"}
	merged := mergeTheme(Theme{}, defaults)
	if merged.Labels != "yellow" {
		t.Errorf("mergeTheme() labels = %q, want default yellow", merged.Labels)
	}
	if merged.Background != "black" {
		t.Errorf("mergeTheme() background = %q, want default black", merged.Background)
	}
	kept := mergeTheme(Theme{Background: "white"}, defaults)
	if kept.Background != "white" {
		t.Errorf("mergeTheme() background = %q, want configured white kept", kept.Background)
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

func TestConfigThemeLightSectionMergesAgainstLightPalette(t *testing.T) {
	dir := testConfigDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("themeLight:\n  labels: orange\n"), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}
	got, err := LoadConfig(Theme{Mode: "dark"}, nil, AutoUpdate{}, Infobar{})
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got.ThemeLight.Labels != "orange" {
		t.Fatalf("LoadConfig() light labels = %q, want configured orange kept", got.ThemeLight.Labels)
	}
	want := DefaultLightTheme()
	if got.ThemeLight.Background != want.Background || got.ThemeLight.Text != want.Text {
		t.Fatalf("LoadConfig() light section = %+v, want unset keys filled from the built-in light palette", got.ThemeLight)
	}
	onDisk := readConfigFile(t, dir)
	if onDisk.ThemeLight.Labels != "orange" {
		t.Fatalf("config file light labels = %q, want orange persisted", onDisk.ThemeLight.Labels)
	}
}

// TestConfigThemesMapMigration pins the pack migration: a file with only
// the legacy sections gets them wrapped into a default pack, while a file
// that already carries packs keeps them, gets their empty keys filled in
// and persisted, and falls back to the default pack when the selection
// points at a pack that does not exist.
func TestConfigThemesMapMigration(t *testing.T) {
	defaultTheme := Theme{Mode: "system", Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", Focus: "yellow", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
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
		spec, ok := got.Themes[DefaultThemeName]
		if !ok {
			t.Fatalf("LoadConfig() themes missing the %q pack: %+v", DefaultThemeName, got.Themes)
		}
		if len(got.Themes) != 1+len(BundledThemeSpecs()) {
			t.Fatalf("LoadConfig() themes has %d packs, want the default plus %d bundles", len(got.Themes), len(BundledThemeSpecs()))
		}
		if spec.Dark.Labels != "aqua" || spec.Dark.Text != defaultTheme.Text {
			t.Fatalf("default pack dark = %+v, want explicit labels with filled text", spec.Dark)
		}
		if got.ThemeName != DefaultThemeName {
			t.Fatalf("LoadConfig() themeName = %q, want %q", got.ThemeName, DefaultThemeName)
		}
		onDisk := readConfigFile(t, dir)
		if onDisk.Themes[DefaultThemeName].Dark.Labels != "aqua" {
			t.Fatalf("config file default pack dark = %+v, want aqua persisted", onDisk.Themes[DefaultThemeName].Dark)
		}
	})

	t.Run("existing packs are kept, filled in and persisted", func(t *testing.T) {
		dir := testConfigDir(t)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating config dir: %v", err)
		}
		content := "themeName: ocean\nthemes:\n  ocean:\n    dark:\n      labels: aqua\n    light:\n      labels: teal\n  forest:\n    dark:\n      labels: olive\n"
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
			t.Fatalf("writing config file: %v", err)
		}
		got, err := LoadConfig(defaultTheme, nil, defaultAutoUpdate, defaultInfobar)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if len(got.Themes) != 2+len(BundledThemeSpecs()) {
			t.Fatalf("LoadConfig() themes has %d packs, want ocean, forest and the %d bundles", len(got.Themes), len(BundledThemeSpecs()))
		}
		ocean := got.Themes["ocean"]
		if ocean.Dark.Labels != "aqua" || ocean.Dark.Text != defaultTheme.Text {
			t.Fatalf("ocean dark = %+v, want explicit labels with filled text", ocean.Dark)
		}
		if ocean.Light.Labels != "teal" || ocean.Light.Text != DefaultLightTheme().Text {
			t.Fatalf("ocean light = %+v, want explicit labels with light palette fill", ocean.Light)
		}
		if got.ThemeName != "ocean" {
			t.Fatalf("LoadConfig() themeName = %q, want ocean kept", got.ThemeName)
		}
		// The fill-in must persist: a partial pack on disk comes back
		// complete on the next load without rewriting it again.
		onDisk := readConfigFile(t, dir)
		if onDisk.Themes["ocean"].Dark.Text != defaultTheme.Text {
			t.Fatalf("config file ocean dark text = %q, want filled-in value persisted", onDisk.Themes["ocean"].Dark.Text)
		}
	})

	t.Run("missing selection falls back to the default pack", func(t *testing.T) {
		dir := testConfigDir(t)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating config dir: %v", err)
		}
		content := "themeName: no-such-pack\nthemes:\n  default:\n    dark:\n      labels: aqua\n"
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
	defaultTheme := Theme{Mode: "system", Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", Focus: "yellow", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
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
		if monokai.Dark.Labels != "#a6e22e" || monokai.Dark.Background != "#272822" {
			t.Fatalf("monokai dark = %+v, want the bundled palette", monokai.Dark)
		}
		// Gruvbox, solarized and one-dark ship their canonical light
		// variants; the rest fill from the built-in light palette.
		if got.Themes["gruvbox"].Light.Background != "#fbf1c7" {
			t.Fatalf("gruvbox light background = %q, want the bundled light variant", got.Themes["gruvbox"].Light.Background)
		}
		if got.Themes["solarized"].Light.Background != "#fdf6e3" {
			t.Fatalf("solarized light background = %q, want the bundled light variant", got.Themes["solarized"].Light.Background)
		}
		if got.Themes["one-dark"].Light.Background != "#fafafa" {
			t.Fatalf("one-dark light background = %q, want the bundled light variant", got.Themes["one-dark"].Light.Background)
		}
		if got.Themes["monokai"].Light.Background != DefaultLightTheme().Background {
			t.Fatalf("monokai light background = %q, want the built-in light palette fill", got.Themes["monokai"].Light.Background)
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
		if nord.Dark.Labels != "lime" {
			t.Fatalf("nord dark labels = %q, want the user's lime kept over the bundle", nord.Dark.Labels)
		}
	})
}
