package app

import (
	"testing"
	"time"

	"github.com/elferherrera/lazycloud/pkg/providers"
)

func stubViewRefresh(t *testing.T, kind viewKind) *int {
	t.Helper()
	original := views[kind]
	calls := 0
	def := original
	def.refresh = func(a *app) { calls++ }
	views[kind] = def
	t.Cleanup(func() {
		views[kind] = original
	})
	return &calls
}

func TestAutoFetchTriggersOnEmptyView(t *testing.T) {
	a := newFilterTestApp(t)
	a.autoFetched = map[string]bool{}
	calls := stubViewRefresh(t, viewIPs)
	appState.ProjectID = "p1"
	a.setView(viewIPs)
	if *calls != 1 {
		t.Fatalf("empty view triggered %d refreshes, want 1", *calls)
	}
	if !a.autoFetched["ips/p1"] {
		t.Fatalf("auto fetch key missing, have %v", a.autoFetched)
	}
	a.setView(viewIPs)
	if *calls != 1 {
		t.Fatalf("re-entering empty view re-triggered refresh, calls = %d, want 1", *calls)
	}
}

func TestAutoFetchSkipsNonEmptyView(t *testing.T) {
	a := newFilterTestApp(t)
	a.autoFetched = map[string]bool{}
	calls := stubViewRefresh(t, viewVMs)
	appState.ProjectID = "p1"
	a.setView(viewVMs)
	if *calls != 0 {
		t.Fatalf("non-empty view triggered %d refreshes, want 0", *calls)
	}
	if len(a.autoFetched) != 0 {
		t.Fatalf("non-empty view marked keys, have %v", a.autoFetched)
	}
}

func TestAutoFetchDisabledWithoutMap(t *testing.T) {
	a := newFilterTestApp(t)
	calls := stubViewRefresh(t, viewIPs)
	appState.ProjectID = "p1"
	a.setView(viewIPs)
	if *calls != 0 {
		t.Fatalf("nil autoFetched map still triggered %d refreshes, want 0", *calls)
	}
}

func TestRefreshViewMarksAutoFetch(t *testing.T) {
	a := newFilterTestApp(t)
	a.autoFetched = map[string]bool{}
	calls := stubViewRefresh(t, viewIPs)
	appState.ProjectID = "p1"
	appState.ViewKind = viewIPs
	a.refreshView()
	if *calls != 1 {
		t.Fatalf("refreshView called refresh %d times, want 1", *calls)
	}
	a.setView(viewIPs)
	if *calls != 1 {
		t.Fatalf("fetch completion re-triggered refresh, calls = %d, want 1", *calls)
	}
}

func TestAutoFetchScope(t *testing.T) {
	appState = State{ProjectID: "p1", Cluster: "cl-1", ClusterProject: "pc", Zone: "z-1", ZoneProject: "pz"}
	cases := map[viewKind]string{
		viewVMs:          "p1",
		viewVMsAll:       "",
		viewProjects:     "",
		viewPublicImages: "",
		viewWorkloads:    "pc/cl-1",
		viewNodePools:    "pc/cl-1",
		viewDNSRecords:   "pz/z-1",
	}
	for kind, want := range cases {
		if got := autoFetchScope(kind); got != want {
			t.Errorf("autoFetchScope(%q) = %q, want %q", kind, got, want)
		}
	}
	if got := autoFetchKey(viewVMs, autoFetchScope(viewVMs)); got != "vms/p1" {
		t.Errorf("autoFetchKey() = %q, want %q", got, "vms/p1")
	}
}

