package app

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/elferherrera/lazycloud/cmd/providers"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// newLoadingTestApp builds the minimal app the clock needs: an info
// panel wired through the real populateInfoPanel path, with the clock
// draw function installed the way run.go installs it.
func newLoadingTestApp(t *testing.T) *app {
	t.Helper()
	cache, err := providers.NewCacheAt(t.TempDir() + "/cache.db")
	if err != nil {
		t.Fatalf("NewCacheAt() error = %v", err)
	}
	t.Cleanup(func() { cache.Close() })
	a := &app{
		tapp:   tview.NewApplication(),
		cmd:    tview.NewInputField(),
		info:   newInfoPanel(),
		status: tview.NewTextView().SetDynamicColors(true),
		pages:  tview.NewPages(),
		cache:  cache,
	}
	a.info.SetBorder(true)
	a.installLoadingDraw()
	a.viewHeaders = []string{"Name", "Status"}
	return a
}

// drawLoading renders the clock through the panel's real draw function
// and returns the painted rows as strings.
func drawLoading(t *testing.T, a *app, width, height int) []string {
	t.Helper()
	return strings.Split(drawScreen(t, width, height, func(screen tcell.Screen) {
		a.info.SetRect(0, 0, width, height)
		a.info.Draw(screen)
	}), "\n")
}

// TestClockPaintsWhileFetching pins the on state: a fetch that reports
// progress must stamp the clock on the empty panel's center, with the
// message and the elapsed time under it, and no data rows touched.
func TestClockPaintsWhileFetching(t *testing.T) {
	a := newLoadingTestApp(t)
	a.applyFilter() // empty panel: headers only

	a.statusProgress("Fetching VMs...")
	a.loading.frame = 3 // pin a hand so the glyph is deterministic

	rows := drawLoading(t, a, 40, 12)
	var painted []string
	for _, r := range rows {
		if strings.Contains(r, "🕓") || strings.Contains(r, "Fetching VMs") || strings.Contains(r, "elapsed") {
			painted = append(painted, r)
		}
	}
	if len(painted) == 0 {
		t.Fatalf("clock never painted: %q", rows)
	}
	clockRow, msgRow, elapsedRow := -1, -1, -1
	for i, r := range rows {
		if strings.Contains(r, "🕓") {
			clockRow = i
		}
		if strings.Contains(r, "Fetching VMs") {
			msgRow = i
		}
		if strings.Contains(r, "elapsed") {
			elapsedRow = i
		}
	}
	if clockRow == -1 || msgRow == -1 || elapsedRow == -1 {
		t.Fatalf("clock incomplete: clock=%d msg=%d elapsed=%d", clockRow, msgRow, elapsedRow)
	}
	if msgRow != clockRow+1 || elapsedRow != clockRow+2 {
		t.Fatalf("clock lines stacked wrong: clock=%d msg=%d elapsed=%d", clockRow, msgRow, elapsedRow)
	}
}

// TestClockCentersInPanel pins the geometry: the width-2 emoji glyph
// sits with equal gaps to the left and right borders, the centered
// position a rune count would miss by one.
func TestClockCentersInPanel(t *testing.T) {
	a := newLoadingTestApp(t)
	a.applyFilter()

	a.statusProgress("Fetching VMs...")
	a.loading.frame = 0

	const width = 42 // even inner width: a width-2 glyph can center exactly
	rows := drawLoading(t, a, width, 12)
	for i, r := range rows {
		col := runeIndex(r, "🕐")
		if col < 0 {
			continue
		}
		// Glyph spans display cols col and col+1; the gaps to the borders
		// (col 0 and width-1) must match for true centering.
		leftGap := col - 1
		rightGap := width - 1 - (col + 2)
		if leftGap != rightGap {
			t.Fatalf("clock off center at row %d: left gap %d, right gap %d", i, leftGap, rightGap)
		}
		return
	}
	t.Fatal("clock glyph never painted")
}

// runeIndex returns the rune column where needle starts in s, or -1.
func runeIndex(s, needle string) int {
	col := 0
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], needle) {
			return col
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		col++
	}
	return -1
}

