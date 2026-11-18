package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

// chromeTestApp builds the app with the widget set applyThemeChrome touches,
// so the test can read the painted colors the way the running UI holds them.
func chromeTestApp(t *testing.T) *app {
	t.Helper()
	a := settingsTestApp(t)
	a.buildSettingsPage()
	return a
}

// TestCycleThemeNameRestylesOverlayChrome pins the refresh fix: cycling the
// theme pack must repaint the settings overlay's box chrome (background,
// border, title) and the main widgets, not just the form items. tview
// snapshots the global styles at construction time, so a pack switch that
// skips applyThemeChrome leaves the whole overlay painted with the palette
// active when the panel was first built.
func TestCycleThemeNameRestylesOverlayChrome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	originalTheme := theme
	t.Cleanup(func() { theme = originalTheme })
	a := chromeTestApp(t)
	originalFollow := configuredFollowTerminal
	configuredFollowTerminal = false
	t.Cleanup(func() { configuredFollowTerminal = originalFollow })

	configuredThemePacks = map[string]providers.Theme{
		"lazycloud_dark": {
			Background: "black",
			Borders:    "white",
			Title:      "white",
			Labels:     "yellow",
		},
		"ocean": {
			Background: "navy",
			Borders:    "aqua",
			Title:      "white",
			Labels:     "aqua",
		},
	}
	configuredThemeName = "lazycloud_dark"
	setConfiguredThemes(providers.Config{Themes: configuredThemePacks, ThemeName: "lazycloud_dark"})

	a.cycleThemeName(1) // lazycloud_dark -> ocean

	if got := a.settingsForm.GetBackgroundColor(); got != tcell.GetColor("navy") {
		t.Errorf("settings form background = %v, want the ocean pack's navy", got)
	}
	if got := a.settingsForm.GetBorderColor(); got != tcell.GetColor("aqua") {
		t.Errorf("settings form border = %v, want the ocean pack's aqua", got)
	}
	if got := a.info.GetBackgroundColor(); got != tcell.GetColor("navy") {
		t.Errorf("main table background = %v, want the ocean pack's navy", got)
	}
	if got := a.info.GetBorderColor(); got != tcell.GetColor("aqua") {
		t.Errorf("main table border = %v, want the ocean pack's aqua", got)
	}
}

// TestOpenSettingsLeavesStatusBarAlone pins that opening the panel writes
// no key summary: the status bar keeps whatever it showed before, so the
// panel opens silently.
func TestOpenSettingsLeavesStatusBarAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Background: "black"}
	configuredFollowTerminal = false
	t.Cleanup(func() { configuredFollowTerminal = false })
	a.buildSettingsPage()

	a.openSettings()
	if got := a.status.GetText(false); strings.Contains(got, "tab") {
		t.Errorf("status = %q, want no key summary on open", got)
	}
	if a.statusShowing {
		t.Error("openSettings() pinned the status row, want it left as it was")
	}

	configuredFollowTerminal = true
	a.openSettings()
	if got := a.status.GetText(false); strings.Contains(got, "tab") {
		t.Errorf("follow-terminal status = %q, want no key summary on open", got)
	}
}
