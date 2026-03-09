package migrate

import (
	"fmt"
	"os"

	"github.com/pablogore/go-specs/internal/cli"
)

// Register adds migrate commands to the root CLI.
func Register(root *cli.Root) {
	root.RegisterGroup("migrate", map[string]cli.Runner{
		"tests": runTests,
	})
}

func runTests(args []string) int {
	fmt.Fprintf(os.Stderr, "%s migrate tests: not implemented\n", cli.ProgramName())
	return 1
}
