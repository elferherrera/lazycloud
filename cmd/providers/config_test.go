package providers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

	defaultTheme := Theme{Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	defaultShortcuts := map[string]string{"1": "vms", "2": "buckets", "0": "firewalls"}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}

	got, err := LoadConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate)
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

	defaultTheme := Theme{Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	defaultShortcuts := map[string]string{"1": "vms", "2": "buckets", "0": "firewalls"}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}

	got, err := LoadConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := Theme{Labels: "aqua", Text: "gray", Borders: "green", Title: "orange", ShortcutLabels: "aqua", ShortcutText: "gray", StatusLabels: "yellow", StatusText: "gray", SummaryLabels: "aqua", SummaryText: "gray", DetailLabels: "yellow", DetailText: "gray", ActiveRowText: "black", ActiveRowBackground: "gray"}
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
		Theme:      Theme{Labels: "aqua", Text: "gray", Borders: "green", Title: "orange", ShortcutLabels: "red", ShortcutText: "silver", StatusLabels: "teal", StatusText: "olive", SummaryLabels: "maroon", SummaryText: "navy", DetailLabels: "purple", DetailText: "pink", ActiveRowText: "lime", ActiveRowBackground: "teal"},
		Shortcuts:  map[string]string{"1": "clusters", "5": "ips"},
		AutoUpdate: AutoUpdate{Enabled: false, ThresholdInMinutes: 30},
	}
	if err := SaveConfig(custom); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	got, err := LoadConfig(Theme{}, nil, AutoUpdate{})
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

	defaultTheme := Theme{Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	defaultShortcuts := map[string]string{"1": "vms", "2": "buckets", "0": "firewalls"}
	defaultAutoUpdate := AutoUpdate{Enabled: true, ThresholdInMinutes: 60}

	got, err := LoadConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := Theme{Labels: "aqua", Text: "white", Borders: "green", Title: "white", ShortcutLabels: "aqua", ShortcutText: "white", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "aqua", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
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

	got, err := LoadConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate)
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
