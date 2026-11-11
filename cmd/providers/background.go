package providers

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// BackgroundKind is the color scheme of the terminal as detected, or
// assumed, by lazycloud.
type BackgroundKind string

const (
	BackgroundDark  BackgroundKind = "dark"
	BackgroundLight BackgroundKind = "light"
)

// oscTimeout bounds the wait for a terminal reply to the background query.
// A var so tests can shrink it.
var oscTimeout = 3 * time.Second

// Terminal access seams. Tests swap them because pipes are not terminals and
// cannot enter raw mode. The makeRaw success path needs a real tty fd: it is
// exercised only when a user launches lazycloud in an interactive terminal,
// not by these tests.
var (
	isTerminal  = isatty
	setRawMode  = makeRaw
	waitForData = waitReadable
)

// DetectBackground asks the terminal for its background color and reports
// whether it is light or dark. Terminals that cannot answer the query
// (pipes, screen, tmux, dumb terminals) fall back to COLORFGBG and then to
// dark, the common terminal default.
func DetectBackground() BackgroundKind {
	color, ok := queryBackgroundColor(os.Stdin, os.Stdout)
	if !ok {
		color = colorFGBG()
	}
	if isLightBackgroundColor(color) {
		return BackgroundLight
	}
	return BackgroundDark
}

// queryBackgroundColor sends the OSC 11 background query followed by a
// cursor position request. Terminals that ignore OSC 11 still answer the
// cursor request, so it doubles as the end marker for the exchange.
func queryBackgroundColor(in *os.File, out *os.File) (string, bool) {
	if !isTerminal(in) || !isTerminal(out) {
		return "", false
	}
	term := os.Getenv("TERM")
	if term == "" || strings.HasPrefix(term, "screen") || strings.HasPrefix(term, "tmux") || strings.HasPrefix(term, "dumb") {
		return "", false
	}
	restore, err := setRawMode(in)
	if err != nil {
		return "", false
	}
	defer restore()

	if _, err := out.WriteString("\x1b]11;?\x07\x1b[6n"); err != nil {
		return "", false
	}
	reply, ok := readReply(in)
	if !ok {
		return "", false
	}
	marker := strings.Index(reply, "rgb:")
	if marker < 0 {
		return "", false
	}
	rest := reply[marker+len("rgb:"):]
	end := strings.IndexAny(rest, "\x07\x1b")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

// readReply consumes terminal replies until the cursor position report
// (terminated by R) closes the exchange, or the timeout expires.
func readReply(in *os.File) (string, bool) {
	fd := int(in.Fd())
	deadline := time.Now().Add(oscTimeout)
	var reply []byte
	for remaining := time.Until(deadline); remaining > 0; remaining = time.Until(deadline) {
		if !waitForData(fd, remaining) {
			return "", false
		}
		var b [1]byte
		if _, err := in.Read(b[:]); err != nil {
			return "", false
		}
		reply = append(reply, b[0])
		if b[0] == 'R' {
			return string(reply), true
		}
		if len(reply) > 100 {
			return "", false
		}
	}
	return "", false
}

// colorFGBG returns the background palette index from the COLORFGBG
// convention some terminals export, empty when absent or malformed.
func colorFGBG() string {
	value := os.Getenv("COLORFGBG")
	if value == "" {
		return ""
	}
	fields := strings.Split(value, ";")
	last := fields[len(fields)-1]
	if _, err := strconv.Atoi(last); err != nil {
		return ""
	}
	return last
}

// isLightBackgroundColor classifies an "rrrr/gggg/bbbb" OSC reply or a
// COLORFGBG palette index. Anything unparsable counts as dark.
func isLightBackgroundColor(color string) bool {
	color = strings.TrimSpace(color)
	if color == "" {
		return false
	}
	if index, err := strconv.Atoi(color); err == nil {
		// Standard palette: 7 (white) and the bright range 9-15 are light.
		return index == 7 || (index >= 9 && index <= 15)
	}
	r, g, b, ok := parseRGBReply(color)
	if !ok {
		return false
	}
	// BT.601 luma, the weighting Vim uses for background detection.
	luma := (299*int(r) + 587*int(g) + 114*int(b)) / 1000
	return luma > 127
}

// parseRGBReply parses the "rrrr/gggg/bbbb" payload of an OSC 11 reply.
// Components carry 1 to 4 hex digits and are scaled to 8 bits.
func parseRGBReply(reply string) (uint8, uint8, uint8, bool) {
	parts := strings.Split(reply, "/")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var rgb [3]uint8
	for i, part := range parts {
		if part == "" {
			return 0, 0, 0, false
		}
		if len(part) > 4 {
			return 0, 0, 0, false
		}
		v, err := strconv.ParseUint(part, 16, 32)
		if err != nil {
			return 0, 0, 0, false
		}
		if len(part) == 1 {
			v *= 0x11
		} else if len(part) == 3 {
			v = v * 255 / 0xFFF
		}
		if v > 0xFF {
			v >>= 8
		}
		rgb[i] = uint8(v)
	}
	return rgb[0], rgb[1], rgb[2], true
}
