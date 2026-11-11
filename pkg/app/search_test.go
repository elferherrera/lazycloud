package app

import (
	"strings"
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
	"github.com/rivo/tview"
)

// seedSearchRows fills the cache with one VM in p1 and one in p2, plus a
// bucket in p1 only, so index and panel behavior has real rows to match.
func seedSearchRows(t *testing.T, a *app) {
	t.Helper()
	if err := a.cache.SaveProjects([]providers.Project{
		{ProjectID: "p1", Name: "P One", ProjectNumber: "111"},
		{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"},
	}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	if err := a.cache.SaveVMs("p1", []providers.VM{{Name: "alpha-web", Zone: "z1", Status: "RUNNING"}}); err != nil {
		t.Fatalf("SaveVMs(p1) error = %v", err)
	}
	if err := a.cache.SaveVMs("p2", []providers.VM{{Name: "beta-db", Zone: "z2", Status: "RUNNING"}}); err != nil {
		t.Fatalf("SaveVMs(p2) error = %v", err)
	}
	if err := a.cache.SaveBuckets("p1", []providers.Bucket{{Name: "alpha-bucket"}}); err != nil {
		t.Fatalf("SaveBuckets(p1) error = %v", err)
	}
}

func buildSearchIndexSync(t *testing.T, a *app) {
	t.Helper()
	if a.searchIndex == nil {
		a.searchIndex = newSearchIndex()
	}
	// Pre-mark every pull guard so the builder never reaches the network:
	// these tests exercise the cache-only index, not the background fetch.
	for _, kind := range searchKinds {
		a.searchIndex.pulled[kind] = true
	}
	a.searchIndex.projectsPulled = true
	a.ensureSearchIndex(true)
	// The builder goroutine finishes when every kind is warm; entries for
	// kinds with no cached rows come back empty, not dirty.
	idx := a.searchIndex
	for {
		idx.mu.Lock()
		done := true
		for _, kind := range searchKinds {
			if idx.dirty[kind] {
				done = false
				break
			}
		}
		building := idx.building
		idx.mu.Unlock()
		if done && !building {
			return
		}
	}
}

// newSearchTestApp returns an app with the panel built and wired into a
// search column, mirroring what run() sets up. Built on the fetch test
// app so background pulls have a logger.
func newSearchTestApp(t *testing.T) *app {
	t.Helper()
	a := newFetchTestApp(t)
	a.buildSearchPanel()
	mainView := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.summary, 4, 1, false).
		AddItem(a.cmd, 0, 0, false).
		AddItem(a.info, 0, 1, false).
		AddItem(a.status, 1, 1, false)
	a.searchColumn = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(a.searchPanel, 0, 0, false).
		AddItem(mainView, 0, 1, false)
	return a
}

func TestSearchIndexedKindMapping(t *testing.T) {
	cases := []struct {
		kind viewKind
		want viewKind
	}{
		{viewVMs, viewVMsAll},
		{viewVMsAll, viewVMsAll},
		{viewProjects, ""},
		{viewDNSRecords, ""},
		{viewBucketIAM, ""},
	}
	for _, tc := range cases {
		if got := searchIndexedKind(tc.kind); got != tc.want {
			t.Errorf("searchIndexedKind(%s) = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestParseSearchTerms(t *testing.T) {
	includes, excludes := parseSearchTerms("web alpha -db")
	if len(includes) != 2 || includes[0] != "web" || includes[1] != "alpha" {
		t.Errorf("includes = %v, want [web alpha]", includes)
	}
	if len(excludes) != 1 || excludes[0] != "db" {
		t.Errorf("excludes = %v, want [db]", excludes)
	}
}

func TestSearchMatchScopeAndExcludes(t *testing.T) {
	a := newFilterTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)

	hits := a.searchIndex.match("alpha")
	if len(hits) == 0 {
		t.Fatal("match(alpha) returned no hits")
	}
	for _, hit := range hits {
		if !strings.Contains(hit.text, "alpha") {
			t.Errorf("hit %q does not contain alpha", hit.text)
		}
	}

	hits = a.searchIndex.match("beta")
	if len(hits) != 1 {
		t.Fatalf("match(beta) = %d hits, want 1", len(hits))
	}
	if hits[0].project != "p2" {
		t.Errorf("hit project = %q, want p2", hits[0].project)
	}

	hits = a.searchIndex.match("alpha -bucket")
	for _, hit := range hits {
		if strings.Contains(hit.text, "bucket") {
			t.Errorf("excluded term matched: %q", hit.text)
		}
	}
}

func TestSearchOpenClosePreservesQuery(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)

	a.openSearch()
	a.searchInput.SetText("alpha")
	if !a.searchOpen.Load() {
		t.Fatal("searchOpen not set after openSearch")
	}
	if a.searchInput.GetText() != "alpha" {
		t.Fatalf("input text = %q, want alpha", a.searchInput.GetText())
	}
	if got := len(a.searchIndex.match("alpha")); got == 0 {
		t.Error("no hits for seeded query")
	}
	a.closeSearch()
	if a.searchOpen.Load() {
		t.Fatal("searchOpen still set after closeSearch")
	}
	if a.searchQuery != "alpha" {
		t.Fatalf("searchQuery = %q, want alpha", a.searchQuery)
	}
	a.openSearch()
	if a.searchInput.GetText() != "alpha" {
		t.Fatalf("reopened input text = %q, want alpha", a.searchInput.GetText())
	}
	a.closeSearch()
}

func TestSearchPanelGroupsResultsByKind(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)

	a.openSearch()
	a.searchInput.SetText("alpha")
	a.runSearch()

	if len(a.searchGroupRows) == 0 {
		t.Fatal("no grouped rows after search")
	}
	// "alpha" hits one VM in p1 and one bucket in p1: the panel shows one bold
	// header per kind, then that group's projects as indented subrows.
	if len(a.searchGroupRows) != 2 {
		t.Fatalf("group rows = %d, want 2 (vms + buckets)", len(a.searchGroupRows))
	}
	if a.searchGroupRows[0].kind != viewVMsAll || a.searchGroupRows[0].count != 1 {
		t.Fatalf("first group = %+v, want vms(1)", a.searchGroupRows[0])
	}
	// Two headers plus one subrow each: vms(1) with p1, buckets(1) with p1.
	wantRows := 0
	for _, group := range a.searchGroupRows {
		wantRows += 1 + len(group.projects)
	}
	if got := a.searchResults.GetRowCount(); got != wantRows {
		t.Fatalf("table rows = %d, want %d (headers + subrows)", got, wantRows)
	}
	if len(a.searchRowGroups) != wantRows {
		t.Fatalf("row->group map = %d, want %d", len(a.searchRowGroups), wantRows)
	}
	// The cell text is tag-escaped, so compare against the escaped header.
	cell := a.searchResults.GetCell(0, 0)
	if cell.Text != tview.Escape("VMs(1)") {
		t.Errorf("first header row = %q, want %q", cell.Text, tview.Escape("VMs(1)"))
	}
	sub := a.searchResults.GetCell(1, 0)
	if sub.Text != tview.Escape(searchGroupIndent+"p1 (1)") {
		t.Errorf("first subrow = %q, want indented p1 (1)", sub.Text)
	}
	if a.searchRowGroups[1] != 0 {
		t.Errorf("subrow maps to group %d, want 0", a.searchRowGroups[1])
	}
}

func TestSearchGroupSelectOpensAllViewWithPresetFilter(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)
	resetAppState()

	a.openSearch()
	a.searchInput.SetText("alpha")
	a.runSearch()

	// Selecting the VMs group opens the all-projects VM view with the query
	// preset as the live filter.
	a.searchSelect(0)
	if appState.ViewKind != viewVMsAll {
		t.Errorf("ViewKind = %q, want vms-all", appState.ViewKind)
	}
	if !a.searchOpen.Load() {
		t.Error("panel closed by group select, want it open as sidebar")
	}
	if !appState.AllProjects {
		t.Error("AllProjects not set by group select")
	}
	if a.cmd.GetText() != "alpha" {
		t.Errorf("filter text = %q, want alpha", a.cmd.GetText())
	}
	if appState.CmdMode != modeFilter {
		t.Errorf("CmdMode = %v, want filter", appState.CmdMode)
	}
	if len(a.visibleRows) != 1 {
		t.Fatalf("visibleRows = %d, want 1 (alpha-web)", len(a.visibleRows))
	}
	a.closeSearch()
}

