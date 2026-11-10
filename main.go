package main

import (
	"fmt"
	"os"

	"github.com/elferherrera/lazycloud/pkg/app"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running application:", err)
		os.Exit(1)
	}
}