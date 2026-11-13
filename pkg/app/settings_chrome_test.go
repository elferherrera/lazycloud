package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/elferherrera/lazycloud/pkg/providers"
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
	if a.settingsHint == nil {
		t.Fatal("settings overlay must carry the usage hint text view")
	}
	if got := a.settingsHint.GetBackgroundColor(); got != tcell.GetColor("navy") {
		t.Errorf("usage hint background = %v, want the ocean pack's navy", got)
	}
}

// TestSettingsOverlayDrawsUsageHint renders the whole settings overlay
// (column flex, not just the form) and pins that the key summary paints
// below the form, so the flex wiring cannot silently drop it.
func TestSettingsOverlayDrawsUsageHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Background: "black"}
	configuredFollowTerminal = false
	t.Cleanup(func() { configuredFollowTerminal = false })
	a.buildSettingsPage()

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 40)
	a.settingsColumn.SetRect(0, 0, 80, 40)
	a.settingsColumn.Draw(screen)
	screen.Show()
	cells, cw, _ := screen.GetContents()
	var joined strings.Builder
	for y := 0; y < 40; y++ {
		for x := 0; x < cw; x++ {
			joined.WriteRune(cells[y*cw+x].Runes[0])
		}
		joined.WriteByte('\n')
	}
	rendered := joined.String()
	for _, want := range []string{"tab moves", "space toggles", "esc saves and closes"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("settings overlay render is missing %q; drawn:\n%s", want, rendered)
		}
	}
}

// TestSettingsUsageHintNamesRealKeys guards the key summary the overlay
// shows: every key named in the hint must be a key the panel actually
// handles, because a wrong hint is worse than none.
func TestSettingsUsageHintNamesRealKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, key := range []string{"tab", "space", "arrows", "esc"} {
		if !strings.Contains(settingsUsageHint, key) {
			t.Errorf("usage hint is missing the %q key", key)
		}
	}
	// The hint must also fit the overlay width so it never wraps.
	if width := len(strings.Split(settingsUsageHint, "\n")[0]); width > 54 {
		t.Errorf("usage hint line 1 is %d columns, want at most 54 to fit the 56-column overlay", width)
	}
	if width := len(strings.Split(settingsUsageHint, "\n")[1]); width > 54 {
		t.Errorf("usage hint line 2 is %d columns, want at most 54 to fit the 56-column overlay", width)
	}
}
