package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// shadowedPrimitive wraps the bordered body of an overlay and paints a
// one-cell drop shadow on its right and bottom edge after the body
// draws: the strip darkens whatever the main view already painted there,
// so the panel reads as a window floating above the background instead
// of a flat region of it. The shadow keeps each cell's rune and only
// darkens its colors, so the panel's own rows and border stay untouched.
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
		shadowCell(screen, x+width, row)
	}
	// Bottom edge, one row out, corner included.
	for col := x; col <= x+width && col < screenWidth; col++ {
		shadowCell(screen, col, y+height)
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

// shadowCell repaints one strip cell so it reads as cast shadow: the
// background scales toward black and the glyph keeps its rune, dimmed,
// so content under the strip recedes instead of vanishing. A near-black
// background would stay black under scaling, so it gets a dark gray
// floor instead; that floor is what makes the strip visible on the
// default dark packs. Cells painted with the terminal's own default
// background (follow-terminal mode) carry no color to darken, so those
// keep only the dim attribute.
func shadowCell(screen tcell.Screen, x, y int) {
	sw, sh := screen.Size()
	if x < 0 || y < 0 || x >= sw || y >= sh {
		return
	}
	mainRune, combining, style, _ := screen.GetContent(x, y)
	style = style.Dim(true)
	_, bg, _ := style.Decompose()
	if bg != tcell.ColorDefault {
		style = style.Background(shadowBackgroundColor(bg))
	}
	screen.SetContent(x, y, mainRune, combining, style)
}

// shadowBackgroundColor scales a background color to 45% brightness, the
// level a cast shadow sits at relative to its surface.
func shadowBackgroundColor(bg tcell.Color) tcell.Color {
	r, g, b := bg.TrueColor().RGB()
	r, g, b = r*45/100, g*45/100, b*45/100
	if r+g+b < 90 {
		// Near-black surfaces scale to black, which reads as no shadow
		// at all; a fixed dark gray keeps the strip visible over them.
		return tcell.NewRGBColor(60, 60, 60)
	}
	return tcell.NewRGBColor(r, g, b)
}
