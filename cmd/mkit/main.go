// Command mkit is the entrypoint only; it stays thin and defers to internal/cli.
package main

import (
	"os"

	"github.com/masterik/mk-toolkit/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		os.Exit(cli.Fail(os.Stderr, err))
	}
}
