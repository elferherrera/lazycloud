package app

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elferherrera/lazycloud/cmd/providers"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TestRunBootsAgainstScreenAndStops covers the full Run() path: boot with a
// fresh HOME, the live loop, and a clean Ctrl-C shutdown.
func TestRunBootsAgainstScreenAndStops(t *testing.T) {
	restore := restoreSeams(t)
	emptySeams(t)
	savedTApp := newTApp
	t.Cleanup(func() { newTApp = savedTApp })
	defer restore()

	home := t.TempDir()
	t.Setenv("HOME", home)

	sim := tcell.NewSimulationScreen("UTF-8")
	screenReady := make(chan struct{})
	newTApp = func() *tview.Application {
		app := tview.NewApplication()
		app.SetScreen(sim)
		// SetScreen ran Init: the sim's event channels exist from here on.
		close(screenReady)
		return app
	}

	done := make(chan error, 1)
	go func() { done <- Run() }()

	deadline := time.After(10 * time.Second)
	// Injecting before bootApp reaches newTApp blocks forever: tcell's
	// postEvent selects on the sim's channels, which are nil until Init.
	// Wait for the screen, then keep retrying on a slow cadence in case an
	// event lands before the poll goroutine exists.
	select {
	case <-screenReady:
	case err := <-done:
		t.Fatalf("Run() returned before the screen was up: %v", err)
	case <-deadline:
		t.Fatal("boot never created the screen")
	}
	for {
		sim.InjectKey(tcell.KeyCtrlC, 0, tcell.ModNone)
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			return
		case <-time.After(200 * time.Millisecond):
		case <-deadline:
			t.Fatal("Run() did not return after Ctrl-C")
		}
	}
}

func TestBootAppFailsWithUnwritableCacheDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	blocker := filepath.Join(home, ".config")
	if err := os.WriteFile(blocker, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := bootApp(); err == nil || !strings.Contains(err.Error(), "opening cache") {
		t.Fatalf("bootApp() error = %v, want cache open failure", err)
	}
}

func TestBootAppFailsWhenSeedingHitsReadonlyDB(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "lazycloud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "cache.db")
	// Build a fully migrated db, then make it read-only: opening and the
	// schema no-ops still succeed, the seeding INSERT is the first write.
	cache, err := providers.NewCacheAt(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	cache.Close()
	if err := os.Chmod(dbPath, 0o444); err != nil {
		t.Fatal(err)
	}
	if _, _, err := bootApp(); err == nil || !strings.Contains(err.Error(), "seeding view updates") {
		t.Fatalf("bootApp() error = %v, want seeding failure", err)
	}
}

func TestBootAppFailsWithBrokenConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "lazycloud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("theme: [broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := bootApp(); err == nil || !strings.Contains(err.Error(), "loading config") {
		t.Fatalf("bootApp() error = %v, want config failure", err)
	}
}

func TestBootAppFailsWithBrokenLastProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "lazycloud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "last_project.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := bootApp(); err == nil || !strings.Contains(err.Error(), "loading last project") {
		t.Fatalf("bootApp() error = %v, want last project failure", err)
	}
}

func TestRunUIQueuedUpdatePath(t *testing.T) {
	a := newFetchTestApp(t)
	sim := tcell.NewSimulationScreen("UTF-8")
	a.tapp.SetScreen(sim)
	a.tapp.SetRoot(a.pages, true)

	done := make(chan error, 1)
	go func() { done <- a.runUI() }()
	waitFor(t, a, func() bool { return a.screen != nil })

	// With the loop live, queueUpdateDraw must go through the queue.
	ran := make(chan struct{})
	a.queueUpdateDraw(func() { close(ran) })
	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("queued update never ran")
	}

	sim.InjectKey(tcell.KeyCtrlC, 0, tcell.ModNone)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runUI() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runUI() did not return")
	}
}