func TestSearchGroupSelectKeepsPanelFocusStateConsistent(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)
	resetAppState()

	a.openSearch()
	a.searchInput.SetText("alpha")
	a.runSearch()
	a.searchSelect(0)
	if a.searchPanelFocus {
		t.Error("panel still focused after group select, want main view")
	}
	// S must bring the user back to the panel query input.
	a.openSearch()
	if got := a.tapp.GetFocus(); got != a.searchInput {
		t.Fatalf("focus after reopen = %T, want search input", got)
	}
}

func TestSearchGroupSelectBucketsOpensBucketsAll(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)
	resetAppState()

	a.openSearch()
	a.searchInput.SetText("alpha")
	a.runSearch()

	if len(a.searchGroupRows) < 2 {
		t.Fatalf("searchGroupRows = %d, want at least 2", len(a.searchGroupRows))
	}
	if a.searchGroupRows[1].kind != viewBucketsAll {
		t.Fatalf("second group kind = %q, want buckets-all", a.searchGroupRows[1].kind)
	}
	// Row 0 is the vms header, row 1 its p1 subrow, row 2 the buckets header.
	if got := a.searchRowGroups[2]; got != 1 {
		t.Fatalf("buckets header maps to group %d, want 1", got)
	}
	a.searchSelect(2)
	if appState.ViewKind != viewBucketsAll {
		t.Errorf("ViewKind = %q, want buckets-all", appState.ViewKind)
	}
	if len(a.visibleRows) != 1 {
		t.Errorf("visibleRows = %d, want 1 (alpha-bucket)", len(a.visibleRows))
	}
	a.closeSearch()
}

