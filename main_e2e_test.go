package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMainFunctionE2E runs the real binary in a subprocess, the standard
// pattern for covering the os.Exit adapter in main(). The binary is built
// with -cover; the coverage run (mise run coverage) starts the test binary
// with GOCOVERDIR set, which propagates to the child so main() lands in the
// merged profile. Plain runs set a temp GOCOVERDIR because an
// instrumented binary without it warns on stderr.
func TestMainFunctionE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary, skipped in -short mode")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go not on PATH: %v", err)
	}

	bin := filepath.Join(t.TempDir(), "lazycloud-under-test")
	build := exec.Command(goBin, "build", "-cover", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build -cover failed: %v\n%s", err, out)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, "--version")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	coverDir := os.Getenv("GOCOVERDIR")
	if coverDir == "" {
		coverDir = t.TempDir()
	}
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)

	if err := cmd.Run(); err != nil {
		t.Fatalf("binary exited with error: %v\nstderr: %s", err, stderr.String())
	}
	if code := cmd.ProcessState.ExitCode(); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got := stdout.String(); got != version+"\n" {
		t.Errorf("--version printed %q, want %q", got, version+"\n")
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}
