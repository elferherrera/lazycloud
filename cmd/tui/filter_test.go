package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rivo/tview"

	"lazycloud/cmd/providers"
)

func newFilterTestApp(t *testing.T) *app {
	t.Helper()
	cache, err := providers.NewCacheAt(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("NewCacheAt() error = %v", err)
	}
	t.Cleanup(func() { cache.Close() })
	a := &app{
		tapp:     tview.NewApplication(),
		cmd:      tview.NewInputField(),
		info:     newInfoPanel(),
		summary:  newSummaryPanel(),
		status:   tview.NewTextView().SetDynamicColors(true),
		pages:    tview.NewPages(),
		cache:    cache,
		shortcuts: newShortcutsPanel(),
	}
	_, a.helpRow = newHelpOverlay(a.shortcuts)
	a.info.SetBorder(true)
	a.cmd.SetDoneFunc(a.done)
	a.cmd.SetChangedFunc(a.changed)
	if err := a.cache.SaveProjects([]providers.Project{{ProjectID: "p1", Name: "P One", ProjectNumber: "111"}}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	if err := a.cache.SaveClusters("p1", []providers.Cluster{{
		ProjectID: "p1",
		Name:      "cl-1",
		NodePools: []providers.ClusterNodePool{{Name: "pool-a", Version: "1.31", NodeCount: 2}},
	}}); err != nil {
		t.Fatalf("SaveClusters() error = %v", err)
	}
	vms := []providers.VM{
		{ProjectID: "p1", Name: "gke-cl-1-pool-a-aaa", Zone: "europe-west2-b", Status: "RUNNING", Labels: map[string]string{"goog-k8s-node-pool-name": "pool-a"}},
		{ProjectID: "p1", Name: "gke-cl-1-pool-a-bbb", Zone: "europe-west2-b", Status: "RUNNING", Labels: map[string]string{"goog-k8s-node-pool-name": "pool-a"}},
		{ProjectID: "p1", Name: "other-vm", Zone: "europe-west2-c", Status: "RUNNING", Labels: map[string]string{"env": "prod"}},
	}
	if err := a.cache.SaveVMs("p1", vms); err != nil {
		t.Fatalf("SaveVMs() error = %v", err)
	}
	return a
}

func TestShowNodePoolVMsFilter(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewNodePools
	appState.Cluster = "cl-1"
	appState.ClusterProject = "p1"
	appState.ProjectID = ""
	appState.AllProjects = true
	a.setView(viewNodePools)
	a.info.Select(1, 0)
	a.showNodePoolVMs()

	if appState.ViewKind != viewVMs {
		t.Fatalf("ViewKind = %q, want vms", appState.ViewKind)
	}
	if appState.ProjectID != "p1" {
		t.Fatalf("ProjectID = %q, want p1", appState.ProjectID)
	}
	if appState.AllProjects {
		t.Fatal("AllProjects = true, want false")
	}
	if len(a.visibleRows) != 2 {
		t.Fatalf("visibleRows len = %d, want 2 (pool VMs only): %q", len(a.visibleRows), a.visibleRows)
	}
	for _, row := range a.visibleRows {
		if !strings.HasPrefix(row[0], "gke-cl-1-pool-a-") {
			t.Fatalf("row %q is not a pool-a node VM", row[0])
		}
	}
	if len(a.filterBase) != 2 {
		t.Fatalf("filterBase len = %d, want 2", len(a.filterBase))
	}
	if !strings.Contains(a.status.GetText(true), "2 VMs for node pool pool-a") {
		t.Fatalf("status = %q, want pool-a count", a.status.GetText(true))
	}
	a.escapePressed()
	if len(a.visibleRows) != 3 {
		t.Fatalf("after Esc visibleRows len = %d, want 3 (all project VMs)", len(a.visibleRows))
	}
}

func TestApplyNodePoolVMFilterNoMatch(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	a.setView(viewVMs)
	a.applyNodePoolVMFilter("missing-pool")

	if len(a.visibleRows) != 0 {
		t.Fatalf("visibleRows len = %d, want 0", len(a.visibleRows))
	}
	if !strings.Contains(a.status.GetText(true), `No cached VMs have a label value "missing-pool"`) {
		t.Fatalf("status = %q, want no-match hint", a.status.GetText(true))
	}
}