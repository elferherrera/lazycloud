package app

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// searchEntry is one indexed resource row: the view it came from, the row
// as rendered by that view, the project the row belongs to, the scope for
// the cluster/zone/bucket-scoped views (empty for project rows), and the
// row text pre-lowered for matching.
type searchEntry struct {
	kind    viewKind
	row     []string
	project string
	scope   string
	text    string
}

// searchIndex is a lazily built, in-memory index over the cached data of the
// All views (the cache-only variants of every resource list). It exists for
// the lifetime of the app. Entries are built in the background because view
// loads are cache-only, and re-marked dirty whenever a cache save lands. The
// state is guarded by one mutex because reads (matching, from the UI thread)
// and writes (building, from a background goroutine) race.
type searchIndex struct {
	mu             sync.Mutex
	entries        map[viewKind][]searchEntry
	warm           map[viewKind]bool
	dirty          map[viewKind]bool
	pulled         map[viewKind]bool
	projectsPulled bool
	building       bool
}

func newSearchIndex() *searchIndex {
	return &searchIndex{
		entries: map[viewKind][]searchEntry{},
		warm:    map[viewKind]bool{},
		dirty:   map[viewKind]bool{},
		pulled:  map[viewKind]bool{},
	}
}

// searchKinds lists the views the index covers, in build order. The most
// commonly searched resources come first so progressive results are useful
// early. The last three are the cluster/zone/bucket-scoped views: they have
// no All variant, so their entries carry the scope and their subrows land
// the scoped view with the scope preset.
var searchKinds = []viewKind{
	viewVMsAll,
	viewIPsAll,
	viewNetworksAll,
	viewBucketsAll,
	viewLBsAll,
	viewClustersAll,
	viewInstanceGroupsAll,
	viewInstanceTemplatesAll,
	viewDisksAll,
	viewSnapshotsAll,
	viewImagesAll,
	viewRoutesAll,
	viewFirewallsAll,
	viewPeeringsAll,
	viewAccountsAll,
	viewDNSZonesAll,
	viewCloudRoutersAll,
	viewVPNsAll,
	viewPSCAll,
	viewReservationsAll,
	viewIAMAll,
	viewWorkloads,
	viewDNSRecords,
	viewBucketIAM,
}

// searchScopedKind reports whether a kind is one of the scoped views the
// index covers: entries carry a scope (cluster, zone or bucket name) and
// landing one applies that scope to the view instead of the All-projects
// toggle.
func searchScopedKind(kind viewKind) bool {
	switch kind {
	case viewWorkloads, viewDNSRecords, viewBucketIAM:
		return true
	}
	return false
}

// searchIndexedKind maps any view kind to the indexed kind its cache feeds.
// Per-project kinds share the cache tables of their All twin, so indexing
// the All views covers both. The scoped views are indexed under their own
// kind. Kinds that are neither indexed nor cache-fed by an indexed view
// are not indexed.
func searchIndexedKind(kind viewKind) viewKind {
	switch kind {
	case viewWorkloads, viewDNSRecords, viewBucketIAM:
		return kind
	case viewProjects, viewPublicImages, viewNodePools:
		return ""
	}
	if strings.HasSuffix(string(kind), "-all") {
		return kind
	}
	return viewKind(string(kind) + "-all")
}

