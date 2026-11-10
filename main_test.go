package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/elferherrera/lazycloud/pkg/cli"
)

func TestRunVersionFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tuiStarted := false
	startTUI := func() error { tuiStarted = true; return nil }

	code := run([]string{"lazycloud", "--version"}, &stdout, &stderr, "v1.2.3", startTUI)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got := stdout.String(); got != "v1.2.3\n" {
		t.Errorf("stdout = %q, want %q", got, "v1.2.3\n")
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
	if tuiStarted {
		t.Error("TUI must not start for --version")
	}
}

func TestRunVersionFlagIgnoresExtraArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	started := false
	startTUI := func() error { started = true; return nil }

	code := run([]string{"lazycloud", "--version", "extra"}, &stdout, &stderr, "v1.2.3", startTUI)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got := stdout.String(); got != "v1.2.3\n" {
		t.Errorf("stdout = %q, want %q", got, "v1.2.3\n")
	}
	if started {
		t.Error("extra args after --version must not start the TUI")
	}
}

func TestRunDefaultStartsTUI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	started := false
	startTUI := func() error { started = true; return nil }

	code := run([]string{"lazycloud"}, &stdout, &stderr, "v1.2.3", startTUI)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if started != true {
		t.Error("no args must start the TUI")
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunUnknownArgStartsTUI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	started := false
	startTUI := func() error { started = true; return nil }

	code := run([]string{"lazycloud", "not-a-flag"}, &stdout, &stderr, "v1.2.3", startTUI)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !started {
		t.Error("unknown args must keep starting the TUI, preserving pre-CLI behavior")
	}
}

func TestRunTUIErrorReportsAndExitsOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	startTUI := func() error { return errors.New("open /dev/tty: device not configured") }

	code := run([]string{"lazycloud"}, &stdout, &stderr, "v1.2.3", startTUI)

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	want := "error running application: open /dev/tty: device not configured\n"
	if got := stderr.String(); got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunVersionAtSecondPositionOnly(t *testing.T) {
	// --version anywhere but args[1] is not a version query; the TUI
	// starts. Same contract as the pre-CLI main.go.
	var stdout, stderr bytes.Buffer
	started := false
	startTUI := func() error { started = true; return nil }

	code := run([]string{"lazycloud", "list", "--version"}, &stdout, &stderr, "v1.2.3", startTUI)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !started {
		t.Error("--version after another arg must not be treated as the version flag")
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

// TestRunDevVersionDefault locks the ldflags contract: without -X the
// binary must print the dev default, exactly what CI verifies with the
// tag value.
func TestRunDevVersionDefault(t *testing.T) {
	var stdout, stderr bytes.Buffer
	startTUI := func() error { return nil }

	code := run([]string{"lazycloud", "--version"}, &stdout, &stderr, "dev", startTUI)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got := stdout.String(); got != "dev\n" {
		t.Errorf("stdout = %q, want %q", got, "dev\n")
	}
}

var _ = cli.Run // compile-time link to the dispatch package under test
