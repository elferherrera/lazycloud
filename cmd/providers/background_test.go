package providers

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseThemeMode(t *testing.T) {
	cases := []struct {
		value string
		want  ThemeMode
	}{
		{"system", ModeSystem},
		{"light", ModeLight},
		{"dark", ModeDark},
		{"", ModeDark},
		{"nonsense", ModeDark},
		{"SYSTEM", ModeDark},
	}
	for _, tc := range cases {
		if got := ParseThemeMode(tc.value); got != tc.want {
			t.Errorf("ParseThemeMode(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestIsLightColor(t *testing.T) {
	cases := []struct {
		color string
		want  bool
	}{
		{"ffff/ffff/ffff", true},
		{"0000/0000/0000", false},
		{"4c4c/4c4c/4c4c", false},
		{"8080/8080/8080", true},
		{"7", true},
		{"0", false},
		{"8", false},
		{"9", true},
		{"15", true},
		{"16", false},
		{"", false},
		{"not-a-color", false},
		{"f/f/f", true},
		{"ff/ff/ff", true},
		{"zzzz/ffff/ffff", false},
		{"ffff/ffff", false},
	}
	for _, tc := range cases {
		if got := isLightBackgroundColor(tc.color); got != tc.want {
			t.Errorf("isLightBackgroundColor(%q) = %v, want %v", tc.color, got, tc.want)
		}
	}
}

func TestParseRGBReply(t *testing.T) {
	cases := []struct {
		reply   string
		r, g, b uint8
		want    bool
	}{
		{"ffff/ffff/ffff", 255, 255, 255, true},
		{"0000/0000/0000", 0, 0, 0, true},
		{"1a2b/3c4d/5e6f", 0x1a, 0x3c, 0x5e, true},
		{"f/f/f", 255, 255, 255, true},
		{"ff/ff/ff", 255, 255, 255, true},
		{"fff/fff/fff", 255, 255, 255, true},
		{"000/fff/000", 0, 255, 0, true},
		{"ff00/ff00/ff00", 255, 255, 255, true},
		{"ffff/ffff", 0, 0, 0, false},
		{"zzzz/ffff/ffff", 0, 0, 0, false},
		{"fffff/ffff/ffff", 0, 0, 0, false},
		{"/ffff/ffff", 0, 0, 0, false},
	}
	for _, tc := range cases {
		r, g, b, ok := parseRGBReply(tc.reply)
		if ok != tc.want || (ok && (r != tc.r || g != tc.g || b != tc.b)) {
			t.Errorf("parseRGBReply(%q) = (%d,%d,%d,%v), want (%d,%d,%d,%v)", tc.reply, r, g, b, ok, tc.r, tc.g, tc.b, tc.want)
		}
	}
}

func TestColorFGBG(t *testing.T) {
	t.Setenv("COLORFGBG", "15;0")
	if got := colorFGBG(); got != "0" {
		t.Errorf("colorFGBG() = %q, want 0", got)
	}
	t.Setenv("COLORFGBG", "")
	if got := colorFGBG(); got != "" {
		t.Errorf("colorFGBG() empty = %q, want empty", got)
	}
	t.Setenv("COLORFGBG", "15;7")
	if got := colorFGBG(); got != "7" {
		t.Errorf("colorFGBG() = %q, want 7", got)
	}
	t.Setenv("COLORFGBG", "junk")
	if got := colorFGBG(); got != "" {
		t.Errorf("colorFGBG() junk = %q, want empty", got)
	}
}

func TestQueryBackgroundColorNonTTY(t *testing.T) {
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Skipf("opening dev null: %v", err)
	}
	defer devNull.Close()
	if _, ok := queryBackgroundColor(devNull, devNull); ok {
		t.Error("queryBackgroundColor(pipe) = ok, want not ok for non-tty")
	}
}

func TestQueryBackgroundColorScreenTerm(t *testing.T) {
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Skipf("opening dev null: %v", err)
	}
	defer devNull.Close()
	for _, term := range []string{"screen", "screen-256color", "tmux-256color", "dumb", ""} {
		t.Setenv("TERM", term)
		if _, ok := queryBackgroundColor(devNull, devNull); ok {
			t.Errorf("queryBackgroundColor(TERM=%q) = ok, want not ok", term)
		}
	}
}

func TestReadReplyTimeout(t *testing.T) {
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Skipf("opening dev null: %v", err)
	}
	defer devNull.Close()
	start := time.Now()
	if _, ok := readReply(devNull); ok {
		t.Error("readReply(devnull) = ok, want timeout")
	}
	elapsed := time.Since(start)
	if elapsed > 2*oscTimeout {
		t.Errorf("readReply() took %v, want bounded by oscTimeout", elapsed)
	}
}

func TestDetectBackgroundDarkFallback(t *testing.T) {
	t.Setenv("COLORFGBG", "")
	t.Setenv("TERM", "screen")
	got := DetectBackground()
	if got != BackgroundDark {
		t.Errorf("DetectBackground() no reply = %q, want dark", got)
	}
}