// invalidate marks the entries for kind stale. The next ensureSearchIndex
// rebuilds them.
func (idx *searchIndex) invalidate(kind viewKind) {
	if kind == "" {
		return
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.warm[kind] = false
	idx.dirty[kind] = true
}

// ensureSearchIndex queues a background build of every cold or invalidated
// kind. With force it rebuilds all kinds. Safe to call repeatedly: while a
// build runs, new dirty marks are picked up by the running loop.
func (a *app) ensureSearchIndex(force bool) {
	idx := a.searchIndex
	if idx == nil {
		return
	}
	idx.mu.Lock()
	for _, kind := range searchKinds {
		if force || !idx.warm[kind] {
			idx.dirty[kind] = true
		}
	}
	if idx.building {
		idx.mu.Unlock()
		return
	}
	idx.building = true
	idx.mu.Unlock()
	a.goBackground(a.buildSearchIndex)
}

// buildSearchIndex builds every dirty kind one at a time. After each kind it
// re-runs the live search so results appear progressively while the rest of
// the index is still building. Load errors leave the kind cold and are not
// retried in this pass (a persistent error must not spin the loop); the next
// open or invalidation retries it.
//
// A kind with no cached entries at all is assumed to be missing data, not
// empty infrastructure: the builder pulls it in the background (one pass per
// kind per app run, so a project without VMs does not refetch forever). The
// pull saves through the normal cache path, which invalidates the kind and
// queues this loop again, so results keep arriving while search is open.
func (a *app) buildSearchIndex() {
	idx := a.searchIndex
	for {
		idx.mu.Lock()
		var kind viewKind
		for _, k := range searchKinds {
			if idx.dirty[k] {
				kind = k
				break
			}
		}
		if kind == "" {
			idx.building = false
			idx.mu.Unlock()
			return
		}
		delete(idx.dirty, kind)
		idx.mu.Unlock()

		entries, err := a.loadSearchEntries(kind)
		idx.mu.Lock()
		if err != nil {
			idx.entries[kind] = nil
			idx.warm[kind] = false
		} else {
			idx.entries[kind] = entries
			idx.warm[kind] = true
			if len(entries) == 0 && !idx.pulled[kind] {
				idx.pulled[kind] = true
				a.goBackground(func() { a.pullSearchKind(kind) })
			}
		}
		idx.mu.Unlock()

		if a.searchOpen.Load() {
			a.queueUpdateDraw(a.runSearch)
		}
	}
}

// pullSearchKind fetches the kind's resources for every cached project in the
// background and saves them through the view's normal fetch path. It runs
// outside idx.mu because network calls are slow; markViewUpdated re-arms the
// builder when the saves land.
func (a *app) pullSearchKind(kind viewKind) {
	if searchScopedKind(kind) {
		a.pullScopedSearchKind(kind)
		return
	}
	projects, err := a.cache.ListProjects()
	if err != nil || len(projects) == 0 {
		return
	}
	perKind := viewKind(strings.TrimSuffix(string(kind), "-all"))
	def, ok := views[perKind]
	if !ok || def.fetch == nil {
		return
	}
	label := views[kind].title
	for _, proj := range projects {
		if _, _, err := def.fetch(a, proj.ProjectID); err != nil {
			a.reportError(string(perKind), proj.ProjectID, "Search pull failed for "+label, err)
		}
	}
}

// pullScopedSearchKind fetches one scoped kind across every cached scope:
// workloads per cached cluster (gcloud/kubectl shell-out), DNS records per
// cached zone, bucket IAM per cached bucket. The saves land through
// markViewUpdated, which invalidates the kind's index entries and re-arms
// the builder, so hits keep arriving while search is open. Failures are
// logged per scope, not fatal.
func (a *app) pullScopedSearchKind(kind viewKind) {
	switch kind {
	case viewWorkloads:
		clusters, err := a.cache.ListAllClusters()
		if err != nil {
			return
		}
		for _, cl := range clusters {
			workloads, err := listClusterWorkloads(cl.ProjectID, cl.Location, cl.Name)
			if err != nil {
				a.reportError(string(kind), cl.ProjectID+"/"+cl.Name, "Search pull failed for Workloads", err)
				continue
			}
			if err := a.cache.SaveWorkloads(cl.ProjectID, cl.Name, workloads); err != nil {
				a.reportError(string(kind), cl.ProjectID+"/"+cl.Name, "Search pull failed for Workloads", err)
				continue
			}
			a.markViewUpdated(viewWorkloads, cl.ProjectID)
		}
	case viewDNSRecords:
		zones, err := a.cache.ListAllDNSZones()
		if err != nil {
			return
		}
		for _, z := range zones {
			records, err := listZoneDNSRecords(z.ProjectID, z.Name)
			if err != nil {
				a.reportError(string(kind), z.ProjectID+"/"+z.Name, "Search pull failed for DNS Records", err)
				continue
			}
			if err := a.cache.SaveDNSRecords(z.ProjectID, z.Name, records); err != nil {
				a.reportError(string(kind), z.ProjectID+"/"+z.Name, "Search pull failed for DNS Records", err)
				continue
			}
			a.markViewUpdated(viewDNSRecords, z.ProjectID)
		}
	case viewBucketIAM:
		buckets, err := a.cache.ListAllBuckets()
		if err != nil {
			return
		}
		for _, b := range buckets {
			bindings, err := listBucketIAMPolicy(b.ProjectID, b.Name)
			if err != nil {
				a.reportError(string(kind), b.ProjectID+"/"+b.Name, "Search pull failed for Bucket IAM policy", err)
				continue
			}
			if err := a.cache.SaveBucketIAMBindings(b.ProjectID, b.Name, bindings); err != nil {
				a.reportError(string(kind), b.ProjectID+"/"+b.Name, "Search pull failed for Bucket IAM policy", err)
				continue
			}
			a.markViewUpdated(viewBucketIAM, b.ProjectID)
		}
	}
}

// ensureSearchProjects bootstraps an empty project cache: without a project
// list there is nothing to index and nothing to pull per project. One attempt
// per app run, in the background; the save lands through markViewUpdated,
// which re-arms the index builder. It must not deadlock the UI: cache saves
// from a background goroutine are the same pattern allProjectsRefresh uses.
func (a *app) ensureSearchProjects() {
	idx := a.searchIndex
	if idx == nil {
		return
	}
	idx.mu.Lock()
	if idx.projectsPulled {
		idx.mu.Unlock()
		return
	}
	idx.projectsPulled = true
	idx.mu.Unlock()
	a.goBackground(func() {
		projects, err := listGcloudProjects()
		if err != nil {
			a.reportError("projects", "all", "Search failed to fetch projects", err)
			return
		}
		if err := a.cache.SaveProjects(projects); err != nil {
			a.reportError("projects", "all", "Search failed to update cache", err)
			return
		}
		a.ensureSearchIndex(true)
	})
}

// loadSearchEntries reads one view through its cache-only load. The last
// column of every All view is the project the row belongs to; the scoped
// views read their whole cache table and carry the scope instead.
func (a *app) loadSearchEntries(kind viewKind) ([]searchEntry, error) {
	if searchScopedKind(kind) {
		return a.loadScopedSearchEntries(kind)
	}
	def := views[kind]
	rows, err := def.load(a)
	if err != nil {
		return nil, err
	}
	perProject := len(def.headers) > 0 && def.headers[len(def.headers)-1] == "Project"
	entries := make([]searchEntry, 0, len(rows))
	for _, row := range rows {
		project := ""
		if perProject && len(row) > 0 {
			project = row[len(row)-1]
		}
		entries = append(entries, searchEntry{
			kind:    kind,
			row:     row,
			project: project,
			text:    strings.ToLower(strings.Join(row, " ")),
		})
	}
	return entries, nil
}

// loadScopedSearchEntries indexes one scoped view across every cached scope:
// workloads across clusters, DNS records across zones, bucket IAM across
// buckets. The scope lands in the entry so a subrow can reopen the view at
// the right cluster/zone/bucket.
func (a *app) loadScopedSearchEntries(kind viewKind) ([]searchEntry, error) {
	var entries []searchEntry
	add := func(rows [][]string, project, scope string) {
		if entries == nil {
			entries = make([]searchEntry, 0, len(rows))
		}
		for _, row := range rows {
			entries = append(entries, searchEntry{
				kind:    kind,
				row:     row,
				project: project,
				scope:   scope,
				text:    strings.ToLower(strings.Join(row, " ")),
			})
		}
	}
	switch kind {
	case viewWorkloads:
		workloads, err := a.cache.ListAllWorkloads()
		if err != nil {
			return nil, err
		}
		rows := workloadRows(workloads)
		for i := range rows {
			add(rows[i:i+1], workloads[i].ProjectID, workloads[i].Cluster)
		}
	case viewDNSRecords:
		records, err := a.cache.ListAllDNSRecords()
		if err != nil {
			return nil, err
		}
		rows := dnsRecordRows(records)
		for i := range rows {
			add(rows[i:i+1], records[i].ProjectID, records[i].Zone)
		}
	case viewBucketIAM:
		bindings, err := a.cache.ListAllBucketIAMBindings()
		if err != nil {
			return nil, err
		}
		rows := bucketIAMRows(bindings)
		for i := range rows {
			add(rows[i:i+1], bindings[i].ProjectID, bindings[i].Bucket)
		}
	}
	return entries, nil
}

const maxSearchHits = 100

// parseSearchTerms splits a query into lowered include terms and exclude
// terms ("-x" must not match). Every include must match, no exclude may.
func parseSearchTerms(query string) (includes, excludes []string) {
	for _, term := range strings.Fields(query) {
		if strings.HasPrefix(term, "-") && len(term) > 1 {
			excludes = append(excludes, strings.ToLower(term[1:]))
		} else {
			includes = append(includes, strings.ToLower(term))
		}
	}
	return includes, excludes
}

func searchEntryMatches(text string, includes, excludes []string) bool {
	for _, term := range includes {
		if !strings.Contains(text, term) {
			return false
		}
	}
	for _, term := range excludes {
		if strings.Contains(text, term) {
			return false
		}
	}
	return true
}

// match returns the hits for query. It reads the index under lock; a kind
// still building simply has no entries yet. Results are capped just above the
// display limit so the title can say "more than N".
func (idx *searchIndex) match(query string) []searchEntry {
	return idx.matchLimit(query, maxSearchHits+1)
}

// matchAll returns every hit for query with no cap, so write's global-search
// mode exports all occurrences of the match, not just the first page of results.
func (idx *searchIndex) matchAll(query string) []searchEntry {
	return idx.matchLimit(query, 0)
}

// matchLimit collects the hits of every warm kind for query, stopping once
// limit entries have been collected; a zero limit collects everything.
func (idx *searchIndex) matchLimit(query string, limit int) []searchEntry {
	includes, excludes := parseSearchTerms(query)
	idx.mu.Lock()
	defer idx.mu.Unlock()
	var hits []searchEntry
	for _, kind := range searchKinds {
		for _, e := range idx.entries[kind] {
			if !searchEntryMatches(e.text, includes, excludes) {
				continue
			}
			hits = append(hits, e)
			if limit > 0 && len(hits) >= limit {
				return hits
			}
		}
	}
	return hits
}

// searchHitsPayload groups the query's hits the way the results panel does,
// nested one level deeper: the view title first, then the project, then the
// list of matching components. Every component is resolved to its full
// details: the cached resource, wrapped like the y copy payload (resource
// fields plus the Columns map of the rendered row cells).
func (a *app) searchHitsPayload(hits []searchEntry) (map[string]map[string][]any, error) {
	payload := map[string]map[string][]any{}
	for _, hit := range hits {
		details, err := a.searchEntryDetails(hit)
		if err != nil {
			return nil, err
		}
		view := views[hit.kind].title
		projects, ok := payload[view]
		if !ok {
			projects = map[string][]any{}
			payload[view] = projects
		}
		projects[hit.project] = append(projects[hit.project], details)
	}
	return payload, nil
}

// searchEntryDetails resolves one search hit to its cached resource. The All
// views' resource hooks read the project from the row itself; the scoped
// kinds' hooks read the focused appState instead, so they are resolved
// directly with the hit's project and scope.
func (a *app) searchEntryDetails(hit searchEntry) (any, error) {
	var res any
	var err error
	switch hit.kind {
	case viewWorkloads:
		res, err = workloadResource(a, hit.project, hit.scope, hit.row[1], hit.row[0], hit.row[2])
	case viewDNSRecords:
		res, err = dnsRecordResource(a, hit.project, hit.scope, hit.row[0], hit.row[1])
	case viewBucketIAM:
		res, err = bucketIAMResource(a, hit.project, hit.scope, hit.row[0], hit.row[2])
	default:
		def := views[hit.kind]
		if def.resource == nil {
			return rowCellsMap(def.headers, hit.row), nil
		}
		res, err = def.resource(a, hit.row)
	}
	if err != nil {
		return nil, err
	}
	return rowCopyPayload(views[hit.kind].headers, hit.row, res)
}

func (idx *searchIndex) warmCount() int {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	count := 0
	for _, kind := range searchKinds {
		if idx.warm[kind] {
			count++
		}
	}
	return count
}

// searchPanelWidth is the fixed width the left panel takes from the main
// view while search is open.
const searchPanelWidth = 46

// buildSearchPanel creates the left search panel: a query input on top and a
// results table below, stacked so they share the column width. The panel
// lives in searchColumn next to the main view; opening search resizes that
// column so the main view shrinks.
func (a *app) buildSearchPanel() {
	input := tview.NewInputField().SetLabel(" / ").SetFieldWidth(0)
	input.SetFieldStyle(tcell.StyleDefault.Foreground(tview.Styles.PrimaryTextColor).Background(tview.Styles.PrimitiveBackgroundColor))
	input.SetBorder(true)
	results := tview.NewTable().SetBorders(false).SetSelectable(true, false).SetSeparator('│')
	results.SetBorder(true)
	a.searchInput = input
	a.searchResults = results

	input.SetChangedFunc(func(text string) { a.runSearch() })
	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter && results.GetRowCount() > 0 {
			a.tapp.SetFocus(results)
		}
	})
	results.SetSelectionChangedFunc(func(row, _ int) { a.searchPreview(row) })
	results.SetSelectedFunc(func(row, _ int) { a.searchSelect(row) })
	// Focus moves through more than the Tab cycle (Enter on the input jumps
	// to the results, select lands on the main view), so the focus accent
	// follows via the focus hooks. Blur hooks must not read the app's focus
	// primitive: tview fires Blur inside the locked section of SetFocus, so
	// they only restore their own border.
	input.SetFocusFunc(func() { a.updateFocusStyles() })
	input.SetBlurFunc(func() { a.searchInput.SetBorderColor(theme.borderColor()) })
	results.SetFocusFunc(func() { a.updateFocusStyles() })
	results.SetBlurFunc(func() { a.searchResults.SetBorderColor(theme.borderColor()) })

	panel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(input, 3, 1, true).
		AddItem(results, 0, 1, true)
	a.searchPanel = panel
}

