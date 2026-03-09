package generate

import (
	"fmt"
	"os"

	"github.com/pablogore/go-specs/internal/cli"
	coverageheatmap "github.com/pablogore/go-specs/tools/coverageheatmap"
)

// Register adds generate commands to the root CLI.
func Register(root *cli.Root) {
	root.RegisterGroup("generate", map[string]cli.Runner{
		"tests": runTests,
	})
}

func runTests(args []string) int {
	args = append(args, "-generate-tests")
	if err := coverageheatmap.RunAnalysis(args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cli.ProgramName(), err)
		return 1
	}
	return 0
}