func TestSearchEnterSelectsFirstResult(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)
	resetAppState()

	a.openSearch()
	a.searchInput.SetText("alpha")
	a.runSearch()
	if a.searchResults.GetRowCount() < 1 {
		t.Fatal("no results to select")
	}
	a.searchResults.Select(0, 0)
	handler := a.searchResults.InputHandler()
	if handler == nil {
		t.Fatal("search results table has no input handler")
	}
	handler(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), nil)
	if appState.ViewKind != viewVMsAll {
		t.Fatalf("Enter on a group did not jump, ViewKind = %q", appState.ViewKind)
	}
	if a.cmd.GetText() != "alpha" {
		t.Fatalf("filter after Enter = %q, want alpha", a.cmd.GetText())
	}
	a.closeSearch()
}

func TestEnsureSearchIndexSkipsWhileBuilding(t *testing.T) {
	a := newSearchTestApp(t)
	a.searchIndex = newSearchIndex()
	a.searchIndex.building = true
	a.ensureSearchIndex(false)
	if a.searchIndex.building != true {
		t.Fatal("ensureSearchIndex must not spawn a second builder")
	}
}

func TestBuildSearchIndexPullsEmptyKind(t *testing.T) {
	a := newSearchTestApp(t)
	defer restoreSeams(t)()
	emptySeams(t)
	a.searchIndex = newSearchIndex()

	// The base fixture seeds VMs, so vms-all has entries; snapshots-all is
	// empty: the builder must mark it pulled and start a background pull.
	a.ensureSearchIndex(true)
	waitFor(t, a, func() bool {
		a.searchIndex.mu.Lock()
		defer a.searchIndex.mu.Unlock()
		return a.searchIndex.pulled[viewSnapshotsAll]
	})
	a.closeSearch()
}

func TestPullSearchKindWithoutProjects(t *testing.T) {
	a := newSearchTestApp(t)
	defer restoreSeams(t)()
	emptySeams(t)
	a.searchIndex = newSearchIndex()
	if _, err := a.cache.DeleteProject("p1"); err != nil {
		t.Fatal(err)
	}

	// No cached projects: nothing to pull, no error.
	a.pullSearchKind(viewVMsAll)
}

func TestPullSearchKindWithFailingFetch(t *testing.T) {
	a := newSearchTestApp(t)
	defer restoreSeams(t)()
	failingSeams(t)
	resetAppState()
	if err := a.cache.SaveProjects([]providers.Project{{ProjectID: "p1"}}); err != nil {
		t.Fatal(err)
	}

	a.pullSearchKind(viewVMsAll)
	waitFor(t, a, func() bool {
		return strings.Contains(a.summary.GetText(false), "Search pull failed")
	})
}

func TestPullSearchKindUnknownKindIsNoop(t *testing.T) {
	a := newSearchTestApp(t)
	resetAppState()
	a.pullSearchKind(viewKind("nonexistent-all"))
}

func TestRunSearchTruncatesOverflowTitle(t *testing.T) {
	a := newSearchTestApp(t)
	seedSearchRows(t, a)
	buildSearchIndexSync(t, a)

	entries := make([]searchEntry, maxSearchHits+5)
	for i := range entries {
		entries[i].text = "overflow-vm"
		entries[i].kind = viewVMsAll
	}
	a.searchIndex.mu.Lock()
	a.searchIndex.entries[viewVMsAll] = append(a.searchIndex.entries[viewVMsAll], entries...)
	a.searchIndex.warm[viewVMsAll] = true
	a.searchIndex.mu.Unlock()

	a.openSearch()
	a.searchInput.SetText("overflow-vm")
	a.runSearch()

	if len(a.searchHits) <= maxSearchHits {
		t.Fatalf("searchHits = %d, want overflow", len(a.searchHits))
	}
	title := a.searchResults.GetTitle()
	if !strings.Contains(title, "+") {
		t.Fatalf("title = %q, want overflow marker", title)
	}
	a.closeSearch()
}

func TestAllProjectsRefreshReportsListFailure(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	emptySeams(t)
	a.cache.Close()

	allProjectsRefresh(viewVMsAll, "vms", "vms", "vms", fetchProjectVMs)(a)
	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Failed to list projects")
	})
}

func TestProjectRefreshTickerUpdatesStatus(t *testing.T) {
	a := newFetchTestApp(t)
	p := newProgress(2)
	stop := a.startProgressTicker(p)
	p.advance("step one")
	waitFor(t, a, func() bool {
		return a.status.GetText(false) != ""
	})
	stop()
}