// buildMainLayout wires the shared widget tree run() mounts: the header row
// on top spanning the full width, the status bar right under it, then the
// search side panel next to the main view in a zero-width column, with the
// infobar spanning the full width at the bottom. It exists apart from run()
// so tests can exercise the real wiring without starting the terminal loop.
func (a *app) buildMainLayout(helpOverlay *tview.Flex) {
	a.buildSearchPanel()
	a.info.SetFocusFunc(func() { a.updateFocusStyles() })
	a.info.SetBlurFunc(func() { a.info.SetBorderColor(theme.borderColor()) })

	// The header row spans the full width; the search panel splits only the
	// area below it, so the project context stays visible while searching.
	summary := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(a.summary, 0, 1, false).
		AddItem(a.shortcutsSummary, 0, 1, false)

	mainView := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.cmd, 0, 0, false).
		AddItem(a.info, 0, 1, false)
	a.mainView = mainView

	// The search column stays zero-width until search opens, then takes a
	// fixed slice so the main view shrinks instead of being covered.
	searchColumn := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(a.searchPanel, 0, 0, false).
		AddItem(mainView, 0, 1, true)
	a.searchColumn = searchColumn

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(summary, 4, 1, false).
		AddItem(a.status, 0, 0, false).
		AddItem(searchColumn, 0, 1, true).
		AddItem(a.infobar, infobarHeight(infobarEnabled), infobarProportion(infobarEnabled), false)
	a.rootView = root

	a.pages.AddPage("main", root, true, true)
	a.pages.AddPage("help", helpOverlay, true, false)
}

