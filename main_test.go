package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
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
	// --version is only a version query at args[1]. With the CLI
	// subcommands, "list --version" now fails as a usage error inside
	// the list command instead of falling through to the TUI.
	var stdout, stderr bytes.Buffer
	started := false
	startTUI := func() error { started = true; return nil }

	code := run([]string{"lazycloud", "list", "--version"}, &stdout, &stderr, "v1.2.3", startTUI)

	if code != 1 {
		t.Errorf("exit code = %d, want 1 for usage error", code)
	}
	if started {
		t.Error("list is a CLI command, --version must not start the TUI")
	}
	if stderr.Len() == 0 {
		t.Error("usage error must print a diagnostic to stderr")
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

func TestRunResourcesCommandHandled(t *testing.T) {
	// A real command flows through cli.Execute and its exit code
	// reaches the process; the TUI never starts.
	var stdout, stderr bytes.Buffer
	started := false
	startTUI := func() error { started = true; return nil }

	code := run([]string{"lazycloud", "resources"}, &stdout, &stderr, "v1.2.3", startTUI)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "projects") {
		t.Errorf("resources = %q", stdout.String())
	}
	if started {
		t.Error("resources is a CLI command, must not start the TUI")
	}
}

func TestRunListExitCodePropagates(t *testing.T) {
	// Exit codes must reach the process untouched: agents depend on
	// 1/2/3 meaning usage/not-found/infra.
	var stdout, stderr bytes.Buffer
	startTUI := func() error { return nil }

	code := run([]string{"lazycloud", "list", "dragons"}, &stdout, &stderr, "v1.2.3", startTUI)

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr.Len() == 0 {
		t.Error("usage error must print to stderr")
	}
}
