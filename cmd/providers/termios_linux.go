//go:build linux

package providers

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func isatty(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	return err == nil
}

// makeRaw turns off ECHO and canonical mode so the reply bytes arrive
// unfiltered. It returns a restore function.
func makeRaw(f *os.File) (func(), error) {
	fd := int(f.Fd())
	old, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}
	raw := *old
	raw.Lflag &^= unix.ECHO | unix.ICANON
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		return nil, err
	}
	return func() { _ = unix.IoctlSetTermios(fd, unix.TCSETS, old) }, nil
}

// waitReadable reports whether fd has data within the timeout.
func waitReadable(fd int, timeout time.Duration) bool {
	var fds unix.FdSet
	fds.Set(fd)
	tv := unix.NsecToTimeval(timeout.Nanoseconds())
	n, err := unix.Select(fd+1, &fds, nil, nil, &tv)
	return err == nil && n > 0
}