// searchPlaceholder is shown instead of results while the query is empty, so
// opening the panel does not read as "every resource matches nothing".
const searchPlaceholder = "type to search"

// searchPanelWidthFor scales the side panel to the terminal: the full 46
// columns on wide terminals, a smaller share below 100 so the main view
// keeps usable room.
func searchPanelWidthFor(width int) int {
	if width <= 0 {
		return searchPanelWidth
	}
	if width < 100 {
		return width * 2 / 5
	}
	return searchPanelWidth
}

func (a *app) openSearch() {
	if a.searchIndex == nil {
		a.searchIndex = newSearchIndex()
	}
	if a.searchPanel == nil || a.searchColumn == nil {
		return
	}
	a.searchInput.SetText(a.searchQuery)
	a.searchPanelFocus = true
	a.searchOpen.Store(true)
	width := 0
	if a.screen != nil {
		width, _ = a.screen.Size()
	}
	a.searchColumn.ResizeItem(a.searchPanel, searchPanelWidthFor(width), 1)
	a.tapp.SetFocus(a.searchInput)
	a.runSearch()
	a.updateFocusStyles()
	a.refreshInfobar()
	a.ensureSearchIndex(false)
	a.ensureSearchProjects()
}

func (a *app) closeSearch() {
	if !a.searchOpen.Load() {
		return
	}
	a.searchOpen.Store(false)
	a.searchPanelFocus = false
	a.searchQuery = strings.TrimSpace(a.searchInput.GetText())
	if a.searchColumn != nil {
		a.searchColumn.ResizeItem(a.searchPanel, 0, 0)
	}
	a.updateFocusStyles()
	a.tapp.SetFocus(a.info)
}

