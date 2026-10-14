package main

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

func assertNoNumericShortcutCells(t *testing.T, table *tview.Table) {
	t.Helper()
	for r := 0; r < table.GetRowCount(); r++ {
		key := table.GetCell(r, 0).Text
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			t.Errorf("shortcuts panel shows numeric shortcut %q", key)
		}
	}
}

func TestShortcutsPanelRenders(t *testing.T) {
	appState.ViewKind = viewProjects
	height := shortcutsRowCount() + 2
	table := newShortcutsPanel()
	table.SetRect(0, 0, 60, height)
	rendered := drawScreen(t, 60, height, table.Draw)
	assertNoNumericShortcutCells(t, table)
	for _, s := range shortcuts {
		if s.visible != nil && !s.visible() {
			continue
		}
		if !strings.Contains(rendered, s.desc) {
			t.Errorf("shortcuts panel is missing description %q", s.desc)
		}
		for _, c := range s.commands {
			if !strings.Contains(rendered, c.cmd) {
				t.Errorf("shortcuts panel is missing command %q", c.cmd)
			}
		}
	}
}

func TestShortcutsPanelHidesProjectActionsOutsideProjectsView(t *testing.T) {
	appState.ViewKind = viewIPs
	height := shortcutsRowCount() + 2
	table := newShortcutsPanel()
	table.SetRect(0, 0, 60, height)
	rendered := drawScreen(t, 60, height, table.Draw)
	for _, s := range shortcuts {
		contains := strings.Contains(rendered, s.desc)
		if s.visible != nil && !s.visible() {
			if contains {
				t.Errorf("shortcuts panel shows hidden description %q", s.desc)
			}
		} else if !contains {
			t.Errorf("shortcuts panel is missing description %q", s.desc)
		}
	}
}

func TestHelpOverlayRenders(t *testing.T) {
	appState.ViewKind = viewProjects
	table := newShortcutsPanel()
	overlay, _ := newHelpOverlay(table)
	overlay.SetRect(0, 0, 120, 40)
	rendered := drawScreen(t, 120, 40, overlay.Draw)
	assertNoNumericShortcutCells(t, table)
	if !strings.Contains(rendered, "Shortcuts") {
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
