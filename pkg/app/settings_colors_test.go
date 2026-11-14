package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
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

// settingsColorsTestState restores the palette globals each of these tests
// overrides.
func settingsColorsTestState(t *testing.T) {
	t.Helper()
	originalTheme := theme
	originalFollow := configuredFollowTerminal
	t.Cleanup(func() {
		theme = originalTheme
		configuredFollowTerminal = originalFollow
	})
	configuredFollowTerminal = false
}

// TestSettingsPanelPaintsWithSettingsKeys pins the settings color keys: the
// form's row labels follow settingsLabels, not the shortcut bar keys, and
// the field values follow settingsText.
func TestSettingsPanelPaintsWithSettingsKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	settingsColorsTestState(t)

	a := settingsTestApp(t)
	configuredTheme = providers.Theme{
		Labels:         "yellow",
		Text:           "white",
		SettingsLabels: "orange",
		SettingsText:   "aqua",
	}
	applyActiveTheme()

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

// TestSettingsPanelLiveEditRestylesItself proves an edit to the keys
// repaints the open panel through the preview path, the same behavior the
// other color fields have.
func TestSettingsPanelLiveEditRestylesItself(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	settingsColorsTestState(t)

	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Background: "black"}
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

	if configuredTheme.SettingsLabels != "teal" {
		t.Errorf("edited settingsLabels = %q, want teal", configuredTheme.SettingsLabels)
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
