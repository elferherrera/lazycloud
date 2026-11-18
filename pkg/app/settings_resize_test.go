package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

// panelHeight draws the settings page at a fixed screen size and returns
// the height the flex actually gave the form (the bordered window the user
// sees — the panel around it can grow while the form stays put, which is
// not a visible resize).
func panelHeight(t *testing.T, a *app, screen tcell.SimulationScreen) (int, string) {
	t.Helper()
	a.settingsColumn.SetRect(0, 0, 80, 40)
	a.settingsColumn.Draw(screen)
	screen.Show()
	_, _, _, height := a.settingsForm.GetRect()
	cells, cw, ch := screen.GetContents()
	var joined strings.Builder
	for y := 0; y < ch; y++ {
		for x := 0; x < cw; x++ {
			joined.WriteRune(cells[y*cw+x].Runes[0])
		}
		joined.WriteRune('\n')
	}
	return height, joined.String()
}

// themeCycleRow finds the Theme row so tests can cycle the selection the
// way the panel does.
func themeCycleRow(t *testing.T, a *app) settingsRow {
	t.Helper()
	for _, row := range a.settingsRows() {
		if row.label == themeRowLabel {
			return row
		}
	}
	t.Fatal("settings panel is missing the Theme row")
	return settingsRow{}
}

// TestThemeCycleResizesSettingsPanel pins the window resizing: the panel
// grows when the cycle reaches the custom palette (its color fields join
// the form) and shrinks back when it leaves (a config pack or follow
// terminal drops them), instead of keeping the height it opened with.
func TestThemeCycleResizesSettingsPanel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	snapshotSettingsGlobals(t)
	a := settingsTestApp(t)
	configuredThemePacks = map[string]providers.Theme{
		"ocean": {Background: "navy", Labels: "aqua"},
	}
	configuredThemeName = "ocean"
	configuredTheme = configuredThemePacks["ocean"]
	configuredFollowTerminal = false

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 40)
	a.screen = screen

	a.buildSettingsPage()
	a.openSettings()
	packHeight, drawn := panelHeight(t, a, screen)
	if want := settingsFormHeight(a); packHeight != want {
		t.Fatalf("panel height on a pack = %d, want %d", packHeight, want)
	}
	if strings.Contains(drawn, "Active row background") {
		t.Error("pack selection drew the color fields")
	}

	row := themeCycleRow(t, a)
	row.set(true) // ocean -> custom
	customHeight, drawn := panelHeight(t, a, screen)
	if customHeight != packHeight+len(themeColorKeys) {
		t.Errorf("panel height in custom = %d, want %d (pack height + color fields)", customHeight, packHeight+len(themeColorKeys))
	}
	if !strings.Contains(drawn, "Active row background") {
		t.Errorf("custom selection clipped the last color field; drawn rows:\n%s", drawn)
	}

	row.set(true) // custom -> follow terminal
	followHeight, _ := panelHeight(t, a, screen)
	if followHeight != settingsFormHeight(a) {
		t.Errorf("panel height in follow terminal = %d, want %d", followHeight, settingsFormHeight(a))
	}

	row.set(true) // follow terminal -> wrap to the first pack
	if got, _ := panelHeight(t, a, screen); got != packHeight {
		t.Errorf("panel height after the wrap = %d, want %d", got, packHeight)
	}
}