func TestRefreshProjectsReportsFailure(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	listGcloudProjects = func() ([]providers.Project, error) {
		return nil, errorString("nope")
	}
	resetAppState()

	views[viewProjects].refresh(a)
	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Failed to fetch projects")
	})
}

func TestRefreshProjectsSucceeds(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	listGcloudProjects = func() ([]providers.Project, error) {
		return []providers.Project{{ProjectID: "p2", Name: "P Two"}}, nil
	}
	resetAppState()

	// The success tail goes through tapp.QueueUpdateDraw, which needs the
	// live loop; assert on the cache effect instead, the goroutine's
	// durable step before that call.
	views[viewProjects].refresh(a)
	waitFor(t, a, func() bool {
		projects, err := a.cache.ListProjects()
		return err == nil && len(projects) == 1 && projects[0].ProjectID == "p2"
	})
}

func TestBucketIAMRefreshReportsFailure(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	failingSeams(t)
	resetAppState()
	appState.BucketProject = "p1"
	appState.Bucket = "b1"

	views[viewBucketIAM].refresh(a)
	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Failed to fetch bucket IAM policy")
	})
}

func TestDNSRecordsRefreshReportsFailure(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	failingSeams(t)
	resetAppState()
	appState.ZoneProject = "p1"
	appState.Zone = "z1"

	views[viewDNSRecords].refresh(a)
	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Failed to fetch dns records")
	})
}

func TestWorkloadsRefreshReportsFailure(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	listClusterWorkloads = func(_, _, _ string) ([]providers.Workload, error) {
		return nil, errorString("boom")
	}
	resetAppState()
	appState.ClusterProject = "p1"
	appState.Cluster = "cl-1"

	views[viewWorkloads].refresh(a)
	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Failed to fetch workloads")
	})
}

func TestNodePoolsRefreshReportsFailure(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	listClusterNodes = func(_, _, _ string) ([]providers.ClusterNode, error) {
		return nil, errorString("boom")
	}
	resetAppState()
	appState.ClusterProject = "p1"
	appState.Cluster = "cl-1"

	views[viewNodePools].refresh(a)
	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Failed to fetch nodes")
	})
}

func TestNodePoolsRefreshSaveFailure(t *testing.T) {
	// Dropping the cluster_nodes table leaves reads intact and fails the
	// node save, isolating the save arm of the node pools refresher.
	dbPath := filepath.Join(t.TempDir(), "cache.db")
	cache, err := providers.NewCacheAt(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cache.Close() })
	logger, err := providers.NewLogger()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { logger.Close() })
	a := &app{
		tapp:    tview.NewApplication(),
		cache:   cache,
		logger:  logger,
		status:  tview.NewTextView().SetDynamicColors(true),
		summary: tview.NewTextView(),
	}
	if err := a.cache.SaveProjects([]providers.Project{{ProjectID: "p1", Name: "P One"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.cache.SaveClusters("p1", []providers.Cluster{{ProjectID: "p1", Name: "cl-1", Location: "europe-west1"}}); err != nil {
		t.Fatal(err)
	}
	breaker, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := breaker.Exec("DROP TABLE cluster_nodes"); err != nil {
		breaker.Close()
		t.Fatal(err)
	}
	breaker.Close()
	defer restoreSeams(t)()
	listClusterNodes = func(_, _, _ string) ([]providers.ClusterNode, error) {
		return []providers.ClusterNode{{ProjectID: "p1", Cluster: "cl-1", Name: "node-a"}}, nil
	}
	resetAppState()
	appState.ClusterProject = "p1"
	appState.Cluster = "cl-1"

	views[viewNodePools].refresh(a)
	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Failed to update cache")
	})
}

