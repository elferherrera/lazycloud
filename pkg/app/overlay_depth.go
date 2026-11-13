package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// shadowedPrimitive wraps the bordered body of an overlay and paints a
// one-cell drop shadow on its right and bottom edge after the body
// draws: the strip dims whatever the main view already painted there,
// so the panel reads as a window floating above the background instead
// of a flat region of it. The shadow keeps each cell's rune and colors
// and only adds the dim attribute, so no theme pack can break its
// contrast, and the panel's own rows and border stay untouched.
type shadowedPrimitive struct {
	*tview.Box
	child tview.Primitive
}

// newShadowedOverlay wraps the panel body; focus and input stay the
// child's, the wrapper only adds the shadow pass to Draw.
func newShadowedOverlay(child tview.Primitive) *shadowedPrimitive {
	return &shadowedPrimitive{Box: tview.NewBox(), child: child}
}

// Draw lays the child out over the wrapper's rectangle, paints it, then
// dims the L-shaped strip one cell right of and one row below it.
func (s *shadowedPrimitive) Draw(screen tcell.Screen) {
	s.Box.DrawForSubclass(screen, s)
	x, y, width, height := s.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}
	s.child.SetRect(x, y, width, height)
	s.child.Draw(screen)
	screenWidth, screenHeight := screen.Size()
	// Right edge, one cell out; the bottom strip paints the corner.
	for row := y; row < y+height && row < screenHeight; row++ {
		dimCell(screen, x+width, row)
	}
	// Bottom edge, one row out, corner included.
	for col := x; col <= x+width && col < screenWidth; col++ {
		dimCell(screen, col, y+height)
	}
}

func (s *shadowedPrimitive) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return s.child.InputHandler()
}

func (s *shadowedPrimitive) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return s.child.MouseHandler()
}

func (s *shadowedPrimitive) PasteHandler() func(text string, setFocus func(p tview.Primitive)) {
	return s.child.PasteHandler()
}

func (s *shadowedPrimitive) Focus(delegate func(p tview.Primitive)) {
	s.child.Focus(delegate)
}

func (s *shadowedPrimitive) HasFocus() bool {
	return s.child.HasFocus()
}

func (s *shadowedPrimitive) Blur() {
	s.child.Blur()
}

// dimCell re-paints one cell with the dim attribute on top of whatever
// is already there: a blank cell shows a dimmed backdrop, a cell with
// main-view text keeps its glyph but reads pressed back one plane.
func dimCell(screen tcell.Screen, x, y int) {
	sw, sh := screen.Size()
	if x < 0 || y < 0 || x >= sw || y >= sh {
		return
	}
	mainRune, combining, style, _ := screen.GetContent(x, y)
	screen.SetContent(x, y, mainRune, combining, style.Dim(true))
}
