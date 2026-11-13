package app

import (
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TestSettingsOverlayPaintsDropShadow draws the settings overlay over a
// painted main view and pins the depth effect: the cells one step right
// of and one row below the panel carry the dim attribute, and the cells
// on the panel edge and inside it do not. The strip is what makes the
// panel read as a floating window instead of a flat region.
func TestSettingsOverlayPaintsDropShadow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	snapshotSettingsGlobals(t)
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Background: "black"}
	configuredFollowTerminal = false
	a.buildSettingsPage()

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 40)

	// Paint a main view behind the overlay so the shadow has real
	// content to dim: every cell gets an x on the default style.
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			screen.SetContent(x, y, 'x', nil, tcell.StyleDefault)
		}
	}

	a.settingsColumn.SetRect(0, 0, 80, 40)
	a.settingsColumn.Draw(screen)
	screen.Show()

	// Panel rect: the row flex centers the shadow wrapper; read the
	// wrapper's rect back through the app field.
	px, py, pw, ph := a.settingsPanel.GetRect()
	if pw <= 0 || ph <= 0 {
		t.Fatalf("settings panel rect = %d,%d %dx%d, want a laid-out panel", px, py, pw, ph)
	}
	if px+pw >= 80 {
		t.Fatalf("panel right edge %d leaves no room for the shadow strip", px+pw)
	}
	if py+ph >= 40 {
		t.Fatalf("panel bottom edge %d leaves no room for the shadow strip", py+ph)
	}

	// The strip below the panel and right of it is dim.
	if style, ok := cellStyle(screen, px, py+ph); !ok || !isDim(style) {
		t.Errorf("cell below the panel left corner (%d,%d) is not dim", px, py+ph)
	}
	if style, ok := cellStyle(screen, px+pw, py); !ok || !isDim(style) {
		t.Errorf("cell right of the panel top (%d,%d) is not dim", px+pw, py)
	}
	// Inside the panel nothing is dim: the effect is only the depth
	// strip, the rows keep their palette contrast.
	if style, ok := cellStyle(screen, px+1, py+1); !ok || isDim(style) {
		t.Errorf("cell inside the panel (%d,%d) must not carry the dim attribute", px+1, py+1)
	}
	// Far from the panel nothing is dim either.
	if style, ok := cellStyle(screen, 0, 0); !ok || isDim(style) {
		t.Errorf("cell in the far margin (0,0) must not carry the dim attribute")
	}
}

// isDim reports whether the style carries the dim attribute.
func isDim(style tcell.Style) bool {
	_, _, attrs := style.Decompose()
	return attrs&tcell.AttrDim != 0
}

// TestShadowedPrimitiveDelegatesFocus pins the wrapper's delegation: the
// shadow is a paint pass, not a focus target, so focus, input and mouse
// all belong to the wrapped panel.
func TestShadowedPrimitiveDelegatesFocus(t *testing.T) {
	inner := tview.NewForm()
	wrapper := newShadowedOverlay(inner)
	if wrapper.HasFocus() != inner.HasFocus() {
		t.Fatal("wrapper HasFocus disagrees with the wrapped panel")
	}
	if wrapper.InputHandler() == nil && inner.InputHandler() != nil {
		t.Fatal("wrapper swallowed the panel's input handler")
	}
	if wrapper.MouseHandler() == nil && inner.MouseHandler() != nil {
		t.Fatal("wrapper swallowed the panel's mouse handler")
	}
}

// cellStyle reads one cell's style off the simulation screen.
func cellStyle(screen tcell.Screen, x, y int) (tcell.Style, bool) {
	sim, ok := screen.(tcell.SimulationScreen)
	if !ok {
		return tcell.StyleDefault, false
	}
	cells, width, _ := sim.GetContents()
	idx := y*width + x
	if idx < 0 || idx >= len(cells) {
		return tcell.StyleDefault, false
	}
	return cells[idx].Style, true
}
