//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package providers

import (
	"errors"
	"os"
	"time"
)

// isatty reports false because this platform cannot enter raw mode, so
// no fd qualifies as a queryable terminal and detection falls back to
// COLORFGBG or dark.
func isatty(f *os.File) bool {
	return false
}

// makeRaw is unsupported on this platform, so the background query is
// skipped and detection falls back to COLORFGBG or dark.
func makeRaw(f *os.File) (func(), error) {
	return nil, errors.New("raw mode unsupported on this platform")
}

func waitReadable(fd int, timeout time.Duration) bool {
	return false
}
