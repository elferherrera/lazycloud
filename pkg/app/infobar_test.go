package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func infobarCellText(t *testing.T, table *tview.Table) []string {
	t.Helper()
	cells := make([]string, 0, table.GetColumnCount())
	for c := 0; c < table.GetColumnCount(); c++ {
		cell := table.GetCell(0, c)
		if cell == nil {
			continue
		}
		cells = append(cells, cell.Text)
	}
	return cells
}

func TestInfobarShowsVisibleShortcutsPerView(t *testing.T) {
	appState.ViewKind = viewProjects
	projects := infobarShortcuts()
	if len(projects) == 0 {
		t.Fatal("infobar shows no shortcuts in projects view")
	}
	for _, s := range projects {
		if s.key >= '0' && s.key <= '9' {
			t.Errorf("infobar shows numeric shortcut %q", string(s.key))
		}
		if s.visible != nil && !s.visible() {
			t.Errorf("infobar shows shortcut %q hidden in projects view", s.label())
		}
	}
	cells := infobarCellText(t, newInfobarPanel())
	for _, s := range projects {
		if !sliceContains(cells, s.label()) {
			t.Errorf("infobar panel is missing key %q", s.label())
		}
	}
}

func TestInfobarShowsPanelShortcutsBeforeGlobals(t *testing.T) {
	appState.ViewKind = viewClusters
	entries := infobarShortcuts()
	if len(entries) == 0 {
		t.Fatal("infobar shows no shortcuts in clusters view")
	}
	if entries[0].label() != "w" && entries[0].label() != "n" {
		t.Errorf("clusters infobar starts with %q, want a panel shortcut first", entries[0].label())
	}
	hasWorkloads := false
	for _, s := range entries {
		if s.label() == "w" {
			hasWorkloads = true
		}
	}
	if !hasWorkloads {
		t.Error("infobar in clusters view is missing the w workloads shortcut")
	}

	appState.ViewKind = viewRoutes
	hasChart := false
	for _, s := range infobarShortcuts() {
		if s.label() == "c" && infobarDesc(s) == "chart" {
			hasChart = true
		}
	}
	if !hasChart {
		t.Error("infobar in routes view is missing the c chart shortcut")
	}
}

func TestInfobarKeepsDistinctCaseSensitiveKeys(t *testing.T) {
	appState.ViewKind = viewProjects
	hasLowerU, hasUpperU := false, false
	for _, s := range infobarShortcuts() {
		if s.label() == "u" {
			hasLowerU = true
		}
		if s.label() == "U" {
			hasUpperU = true
		}
	}
	if !hasLowerU || !hasUpperU {
		t.Errorf("infobar missing u/U pair, lower = %v upper = %v", hasLowerU, hasUpperU)
	}
}

func TestInfobarHidesDuplicateLabels(t *testing.T) {
	appState.ViewKind = viewWorkloads
	seen := map[string]bool{}
	for _, s := range infobarShortcuts() {
		if seen[s.label()] {
			t.Errorf("infobar shows duplicate key %q", s.label())
		}
		seen[s.label()] = true
	}
}

func TestInfobarFitsCharBudget(t *testing.T) {
	for _, kind := range []viewKind{viewProjects, viewVMs, viewClusters, viewWorkloads, viewRoutes, viewDNSRecords} {
		appState.ViewKind = kind
		width := 0
		for _, s := range infobarShortcuts() {
			width += len(s.label()) + 1 + len(infobarDesc(s)) + 1
		}
		if width > infobarBudget {
			t.Errorf("infobar in view %q uses %d chars, budget %d", kind, width, infobarBudget)
		}
	}
}

func TestInfobarRendersOneRow(t *testing.T) {
	appState.ViewKind = viewProjects
	table := newInfobarPanel()
	if table.GetRowCount() != 1 {
		t.Fatalf("infobar row count = %d, want 1", table.GetRowCount())
	}
}

func TestInfobarDisabledHidesPanel(t *testing.T) {
	original := infobarEnabled
	t.Cleanup(func() { infobarEnabled = original })

	infobarEnabled = false
	a := newFilterTestApp(t)
	mainView := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.cmd, 0, 0, false).
		AddItem(a.info, 0, 1, false).
		AddItem(a.infobar, infobarHeight(infobarEnabled), 1, false).
		AddItem(a.status, 1, 1, false)
	a.mainView = mainView
	mainView.SetRect(0, 0, 80, 24)
	mainView.Draw(tcell.NewSimulationScreen("UTF-8"))

	a.setInfobarEnabled(true)
	if !infobarEnabled {
		t.Fatal("setInfobarEnabled(true) did not enable the infobar")
	}
	if infobarHeight(false) != 0 || infobarHeight(true) != 1 {
		t.Fatalf("infobarHeight = %d/%d, want 0/1", infobarHeight(false), infobarHeight(true))
	}

	a.setInfobarEnabled(false)
	if infobarEnabled {
		t.Fatal("setInfobarEnabled(false) did not disable the infobar")
	}
}

func TestInfobarRenderMatchesCurrentView(t *testing.T) {
	appState.ViewKind = viewClusters
	table := newInfobarPanel()
	table.SetRect(0, 0, 180, 1)
	rendered := drawScreen(t, 180, 1, table.Draw)
	if !strings.Contains(rendered, "w") || !strings.Contains(rendered, "workloads") {
		t.Errorf("clusters infobar is missing the w workloads pair, rendered: %q", rendered)
	}

	appState.ViewKind = viewVMs
	sshKey := false
	for _, s := range infobarShortcuts() {
		if s.label() == "s" {
			sshKey = true
		}
	}
	if !sshKey {
		t.Error("vms infobar is missing the s ssh shortcut")
	}
	workloads := false
	for _, s := range infobarShortcuts() {
		if infobarDesc(s) == "workloads" {
			workloads = true
		}
	}
	if workloads {
		t.Error("vms infobar shows the clusters-only workloads shortcut")
	}
}

func sliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
