package coverage

import (
	"fmt"
	"os"

	"github.com/pablogore/go-specs/internal/cli"
	coverageheatmap "github.com/pablogore/go-specs/tools/coverageheatmap"
)

// Register adds coverage commands to the root CLI.
func Register(root *cli.Root) {
	root.RegisterGroup("coverage", map[string]cli.Runner{
		"analyze":  runAnalyze,
		"heatmap":  runHeatmap,
		"report":   runReport,
		"badge":    runBadge,
		"diff":     runDiff,
		"generate": runGenerate,
		"enforce":  runEnforce,
	})
}

func runAnalyze(args []string) int {
	if err := coverageheatmap.RunAnalysis(args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cli.ProgramName(), err)
		return 1
	}
	return 0
}

func runHeatmap(args []string) int {
	if err := coverageheatmap.RunHeatmap(args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cli.ProgramName(), err)
		return 1
	}
	return 0
}

func runReport(args []string) int {
	if err := coverageheatmap.RunReport(args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cli.ProgramName(), err)
		return 1
	}
	return 0
}

func runBadge(args []string) int {
	if err := coverageheatmap.RunBadge(args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cli.ProgramName(), err)
		return 1
	}
	return 0
}

func runDiff(args []string) int {
	fmt.Fprintf(os.Stderr, "%s coverage diff: not implemented\n", cli.ProgramName())
	return 1
}

func runGenerate(args []string) int {
	// coverage generate = analyze with -generate-tests
	args = append(args, "-generate-tests")
	if err := coverageheatmap.RunAnalysis(args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cli.ProgramName(), err)
		return 1
	}
	return 0
}

func runEnforce(args []string) int {
	// coverage enforce = report with thresholds (exit non-zero if below)
	if err := coverageheatmap.RunReport(args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cli.ProgramName(), err)
		return 1
	}
	return 0
}
