package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/elferherrera/lazycloud/cmd/providers"
	"github.com/gdamore/tcell/v2"
)

// TestRunUIAgainstSimulationScreen boots the same app Run() builds and drives
// the real event loop against a simulation screen. The before-draw hook
// records the screen, queueUpdateDraw switches to the queued path, Ctrl-C
// flows through inputCapture and the loop ends.
func TestRunUIAgainstSimulationScreen(t *testing.T) {
	restore := restoreSeams(t)
	emptySeams(t)
	defer restore()

	appState.CmdMode = modeCmd
	appState.ViewKind = viewProjects

	a, cleanup, err := bootApp()
	if err != nil {
		t.Fatalf("bootApp() error = %v", err)
	}
	defer cleanup()

	sim := tcell.NewSimulationScreen("UTF-8")
	a.tapp.SetScreen(sim)

	done := make(chan error, 1)
	go func() { done <- a.runUI() }()

	waitFor(t, a, func() bool { return a.screen != nil })
	sim.InjectKey(tcell.KeyCtrlC, 0, tcell.ModNone)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runUI() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runUI() did not return after Ctrl-C")
	}
}

// TestBootAppRejectsNestedExecution pins the guard the binary relies on:
// inside a lazycloud context booting again must fail without touching any
// state.
func TestBootAppRejectsNestedExecution(t *testing.T) {
	t.Setenv("LAZYCLOUD_CONTEXT", "true")
	if err := Run(); err == nil {
		t.Fatal("Run() inside lazycloud context must fail")
	}
}

func TestProjectRefreshWrapperReportsFetchResults(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	emptySeams(t)
	resetAppState()
	appState.ProjectID = "p1"
	appState.ViewKind = viewBuckets

	projectRefresh(viewBuckets, "buckets", fetchProjectBuckets)(a)

	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Fetched 0 buckets")
	})
}

func TestProjectRefreshWrapperWithoutProject(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	resetAppState()
	appState.ProjectID = ""

	projectRefresh(viewBuckets, "buckets", fetchProjectBuckets)(a)
	if !strings.Contains(a.status.GetText(false), "No project selected") {
		t.Fatalf("status = %q, want no-project warning", a.status.GetText(false))
	}
}

func TestProjectRefreshWrapperSurfacesFetchError(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	failingSeams(t)
	resetAppState()
	appState.ProjectID = "p1"

	projectRefresh(viewBuckets, "buckets", fetchProjectBuckets)(a)

	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Failed to fetch buckets")
	})
}

func TestRefreshProjectViewsSummaries(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	emptySeams(t)
	resetAppState()
	appState.ProjectID = "p1"

	a.refreshProjectViews()

	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), fmt.Sprintf("Updated %d views for p1", len(projectViewKinds)))
	})
}

func TestRefreshProjectViewsWithoutProject(t *testing.T) {
	a := newFetchTestApp(t)
	resetAppState()
	appState.ProjectID = ""

	a.refreshProjectViews()
	if !strings.Contains(a.status.GetText(false), "No project selected") {
		t.Fatalf("status = %q, want no-project warning", a.status.GetText(false))
	}
}

func TestRefreshProjectViewsCountsFailures(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	failingSeams(t)
	resetAppState()
	appState.ProjectID = "p1"

	a.refreshProjectViews()

	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "views failed")
	})
}

func TestRefreshProjectViewsAllProjectsWalksEveryProject(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	emptySeams(t)
	resetAppState()
	appState.AllProjects = true
	if err := a.cache.SaveProjects([]providers.Project{
		{ProjectID: "p1", Name: "P One", ProjectNumber: "111"},
		{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"},
	}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}

	a.refreshProjectViews()

	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "views across 2 projects")
	})

	for _, projectID := range []string{"p1", "p2"} {
		updatedAt, err := a.cache.GetViewUpdate(string(viewVMs), projectID)
		if err != nil {
			t.Fatalf("GetViewUpdate(vms, %s) error = %v", projectID, err)
		}
		if updatedAt == "" {
			t.Fatalf("vms view not marked updated for %s", projectID)
		}
	}
}

func TestRefreshProjectViewsAllProjectsWithoutProjects(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	emptySeams(t)
	resetAppState()
	appState.AllProjects = true
	// The test fixture seeds project p1; drop it so the cache holds none.
	if _, err := a.cache.DeleteProject("p1"); err != nil {
		t.Fatalf("DeleteProject(p1) error = %v", err)
	}

	a.refreshProjectViews()

	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "No cached projects to update")
	})
}

