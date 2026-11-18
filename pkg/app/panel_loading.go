package app

import (
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// clockFrames sweeps a clock hand around the dial: 12 glyphs, one per
// hour mark, the same reading order a wall clock walks.
var clockFrames = []string{
	"🕐", "🕑", "🕒", "🕓", "🕔", "🕕",
	"🕖", "🕗", "🕘", "🕙", "🕚", "🕛",
}

// panelLoadingTick is how often the clock advances one frame. Slow
// enough to feel calm, fast enough to read as alive.
const panelLoadingTick = 500 * time.Millisecond

// panelLoading drives the clock painted inside the view panel. The
// status line already reports fetches, but an empty panel reads as
// broken while one runs: this stamps a rotating clock, the message,
// and the elapsed time on the panel's center until the fetch ends.
//
// The clock is painted from the table's draw function, not as a data
// row, so repaints and resizes cannot dislodge it and it never takes a
// real row's place. The counter half (frame, started) only moves on the
// ticker goroutine while loading is on, so it needs no lock of its own:
// reads happen on the UI thread after queueUpdateDraw serialized the
// write. The message half is written from fetch goroutines through
// queueUpdateDraw, which the ui mutex already serializes.
type panelLoading struct {
	frame   int
	started time.Time
	msg     string
	stop    chan struct{}
	once    sync.Once
}

// startLoading turns the clock on. The panel repaints with the clock
// right away and a ticker goroutine advances the hand until stop.
func (a *app) startLoading(msg string) {
	if a.info == nil {
		return
	}
	if a.loading == nil {
		a.loading = &panelLoading{}
	}
	l := a.loading
	l.msg = msg
	if l.stop != nil {
		// Already loading: the running ticker picks the new message up.
		return
	}
	l.stop = make(chan struct{})
	l.once = sync.Once{}
	l.frame = 0
	l.started = time.Now()
	// Deliberately untracked: the goroutine exits through stop and only
	// touches the panel through queueUpdateDraw, the same lifetime the
	// progress ticker already has. Tracked fetch goroutines are the
	// ones tests must drain.
	go func() {
		ticker := time.NewTicker(panelLoadingTick)
		defer ticker.Stop()
		for {
			select {
			case <-l.stop:
				return
			case <-ticker.C:
				a.queueUpdateDraw(func() {
					l.frame = (l.frame + 1) % len(clockFrames)
				})
			}
		}
	}()
}

// stopLoading turns the clock off. Any terminal status ends the fetch it
// belonged to, so the next repaint shows rows or a clean panel. Safe
// when no load runs.
func (a *app) stopLoading() {
	if a.info == nil || a.loading == nil {
		return
	}
	l := a.loading
	if l.stop != nil {
		l.once.Do(func() { close(l.stop) })
		l.stop = nil
	}
}

// drawLoadingClock stamps the clock, the message, and the elapsed time
// on the panel's center. tview calls it after the table's frame is
// painted and before the cells, only while a fetch runs and the panel
// has no rows to show.
func (a *app) drawLoadingClock(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
	innerX, innerY, innerW, innerH := a.info.GetInnerRect()
	l := a.loading
	if l == nil || l.stop == nil || len(a.visibleRows) > 0 {
		return innerX, innerY, innerW, innerH
	}
	if innerW <= 0 || innerH <= 1 {
		return innerX, innerY, innerW, innerH
	}

	msg := strings.TrimSpace(l.msg)
	elapsed := time.Since(l.started).Truncate(time.Second)
	// The clock sits on the panel's middle row, message and elapsed
	// time under it, so nothing crowds the header row.
	clockY := innerY + innerH/2
	color := theme.statusTextColor()

	printClockLine(screen, innerX, clockY, innerW, clockFrames[l.frame%len(clockFrames)], color)
	if innerH >= 4 && msg != "" {
		printClockLine(screen, innerX, clockY+1, innerW, msg, color)
	}
	if innerH >= 5 {
		printClockLine(screen, innerX, clockY+2, innerW, elapsed.String()+" elapsed", color)
	}
	return innerX, innerY, innerW, innerH
}

// printClockLine centers one clock line inside the panel's inner rect.
// tview.Print walks the string with uniseg, so a width-2 emoji clock
// centers correctly where a rune count would drift.
func printClockLine(screen tcell.Screen, x, row, width int, s string, color tcell.Color) {
	tview.Print(screen, s, x, row, width, tview.AlignCenter, color)
}

// loadingMessage keeps the panel's message in step with the status line:
// strip the leading spinner and step counter so the panel stays short.
func loadingMessage(status string) string {
	msg := status
	// Drop a leading spinner frame followed by its space.
	if runes := []rune(msg); len(runes) > 1 && isSpinnerRune(runes[0]) && runes[1] == ' ' {
		msg = string(runes[2:])
	}
	// Drop a trailing step counter like (3/12).
	if i := strings.LastIndex(msg, " ("); i != -1 && strings.HasSuffix(msg, ")") {
		if strings.Trim(msg[i+2:len(msg)-1], "0123456789/") == "" {
			msg = msg[:i]
		}
	}
	return msg
}

// isSpinnerRune reports whether r is one of the status-line frames. The
// frames are multi-byte braille runes, so callers must decode runes, not
// index bytes.
func isSpinnerRune(r rune) bool {
	for _, f := range spinnerFrames {
		if f == string(r) {
			return true
		}
	}
	return false
}

// installLoadingDraw wires the clock into the panel's draw function.
// Called once at setup, after newInfoPanel.
func (a *app) installLoadingDraw() {
	if a.info == nil {
		return
	}
	a.info.SetDrawFunc(a.drawLoadingClock)
}