// cycleSearchFocus walks Tab through the three sections while the search
// panel is open — query input, results table, main view — and Backtab runs
// the same cycle backwards. The panel stays open the whole way.
func (a *app) cycleSearchFocus(backward bool) {
	if !a.searchOpen.Load() {
		return
	}
	sections := []tview.Primitive{a.searchInput, a.searchResults, a.info}
	current := 0
	switch a.tapp.GetFocus() {
	case a.searchResults:
		current = 1
	case a.info:
		current = 2
	}
	step := 1
	if backward {
		step = len(sections) - 1
	}
	next := sections[(current+step)%len(sections)]
	a.searchPanelFocus = next != a.info
	a.tapp.SetFocus(next)
	a.updateFocusStyles()
}

// updateFocusStyles marks the focused section with the accent border while
// the search panel is open: the one of the query input, the results table or
// the info table holding focus takes the accent, the others stay on the
// theme border. With the panel closed, or focus elsewhere (the bottom filter
// input, overlays), all three revert to the theme border.
func (a *app) updateFocusStyles() {
	if a.searchInput == nil || a.searchResults == nil || a.info == nil {
		return
	}
	plain := theme.borderColor()
	inputBorder, resultsBorder, infoBorder := plain, plain, plain
	if a.searchOpen.Load() {
		switch a.tapp.GetFocus() {
		case a.searchInput:
			inputBorder = theme.focusBorderColor()
		case a.searchResults:
			resultsBorder = theme.focusBorderColor()
		case a.info:
			infoBorder = theme.focusBorderColor()
		}
	}
	a.searchInput.SetBorderColor(inputBorder)
	a.searchResults.SetBorderColor(resultsBorder)
	a.info.SetBorderColor(infoBorder)
}

