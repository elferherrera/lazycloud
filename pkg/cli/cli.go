// Package cli dispatches lazycloud's non-TUI invocation modes: the
// --version flag today, with subcommands (list, get) planned per
// docs/agent-cli-proposal.md. Dispatch is pure: no printing, no
// os.Exit, so it stays unit-testable.
package cli

import (
	"fmt"
	"io"
)

// Result reports how one invocation was dispatched.
type Result struct {
	// Handled is true when the CLI fully served the invocation (the
	// --version flag) and the caller must not start the TUI.
	Handled bool
	// Output is the stdout text for a handled invocation, printed with
	// a trailing newline by Print.
	Output string
}

// Run decides the behavior for args (args[0] is the program name,
// matching os.Args) and the build version. Only --version at args[1]
// is handled today; every other argument list requests the TUI.
func Run(args []string, version string) Result {
	if len(args) > 1 && args[1] == "--version" {
		return Result{Handled: true, Output: version}
	}
	return Result{}
}

// Print writes a handled result's output to w. Unhandled results and
// empty output write nothing, so callers can call it unconditionally.
func Print(r Result, w io.Writer) {
	if !r.Handled || r.Output == "" {
		return
	}
	fmt.Fprintln(w, r.Output)
}
