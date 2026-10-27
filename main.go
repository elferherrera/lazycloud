package main

import (
	"fmt"
	"os"

	"github.com/elferherrera/lazycloud/pkg/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running application:", err)
		os.Exit(1)
	}
}