package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/pkg/providers"
)

// overlayCellStyles draws the settings overlay onto a simulation screen
// and returns the styles of the cells spelling the wanted text, so tests
// assert the colors actually painted, not just the widget state.
func overlayCellStyles(a *app, want string) (map[int]tcell.Style, bool) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		panic(err)
	}
	defer screen.Fini()
	screen.SetSize(90, 45)
	a.settingsColumn.SetRect(0, 0, 90, 45)
	a.settingsColumn.Draw(screen)
	screen.Show()
	cells, _, _ := screen.GetContents()
	for i := 0; i+len(want) <= len(cells); i++ {
		match := true
		for j := 0; j < len(want); j++ {
			r := cells[i+j].Runes
			if len(r) == 0 || r[0] != rune(want[j]) {
				match = false
				break
			}
		}
		if match {
			styles := map[int]tcell.Style{}
			for j := 0; j < len(want); j++ {
				styles[j] = cells[i+j].Style
			}
			return styles, true
		}
	}
	return nil, false
}

// TestSettingsPanelPaintsWithSettingsKeys pins the new settings color keys:
// the form's row labels follow settingsLabels, not the shortcut bar keys,
// and the field values follow settingsText.
func TestSettingsPanelPaintsWithSettingsKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	originalTheme := theme
	t.Cleanup(func() { theme = originalTheme })
	originalDetect := detectBackground
	detectBackground = func() providers.BackgroundKind { return providers.BackgroundDark }
	cachedBackground = ""
	t.Cleanup(func() {
		detectBackground = originalDetect
		cachedBackground = ""
	})

	a := settingsTestApp(t)
	configuredTheme = providers.Theme{
		Mode:           string(providers.ModeDark),
		Labels:         "yellow",
		Text:           "white",
		SettingsLabels: "orange",
		SettingsText:   "aqua",
	}
	configuredThemesDark = configuredTheme
	configuredThemesLight = providers.DefaultLightTheme()
	applyConfigTheme(configuredThemesDark, configuredThemesLight)

	a.buildSettingsPage()
	a.openSettings()

	// Row label color comes from settingsLabels.
	labelStyles, found := overlayCellStyles(a, "Shortcut bar")
	if !found {
		t.Fatal("settings overlay render is missing the Shortcut bar label")
	}
	wantLabel := tcell.GetColor("orange")
	for i, s := range labelStyles {
		if fg, _, _ := s.Decompose(); fg != wantLabel && "Shortcut bar"[i] != ' ' {
			t.Errorf("label %q painted with %v, want orange (settingsLabels)", string("Shortcut bar"[i]), fg)
		}
	}
	// Field value color comes from settingsText: the config path row's
	// value text is painted with it.
	valueStyles, found := overlayCellStyles(a, "config.yaml")
	if !found {
		t.Fatal("settings overlay render is missing the config path value")
	}
	wantValue := tcell.GetColor("aqua")
	for i, s := range valueStyles {
		if fg, _, _ := s.Decompose(); fg != wantValue && "config.yaml"[i] != ' ' {
			t.Errorf("value %q painted with %v, want aqua (settingsText)", string("config.yaml"[i]), fg)
		}
	}
}

// TestSettingsPanelLiveEditRestylesItself proves an edit to the new keys
// repaints the open panel through the preview path, the same behavior the
// other color fields have.
func TestSettingsPanelLiveEditRestylesItself(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	originalTheme := theme
	t.Cleanup(func() { theme = originalTheme })
	originalDetect := detectBackground
	detectBackground = func() providers.BackgroundKind { return providers.BackgroundDark }
	cachedBackground = ""
	t.Cleanup(func() {
		detectBackground = originalDetect
		cachedBackground = ""
	})

	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Mode: string(providers.ModeDark)}
	configuredThemesDark = configuredTheme
	configuredThemesLight = providers.DefaultLightTheme()
	a.buildSettingsPage()
	a.openSettings()

	var field *tview.InputField
	for i := 0; i < a.settingsForm.GetFormItemCount(); i++ {
		if pf, ok := a.settingsForm.GetFormItem(i).(*colorPreviewField); ok && pf.baseLabel == "Settings labels" {
			field = pf.InputField
			break
		}
	}
	if field == nil {
		t.Fatal("settings form has no Settings labels field")
	}
	field.SetText("teal")

	if configuredThemesDark.SettingsLabels != "teal" {
		t.Errorf("edited settingsLabels = %q, want teal", configuredThemesDark.SettingsLabels)
	}
	// The preview path repaints the overlay, so the row labels must now
	// carry the edited color.
	labelStyles, found := overlayCellStyles(a, "Shortcut bar")
	if !found {
		t.Fatal("settings overlay render is missing the Shortcut bar label")
	}
	want := tcell.GetColor("teal")
	sawTeal := false
	for i, s := range labelStyles {
		fg, _, _ := s.Decompose()
		if fg == want && "Shortcut bar"[i] != ' ' {
			sawTeal = true
		}
	}
	if !sawTeal {
		t.Error("no row label painted teal after the live edit; preview did not restyle the panel")
	}
}

// TestSettingsPanelDoesNotRequeryTerminal pins the boot-cache behavior: the
// panel reads the background detected at boot and never calls the detection
// seam again, because inside the running TUI tcell owns stdin and a live
// OSC 11 query only stalls until the timeout and falls back to COLORFGBG.
func TestSettingsPanelDoesNotRequeryTerminal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	originalDetect := detectBackground
	cachedBackground = ""
	t.Cleanup(func() {
		detectBackground = originalDetect
		cachedBackground = ""
	})

	calls := 0
	detectBackground = func() providers.BackgroundKind {
		calls++
		return providers.BackgroundDark
	}

	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Mode: string(providers.ModeSystem)}
	configuredThemesDark = configuredTheme
	configuredThemesLight = providers.DefaultLightTheme()

	// Simulate the boot detection the app runs before tcell starts.
	cachedBackground = detectBackground()
	calls = 0

	a.buildSettingsPage()
	a.openSettings()
	a.rebuildSettingsForm()
	a.previewThemeColors()
	a.closeSettings()

	if calls != 0 {
		t.Errorf("detection ran %d times inside the panel, want 0 (boot cache only)", calls)
	}
}

// TestSettingsFormCoversSettingsKeys proves the color field list exposes
// the settings panel's own colors, so users can edit them like every other
// role.
func TestSettingsFormCoversSettingsKeys(t *testing.T) {
	labels := make([]string, 0, len(themeColorKeys))
	for _, key := range themeColorKeys {
		labels = append(labels, key.label)
	}
	joined := strings.Join(labels, "\n")
	if !strings.Contains(joined, "Settings labels") || !strings.Contains(joined, "Settings text") {
		t.Errorf("themeColorKeys misses the settings panel keys: %v", labels)
	}
}