func TestDetectBackgroundCOLORFGBGLight(t *testing.T) {
	t.Setenv("TERM", "screen")
	t.Setenv("COLORFGBG", "15;7")
	got := DetectBackground()
	if got != BackgroundLight {
		t.Errorf("DetectBackground() COLORFGBG 7 = %q, want light", got)
	}
}
func stubTerminalSeams(t *testing.T) {
	t.Helper()
	originalIsTerminal, originalSetRawMode, originalWaitForData := isTerminal, setRawMode, waitForData
	t.Cleanup(func() {
		isTerminal, setRawMode, waitForData = originalIsTerminal, originalSetRawMode, originalWaitForData
	})
	isTerminal = func(*os.File) bool { return true }
	setRawMode = func(*os.File) (func(), error) { return func() {}, nil }
	waitForData = func(int, time.Duration) bool { return true }
}

func TestQueryBackgroundColorParsesReply(t *testing.T) {
	stubTerminalSeams(t)
	t.Setenv("TERM", "xterm-256color")

	inRead, inWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	defer inRead.Close()
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}
	defer outRead.Close()

	reply := "\x1b]11;rgb:ffff/ffff/ffff\x1b\\\x1b[1;1R"
	go func() {
		inWrite.WriteString(reply)
		inWrite.Close()
	}()

	color, ok := queryBackgroundColor(inRead, outWrite)
	if !ok {
		outWrite.Close()
		t.Fatal("queryBackgroundColor() = not ok, want parsed reply")
	}
	if color != "ffff/ffff/ffff" {
		outWrite.Close()
		t.Errorf("queryBackgroundColor() color = %q, want ffff/ffff/ffff", color)
	}
	outWrite.Close()

	query, err := io.ReadAll(outRead)
	if err != nil {
		t.Fatalf("reading query: %v", err)
	}
	if !strings.Contains(string(query), "\x1b]11;?\x07") {
		t.Errorf("query = %q, want OSC 11 background query", query)
	}
	if !strings.Contains(string(query), "\x1b[6n") {
		t.Errorf("query = %q, want cursor position request", query)
	}
}

func TestQueryBackgroundColorNoRGBInReply(t *testing.T) {
	stubTerminalSeams(t)
	t.Setenv("TERM", "xterm-256color")

	inRead, inWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	defer inRead.Close()
	outWrite, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("creating output file: %v", err)
	}
	defer outWrite.Close()

	go func() {
		inWrite.WriteString("\x1b[1;1R")
		inWrite.Close()
	}()

	if _, ok := queryBackgroundColor(inRead, outWrite); ok {
		t.Error("queryBackgroundColor() cursor-only reply = ok, want not ok")
	}
}

func TestQueryBackgroundColorRawModeFailure(t *testing.T) {
	originalIsTerminal, originalSetRawMode := isTerminal, setRawMode
	t.Cleanup(func() { isTerminal, setRawMode = originalIsTerminal, originalSetRawMode })
	isTerminal = func(*os.File) bool { return true }
	setRawMode = func(*os.File) (func(), error) { return nil, errors.New("no raw mode") }

	t.Setenv("TERM", "xterm-256color")
	if _, ok := queryBackgroundColor(os.Stdin, os.Stdout); ok {
		t.Error("queryBackgroundColor() raw mode failure = ok, want not ok")
	}
}

func TestQueryBackgroundColorWriteError(t *testing.T) {
	stubTerminalSeams(t)
	t.Setenv("TERM", "xterm-256color")

	devNullRead, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("opening dev null: %v", err)
	}
	defer devNullRead.Close()
	outFile, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("creating output file: %v", err)
	}
	defer outFile.Close()

	if _, ok := queryBackgroundColor(devNullRead, outFile); ok {
		// Reading /dev/null returns EOF on the first read; the query itself
		// writes fine to stdout, so the not-ok comes from readReply EOF.
		t.Error("queryBackgroundColor() devnull stdin = ok, want not ok")
	}
}

func TestQueryBackgroundColorReplyTooLong(t *testing.T) {
	stubTerminalSeams(t)
	t.Setenv("TERM", "xterm-256color")

	inRead, inWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	defer inRead.Close()
	outWrite, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("creating output file: %v", err)
	}
	defer outWrite.Close()

	go func() {
		for i := 0; i < 150; i++ {
			inWrite.WriteString("x")
		}
		inWrite.Close()
	}()

	if _, ok := queryBackgroundColor(inRead, outWrite); ok {
		t.Error("queryBackgroundColor() oversized reply = ok, want not ok")
	}
}

func TestReadReplyReadError(t *testing.T) {
	stubTerminalSeams(t)
	inRead, _, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	inRead.Close()

	if _, ok := readReply(inRead); ok {
		t.Error("readReply(closed) = ok, want read error")
	}
}

