package cli

import (
	"bytes"
	"testing"
)

func TestRunHandlesVersionFlag(t *testing.T) {
	r := Run([]string{"lazycloud", "--version"}, "v1.2.3")
	if !r.Handled {
		t.Fatal("Run must handle --version")
	}
	if r.Output != "v1.2.3" {
		t.Errorf("Output = %q, want %q", r.Output, "v1.2.3")
	}
}

func TestRunHandlesVersionFlagWithEmptyVersion(t *testing.T) {
	// Zero-value version (bad ldflags) still counts as handled: the flag
	// was recognized, printing an empty line is the honest outcome.
	r := Run([]string{"lazycloud", "--version"}, "")
	if !r.Handled {
		t.Fatal("Run must handle --version even with an empty version string")
	}
	if r.Output != "" {
		t.Errorf("Output = %q, want empty", r.Output)
	}
}

func TestRunNoArgsRequestsTUI(t *testing.T) {
	r := Run([]string{"lazycloud"}, "v1.2.3")
	if r.Handled {
		t.Error("no args must not be handled, the TUI starts")
	}
	if r.Output != "" {
		t.Errorf("Output = %q, want empty for TUI requests", r.Output)
	}
}

func TestRunEmptyArgsRequestsTUI(t *testing.T) {
	// Defensive: os.Args is never empty in practice, but Run must not
	// panic if it ever is.
	r := Run(nil, "v1.2.3")
	if r.Handled {
		t.Error("empty args must not be handled")
	}
}

func TestRunUnknownArgRequestsTUI(t *testing.T) {
	for _, arg := range []string{"list", "-v", "--help", "--version=1", ""} {
		r := Run([]string{"lazycloud", arg}, "v1.2.3")
		if r.Handled {
			t.Errorf("arg %q must request the TUI, preserving pre-CLI behavior", arg)
		}
	}
}

func TestPrintWritesHandledOutput(t *testing.T) {
	var buf bytes.Buffer
	Print(Result{Handled: true, Output: "v1.2.3"}, &buf)
	if got := buf.String(); got != "v1.2.3\n" {
		t.Errorf("printed %q, want %q", got, "v1.2.3\n")
	}
}

func TestPrintSkipsUnhandled(t *testing.T) {
	var buf bytes.Buffer
	Print(Result{}, &buf)
	if buf.Len() != 0 {
		t.Errorf("printed %q for unhandled result, want nothing", buf.String())
	}
}

func TestPrintSkipsEmptyOutput(t *testing.T) {
	var buf bytes.Buffer
	Print(Result{Handled: true}, &buf)
	if buf.Len() != 0 {
		t.Errorf("printed %q for empty output, want nothing", buf.String())
	}
}
