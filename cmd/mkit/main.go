// Command mkit is the entrypoint only; it stays thin and defers to internal/cli.
package main

import (
	"fmt"
	"os"

	"github.com/masterik/mk-toolkit/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "mkit:", err)
		os.Exit(1)
	}
}