func TestNodePoolsRefreshSucceeds(t *testing.T) {
	a := newFetchTestApp(t)
	defer restoreSeams(t)()
	listClusterNodes = func(_, _, _ string) ([]providers.ClusterNode, error) {
		return []providers.ClusterNode{{ProjectID: "p1", Cluster: "cl-1", Name: "node-a"}}, nil
	}
	resetAppState()
	appState.ClusterProject = "p1"
	appState.Cluster = "cl-1"

	// The success tail queues through tapp directly; assert the nodes
	// landed in the cache, the step before that call.
	views[viewNodePools].refresh(a)
	waitFor(t, a, func() bool {
		nodes, err := a.cache.ListClusterNodes("p1", "cl-1")
		return err == nil && len(nodes) == 1
	})
}

func TestFetchNodePoolVMsArms(t *testing.T) {
	t.Run("list failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		listProjectVMs = func(string) ([]providers.VM, error) { return nil, errorString("boom") }
		resetAppState()
		appState.ProjectID = "p1"

		a.fetchNodePoolVMs("pool")
		waitFor(t, a, func() bool {
			return strings.Contains(a.status.GetText(false), "Failed to fetch VMs")
		})
	})
	t.Run("save failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		emptySeams(t)
		resetAppState()
		appState.ProjectID = "p1"
		a.cache.Close()

		a.fetchNodePoolVMs("pool")
		waitFor(t, a, func() bool {
			return strings.Contains(a.status.GetText(false), "Failed to update cache")
		})
	})
	t.Run("disk list failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		listProjectVMs = func(string) ([]providers.VM, error) { return []providers.VM{{ProjectID: "p1", Name: "vm"}}, nil }
		listProjectDisks = func(string) ([]providers.Disk, error) { return nil, errorString("boom") }
		resetAppState()
		appState.ProjectID = "p1"

		// The final redraw overwrites the status line, so assert on the
		// summary, which holds the error.
		a.fetchNodePoolVMs("pool")
		waitFor(t, a, func() bool {
			return strings.Contains(a.summary.GetText(false), "Failed to fetch disks")
		})
	})
	t.Run("disk save failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		listProjectVMs = func(string) ([]providers.VM, error) { return []providers.VM{{ProjectID: "p1", Name: "vm"}}, nil }
		listProjectDisks = func(string) ([]providers.Disk, error) { return []providers.Disk{{ProjectID: "p1", Name: "d"}}, nil }
		resetAppState()
		appState.ProjectID = "p1"

		a.cache.Close()
		a.fetchNodePoolVMs("pool")
		waitFor(t, a, func() bool {
			return strings.Contains(a.summary.GetText(false), "Failed to update cache")
		})
	})
}