// runSearch re-runs the query against the index and refreshes the results
// table. It runs on the UI thread (keystrokes) and from the index builder
// via queueUpdateDraw.
func (a *app) runSearch() {
	if !a.searchOpen.Load() {
		return
	}
	query := a.searchInput.GetText()
	a.searchHits = a.searchIndex.match(query)
	a.refreshSearchResults()
}

// searchGroupRow is one results row: a resource kind, the query it matched,
// how many of its rows matched, and the distinct subrow keys the matches span
// (a project for All views, "project/scope" for the scoped kinds). The query
// rides along so a later refresh with new text cannot make an old row preset
// a stale filter.
type searchGroupRow struct {
	kind       viewKind
	query      string
	count      int
	keys       []string
	keyCounts  []int
}

// searchGroupIndent prefixes every project subrow. Terminal tables do not
// honor tab stops, so the indentation is spaces standing in for one tab.
const searchGroupIndent = "    "

// refreshSearchResults refills the results table with one bold header row per
// matching resource kind, followed by one indented project row per project in
// that kind's sublist. While the index is still building, the title keeps a
// "searching" marker so partial results are not mistaken for the final count.
func (a *app) refreshSearchResults() {
	table := a.searchResults
	table.Clear()
	a.searchGroupRows = a.searchGroupRows[:0]
	a.searchRowGroups = a.searchRowGroups[:0]
	a.searchRowProjects = a.searchRowProjects[:0]
	shown := a.searchHits
	overflow := 0
	if len(shown) > maxSearchHits {
		shown = shown[:maxSearchHits]
		overflow = len(a.searchHits) - maxSearchHits
	}
	query := strings.TrimSpace(a.searchInput.GetText())
	if query == "" {
		shown = nil
		overflow = 0
	}
	for i := range shown {
		kind := shown[i].kind
		if len(a.searchGroupRows) > 0 && a.searchGroupRows[len(a.searchGroupRows)-1].kind == kind {
			continue
		}
		count, keys, keyCounts := searchKindSummary(shown, kind)
		a.searchGroupRows = append(a.searchGroupRows, searchGroupRow{
			kind:      kind,
			query:     query,
			count:     count,
			keys:      keys,
			keyCounts: keyCounts,
		})
	}

	for i := range a.searchGroupRows {
		group := &a.searchGroupRows[i]
		// tview parses bracket tags at draw time; a project id could read as a
		// color tag and vanish, so escape every cell.
		table.SetCell(len(a.searchRowGroups), 0, tview.NewTableCell(tview.Escape(fmt.Sprintf("%s(%d)", views[group.kind].title, group.count))).
			SetTextColor(theme.titleColor()).
			SetSelectable(true).
			SetAttributes(tcell.AttrBold).
			SetExpansion(1))
		a.searchRowGroups = append(a.searchRowGroups, i)
		a.searchRowProjects = append(a.searchRowProjects, "")
		for j, key := range group.keys {
			label := fmt.Sprintf("%s%s (%d)", searchGroupIndent, key, group.keyCounts[j])
			table.SetCell(len(a.searchRowGroups), 0, tview.NewTableCell(tview.Escape(label)).
				SetTextColor(theme.shortcutTextColor()).
				SetSelectable(true).
				SetExpansion(1))
			a.searchRowGroups = append(a.searchRowGroups, i)
			a.searchRowProjects = append(a.searchRowProjects, key)
		}
	}
	if len(a.searchRowGroups) > 0 {
		table.Select(0, 0)
	}

	title := fmt.Sprintf(" %d", len(a.searchHits))
	if overflow > 0 {
		title = fmt.Sprintf(" %d+", maxSearchHits)
	}
	if query == "" {
		title = " " + searchPlaceholder + " "
	} else if a.searchIndex.warmCount() < len(searchKinds) {
		title += " matches | searching… "
	} else {
		title += " matches "
	}
	table.SetTitle(title)
}