func TestRefreshProjectViewsAllProjectsCountsFailures(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	failingSeams(t)
	resetAppState()
	appState.AllProjects = true
	if err := a.cache.SaveProjects([]providers.Project{
		{ProjectID: "p1", Name: "P One", ProjectNumber: "111"},
		{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"},
	}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}

	a.refreshProjectViews()

	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "views failed")
	})
}

func TestMarkViewUpdatedInvalidatesSearchIndex(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)
	resetAppState()

	a.openSearch()
	a.searchInput.SetText("alpha")
	a.runSearch()
	if a.searchIndex.warmCount() == 0 {
		t.Fatal("index has no warm kinds before invalidate")
	}

	// With the panel open, marking a view updated must cool the matching
	// search kind and re-arm the background builder.
	a.markViewUpdated(viewVMs, "p1")
	kind := searchIndexedKind(viewVMs)
	a.searchIndex.mu.Lock()
	warm := a.searchIndex.warm[kind]
	a.searchIndex.mu.Unlock()
	if warm {
		t.Fatalf("kind %q still warm after markViewUpdated", kind)
	}
	a.closeSearch()
}

func TestMarkViewUpdatedWithoutIndexIsNoop(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	emptySeams(t)
	a.searchIndex = nil
	resetAppState()

	a.markViewUpdated(viewVMs, "p1")
	if a.searchIndex != nil {
		t.Fatal("markViewUpdated must not create an index")
	}
}

func TestInvalidateRejectsEmptyKind(t *testing.T) {
	idx := newSearchIndex()
	idx.warm[""] = true
	idx.invalidate("")
	idx.mu.Lock()
	warm := idx.warm[""]
	idx.mu.Unlock()
	if !warm {
		t.Fatal("invalidate(\"\") must be a no-op")
	}
}

func TestEnsureSearchIndexWithoutIndexIsNoop(t *testing.T) {
	a := newSearchTestApp(t)
	a.searchIndex = nil
	a.ensureSearchIndex(true)
	if a.searchIndex != nil {
		t.Fatal("ensureSearchIndex must not create an index")
	}
}

func TestEnsureSearchProjectsPullsAndSaves(t *testing.T) {
	a := newSearchTestApp(t)
	a.searchIndex = newSearchIndex()
	defer restoreSeams(t)()
	listGcloudProjects = func() ([]providers.Project, error) {
		return []providers.Project{{ProjectID: "p1", Name: "P One", ProjectNumber: "111"}}, nil
	}
	resetAppState()

	a.ensureSearchProjects()
	waitFor(t, a, func() bool {
		projects, err := a.cache.ListProjects()
		return err == nil && len(projects) == 1
	})

	// Second call must not re-pull: the latch holds.
	a.ensureSearchProjects()
	a.searchIndex.mu.Lock()
	pulled := a.searchIndex.projectsPulled
	a.searchIndex.mu.Unlock()
	if !pulled {
		t.Fatal("projectsPulled not latched after first pull")
	}
}

func TestEnsureSearchProjectsReportsPullFailure(t *testing.T) {
	a := newSearchTestApp(t)
	a.searchIndex = newSearchIndex()
	defer restoreSeams(t)()
	listGcloudProjects = func() ([]providers.Project, error) {
		return nil, errorString("pull failed")
	}
	resetAppState()

	a.ensureSearchProjects()
	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Search failed to fetch projects")
	})
}

func TestEnsureSearchProjectsReportsSaveFailure(t *testing.T) {
	a := newSearchTestApp(t)
	a.searchIndex = newSearchIndex()
	defer restoreSeams(t)()
	listGcloudProjects = func() ([]providers.Project, error) {
		return []providers.Project{{ProjectID: "p1"}}, nil
	}
	resetAppState()
	// Closing the cache makes the save fail.
	a.cache.Close()

	a.ensureSearchProjects()
	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Search failed to update cache")
	})
}

func TestMatchOverflowsAtCap(t *testing.T) {
	idx := newSearchIndex()
	entries := make([]searchEntry, maxSearchHits+5)
	for i := range entries {
		entries[i].text = "vm"
		entries[i].kind = viewVMsAll
	}
	idx.entries[viewVMsAll] = entries
	hits := idx.match("vm")
	if len(hits) <= maxSearchHits {
		t.Fatalf("match() = %d hits, want the capped overflow set", len(hits))
	}
}

func TestSearchCommandOpensPanel(t *testing.T) {
	a := newSearchTestApp(t)
	wireSearchApp(a)
	resetAppState()

	a.runCommand([]string{"search"})
	if !a.searchOpen.Load() {
		t.Fatal(":search did not open the panel")
	}
	a.closeSearch()
}
