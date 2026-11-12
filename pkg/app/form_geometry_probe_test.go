package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

// formRows renders the settings form into a simulation screen at the flex
// height it requests and returns the drawn rows, so layout claims are tested
// against what tview actually draws.
func formRows(t *testing.T, width int) []string {
	t.Helper()
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Mode: "dark"}
	configuredThemesDark = configuredTheme
	configuredThemesLight = providers.DefaultLightTheme()
	a.buildSettingsPage()
	form := a.settingsForm
	h := settingsFormHeight(a)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(width, h)
	form.SetRect(10, 0, width-20, h)
	form.Draw(screen)
	screen.Show()
	rows := make([]string, h)
	for y := 0; y < h; y++ {
		var row strings.Builder
		cells, cw, _ := screen.GetContents()
		for x := 0; x < cw; x++ {
			row.WriteRune(cells[y*cw+x].Runes[0])
		}
		rows[y] = row.String()
	}
	return rows
}

// TestSettingsFormShowsEveryRowAndButtons guards the settings form layout:
// at the height the flex requests, the last color field, the config path and
// both buttons must draw, because tview skips items that start at or below
// the bottom limit.
func TestSettingsFormShowsEveryRowAndButtons(t *testing.T) {
	rows := formRows(t, 80)
	joined := strings.Join(rows, "\n")
	for _, want := range []string{"Active row background", "Config file", "Open in editor", "Close"} {
		if !strings.Contains(joined, want) {
			t.Errorf("settings form at height %d is missing %q; drawn rows:\n%s", len(rows), want, joined)
		}
	}
}

// TestCycleThemeColorStepsThroughTheList pins the arrow cycling: a value in
// the list moves to its neighbor with wraparound, an unknown value enters
// at the first entry. The field text is what cycling writes; the section
// only updates through the changed callback the form wires.
func TestCycleThemeColorStepsThroughTheList(t *testing.T) {
	section := providers.Theme{Labels: "yellow"}
	key := themeColorKeys[1] // Labels
	field := tview.NewInputField().SetText("yellow")
	field.SetChangedFunc(func(text string) { key.set(&section, text) })
	cycleThemeColor(&section, key, 1, field)
	if got := field.GetText(); got != "blue" {
		t.Errorf("next after yellow = %q, want blue", got)
	}
	if section.Labels != "blue" {
		t.Errorf("changed callback not fired, section labels = %q", section.Labels)
	}
	cycleThemeColor(&section, key, -1, field)
	if got := field.GetText(); got != "yellow" {
		t.Errorf("prev after blue = %q, want yellow", got)
	}
	// Unknown values enter the cycle at the first entry.
	field.SetText("#ff9900")
	section.Labels = "#ff9900"
	cycleThemeColor(&section, key, 1, field)
	if got := field.GetText(); got != "black" {
		t.Errorf("cycle from unknown = %q, want black", got)
	}
	// Wrap around the list edges.
	field.SetText("navy")
	section.Labels = "navy"
	cycleThemeColor(&section, key, 1, field)
	if got := field.GetText(); got != "black" {
		t.Errorf("wrap after navy = %q, want black", got)
	}
	field.SetText("black")
	section.Labels = "black"
	cycleThemeColor(&section, key, -1, field)
	if got := field.GetText(); got != "navy" {
		t.Errorf("wrap before black = %q, want navy", got)
	}
}