package app

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func newFilterTestApp(t *testing.T) *app {
	t.Helper()
	cache, err := providers.NewCacheAt(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("NewCacheAt() error = %v", err)
	}
	t.Cleanup(func() { cache.Close() })
	a := &app{
		tapp:      tview.NewApplication(),
		cmd:       tview.NewInputField(),
		info:      newInfoPanel(),
		summary:   newSummaryPanel(),
		status:    tview.NewTextView().SetDynamicColors(true),
		pages:     tview.NewPages(),
		cache:     cache,
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
		{ProjectID: "p1", Name: "other-vm", Zone: "europe-west2-c", Status: "RUNNING", Labels: map[string]string{"env": "prod"},
			ServiceAccounts: []providers.VMServiceAccount{{Email: "vm-sa@p1.iam.gserviceaccount.com"}},
			DataDisks:       []providers.VMDisk{{Name: "data-1", DeviceName: "data-1", Mode: "READ_WRITE", SizeGB: 100}}},
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

func TestApplyFilterExclude(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	a.cmd.SetText("GKE - BBB")
	a.applyFilter()

	if len(a.visibleRows) != 1 {
		t.Fatalf("visibleRows len = %d, want 1: %q", len(a.visibleRows), a.visibleRows)
	}
	if a.visibleRows[0][0] != "gke-cl-1-pool-a-aaa" {
		t.Fatalf("row = %q, want gke-cl-1-pool-a-aaa", a.visibleRows[0][0])
	}
}

func TestApplyFilterExcludeOnly(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	a.cmd.SetText("- pool-a")
	a.applyFilter()

	if len(a.visibleRows) != 1 {
		t.Fatalf("visibleRows len = %d, want 1: %q", len(a.visibleRows), a.visibleRows)
	}
	if a.visibleRows[0][0] != "other-vm" {
		t.Fatalf("row = %q, want other-vm", a.visibleRows[0][0])
	}
}

func TestApplyFilterMultipleExcludes(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	a.cmd.SetText("europe - aaa - bbb")
	a.applyFilter()

	if len(a.visibleRows) != 1 {
		t.Fatalf("visibleRows len = %d, want 1: %q", len(a.visibleRows), a.visibleRows)
	}
	if a.visibleRows[0][0] != "other-vm" {
		t.Fatalf("row = %q, want other-vm", a.visibleRows[0][0])
	}
}

func TestApplyFilterTrailingExcludeKeptTyping(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	a.cmd.SetText("gke - ")
	a.applyFilter()

	if len(a.visibleRows) != 2 {
		t.Fatalf("visibleRows len = %d, want 2 (empty exclusion ignored): %q", len(a.visibleRows), a.visibleRows)
	}
}

func TestApplyFilterDashWithoutSpaceIsLiteral(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	a.cmd.SetText("pool-a-aaa")
	a.applyFilter()

	if len(a.visibleRows) != 1 {
		t.Fatalf("visibleRows len = %d, want 1", len(a.visibleRows))
	}
	if a.visibleRows[0][0] != "gke-cl-1-pool-a-aaa" {
		t.Fatalf("row = %q, want gke-cl-1-pool-a-aaa", a.visibleRows[0][0])
	}
}

func TestApplyFilterMultipleIncludes(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	a.cmd.SetText("gke + europe-west2-b + running")
	a.applyFilter()

	if len(a.visibleRows) != 2 {
		t.Fatalf("visibleRows len = %d, want 2: %q", len(a.visibleRows), a.visibleRows)
	}
	for _, row := range a.visibleRows {
		if !strings.HasPrefix(row[0], "gke-cl-1-pool-a-") {
			t.Fatalf("row %q is not a pool-a node VM", row[0])
		}
	}
}

func TestApplyFilterMultipleIncludesNoMatch(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	a.cmd.SetText("gke + other")
	a.applyFilter()

	if len(a.visibleRows) != 0 {
		t.Fatalf("visibleRows len = %d, want 0: %q", len(a.visibleRows), a.visibleRows)
	}
}

func TestApplyFilterTrailingIncludeKeptTyping(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	a.cmd.SetText("gke + ")
	a.applyFilter()

	if len(a.visibleRows) != 2 {
		t.Fatalf("visibleRows len = %d, want 2 (empty include ignored): %q", len(a.visibleRows), a.visibleRows)
	}
}

func TestApplyFilterIncludeWithExclude(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	a.cmd.SetText("europe + running - other")
	a.applyFilter()

	if len(a.visibleRows) != 2 {
		t.Fatalf("visibleRows len = %d, want 2: %q", len(a.visibleRows), a.visibleRows)
	}
	for _, row := range a.visibleRows {
		if !strings.HasPrefix(row[0], "gke-cl-1-pool-a-") {
			t.Fatalf("row %q is not a pool-a node VM", row[0])
		}
	}
}

func TestApplyFilterPlusWithoutSpaceIsLiteral(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	a.cmd.SetText("c+d")
	a.applyFilter()

	if len(a.visibleRows) != 0 {
		t.Fatalf("visibleRows len = %d, want 0 (literal match): %q", len(a.visibleRows), a.visibleRows)
	}
}

func TestApplyFilterDetailIncludeWithPlainInclude(t *testing.T) {
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	a.cmd.SetText("running + labels env=prod")
	a.applyFilter()

	if len(a.visibleRows) != 1 {
		t.Fatalf("visibleRows len = %d, want 1: %q", len(a.visibleRows), a.visibleRows)
	}
	if a.visibleRows[0][0] != "other-vm" {
		t.Fatalf("row = %q, want other-vm", a.visibleRows[0][0])
	}
}

func TestSplitDetailPath(t *testing.T) {
	for _, tc := range []struct {
		head string
		want []string
		ok   bool
	}{
		{head: "labels", want: []string{"labels"}, ok: true},
		{head: "labels.env", want: []string{"labels", "env"}, ok: true},
		{head: "ServiceAccounts.*.Email", want: []string{"ServiceAccounts", "*", "Email"}, ok: true},
		{head: "DataDisks.*.SizeGB", want: []string{"DataDisks", "*", "SizeGB"}, ok: true},
		{head: "a.b.c", want: []string{"a", "b", "c"}, ok: true},
		{head: "labels.env.x", want: []string{"labels", "env", "x"}, ok: true},
		{head: "", ok: false},
		{head: ".labels", ok: false},
		{head: "labels.", ok: false},
		{head: "*.labels", ok: false},
		{head: "labels.*", ok: false},
		{head: "labels.**.env", ok: false},
		{head: "*labels", ok: false},
		{head: "labels.*a.env", ok: false},
	} {
		got, ok := splitDetailPath(tc.head)
		if ok != tc.ok {
			t.Fatalf("splitDetailPath(%q) ok = %v, want %v", tc.head, ok, tc.ok)
		}
		if ok && !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("splitDetailPath(%q) = %q, want %q", tc.head, got, tc.want)
		}
	}
}

func applyVMsFilter(t *testing.T, a *app, query string) [][]string {
	t.Helper()
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	a.cmd.SetText(query)
	a.applyFilter()
	return a.visibleRows
}

func TestApplyFilterDetailLabels(t *testing.T) {
	a := newFilterTestApp(t)
	rows := applyVMsFilter(t, a, "Labels env=prod")
	if len(rows) != 1 || rows[0][0] != "other-vm" {
		t.Fatalf("rows = %q, want only other-vm", rows)
	}
	rows = applyVMsFilter(t, a, "labels pool-a")
	if len(rows) != 2 {
		t.Fatalf("rows len = %d, want 2: %q", len(rows), rows)
	}
}

func TestApplyFilterDetailMapKey(t *testing.T) {
	a := newFilterTestApp(t)
	rows := applyVMsFilter(t, a, "labels.env prod")
	if len(rows) != 1 || rows[0][0] != "other-vm" {
		t.Fatalf("rows = %q, want only other-vm", rows)
	}
	rows = applyVMsFilter(t, a, "labels.env staging")
	if len(rows) != 0 {
		t.Fatalf("rows len = %d, want 0", len(rows))
	}
}

func TestApplyFilterDetailServiceAccounts(t *testing.T) {
	a := newFilterTestApp(t)
	rows := applyVMsFilter(t, a, "ServiceAccounts.*.Email vm-sa")
	if len(rows) != 1 || rows[0][0] != "other-vm" {
		t.Fatalf("rows = %q, want only other-vm", rows)
	}
	rows = applyVMsFilter(t, a, "ServiceAccount.*.Email vm-sa")
	if len(rows) != 1 || rows[0][0] != "other-vm" {
		t.Fatalf("rows = %q, want only other-vm (singular alias)", rows)
	}
}

func TestApplyFilterDetailDataDisks(t *testing.T) {
	a := newFilterTestApp(t)
	rows := applyVMsFilter(t, a, "DataDisks.*.SizeGB 100")
	if len(rows) != 1 || rows[0][0] != "other-vm" {
		t.Fatalf("rows = %q, want only other-vm", rows)
	}
	rows = applyVMsFilter(t, a, "datadisks.*.sizegb 50")
	if len(rows) != 0 {
		t.Fatalf("rows len = %d, want 0", len(rows))
	}
}

func TestApplyFilterDetailStatus(t *testing.T) {
	a := newFilterTestApp(t)
	rows := applyVMsFilter(t, a, "status running")
	if len(rows) != 3 {
		t.Fatalf("rows len = %d, want 3", len(rows))
	}
}

func TestApplyFilterDetailUnknownFieldFallsBackToPlain(t *testing.T) {
	a := newFilterTestApp(t)
	rows := applyVMsFilter(t, a, "env prod")
	if len(rows) != 0 {
		t.Fatalf("rows len = %d, want 0 (plain substring matches no cell)", len(rows))
	}
}

func TestApplyFilterDetailExclude(t *testing.T) {
	a := newFilterTestApp(t)
	rows := applyVMsFilter(t, a, "europe - labels env=prod")
	if len(rows) != 2 {
		t.Fatalf("rows = %q, want the two gke VMs", rows)
	}
	for _, row := range rows {
		if row[0] == "other-vm" {
			t.Fatalf("other-vm should be excluded by the detail exclusion: %q", row)
		}
	}
}

func TestDetailPaths(t *testing.T) {
	paths := detailPaths(views[viewVMs].samples)
	for _, want := range []string{
		"Labels", "ServiceAccounts.*.Email", "ServiceAccounts.*.Scopes",
		"DataDisks.*.SizeGB", "Interfaces.*.Network", "BootDisk.SizeGB", "Zone",
	} {
		if !slices.Contains(paths, want) {
			t.Fatalf("detailPaths(vms) missing %q: %q", want, paths)
		}
	}
	if slices.Contains(paths, "Labels.env") {
		t.Fatalf("detailPaths(vms) must not enumerate map keys: %q", paths)
	}
	if slices.Index(paths, "BootDisk") != 0 || slices.Index(paths, "Zone") != len(paths)-1 {
		t.Fatalf("detailPaths(vms) not sorted with BootDisk first and Zone last: %q", paths)
	}
}

func assertCmdText(t *testing.T, a *app, want string) {
	t.Helper()
	if got := a.cmd.GetText(); got != want {
		t.Fatalf("cmd text = %q, want %q", got, want)
	}
}

func newFilterCompletionTestApp(t *testing.T) *app {
	t.Helper()
	a := newFilterTestApp(t)
	appState.ViewKind = viewVMs
	appState.ProjectID = "p1"
	appState.AllProjects = false
	appState.CmdMode = modeFilter
	a.setView(viewVMs)
	return a
}

func TestCompleteFilterDetailRotates(t *testing.T) {
	a := newFilterCompletionTestApp(t)

	a.completeFilterDetail(true)
	assertCmdText(t, a, "BootDisk ")

	a.cmd.SetText("Labels env=prod")
	a.completeFilterDetail(true)
	assertCmdText(t, a, "MachineType env=prod")

	a.cmd.SetText("labels env")
	a.completeFilterDetail(true)
	assertCmdText(t, a, "MachineType env")

	a.cmd.SetText("Zone x")
	a.completeFilterDetail(true)
	assertCmdText(t, a, "x")

	a.cmd.SetText("vm-sa")
	a.completeFilterDetail(true)
	assertCmdText(t, a, "BootDisk vm-sa")

	a.cmd.SetText("MachineType env")
	a.completeFilterDetail(false)
	assertCmdText(t, a, "Labels env")

	a.cmd.SetText("BootDisk x")
	a.completeFilterDetail(false)
	assertCmdText(t, a, "x")
}

func TestCompleteFilterDetailCompletesPrefix(t *testing.T) {
	a := newFilterCompletionTestApp(t)

	a.cmd.SetText("Servi")
	a.completeFilterDetail(true)
	assertCmdText(t, a, "ServiceAccounts ")
	a.completeFilterDetail(true)
	assertCmdText(t, a, "ServiceAccounts.* ")

	a.cmd.SetText("servi vm-sa")
	a.completeFilterDetail(true)
	assertCmdText(t, a, "ServiceAccounts vm-sa")

	a.cmd.SetText("DataDisks.*.S")
	a.completeFilterDetail(true)
	assertCmdText(t, a, "DataDisks.*.SizeGB ")

	a.cmd.SetText("In")
	a.completeFilterDetail(true)
	assertCmdText(t, a, "InstanceID ")

	a.cmd.SetText("s")
	a.completeFilterDetail(true)
	assertCmdText(t, a, "ServiceAccounts ")

	a.cmd.SetText("Servi")
	a.completeFilterDetail(false)
	assertCmdText(t, a, "ServiceAccounts.*.Scopes ")

	a.cmd.SetText("s")
	a.completeFilterDetail(false)
	assertCmdText(t, a, "Status ")
}
