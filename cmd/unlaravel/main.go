// Command unlaravel is the CLI entrypoint for the Un(la)ravel Laravel
// analysis tool. It delegates all behavior to the internal/cli package and
// translates a failed command into a non-zero process exit code.
package main

import (
	"os"

	"github.com/mawsis/unlaravel/internal/cli"
)

func main() {
	// cli.Execute runs the Cobra command tree. Cobra already prints the
	// error to stderr, so here we only need to signal failure to the shell
	// via the process exit code.
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
