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

// infobarSimScreen returns an initialized simulation screen sized w x h, so
// infobar width fitting sees a known terminal.
func infobarSimScreen(t *testing.T, w, h int) tcell.Screen {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sim.Fini)
	sim.SetSize(w, h)
	return sim
}

func TestRefreshInfobarRefillsWhenScreenKnown(t *testing.T) {
	original := infobarEnabled
	t.Cleanup(func() { infobarEnabled = original })
	originalMode := appState.CmdMode
	t.Cleanup(func() { appState.CmdMode = originalMode })

	infobarEnabled = true
	appState.ViewKind = viewProjects
	appState.CmdMode = modeCmd
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
	if entries[0].label() != "w" && entries[0].label() != "n" && entries[0].label() != "s" {
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
	originalMode := appState.CmdMode
	t.Cleanup(func() { appState.CmdMode = originalMode })

	appState.ViewKind = viewClusters
	appState.CmdMode = modeCmd
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

func TestInfobarShowsCommandSuggestionsInCmdMode(t *testing.T) {
	originalMode := appState.CmdMode
	savedView := appState.ViewKind
	t.Cleanup(func() { appState.CmdMode = originalMode; appState.ViewKind = savedView })

	appState.CmdMode = modeCmd
	appState.ViewKind = viewProjects
	a := newFilterTestApp(t)
	a.screen = infobarSimScreen(t, 200, 40)
	a.tapp.SetFocus(a.cmd)
	a.refreshInfobar()
	if !strings.Contains(a.infobar.GetText(true), ":accounts") {
		t.Fatal("cmd mode infobar does not list commands")
	}
	if !strings.Contains(a.infobar.GetText(true), "show service accounts") {
		t.Fatal("cmd mode infobar does not describe the listed command")
	}
	if !strings.Contains(a.infobar.GetText(true), "more") {
		t.Fatal("cmd mode infobar does not hint at the commands left out")
	}

	a.tapp.SetFocus(a.info)
	a.refreshInfobar()
	if strings.Contains(a.infobar.GetText(true), ":accounts") {
		t.Fatal("infobar keeps listing commands once the input loses focus")
	}
}

func TestInfobarCommandsFilterAsTyped(t *testing.T) {
	view := newInfobarPanel()

	populateInfobarCommands(view, "", 200)
	text := view.GetText(true)
	if !strings.Contains(text, ":accounts") {
		t.Fatal("unfiltered command list is missing :accounts")
	}
	if !strings.Contains(text, "more") {
		t.Fatal("unfiltered command list does not hint at the overflow")
	}

	populateInfobarCommands(view, "vm", 200)
	text = view.GetText(true)
	if !strings.Contains(text, ":vms") || !strings.Contains(text, "show VMs") {
		t.Fatalf("typing vm does not surface :vms, got %q", text)
	}
	for _, hidden := range []string{":projects", ":sort", ":networks", "more"} {
		if strings.Contains(text, hidden) {
			t.Errorf("typing vm still shows %s", hidden)
		}
	}

	populateInfobarCommands(view, "VM", 200)
	if !strings.Contains(view.GetText(true), ":vms") {
		t.Fatal("command filter is not case-insensitive")
	}

	populateInfobarCommands(view, "sort asc", 200)
	if got := view.GetText(true); got != "" {
		t.Errorf("arguments typed still suggest commands, got %q", got)
	}

	populateInfobarCommands(view, "zzz", 200)
	if got := view.GetText(true); got != "" {
		t.Errorf("unmatched text still suggests commands, got %q", got)
	}
}

func TestCommandSuggestionsCoverEveryCommand(t *testing.T) {
	suggestions := commandSuggestions()
	if len(suggestions) != len(commandNames()) {
		t.Fatalf("commandSuggestions has %d entries, want one per command (%d)", len(suggestions), len(commandNames()))
	}
	byName := make(map[string]commandSuggestion, len(suggestions))
	for _, s := range suggestions {
		if byName[s.name].name != "" {
			t.Errorf("command %q suggested twice", s.name)
		}
		byName[s.name] = s
	}
	if byName["gke"].desc != byName["clusters"].desc {
		t.Errorf("alias gke desc = %q, want clusters' %q", byName["gke"].desc, byName["clusters"].desc)
	}
	if byName["vpcs"].desc != byName["networks"].desc {
		t.Errorf("alias vpcs desc = %q, want networks' %q", byName["vpcs"].desc, byName["networks"].desc)
	}
	for _, name := range []string{"sort", "write", "search", "peerings", "psc", "vms", "projects"} {
		if byName[name].desc == "" {
			t.Errorf("command %s has no suggestion description", name)
		}
	}
}

func TestInfobarRestoresShortcutsWhenCommandRuns(t *testing.T) {
	originalMode := appState.CmdMode
	savedView := appState.ViewKind
	t.Cleanup(func() { appState.CmdMode = originalMode; appState.ViewKind = savedView })

	appState.CmdMode = modeCmd
	appState.ViewKind = viewProjects
	a := newFilterTestApp(t)
	a.screen = infobarSimScreen(t, 200, 40)
	a.tapp.SetFocus(a.cmd)
	a.cmd.SetText("projects")
	if !strings.Contains(a.infobar.GetText(true), ":projects") {
		t.Fatal("typing a command does not filter the infobar suggestions")
	}

	a.commandEnter()
	if strings.Contains(a.infobar.GetText(true), ":projects") {
		t.Fatal("infobar lists commands after the command ran")
	}
}

func TestInfobarRestoresShortcutsOnEscapeFromCmdMode(t *testing.T) {
	originalMode := appState.CmdMode
	savedView := appState.ViewKind
	t.Cleanup(func() { appState.CmdMode = originalMode; appState.ViewKind = savedView })

	appState.CmdMode = modeCmd
	appState.ViewKind = viewProjects
	a := newFilterTestApp(t)
	a.screen = infobarSimScreen(t, 200, 40)
	a.tapp.SetFocus(a.cmd)
	a.refreshInfobar()
	if !strings.Contains(a.infobar.GetText(true), ":accounts") {
		t.Fatal("cmd mode infobar does not list commands")
	}

	a.escapePressed()
	if strings.Contains(a.infobar.GetText(true), ":accounts") {
		t.Fatal("infobar lists commands after Esc")
	}
}

func TestInfobarShowsFilterPathsInFilterMode(t *testing.T) {
	originalMode := appState.CmdMode
	savedView := appState.ViewKind
	t.Cleanup(func() { appState.CmdMode = originalMode; appState.ViewKind = savedView })

	appState.CmdMode = modeFilter
	appState.ViewKind = viewVMs
	a := newFilterTestApp(t)
	a.screen = infobarSimScreen(t, 200, 40)
	a.tapp.SetFocus(a.cmd)
	a.refreshInfobar()
	text := a.infobar.GetText(true)
	if !strings.Contains(text, "BootDisk") {
		t.Fatal("filter mode infobar does not list the VM detail paths")
	}
	if !strings.Contains(text, "more") {
		t.Fatal("filter mode infobar does not hint at the overflow")
	}

	a.cmd.SetText("labels")
	text = a.infobar.GetText(true)
	if !strings.Contains(text, "Labels") {
		t.Fatalf("typing labels does not reduce the list to Labels, got %q", text)
	}
	if strings.Contains(text, "ServiceAccounts") {
		t.Errorf("typing labels still shows ServiceAccounts paths, got %q", text)
	}

	a.cmd.SetText("n1")
	if got := a.infobar.GetText(true); got != "" {
		t.Errorf("plain substring with no path match still suggests paths, got %q", got)
	}
}

func TestInfobarFilterPathsReduceAsTyped(t *testing.T) {
	view := newInfobarPanel()

	populateInfobarFilterPaths(view, viewVMs, "", 200)
	text := view.GetText(true)
	if !strings.Contains(text, "BootDisk") || !strings.Contains(text, "more") {
		t.Fatalf("unfiltered VM path list is missing detail paths, got %q", text)
	}

	populateInfobarFilterPaths(view, viewVMs, "service", 200)
	text = view.GetText(true)
	if !strings.Contains(text, "ServiceAccounts.*.Email") {
		t.Fatalf("typing service does not reduce to ServiceAccounts paths, got %q", text)
	}
	if strings.Contains(text, "Labels") {
		t.Errorf("typing service still shows Labels, got %q", text)
	}

	populateInfobarFilterPaths(view, viewVMs, "LAB", 200)
	if !strings.Contains(view.GetText(true), "Labels") {
		t.Fatal("path filter is not case-insensitive")
	}

	populateInfobarFilterPaths(view, viewVMs, "Labels env=prod", 200)
	if !strings.Contains(view.GetText(true), "Labels") {
		t.Fatal("typing a path value hides the path in use")
	}

	populateInfobarFilterPaths(view, viewVMs, "- staging", 200)
	if got := view.GetText(true); got != "" {
		t.Errorf("exclusion term still suggests paths, got %q", got)
	}

	populateInfobarFilterPaths(view, viewVMs, "zzz", 200)
	if got := view.GetText(true); got != "" {
		t.Errorf("unmatched head still suggests paths, got %q", got)
	}
}

func TestInfobarFilterPathsTrackViewSwitch(t *testing.T) {
	originalMode := appState.CmdMode
	savedView := appState.ViewKind
	t.Cleanup(func() { appState.CmdMode = originalMode; appState.ViewKind = savedView })

	appState.CmdMode = modeFilter
	a := newFilterTestApp(t)
	a.screen = infobarSimScreen(t, 200, 40)
	a.setView(viewVMs)
	if !strings.Contains(a.infobar.GetText(true), "BootDisk") {
		t.Fatal("filter mode infobar does not follow the switched view's paths")
	}
	a.setView(viewProjects)
	if !strings.Contains(a.infobar.GetText(true), "ProjectNumber") {
		t.Fatal("filter mode infobar does not switch to the projects view's paths")
	}
	if strings.Contains(a.infobar.GetText(true), "BootDisk") {
		t.Fatal("filter mode infobar keeps the previous view's paths after a switch")
	}
}

func TestInfobarFilterPathsEnterAndLeave(t *testing.T) {
	originalMode := appState.CmdMode
	savedView := appState.ViewKind
	t.Cleanup(func() { appState.CmdMode = originalMode; appState.ViewKind = savedView })

	appState.ViewKind = viewVMs
	a := newFilterTestApp(t)
	a.screen = infobarSimScreen(t, 200, 40)

	a.startFilter()
	if !strings.Contains(a.infobar.GetText(true), "BootDisk") {
		t.Fatal("entering filter mode does not list the view's detail paths")
	}
	if appState.CmdMode != modeFilter {
		t.Fatalf("CmdMode after / = %v, want modeFilter", appState.CmdMode)
	}

	a.escapePressed()
	if strings.Contains(a.infobar.GetText(true), "BootDisk") {
		t.Fatal("infobar lists detail paths after leaving filter mode")
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

func TestInfobarFilterPathsGateWhileSearchOpen(t *testing.T) {
	originalMode := appState.CmdMode
	savedView := appState.ViewKind
	t.Cleanup(func() { appState.CmdMode = originalMode; appState.ViewKind = savedView })

	appState.CmdMode = modeFilter
	appState.ViewKind = viewVMs
	a := newFilterTestApp(t)
	a.screen = infobarSimScreen(t, 200, 40)

	a.refreshInfobar()
	if !strings.Contains(a.infobar.GetText(true), "BootDisk") {
		t.Fatal("filter mode infobar does not list the VM detail paths")
	}

	a.searchOpen.Store(true)
	t.Cleanup(func() { a.searchOpen.Store(false) })
	a.refreshInfobar()
	text := a.infobar.GetText(true)
	if strings.Contains(text, "BootDisk") {
		t.Fatal("infobar lists detail paths while the search panel is open")
	}
	if text == "" {
		t.Fatal("infobar goes blank when the search panel opens in filter mode")
	}

	a.tapp.SetFocus(a.cmd)
	t.Cleanup(func() { a.tapp.SetFocus(a.info) })
	a.refreshInfobar()
	if !strings.Contains(a.infobar.GetText(true), "BootDisk") {
		t.Fatal("infobar does not return to detail paths when the bottom input is focused with the search panel open")
	}
}

func TestInfobarSearchPreviewKeepsShortcuts(t *testing.T) {
	originalMode := appState.CmdMode
	savedView := appState.ViewKind
	t.Cleanup(func() { appState.CmdMode = originalMode; appState.ViewKind = savedView })

	appState.CmdMode = modeCmd
	appState.ViewKind = viewProjects
	a := newFilterTestApp(t)
	a.screen = infobarSimScreen(t, 200, 40)
	a.buildMainLayout(a.helpRow)
	a.setView(viewProjects)
	a.searchIndex = newSearchIndex()
	a.openSearch()
	waitFor(t, a, func() bool { return a.searchIndex.warmCount() == len(searchKinds) })

	a.searchInput.SetText("other")
	if !a.searchOpen.Load() {
		t.Fatal("openSearch did not open the search panel")
	}
	if len(a.searchRowGroups) == 0 {
		t.Fatal("seeded VMs produced no search hits")
	}
	if appState.CmdMode != modeFilter || a.cmd.GetText() != "other" {
		t.Fatalf("searchPreview did not land the query, mode=%v cmd=%q", appState.CmdMode, a.cmd.GetText())
	}
	text := stripColorTags(a.infobar.GetText(true))
	if text == "" {
		t.Fatal("infobar goes blank while search results preview")
	}
	if !strings.Contains(text, "update") {
		t.Fatalf("infobar does not show the shortcuts line while search results preview, got %q", text)
	}
}
