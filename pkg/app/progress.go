package app

import (
	"fmt"
	"sync"
	"time"
)

// spinnerFrames cycles in the status line while a fetch runs.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// progress tracks one background fetch so the status line can show a live
// spinner, the current step, and how many steps remain. The fetch goroutine
// calls advance while a ticker goroutine renders, so all access goes through
// the mutex.
type progress struct {
	mu    sync.Mutex
	msg   string // current step message, empty before the first step
	frame int
	step  int
	total int // known step count, 0 when unknown
	done  bool
}

func newProgress(total int) *progress {
	return &progress{total: total}
}

// advance records a new step and the message describing the work happening.
func (p *progress) advance(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.msg = msg
	p.step++
}

// current returns the status line for the ongoing step, without moving the
// spinner. ok is false before the first step or after finish.
func (p *progress) current() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status(), p.msg != "" && !p.done
}

// tick moves the spinner and returns the status line for the ongoing step.
// ok is false before the first step or after finish, so the caller can stop.
func (p *progress) tick() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.msg == "" || p.done {
		return "", false
	}
	p.frame = (p.frame + 1) % len(spinnerFrames)
	return spinnerFrames[p.frame] + " " + p.status(), true
}

func (p *progress) status() string {
	// caller holds p.mu
	s := p.msg
	if p.total > 0 {
		s += fmt.Sprintf(" (%d/%d)", p.step, p.total)
	}
	return s
}

func (p *progress) finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done = true
}

// queueUpdateDraw runs f on the UI thread. QueueUpdateDraw blocks until the
// event loop picks the update up, which deadlocks when no event loop runs
// (tests, early startup), so before that f runs inline. f must not call
// queueUpdateDraw again: the inline path holds a non-reentrant lock and a
// nested QueueUpdateDraw would block the UI thread.
func (a *app) queueUpdateDraw(f func()) {
	if a.uiRunning.Load() {
		a.tapp.QueueUpdateDraw(func() {
			a.uiMu.Lock()
			defer a.uiMu.Unlock()
			f()
		})
		return
	}
	a.uiMu.Lock()
	defer a.uiMu.Unlock()
	f()
}

// goBackground tracks background fetch goroutines so tests can wait for the
// app to go quiet before touching shared seams or state.
func (a *app) goBackground(f func()) {
	a.bg.Add(1)
	go func() {
		defer a.bg.Done()
		f()
	}()
}

// startProgressTicker refreshes the status line spinner while p runs. The
// returned stop function marks the progress finished and halts the ticker;
// call it before queueing the final status so a late tick cannot overwrite it.
func (a *app) startProgressTicker(p *progress) (stop func()) {
	done := make(chan struct{})
	// Deliberately untracked: the ticker can outlive a test's waitFor and
	// only touches widgets through queueUpdateDraw, which the ui mutex
	// serializes. Tracked fetch goroutines are the ones tests must drain.
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				a.queueUpdateDraw(func() {
					if msg, ok := p.tick(); ok {
						a.statusProgress(msg)
					}
				})
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			p.finish()
			close(done)
		})
	}
}

// allProjectsRefresh builds the refresh action shared by the all-projects
// views: it walks every cached project, fetches the view's resources in the
// background through fetch, and reports live progress in the status line.
// stepLabel names the resource in per-step messages and errors, totalLabel in
// the final summary; logView is the reportError and log view name.
func allProjectsRefresh(kind viewKind, logView, stepLabel, totalLabel string, fetch func(a *app, projectID string) (int, string, error)) func(a *app) {
	return func(a *app) {
		a.statusProgress(fmt.Sprintf("Fetching %s for all projects...", stepLabel))
		a.goBackground(func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError(logView, "all", "Failed to list projects", err)
				return
			}
			p := newProgress(len(projects))
			stop := a.startProgressTicker(p)
			defer stop()
			total, failed := 0, 0
			for _, proj := range projects {
				p.advance(fmt.Sprintf("Fetching %s for %s...", stepLabel, proj.ProjectID))
				a.queueUpdateDraw(func() {
					if cur, ok := p.current(); ok {
						a.statusProgress(cur)
					}
				})
				count, _, err := fetch(a, proj.ProjectID)
				if err != nil {
					a.reportError(logView, proj.ProjectID, "Failed to fetch "+stepLabel+" for "+proj.ProjectID, err)
					failed++
					continue
				}
				total += count
			}
			p.finish()
			a.markViewUpdated(kind, appState.ProjectID)
			a.queueUpdateDraw(func() {
				a.setView(kind)
				status := fmt.Sprintf("[green]Fetched %d %s across %d projects[-]", total, totalLabel, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		})
	}
}
