package main

import (
	"fmt"
	"io"
	"os"

	"github.com/elferherrera/lazycloud/pkg/app"
	"github.com/elferherrera/lazycloud/pkg/cli"
)

// version is overridden at release time with -ldflags "-X main.version=vX.Y.Z";
// source builds keep the dev default.
var version = "dev"

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr, version, app.Run))
}

// run is the testable core of main: it dispatches non-TUI invocations
// and starts the TUI through startTUI, returning the process exit code.
// main stays a three-line adapter over it.
func run(args []string, stdout, stderr io.Writer, version string, startTUI func() error) int {
	result := cli.Run(args, version)
	if result.Handled {
		cli.Print(result, stdout)
		return 0
	}
	if code := cli.Execute(args, stdout, stderr); code != cli.NotHandled {
		return code
	}
	if err := startTUI(); err != nil {
		fmt.Fprintln(stderr, "error running application:", err)
		return 1
	}
	return 0
}
