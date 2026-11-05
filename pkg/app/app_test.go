package app

import (
	"testing"
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