func TestIsTerminalChecks(t *testing.T) {
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Skipf("opening dev null: %v", err)
	}
	defer devNull.Close()
	if isTerminal(devNull) {
		t.Error("isTerminal(devnull) = true, want false for a file")
	}
	if _, err := makeRaw(devNull); err == nil {
		t.Error("makeRaw(devnull) = no error, want ioctl failure for a file")
	}
}

func TestWaitReadableBlocks(t *testing.T) {
	inRead, _, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	defer inRead.Close()
	if waitReadable(int(inRead.Fd()), 50*time.Millisecond) {
		t.Error("waitReadable(empty pipe) = true, want timeout false")
	}
}

func TestQueryBackgroundColorScreenTermWithStubs(t *testing.T) {
	stubTerminalSeams(t)
	for _, term := range []string{"screen", "tmux-256color", "dumb", ""} {
		t.Setenv("TERM", term)
		if _, ok := queryBackgroundColor(os.Stdin, os.Stdout); ok {
			t.Errorf("queryBackgroundColor(TERM=%q) = ok, want not ok", term)
		}
	}
}

func TestQueryBackgroundColorWriteFails(t *testing.T) {
	stubTerminalSeams(t)
	t.Setenv("TERM", "xterm-256color")

	inRead, _, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	defer inRead.Close()
	closed, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatalf("creating output file: %v", err)
	}
	closed.Close()

	if _, ok := queryBackgroundColor(inRead, closed); ok {
		t.Error("queryBackgroundColor(closed out) = ok, want write failure")
	}
}

func TestQueryBackgroundColorNoTerminatorReply(t *testing.T) {
	stubTerminalSeams(t)
	t.Setenv("TERM", "xterm-256color")

	inRead, inWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	defer inRead.Close()
	outWrite, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("creating output file:  %v", err)
	}
	defer outWrite.Close()

	go func() {
		inWrite.WriteString("\x1b]11;rgb:ffff/ffff")
		inWrite.Close()
	}()

	if _, ok := queryBackgroundColor(inRead, outWrite); ok {
		t.Error("queryBackgroundColor(no terminator) = ok, want not ok")
	}
}

func TestQueryBackgroundColorReadReplyFails(t *testing.T) {
	stubTerminalSeams(t)
	originalWaitForData := waitForData
	t.Cleanup(func() { waitForData = originalWaitForData })
	waitForData = func(int, time.Duration) bool { return false }

	t.Setenv("TERM", "xterm-256color")
	inRead, _, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	defer inRead.Close()
	outFile, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("creating output file: %v", err)
	}
	defer outFile.Close()
	if _, ok := queryBackgroundColor(inRead, outFile); ok {
		t.Error("queryBackgroundColor(read fail) = ok, want not ok")
	}
}

func TestReadReplyDeadlineExpiry(t *testing.T) {
	originalTimeout := oscTimeout
	t.Cleanup(func() { oscTimeout = originalTimeout })
	oscTimeout = 100 * time.Millisecond

	inRead, _, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	defer inRead.Close()
	if _, ok := readReply(inRead); ok {
		t.Error("readReply(deadline) = ok, want timeout not ok")
	}
}

func TestQueryBackgroundColorRGBWithoutTerminator(t *testing.T) {
	stubTerminalSeams(t)
	t.Setenv("TERM", "xterm-256color")

	inRead, inWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	defer inRead.Close()
	outWrite, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("creating output file: %v", err)
	}
	defer outWrite.Close()

	go func() {
		// rgb payload present, cursor response absent: no BEL/ESC terminator
		// after the color and no R. readReply consumes until EOF.
		inWrite.WriteString("\x1b]11;rgb:ffff/ffff/ffff")
		inWrite.Close()
	}()

	if _, ok := queryBackgroundColor(inRead, outWrite); ok {
		t.Error("queryBackgroundColor(rgb no terminator) = ok, want not ok")
	}
}

func TestReadReplyLoopNeverEntered(t *testing.T) {
	originalTimeout := oscTimeout
	t.Cleanup(func() { oscTimeout = originalTimeout })
	oscTimeout = -time.Second

	inRead, _, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	defer inRead.Close()
	if _, ok := readReply(inRead); ok {
		t.Error("readReply(negative timeout) = ok, want immediate not ok")
	}
}

func TestQueryBackgroundColorRGBNoTerminatorBeforeCursor(t *testing.T) {
	stubTerminalSeams(t)
	t.Setenv("TERM", "xterm-256color")

	inRead, inWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	defer inRead.Close()
	outWrite, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("creating output file: %v", err)
	}
	defer outWrite.Close()

	go func() {
		// Malformed reply: rgb payload with no ESC/BEL terminator, straight
		// into the cursor response R. readReply succeeds, the color parse
		// must reject it.
		inWrite.WriteString("\x1b]11;rgb:ffff/ffff/ffffR")
		inWrite.Close()
	}()

	if _, ok := queryBackgroundColor(inRead, outWrite); ok {
		t.Error("queryBackgroundColor(no terminator before R) = ok, want not ok")
	}
}
