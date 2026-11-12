package app

import (
	"strings"
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// infobarFittedAt returns the fitted shortcuts for a view at a budget,
// dropping the drop count tests here do not assert on.
func infobarFittedAt(t *testing.T, budget int) []shortcut {
	t.Helper()
	fitted, _ := infobarShortcuts(budget, "")
	return fitted
}

func TestRefreshInfobarRefillsWhenScreenKnown(t *testing.T) {
	original := infobarEnabled
	t.Cleanup(func() { infobarEnabled = original })

	infobarEnabled = true
	appState.ViewKind = viewProjects
	a := newFilterTestApp(t)

	a.refreshInfobar()
	if !strings.Contains(a.infobar.GetText(true), "more in ?") {
		t.Fatal("screenless render does not mark dropped entries")
	}

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(200, 40)
	a.screen = sim
	a.refreshInfobar()
	if strings.Contains(a.infobar.GetText(true), "more in ?") {
		t.Fatal("wide-screen render still truncated")
	}
	if a.infobarRenderedWidth != 200 {
		t.Fatalf("infobarRenderedWidth = %d, want 200", a.infobarRenderedWidth)
	}
}

func TestInfobarShowsVisibleShortcutsPerView(t *testing.T) {
	appState.ViewKind = viewProjects
	projects, dropped := infobarShortcuts(infobarBudgetFor(200), "")
	if len(projects) == 0 {
		t.Fatal("infobar shows no shortcuts in projects view")
	}
	if dropped != 0 {
		t.Errorf("200-col budget dropped %d projects-view entries, want them to fit", dropped)
	}
	for _, s := range projects {
		if s.key >= '0' && s.key <= '9' {
			t.Errorf("infobar shows numeric shortcut %q", string(s.key))
		}
		if s.visible != nil && !s.visible() {
			t.Errorf("infobar shows shortcut %q hidden in projects view", s.label())
		}
	}
	line := infobarLine(projects, dropped)
	for _, s := range projects {
		if !strings.Contains(line, s.label()) {
			t.Errorf("infobar line is missing key %q", s.label())
		}
		if !strings.Contains(line, infobarDesc(s)) {
			t.Errorf("infobar line is missing description %q for key %q", infobarDesc(s), s.label())
		}
	}
}

func TestInfobarShowsPanelShortcutsBeforeGlobals(t *testing.T) {
	appState.ViewKind = viewClusters
	entries, _ := infobarShortcuts(infobarBudgetFor(80), "")
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
	chartEntries, _ := infobarShortcuts(infobarBudgetFor(80), "")
	for _, s := range chartEntries {
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
	for _, s := range infobarFittedAt(t, infobarBudgetFor(80)) {
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
	for _, s := range infobarFittedAt(t, infobarBudgetFor(80)) {
		if seen[s.label()] {
			t.Errorf("infobar shows duplicate key %q", s.label())
		}
		seen[s.label()] = true
	}
}

func TestInfobarDedupKeepsFirstEntry(t *testing.T) {
	originalShortcuts := shortcuts
	t.Cleanup(func() { shortcuts = originalShortcuts })

	appState.ViewKind = viewProjects
	duplicate := shortcut{key: 'q', desc: "duplicate quit"}
	shortcuts = append(append([]shortcut(nil), shortcuts...), duplicate)

	entries, _ := infobarShortcuts(200, "")
	quitCount := 0
	for _, s := range entries {
		if s.label() == "q" {
			quitCount++
		}
	}
	if quitCount != 1 {
		t.Fatalf("infobar shows %d q entries, want the first only", quitCount)
	}
	if len(entries) > 0 && entries[len(entries)-1].desc == duplicate.desc {
		t.Error("infobar kept the duplicate entry instead of the first")
	}
}

func TestInfobarBudgetFollowsScreenWidth(t *testing.T) {
	if got := infobarBudgetFor(140); got != 138 {
		t.Errorf("infobarBudgetFor(140) = %d, want 138", got)
	}
	if got := infobarBudgetFor(60); got != 58 {
		t.Errorf("infobarBudgetFor(60) = %d, want 58", got)
	}
	if got := infobarBudgetFor(0); got != 20 {
		t.Errorf("infobarBudgetFor(0) = %d, want 20 (minimum)", got)
	}
}

func TestInfobarSkipsPairsOverBudget(t *testing.T) {
	appState.ViewKind = viewProjects
	if got, _ := infobarShortcuts(0, ""); len(got) != 0 {
		t.Fatalf("infobar with zero budget shows %d shortcuts, want 0", len(got))
	}
	if got, _ := infobarShortcuts(infobarBudgetFor(80), ""); len(got) == 0 {
		t.Fatal("restored budget shows no shortcuts")
	}
}

func TestInfobarReportsDroppedEntries(t *testing.T) {
	appState.ViewKind = viewProjects
	entries, dropped := infobarShortcuts(40, "")
	if len(entries) == 0 {
		t.Fatal("40-char budget shows nothing at all")
	}
	if dropped == 0 {
		t.Fatal("40-char budget reports no dropped entries, want the overflow counted")
	}
	line := infobarLine(entries, dropped)
	if !strings.Contains(line, "+") || !strings.Contains(line, "more in ?") {
		t.Errorf("infobar line missing dropped hint, got %q", line)
	}
}

func TestInfobarGlobalRankKnownAndUnknown(t *testing.T) {
	if got := infobarGlobalRank("u"); got != 0 {
		t.Errorf("infobarGlobalRank(u) = %d, want 0", got)
	}
	if got := infobarGlobalRank("F1"); got != len(infobarGlobalOrder)-1 {
		t.Errorf("infobarGlobalRank(F1) = %d, want %d", got, len(infobarGlobalOrder)-1)
	}
	if got := infobarGlobalRank("no-such-key"); got != len(infobarGlobalOrder) {
		t.Errorf("infobarGlobalRank(unknown) = %d, want %d", got, len(infobarGlobalOrder))
	}
	if infobarGlobalRank("u") >= infobarGlobalRank("q") {
		t.Error("infobarGlobalRank does not order u before q")
	}
}

func TestInfobarDescFallsBackToFullDesc(t *testing.T) {
	s := shortcut{key: 'z', desc: "brand new action with no short form"}
	if got := infobarDesc(s); got != s.desc {
		t.Fatalf("infobarDesc() = %q, want full desc %q", got, s.desc)
	}
	known := shortcut{key: 'u', desc: "update cache"}
	if got := infobarDesc(known); got != "update" {
		t.Fatalf("infobarDesc() = %q, want %q", got, "update")
	}
}

func TestInfobarFitsCharBudget(t *testing.T) {
	for _, kind := range []viewKind{viewProjects, viewVMs, viewClusters, viewWorkloads, viewRoutes, viewDNSRecords} {
		appState.ViewKind = kind
		for _, width := range []int{60, 80, 120, 160} {
			budget := infobarBudgetFor(width)
			used := 0
			for _, s := range infobarFittedAt(t, budget) {
				used += len(s.label()) + 1 + len(infobarDesc(s)) + 1
			}
			if used > budget {
				t.Errorf("infobar in view %q at width %d uses %d chars, budget %d", kind, width, used, budget)
			}
		}
	}
}

func TestInfobarRendersOneRow(t *testing.T) {
	appState.ViewKind = viewProjects
	view := newInfobarPanel()
	if strings.Contains(view.GetText(true), "\n") {
		t.Fatal("infobar renders more than one row")
	}
}

func TestInfobarDisabledHidesPanel(t *testing.T) {
	original := infobarEnabled
	t.Cleanup(func() { infobarEnabled = original })

	infobarEnabled = false
	a := newFilterTestApp(t)
	rootView := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.cmd, 0, 0, false).
		AddItem(a.info, 0, 1, false).
		AddItem(a.infobar, infobarHeight(infobarEnabled), 1, false).
		AddItem(a.status, 1, 1, false)
	a.rootView = rootView
	rootView.SetRect(0, 0, 80, 24)
	rootView.Draw(tcell.NewSimulationScreen("UTF-8"))

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

func TestSetInfobarEnabledNoOpWhenUnchanged(t *testing.T) {
	original := infobarEnabled
	t.Cleanup(func() { infobarEnabled = original })

	a := newFilterTestApp(t)
	infobarEnabled = true
	a.setInfobarEnabled(true)
	if !infobarEnabled {
		t.Fatal("setInfobarEnabled(true) flipped the state")
	}
	infobarEnabled = false
	a.setInfobarEnabled(false)
	if infobarEnabled {
		t.Fatal("setInfobarEnabled(false) flipped the state")
	}
}

func TestSetInfobarEnabledWithoutRootView(t *testing.T) {
	original := infobarEnabled
	t.Cleanup(func() { infobarEnabled = original })

	a := newFilterTestApp(t)
	infobarEnabled = true
	a.setInfobarEnabled(false)
	if infobarEnabled {
		t.Fatal("setInfobarEnabled(false) with nil rootView did not disable the infobar")
	}
	a.setInfobarEnabled(true)
	if !infobarEnabled {
		t.Fatal("setInfobarEnabled(true) with nil rootView did not enable the infobar")
	}
}

func TestRefreshInfobarSkippedWhenDisabled(t *testing.T) {
	original := infobarEnabled
	t.Cleanup(func() { infobarEnabled = original })

	appState.ViewKind = viewProjects
	a := newFilterTestApp(t)
	infobarEnabled = false
	before := a.infobar.GetText(true)
	a.setView(viewProjects)
	if got := a.infobar.GetText(true); got != before {
		t.Fatalf("disabled infobar repopulated, text %q -> %q", before, got)
	}
}

func TestRefreshInfobarRepopulatesWhenEnabled(t *testing.T) {
	original := infobarEnabled
	t.Cleanup(func() { infobarEnabled = original })

	appState.ViewKind = viewClusters
	a := newFilterTestApp(t)
	infobarEnabled = true
	a.infobar.SetText("")
	a.setView(viewClusters)
	if a.infobar.GetText(true) == "" {
		t.Fatal("enabled infobar stayed empty after setView")
	}
}

func TestInfobarRenderMatchesCurrentView(t *testing.T) {
	appState.ViewKind = viewClusters
	view := newInfobarPanel()
	populateInfobarPanel(view, 180)
	view.SetRect(0, 0, 180, 1)
	rendered := drawScreen(t, 180, 1, view.Draw)
	if !strings.Contains(rendered, "w") || !strings.Contains(rendered, "workloads") {
		t.Errorf("clusters infobar is missing the w workloads pair, rendered: %q", rendered)
	}

	appState.ViewKind = viewVMs
	sshKey := false
	for _, s := range infobarFittedAt(t, infobarBudgetFor(80)) {
		if s.label() == "s" {
			sshKey = true
		}
	}
	if !sshKey {
		t.Error("vms infobar is missing the s ssh shortcut")
	}
	workloads := false
	for _, s := range infobarFittedAt(t, infobarBudgetFor(80)) {
		if infobarDesc(s) == "workloads" {
			workloads = true
		}
	}
	if workloads {
		t.Error("vms infobar shows the clusters-only workloads shortcut")
	}
}

func TestInfobarSortAlphabeticalOrdersByLabel(t *testing.T) {
	appState.ViewKind = viewClusters
	entries, _ := infobarShortcuts(200, providers.InfobarSortAlpha)
	if len(entries) < 2 {
		t.Fatal("not enough shortcuts to check alphabetical order")
	}
	for i := 1; i < len(entries); i++ {
		if entries[i-1].label() > entries[i].label() {
			t.Errorf("alphabetical infobar out of order: %q before %q", entries[i-1].label(), entries[i].label())
		}
	}
}

func TestInfobarSortCurrentKeepsPanelFirst(t *testing.T) {
	appState.ViewKind = viewClusters
	entries, _ := infobarShortcuts(200, providers.InfobarSortCurrent)
	if len(entries) == 0 {
		t.Fatal("infobar shows no shortcuts in clusters view")
	}
	if entries[0].label() != "w" && entries[0].label() != "n" {
		t.Errorf("current infobar starts with %q, want a panel shortcut first", entries[0].label())
	}
}

func TestInfobarSortUnknownFallsBackToCurrent(t *testing.T) {
	appState.ViewKind = viewClusters
	fallback, _ := infobarShortcuts(200, "bogus")
	current, _ := infobarShortcuts(200, providers.InfobarSortCurrent)
	if len(fallback) != len(current) {
		t.Fatalf("fallback kept %d entries, current has %d", len(fallback), len(current))
	}
	for i := range fallback {
		if fallback[i].label() != current[i].label() {
			t.Fatalf("fallback order diverges from current at %d: %q vs %q", i, fallback[i].label(), current[i].label())
		}
	}
}

func TestInfobarSortOrderSetBeforeRender(t *testing.T) {
	original := infobarSortOrder
	t.Cleanup(func() { infobarSortOrder = original })

	infobarSortOrder = providers.InfobarSortAlpha
	view := newInfobarPanel()
	populateInfobarPanel(view, 200)
	text := view.GetText(true)
	if text == "" {
		t.Fatal("infobar renders empty with alphabetical order")
	}
	entries, _ := infobarShortcuts(200, providers.InfobarSortAlpha)
	if len(entries) == 0 {
		t.Fatal("no shortcuts fitted to compare against the render")
	}
	first := entries[0]
	want := first.label() + " " + infobarDesc(first)
	if !strings.HasPrefix(text, want) {
		t.Errorf("rendered infobar starts with %q, want the alphabetically first pair %q", text[:len(want)], want)
	}
}

func TestInfobarEveryKeyHasDescription(t *testing.T) {
	for _, kind := range []viewKind{viewProjects, viewVMs, viewClusters, viewWorkloads, viewRoutes, viewDNSRecords} {
		appState.ViewKind = kind
		line := infobarLine(infobarFittedAt(t, infobarBudgetFor(120)), 0)
		for _, pair := range strings.Split(line, "  ") {
			pair = strings.TrimSpace(stripColorTags(pair))
			if pair == "" {
				continue
			}
			key, desc, ok := strings.Cut(pair, " ")
			if !ok || key == "" || desc == "" {
				t.Errorf("view %q infobar pair %q has no key or no description", kind, pair)
			}
		}
	}
}

func stripColorTags(s string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '[':
			inTag = true
		case r == ']':
			inTag = false
		case !inTag:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