func TestDeleteProjectMarksAutoFetch(t *testing.T) {
	a := newFilterTestApp(t)
	a.autoFetched = map[string]bool{}
	calls := stubViewRefresh(t, viewProjects)
	appState.ViewKind = viewProjects
	a.setView(viewProjects)
	a.deleteProject()
	if *calls != 0 {
		t.Fatalf("delete project re-fetched projects view, calls = %d, want 0", *calls)
	}
	if !a.autoFetched["projects/"] {
		t.Fatalf("delete project did not mark projects key, have %v", a.autoFetched)
	}
}
func TestViewUpdateProject(t *testing.T) {
	appState = State{ProjectID: "p1", Cluster: "cl-1", ClusterProject: "pc", Zone: "z-1", ZoneProject: "pz", Bucket: "bkt", BucketProject: "pb"}
	cases := map[viewKind]string{
		viewVMs:        "p1",
		viewIPsAll:     "p1",
		viewWorkloads:  "pc",
		viewNodePools:  "pc",
		viewDNSRecords: "pz",
		viewBucketIAM:  "pb",
		viewProjects:   "p1",
	}
	for kind, want := range cases {
		if got := viewUpdateProject(kind); got != want {
			t.Errorf("viewUpdateProject(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestViewUpdateStale(t *testing.T) {
	if !viewUpdateStale("") {
		t.Error("viewUpdateStale(\"\") = false, want true")
	}
	if !viewUpdateStale("not-a-date") {
		t.Error("viewUpdateStale(\"not-a-date\") = false, want true")
	}
	old := time.Now().UTC().Add(-staleViewAfter).Format(time.RFC3339)
	if !viewUpdateStale(old) {
		t.Errorf("viewUpdateStale(%q) = false, want true", old)
	}
	fresh := time.Now().UTC().Format(time.RFC3339)
	if viewUpdateStale(fresh) {
		t.Errorf("viewUpdateStale(%q) = true, want false", fresh)
	}
}

func TestAutoRefreshStale(t *testing.T) {
	a := newFilterTestApp(t)
	a.staleAutoRefresh = true
	calls := stubViewRefresh(t, viewIPs)
	appState = State{ProjectID: "p1"}

	a.setView(viewIPs)
	if *calls != 1 {
		t.Fatalf("never-updated view triggered %d refreshes, want 1", *calls)
	}

	if err := a.cache.SaveViewUpdate("ips", "p1"); err != nil {
		t.Fatalf("SaveViewUpdate() error = %v", err)
	}
	a.setView(viewIPs)
	if *calls != 1 {
		t.Fatalf("fresh view update re-triggered refresh, calls = %d, want 1", *calls)
	}

	appState.ProjectID = ""
	a.setView(viewIPs)
	if *calls != 1 {
		t.Fatalf("view without project selection triggered refresh, calls = %d, want 1", *calls)
	}
}

func TestAutoRefreshStaleSkipsAllViews(t *testing.T) {
	a := newFilterTestApp(t)
	a.staleAutoRefresh = true
	calls := stubViewRefresh(t, viewIPsAll)
	appState = State{ProjectID: "p1"}
	a.setView(viewIPsAll)
	if *calls != 0 {
		t.Fatalf("all-projects view triggered %d refreshes, want 0", *calls)
	}
}

func TestAutoRefreshStaleSkippedAfterEmptyTrigger(t *testing.T) {
	a := newFilterTestApp(t)
	a.staleAutoRefresh = true
	a.autoFetched = map[string]bool{}
	calls := stubViewRefresh(t, viewIPs)
	appState = State{ProjectID: "p1"}
	a.setView(viewIPs)
	if *calls != 1 {
		t.Fatalf("empty view triggered %d refreshes, want 1 (empty trigger only, no stale double-fire)", *calls)
	}
}

func TestAutoRefreshStaleScopedView(t *testing.T) {
	a := newFilterTestApp(t)
	a.staleAutoRefresh = true
	calls := stubViewRefresh(t, viewWorkloads)
	appState = State{ProjectID: "p1", Cluster: "cl-1", ClusterProject: "pc"}
	a.setView(viewWorkloads)
	if *calls != 1 {
		t.Fatalf("never-updated cluster view triggered %d refreshes, want 1", *calls)
	}
}

func TestAutoRefreshStaleDisabledInTests(t *testing.T) {
	a := newFilterTestApp(t)
	calls := stubViewRefresh(t, viewIPs)
	appState = State{ProjectID: "p1"}
	a.setView(viewIPs)
	if *calls != 0 {
		t.Fatalf("stale auto refresh without Run flag triggered %d refreshes, want 0", *calls)
	}
}
func TestApplyAutoUpdateConfig(t *testing.T) {
	t.Cleanup(func() { staleViewAfter = 60 * time.Minute })

	applyAutoUpdateConfig(providers.AutoUpdate{Enabled: true, ThresholdInMinutes: 5})
	if staleViewAfter != 5*time.Minute {
		t.Fatalf("staleViewAfter = %v, want 5m", staleViewAfter)
	}
	if !viewUpdateStale(time.Now().UTC().Add(-6 * time.Minute).Format(time.RFC3339)) {
		t.Error("date older than configured threshold should be stale")
	}
	if viewUpdateStale(time.Now().UTC().Add(-4 * time.Minute).Format(time.RFC3339)) {
		t.Error("date within configured threshold should not be stale")
	}

	applyAutoUpdateConfig(providers.AutoUpdate{Enabled: true})
	if staleViewAfter != 60*time.Minute {
		t.Fatalf("zero threshold should fall back to 60m, got %v", staleViewAfter)
	}
	applyAutoUpdateConfig(providers.AutoUpdate{Enabled: true, ThresholdInMinutes: -3})
	if staleViewAfter != 60*time.Minute {
		t.Fatalf("negative threshold should fall back to 60m, got %v", staleViewAfter)
	}
}