func TestSearchPreviewHeaderOpensKindViewWithQueryFilter(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)
	resetAppState()

	a.openSearch()
	a.searchInput.SetText("alpha")
	a.runSearch()

	// Moving the selector onto the VMs header previews the all-projects VM
	// view filtered to the query, without committing panel focus changes.
	a.searchPreview(0)
	if appState.ViewKind != viewVMsAll {
		t.Errorf("ViewKind = %q, want vms-all", appState.ViewKind)
	}
	if !appState.AllProjects {
		t.Error("AllProjects not set by preview")
	}
	if a.cmd.GetText() != "alpha" {
		t.Errorf("filter text = %q, want alpha", a.cmd.GetText())
	}
	if appState.CmdMode != modeFilter {
		t.Errorf("CmdMode = %v, want filter", appState.CmdMode)
	}
	if len(a.visibleRows) != 1 {
		t.Fatalf("visibleRows = %d, want 1 (alpha-web)", len(a.visibleRows))
	}
	if !a.searchPanelFocus {
		t.Error("preview moved focus off the panel")
	}
	a.closeSearch()
}

func TestSearchPreviewProjectSubrowFiltersByProject(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)
	resetAppState()

	a.openSearch()
	a.searchInput.SetText("alpha")
	a.runSearch()

	// Row 1 is the vms group's p1 subrow: the preview adds p1 to the filter
	// so only that project's matching rows stay visible.
	if got := a.searchRowProjects[1]; got != "p1" {
		t.Fatalf("row 1 project = %q, want p1", got)
	}
	a.searchPreview(1)
	if appState.ViewKind != viewVMsAll {
		t.Errorf("ViewKind = %q, want vms-all", appState.ViewKind)
	}
	want := "alpha + p1"
	if a.cmd.GetText() != want {
		t.Errorf("filter text = %q, want %q", a.cmd.GetText(), want)
	}
	if len(a.visibleRows) != 1 {
		t.Fatalf("visibleRows = %d, want 1 (alpha-web in p1)", len(a.visibleRows))
	}
	if row := a.visibleRows[0]; !strings.Contains(strings.Join(row, " "), "p1") {
		t.Errorf("visible row = %v, want a p1 row", row)
	}
	a.closeSearch()
}

func TestSearchPreviewProjectSubrowExcludesOtherProjects(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)
	resetAppState()

	a.openSearch()
	// "e" matches alpha-web and beta-db, so the vms group spans p1 and p2.
	a.searchInput.SetText("e")
	a.runSearch()

	// The vms group's rows are header, p1 subrow, p2 subrow. Previewing the
	// p2 subrow must leave only beta-db visible.
	row := a.searchResultsRowForProject(viewVMsAll, "p2")
	if row < 0 {
		t.Fatal("no p2 subrow found in the vms group")
	}
	a.searchPreview(row)
	if appState.ViewKind != viewVMsAll {
		t.Errorf("ViewKind = %q, want vms-all", appState.ViewKind)
	}
	joined := strings.Join(a.visibleRows[0], " ")
	if len(a.visibleRows) != 1 || !strings.Contains(joined, "beta-db") {
		t.Fatalf("visibleRows = %v, want only beta-db", a.visibleRows)
	}
	if a.searchRowProjects[row] != "p2" {
		t.Errorf("row %d project = %q, want p2", row, a.searchRowProjects[row])
	}
	a.closeSearch()
}

func TestSearchGroupQuerySurvivesRefresh(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)
	resetAppState()

	a.openSearch()
	a.searchInput.SetText("alpha")
	a.runSearch()
	row := a.searchGroupRows[0]

	// Retyping a new query must not make the stale row preset an old filter.
	a.searchInput.SetText("beta")
	a.runSearch()
	if len(a.searchGroupRows) == 0 {
		t.Fatal("no groups for beta query")
	}
	if got := a.searchGroupRows[0]; got.query != "beta" || got.kind != viewVMsAll {
		t.Fatalf("group after retype = %+v, want beta vms", got)
	}
	if row.query != "alpha" {
		t.Fatalf("old group query mutated = %q, want alpha", row.query)
	}
	a.closeSearch()
}

func TestSearchTabTogglesFocus(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)

	a.openSearch()
	if !a.searchPanelFocus {
		t.Fatal("panel focus not set after openSearch")
	}
	a.toggleSearchFocus()
	if a.searchPanelFocus {
		t.Fatal("panel focus still set after toggleSearchFocus")
	}
	if !a.searchOpen.Load() {
		t.Fatal("panel closed by focus toggle")
	}
	a.toggleSearchFocus()
	if !a.searchPanelFocus {
		t.Fatal("panel focus not restored by second toggle")
	}
	a.closeSearch()
}