func TestFetchInstanceGroupVMsArms(t *testing.T) {
	t.Run("cache list failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		resetAppState()
		appState.ProjectID = "p1"
		a.cache.Close()

		a.fetchInstanceGroupVMs("europe-west1-b", "group")
		waitFor(t, a, func() bool {
			return strings.Contains(a.status.GetText(false), "Failed to load VMs")
		})
	})
	t.Run("empty cache list failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		// Empty the VM cache so the fetch falls through to the list seam.
		if err := a.cache.SaveVMs("p1", nil); err != nil {
			t.Fatal(err)
		}
		listProjectVMs = func(string) ([]providers.VM, error) { return nil, errorString("boom") }
		resetAppState()
		appState.ProjectID = "p1"

		a.fetchInstanceGroupVMs("europe-west1-b", "group")
		waitFor(t, a, func() bool {
			return strings.Contains(a.summary.GetText(false), "Failed to fetch VMs")
		})
	})
	t.Run("empty cache save failure", func(t *testing.T) {
		// Swap vms for a view: ListVMs still reads it, SaveVMs cannot write it.
		dbPath := filepath.Join(t.TempDir(), "cache.db")
		cache, err := providers.NewCacheAt(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { cache.Close() })
		breaker, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := breaker.Exec(`DROP TABLE vms;
CREATE TABLE vms_readonly (
	project_id TEXT NOT NULL,
	name TEXT NOT NULL,
	instance_id TEXT NOT NULL DEFAULT '',
	zone TEXT NOT NULL,
	machine_type TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT '',
	labels TEXT NOT NULL DEFAULT '{}',
	network_tags TEXT NOT NULL DEFAULT '',
	service_accounts TEXT NOT NULL DEFAULT '[]',
	instance_template TEXT NOT NULL DEFAULT '',
	interfaces TEXT NOT NULL DEFAULT '[]',
	boot_disk TEXT NOT NULL DEFAULT 'null',
	data_disks TEXT NOT NULL DEFAULT '[]',
	updated_at TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (project_id, name, zone)
);`); err != nil {
			t.Fatal(err)
		}
		if _, err := breaker.Exec(`CREATE VIEW vms AS SELECT * FROM vms_readonly;`); err != nil {
			t.Fatal(err)
		}
		breaker.Close()
		logger, err := providers.NewLogger()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { logger.Close() })
		a := &app{
			tapp:    tview.NewApplication(),
			cache:   cache,
			logger:  logger,
			status:  tview.NewTextView().SetDynamicColors(true),
			summary: tview.NewTextView(),
		}
		defer restoreSeams(t)()
		listProjectVMs = func(string) ([]providers.VM, error) { return []providers.VM{{ProjectID: "p1"}}, nil }
		resetAppState()
		appState.ProjectID = "p1"

		a.fetchInstanceGroupVMs("europe-west1-b", "group")
		waitFor(t, a, func() bool {
			return strings.Contains(a.summary.GetText(false), "Failed to update cache")
		})
	})
	t.Run("members failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		listProjectVMs = func(string) ([]providers.VM, error) { return []providers.VM{{ProjectID: "p1", Name: "vm"}}, nil }
		listInstanceGroupVMs = func(string, string, string) ([]providers.InstanceGroupMember, error) {
			return nil, errorString("boom")
		}
		resetAppState()
		appState.ProjectID = "p1"
		if err := a.cache.SaveVMs("p1", []providers.VM{{ProjectID: "p1", Name: "vm"}}); err != nil {
			t.Fatal(err)
		}

		a.fetchInstanceGroupVMs("europe-west1-b", "group")
		waitFor(t, a, func() bool {
			return strings.Contains(a.status.GetText(false), "Failed to fetch instance group VMs")
		})
	})
}

func TestFetchTemplateVMsArms(t *testing.T) {
	t.Run("list failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		listProjectVMs = func(string) ([]providers.VM, error) { return nil, errorString("boom") }
		resetAppState()
		appState.ProjectID = "p1"

		a.fetchTemplateVMs("tpl")
		waitFor(t, a, func() bool {
			return strings.Contains(a.status.GetText(false), "Failed to fetch VMs")
		})
	})
	t.Run("save failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		listProjectVMs = func(string) ([]providers.VM, error) { return []providers.VM{{ProjectID: "p1"}}, nil }
		resetAppState()
		appState.ProjectID = "p1"
		a.cache.Close()

		a.fetchTemplateVMs("tpl")
		waitFor(t, a, func() bool {
			return strings.Contains(a.status.GetText(false), "Failed to update cache")
		})
	})
	t.Run("success", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		listProjectVMs = func(string) ([]providers.VM, error) {
			return []providers.VM{{ProjectID: "p1", Name: "vm", InstanceTemplate: "tpl"}}, nil
		}
		resetAppState()
		appState.ProjectID = "p1"

		a.fetchTemplateVMs("tpl")
		waitFor(t, a, func() bool {
			return strings.Contains(a.status.GetText(false), "1 VMs for template tpl")
		})
	})
}

