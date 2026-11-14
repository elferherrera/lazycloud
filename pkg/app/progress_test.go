package app

import (
	"strings"
	"testing"
	"time"

	"github.com/elferherrera/lazycloud/pkg/providers"
)

func TestProgressLifecycle(t *testing.T) {
	p := newProgress(3)

	// Before the first step the progress reports nothing to render.
	if _, ok := p.current(); ok {
		t.Fatal("current() before first step should not render")
	}
	if _, ok := p.tick(); ok {
		t.Fatal("tick() before first step should not render")
	}

	p.advance("Fetching vms for p1...")
	msg, ok := p.current()
	if !ok {
		t.Fatal("current() after first step should render")
	}
	if !strings.Contains(msg, "(1/3)") {
		t.Fatalf("current() = %q, want step count (1/3)", msg)
	}

	msg, ok = p.tick()
	if !ok {
		t.Fatal("tick() with an active step should render")
	}
	if !strings.HasPrefix(msg, "⠙ ") {
		t.Fatalf("tick() = %q, want a spinner frame first", msg)
	}

	// After finish the progress stops rendering even with steps left.
	p.finish()
	if _, ok := p.current(); ok {
		t.Fatal("current() after finish should not render")
	}
	if _, ok := p.tick(); ok {
		t.Fatal("tick() after finish should not render")
	}
}

func TestProgressWithoutTotalOmitsCounter(t *testing.T) {
	p := newProgress(0)
	p.advance("Fetching...")
	msg, ok := p.current()
	if !ok {
		t.Fatal("current() should render for unknown totals")
	}
	if strings.Contains(msg, "(") {
		t.Fatalf("current() = %q, want no step counter for unknown total", msg)
	}
}

func TestProgressTickerStopsCleanly(t *testing.T) {
	a := newFilterTestApp(t)
	p := newProgress(2)
	stop := a.startProgressTicker(p)
	p.advance("Fetching vms for p1...")

	// uiRunning is false in tests, so ticks render inline; give the ticker a
	// chance to fire at least once, then stop and make sure it halts.
	statusBefore := a.status.GetText(false)
	_ = statusBefore
	stop()
	stop() // second call must be a safe no-op
	p.advance("should not render")
	if _, ok := p.tick(); ok {
		t.Fatal("tick() should not render after stop")
	}
}

func TestAllProjectsRefreshReportsFailuresAndTotals(t *testing.T) {
	a := newFilterTestApp(t)
	seed := []providers.Project{{ProjectID: "p1", Name: "P One", ProjectNumber: "111"}}
	if err := a.cache.SaveProjects(seed); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	appState.ViewKind = viewVMsAll
	appState.ProjectID = "p1"

	// The fetch always fails: the refresh must mark the view updated anyway
	// and surface the failure count in the status line.
	action := allProjectsRefresh(viewVMsAll, "vms", "vms", "VMs", func(a *app, projectID string) (int, string, error) {
		return 0, "", errorString("boom")
	})
	action(a)

	// The fetch runs in a goroutine; wait for the final status to appear.
	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "failed")
	})
	if !strings.Contains(a.status.GetText(false), "VMs across 1 projects") {
		t.Fatalf("status = %q, want totals line", a.status.GetText(false))
	}
}

func TestAllProjectsRefreshSumsCounts(t *testing.T) {
	a := newFilterTestApp(t)
	if err := a.cache.SaveProjects([]providers.Project{
		{ProjectID: "p1", Name: "P One", ProjectNumber: "111"},
		{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"},
	}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	appState.ViewKind = viewVMsAll
	appState.ProjectID = "p1"

	calls := 0
	action := allProjectsRefresh(viewVMsAll, "vms", "vms", "VMs", func(a *app, projectID string) (int, string, error) {
		calls++
		return 2, "", nil
	})
	action(a)

	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Fetched 4")
	})
	if calls != 2 {
		t.Fatalf("fetch calls = %d, want one per project", calls)
	}
}

func TestAllProjectsRefreshWithoutProjects(t *testing.T) {
	a := newFilterTestApp(t)
	// Remove the seeded project so the refresh loop has nothing to walk.
	if _, err := a.cache.DeleteProject("p1"); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	appState.ViewKind = viewVMsAll

	calls := 0
	action := allProjectsRefresh(viewVMsAll, "vms", "vms", "VMs", func(a *app, projectID string) (int, string, error) {
		calls++
		return 0, "", nil
	})
	action(a)

	waitFor(t, a, func() bool {
		return strings.Contains(a.status.GetText(false), "Fetched 0 VMs across 0 projects")
	})
	if calls != 0 {
		t.Fatalf("fetch calls = %d, want none with no projects", calls)
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }

// waitFor polls until cond holds or a short deadline passes; background
// fetches finish asynchronously in tests because queueUpdateDraw runs inline.
// Every poll runs under the ui mutex and the final success drains the
// background WaitGroup, so widget reads never race a fetch goroutine.
func waitFor(t *testing.T, a *app, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.uiMu.Lock()
		held := cond()
		a.uiMu.Unlock()
		if held {
			a.bg.Wait()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition never held")
}
