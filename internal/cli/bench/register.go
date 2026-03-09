package bench

import (
	"fmt"
	"os"

	"github.com/pablogore/go-specs/internal/cli"
	"github.com/pablogore/go-specs/tools/perfcheck"
)

// Register adds bench commands to the root CLI.
func Register(root *cli.Root) {
	root.RegisterGroup("bench", map[string]cli.Runner{
		"run":    runBenchRun,
		"report": runBenchReport,
	})
}

func runBenchRun(args []string) int {
	fmt.Fprintf(os.Stderr, "%s bench run: run 'go test -bench . -benchmem -json' and save output for comparison\n", cli.ProgramName())
	return 1
}

func runBenchReport(args []string) int {
	// perf check: compare current vs baseline
	if err := perfcheck.RunCheck(args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cli.ProgramName(), err)
		return 1
	}
	return 0
}