func TestNetworksAllRefreshArms(t *testing.T) {
	t.Run("list projects failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		a.cache.Close()

		views[viewNetworksAll].refresh(a)
		waitFor(t, a, func() bool {
			return strings.Contains(a.status.GetText(false), "Failed to list projects")
		})
	})
	t.Run("networks failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		listProjectNetworks = func(string) ([]providers.Network, error) { return nil, errorString("boom") }
		listProjectSubnetworks = func(string) ([]providers.Subnetwork, error) {
			return []providers.Subnetwork{{ProjectID: "p1", Name: "sub"}}, nil
		}
		resetAppState()

		views[viewNetworksAll].refresh(a)
		waitFor(t, a, func() bool {
			return strings.Contains(a.summary.GetText(false), "Failed to fetch networks for p1")
		})
	})
	t.Run("subnetworks failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		listProjectNetworks = func(string) ([]providers.Network, error) {
			return []providers.Network{{ProjectID: "p1", Name: "net"}}, nil
		}
		listProjectSubnetworks = func(string) ([]providers.Subnetwork, error) { return nil, errorString("boom") }
		resetAppState()

		views[viewNetworksAll].refresh(a)
		waitFor(t, a, func() bool {
			return strings.Contains(a.summary.GetText(false), "Failed to fetch subnetworks for p1")
		})
	})
	t.Run("save failure", func(t *testing.T) {
		// Drop the networks table so the per-project save fails while the
		// project list keeps working.
		dbPath := filepath.Join(t.TempDir(), "cache.db")
		cache, err := providers.NewCacheAt(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { cache.Close() })
		if err := cache.SaveProjects([]providers.Project{{ProjectID: "p1", Name: "P One"}}); err != nil {
			t.Fatal(err)
		}
		breaker, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := breaker.Exec("DROP TABLE networks"); err != nil {
			breaker.Close()
			t.Fatal(err)
		}
		breaker.Close()
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		a.cache.Close()
		a.cache = cache
		listProjectNetworks = func(string) ([]providers.Network, error) {
			return []providers.Network{{ProjectID: "p1", Name: "net"}}, nil
		}
		listProjectSubnetworks = func(string) ([]providers.Subnetwork, error) {
			return []providers.Subnetwork{{ProjectID: "p1", Name: "sub"}}, nil
		}
		resetAppState()

		views[viewNetworksAll].refresh(a)
		waitFor(t, a, func() bool {
			return strings.Contains(a.summary.GetText(false), "Failed to update cache")
		})
	})
	t.Run("success", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		listProjectNetworks = func(string) ([]providers.Network, error) {
			return []providers.Network{{ProjectID: "p1", Name: "net"}}, nil
		}
		listProjectSubnetworks = func(string) ([]providers.Subnetwork, error) {
			return []providers.Subnetwork{{ProjectID: "p1", Name: "sub"}}, nil
		}
		resetAppState()

		views[viewNetworksAll].refresh(a)
		waitFor(t, a, func() bool {
			return strings.Contains(a.status.GetText(false), "Fetched 1 networks and 1 subnetworks")
		})
	})
}

func TestPublicImagesRefreshArms(t *testing.T) {
	t.Run("fetch failure", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		listProjectImages = func(string) ([]providers.Image, error) { return nil, errorString("boom") }
		resetAppState()

		views[viewPublicImages].refresh(a)
		waitFor(t, a, func() bool {
			return strings.Contains(a.summary.GetText(false), "Failed to fetch public images")
		})
	})
	t.Run("save failure", func(t *testing.T) {
		// Drop the images table so the save fails while the projects list
		// keeps working.
		dbPath := filepath.Join(t.TempDir(), "cache.db")
		cache, err := providers.NewCacheAt(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { cache.Close() })
		if err := cache.SaveProjects([]providers.Project{{ProjectID: "p1", Name: "P One"}}); err != nil {
			t.Fatal(err)
		}
		breaker, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := breaker.Exec("DROP TABLE images"); err != nil {
			breaker.Close()
			t.Fatal(err)
		}
		breaker.Close()
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		a.cache.Close()
		a.cache = cache
		listProjectImages = func(string) ([]providers.Image, error) {
			return []providers.Image{{ProjectID: "p1", Name: "img"}}, nil
		}
		resetAppState()

		views[viewPublicImages].refresh(a)
		waitFor(t, a, func() bool {
			return strings.Contains(a.summary.GetText(false), "Failed to update cache")
		})
	})
	t.Run("success", func(t *testing.T) {
		a := newFetchTestApp(t)
		defer restoreSeams(t)()
		listProjectImages = func(string) ([]providers.Image, error) {
			return []providers.Image{{ProjectID: "p1", Name: "img"}}, nil
		}
		resetAppState()

		views[viewPublicImages].refresh(a)
		waitFor(t, a, func() bool {
			return strings.Contains(a.status.GetText(false), "Fetched")
		})
	})
}
