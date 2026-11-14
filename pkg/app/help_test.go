package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func drawScreen(t *testing.T, width, height int, draw func(screen tcell.Screen)) string {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(width, height)
	draw(screen)
	screen.Show()
	var sb strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, _, _, _ := screen.GetContent(x, y)
			sb.WriteRune(r)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func assertNoNumericShortcutLines(t *testing.T, text string) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		key := fields[0]
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			t.Errorf("help text shows numeric shortcut %q", key)
		}
	}
}

func TestHelpTextRenders(t *testing.T) {
	appState.ViewKind = viewProjects
	text := helpText()
	assertNoNumericShortcutLines(t, text)
	for _, s := range shortcuts {
		if s.visible != nil && !s.visible() {
			continue
		}
		if !strings.Contains(text, s.desc) {
			t.Errorf("help text is missing description %q", s.desc)
		}
		for _, c := range s.commands {
			if !strings.Contains(text, c.cmd) {
				t.Errorf("help text is missing command %q", c.cmd)
			}
		}
	}
}

func TestHelpTextGroupsSectionsInOrder(t *testing.T) {
	appState.ViewKind = viewClusters
	text := helpText()
	last := -1
	for i, section := range helpSections {
		// Only sections with entries render; find each present section
		// header and confirm the order never regresses.
		idx := strings.Index(text, strings.ToUpper(section[:1])+strings.ToUpper(section[1:])+"\n")
		if idx == -1 {
			continue
		}
		if idx < last {
			t.Errorf("section %q renders out of order", section)
		}
		last = idx
		if i == 0 && !strings.Contains(text, "Workloads") {
			t.Error("clusters help is missing the panel-specific workloads entry")
		}
	}
}

func TestHelpTextHidesProjectActionsOutsideProjectsView(t *testing.T) {
	appState.ViewKind = viewIPs
	text := helpText()
	for _, s := range shortcuts {
		contains := strings.Contains(text, s.desc)
		if s.visible != nil && !s.visible() {
			if contains {
				t.Errorf("help text shows hidden description %q", s.desc)
			}
		} else if !contains {
			t.Errorf("help text is missing description %q", s.desc)
		}
	}
}

func TestHelpOverlayRenders(t *testing.T) {
	appState.ViewKind = viewProjects
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 50)
	a := &app{screen: screen, helpText: newHelpTextView()}
	a.helpText.SetText(helpText())
	var overlay *tview.Flex
	overlay, a.helpRow = newHelpOverlay(a.helpText)
	a.helpColumn = overlay
	a.resizeHelpOverlay()
	overlay.SetRect(0, 0, 120, 50)
	rendered := drawScreen(t, 120, 50, overlay.Draw)
	assertNoNumericShortcutLines(t, rendered)
	if !strings.Contains(rendered, "Help") {
		t.Error("help overlay is missing its title")
	}
	for _, s := range shortcuts {
		if s.visible != nil && !s.visible() {
			continue
		}
		if !strings.Contains(rendered, s.desc) {
			t.Errorf("help overlay is missing description %q", s.desc)
		}
		for _, c := range s.commands {
			if !strings.Contains(rendered, c.cmd) {
				t.Errorf("help overlay is missing command %q", c.cmd)
			}
		}
	}
}

func TestHelpOverlayScrollsInsteadOfClippingTop(t *testing.T) {
	appState.ViewKind = viewProjects
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 20)
	a := &app{screen: screen, helpText: newHelpTextView()}
	a.helpText.SetText(helpText())
	var overlay *tview.Flex
	overlay, a.helpRow = newHelpOverlay(a.helpText)
	a.helpColumn = overlay
	a.resizeHelpOverlay()
	overlay.SetRect(0, 0, 100, 20)
	rendered := drawScreen(t, 100, 20, overlay.Draw)
	if !strings.Contains(rendered, "Help") {
		t.Error("short terminal clips the help title")
	}
	if !strings.Contains(rendered, "THIS VIEW") {
		t.Error("short terminal clips the first section instead of scrolling")
	}
}
