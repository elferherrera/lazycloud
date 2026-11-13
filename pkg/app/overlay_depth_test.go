package app

import (
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TestSettingsOverlayPaintsDropShadow draws the settings overlay over a
// painted main view and pins the depth effect: the cells one step right
// of and one row below the panel carry the dim attribute and a darker
// background than the painted surface, and the cells on the panel edge
// and inside it do not. The strip is what makes the panel read as a
// floating window instead of a flat region.
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
	// content to darken: an x on the pack background, like a dark pack's
	// main view paints it.
	bgPaint := tcell.GetColor("black")
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			screen.SetContent(x, y, 'x', nil, tcell.StyleDefault.Background(bgPaint))
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

	// The strip below the panel and right of it is dim and darkened.
	wantBg := shadowBackgroundColor(bgPaint)
	if wantBg == bgPaint {
		t.Fatalf("shadow background %v does not darken %v, test cannot pin the effect", wantBg, bgPaint)
	}
	if style, ok := cellStyle(screen, px, py+ph); !ok || !isDim(style) {
		t.Errorf("cell below the panel left corner (%d,%d) is not dim", px, py+ph)
	} else if _, bg, _ := style.Decompose(); bg != wantBg {
		t.Errorf("cell below the panel left corner (%d,%d) keeps its background %v, want the darkened %v", px, py+ph, bg, wantBg)
	}
	if style, ok := cellStyle(screen, px+pw, py); !ok || !isDim(style) {
		t.Errorf("cell right of the panel top (%d,%d) is not dim", px+pw, py)
	} else if _, bg, _ := style.Decompose(); bg != wantBg {
		t.Errorf("cell right of the panel top (%d,%d) keeps its background %v, want the darkened %v", px+pw, py, bg, wantBg)
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