// searchEntryGroupKey is the per-subrow grouping key of an entry: the
// project for All-view rows, "project/scope" for the scoped kinds (cluster,
// zone or bucket name — none of which may contain a slash, so the split is
// unambiguous).
func searchEntryGroupKey(e searchEntry) string {
	if e.scope != "" {
		return e.project + "/" + e.scope
	}
	return e.project
}

// searchKindSummary counts one kind's hits and lists its distinct subrow
// keys in first-seen order with a per-key hit count, from the shown slice
// only so a group never claims more rows than the hit cap left visible.
func searchKindSummary(shown []searchEntry, kind viewKind) (count int, keys []string, keyCounts []int) {
	seen := map[string]bool{}
	counts := map[string]int{}
	for i := range shown {
		if shown[i].kind != kind {
			continue
		}
		count++
		if key := searchEntryGroupKey(shown[i]); key != "" {
			counts[key]++
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	for _, k := range keys {
		keyCounts = append(keyCounts, counts[k])
	}
	return count, keys, keyCounts
}

// searchSelect acts on one results row: header or subrow both resolve to
// their group and open its view with the filter preset to the query — a
// project subrow additionally keeps its project term, and a scoped kind's
// subrow applies its cluster/zone/bucket scope, so the main view lands on
// exactly the rows the selection counted. The panel stays open as the
// sidebar: S returns to it, Esc closes it.
func (a *app) searchSelect(row int) {
	if row < 0 || row >= len(a.searchRowGroups) {
		return
	}
	group := a.searchGroupRows[a.searchRowGroups[row]]
	if _, ok := views[group.kind]; !ok {
		return
	}
	a.searchLandView(group, row)

	appState.CmdMode = modeFilter
	a.cmd.SetBorder(true)
	a.cmd.SetLabel(" / ")
	a.cmd.SetText(a.searchRowQuery(row))
	a.applyFilter()
	a.tapp.SetFocus(a.info)
	a.searchPanelFocus = false
	a.updateFocusStyles()
	a.statusInfo(fmt.Sprintf("%s: %s (Esc closes panel or clears filter)", views[group.kind].title, a.searchRowQuery(row)))
}

// searchLandView switches the main view to what a results row points at:
// All views keep the All-projects toggle on, scoped kinds instead apply the
// subrow's scope (cluster, zone or bucket) and its project — a kind header
// falls back to the group's first key, so landing a header still shows one
// matching scope instead of an unscoped empty view.
func (a *app) searchLandView(group searchGroupRow, row int) {
	key := a.searchRowProjects[row]
	if searchScopedKind(group.kind) {
		if key == "" && len(group.keys) > 0 {
			key = group.keys[0]
		}
		applySearchScope(group.kind, key)
		appState.AllProjects = false
	} else {
		appState.AllProjects = true
	}
	renderSummary(a.summary)
	a.setView(group.kind)
}

// applySearchScope sets the focused scope of one scoped view from its subrow
// key, so the landed view loads the matching cluster, zone or bucket.
func applySearchScope(kind viewKind, key string) {
	project, scope, _ := strings.Cut(key, "/")
	switch kind {
	case viewWorkloads:
		appState.ClusterProject, appState.Cluster = project, scope
	case viewDNSRecords:
		appState.ZoneProject, appState.Zone = project, scope
	case viewBucketIAM:
		appState.BucketProject, appState.Bucket = project, scope
	}
}

// searchRowQuery composes the filter one results row lands: the group's query
// alone for a kind header, plus the project term for a project subrow of an
// All view — scoped kinds take no extra term because their rows carry no
// project column, and the scope itself is applied through the view state.
func (a *app) searchRowQuery(row int) string {
	group := a.searchGroupRows[a.searchRowGroups[row]]
	query := group.query
	if !searchScopedKind(group.kind) {
		if project := a.searchRowProjects[row]; project != "" {
			query = strings.TrimSpace(query + " + " + project)
		}
	}
	return query
}

// searchPreview mirrors the main view onto the results selection as it moves:
// every row opens its group's view with the filter preset to the query, a
// project subrow additionally appends its project to the filter, and a
// scoped kind's subrow applies its scope, so the main panel shows exactly
// what the selector points at. It only drives the preview while the panel
// holds focus, so Enter still lands the same selection through searchSelect.
func (a *app) searchPreview(row int) {
	if !a.searchOpen.Load() || !a.searchPanelFocus {
		return
	}
	if row < 0 || row >= len(a.searchRowGroups) {
		return
	}
	group := a.searchGroupRows[a.searchRowGroups[row]]
	if _, ok := views[group.kind]; !ok {
		return
	}
	a.searchLandView(group, row)
	appState.CmdMode = modeFilter
	a.cmd.SetBorder(true)
	a.cmd.SetLabel(" / ")
	a.cmd.SetText(a.searchRowQuery(row))
	a.applyFilter()
}

// searchResultsRowForProject returns the results-table row of a group's
// subrow for one project, or -1 when no such row exists. Header rows sit
// directly above their subrows, so the walk only needs the row->group and
// row->project maps.
func (a *app) searchResultsRowForProject(kind viewKind, project string) int {
	for row := range a.searchRowGroups {
		if a.searchGroupRows[a.searchRowGroups[row]].kind == kind && a.searchRowProjects[row] == project {
			return row
		}
	}
	return -1
}
