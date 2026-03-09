package doctor

import (
	"fmt"
	"os"

	"github.com/pablogore/go-specs/internal/cli"
)

// Register adds doctor commands to the root CLI.
func Register(root *cli.Root) {
	root.RegisterGroup("doctor", map[string]cli.Runner{
		"repo": runRepo,
	})
}

func runRepo(args []string) int {
	fmt.Fprintf(os.Stderr, "%s doctor repo: repository health checks (not implemented)\n", cli.ProgramName())
	return 1
}