// TestClockStopsOnTerminalStatus pins the off state: every terminal
// status (error, success, or the plain reset) must stop the ticker, and
// the next paint shows a clean panel, no clock stamped on it.
func TestClockStopsOnTerminalStatus(t *testing.T) {
	a := newLoadingTestApp(t)
	a.applyFilter()

	a.statusProgress("Fetching VMs...")
	if a.loading == nil || a.loading.stop == nil {
		t.Fatal("clock not running after statusProgress")
	}

	a.statusText("[green]Fetched 12 VMs[-]")

	if a.loading.stop != nil {
		t.Error("clock ticker still running after a terminal status")
	}
	rows := drawLoading(t, a, 40, 12)
	for _, r := range rows {
		for _, f := range clockFrames {
			if strings.Contains(r, f) {
				t.Fatalf("clock glyph survived the terminal status: %q", r)
			}
		}
		if strings.Contains(r, "Fetching VMs") || strings.Contains(r, "elapsed") {
			t.Fatalf("clock text survived the terminal status: %q", r)
		}
	}
}

// TestClockStaysOffPopulatedPanels pins the refresh case: when the
// panel already shows rows, a background refresh keeps them on screen
// instead of trading them for the clock.
func TestClockStaysOffPopulatedPanels(t *testing.T) {
	a := newLoadingTestApp(t)
	a.setFilterBase([][]string{{"vm-1", "running"}})
	a.applyFilter()

	a.statusProgress("Fetching VMs...")

	rows := drawLoading(t, a, 40, 12)
	for _, r := range rows {
		for _, f := range clockFrames {
			if strings.Contains(r, f) {
				t.Fatalf("clock painted over data rows: %q", r)
			}
		}
	}
	if !containsRow(rows, "vm-1") {
		t.Fatalf("data row vanished under the clock: %q", rows)
	}
}

// TestClockSurvivesRepaints pins the draw-func placement: filter edits
// repaint the panel constantly while a fetch runs, and the clock must
// still paint after an empty populateInfoPanel.
func TestClockSurvivesRepaints(t *testing.T) {
	a := newLoadingTestApp(t)
	a.applyFilter()

	a.statusProgress("Fetching VMs...")
	a.applyFilter() // the filter changed, panel still empty

	rows := drawLoading(t, a, 40, 12)
	if !containsRow(rows, "Fetching VMs") {
		t.Fatalf("clock erased by a repaint: %q", rows)
	}
}

// TestClockTickerAdvancesFrames pins the animation itself: with the
// ticker running, the frame counter moves within a tick interval.
func TestClockTickerAdvancesFrames(t *testing.T) {
	a := newLoadingTestApp(t)
	a.applyFilter()

	a.statusProgress("Fetching VMs...")
	first := a.loading.frame

	deadline := time.Now().Add(3 * time.Second)
	moved := false
	for time.Now().Before(deadline) {
		a.uiMu.Lock()
		cur := a.loading.frame
		a.uiMu.Unlock()
		if cur != first {
			moved = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	a.statusText("")
	if !moved {
		t.Fatal("clock frame never advanced")
	}
}

// TestLoadingMessageTrimsStatusDecorations pins the message adapter: the
// panel carries the message alone, without the status line's spinner
// prefix or step counter.
func TestLoadingMessageTrimsStatusDecorations(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Fetching VMs...", "Fetching VMs..."},
		{"⠙ Fetching VMs...", "Fetching VMs..."},
		{"Fetching VMs... (3/12)", "Fetching VMs..."},
		{"⠹ Updating all views (2/10)", "Updating all views"},
		{"Fetching things (v2) ok", "Fetching things (v2) ok"}, // not a step counter
	}
	for _, c := range cases {
		if got := loadingMessage(c.in); got != c.want {
			t.Errorf("loadingMessage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestStartLoadingWithoutPanelIsSafe pins the nil guard: an app whose
// info panel never got built must not crash when a fetch reports
// progress.
func TestStartLoadingWithoutPanelIsSafe(t *testing.T) {
	a := &app{status: tview.NewTextView().SetDynamicColors(true)}
	a.statusProgress("Fetching VMs...")
	a.statusText("")
	if a.loading != nil {
		t.Error("loading state created without a panel to paint it on")
	}
}

// containsRow reports whether any rendered row carries s.
func containsRow(rows []string, s string) bool {
	for _, r := range rows {
		if strings.Contains(r, s) {
			return true
		}
	}
	return false
}