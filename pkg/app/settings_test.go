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

func TestRebuildSettingsFormReflectsLiveValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := settingsTestApp(t)
	a.buildSettingsPage()
	if a.settingsForm == nil {
		t.Fatal("buildSettingsPage did not create the form")
	}
	infobarEnabled = false
	a.rebuildSettingsForm()
	if got := a.settingsForm.GetFormItemCount(); got != len(a.settingsRows())+1 {
		t.Errorf("form has %d items, want %d checkboxes plus config path", got, len(a.settingsRows()))
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
