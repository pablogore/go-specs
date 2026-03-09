package testcmd

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/pablogore/go-specs/internal/cli"
	"github.com/pablogore/go-specs/specs"
)

// RunTestRun runs "go test" with optional -shard and -verbose. Packages default to "./..." if not provided.
// By default prints formatted output (✓/✗/⚠ per package and summary). With -verbose streams raw go test output and then summary.
func RunTestRun(args []string) int {
	fs := flag.NewFlagSet("test run", flag.ExitOnError)
	shard := fs.String("shard", "", "run only shard N/M (e.g. 2/10)")
	verbose := fs.Bool("verbose", false, "show raw go test output")
	_ = fs.Parse(args)
	packages := fs.Args()
	if len(packages) == 0 {
		packages = []string{"./..."}
	}
	testArgs := []string{"test", "-v"}
	testArgs = append(testArgs, packages...)
	if *shard != "" {
		if _, _, ok := specs.ParseShardString(*shard); !ok {
			fmt.Fprintf(os.Stderr, "invalid -shard value %q (expected N/M, e.g. 2/10)\n", *shard)
			return 2
		}
		testArgs = append(testArgs, "-args", "-shard", *shard)
	}
	start := time.Now()
	out, exitCode, err := runGoTest(testArgs, *verbose)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cli.ProgramName(), err)
		return 1
	}
	result := parseGoTestOutput(out)
	if *verbose {
		fmt.Println()
	}
	printFormattedOutput(result, elapsed)
	return exitCode
}

// RunTree prints the spec tree.
func RunTree(args []string) int {
	suite := specs.Analyze(func() {})
	if suite == nil || suite.Arena == nil || len(suite.Arena.Nodes) <= 1 {
		fmt.Fprintln(os.Stderr, "no specs found to display")
		return 1
	}
	specs.PrintTreeArena(suite.Arena, suite.RootID, 0, os.Stdout)
	return 0
}

// RunStats prints basic suite statistics.
func RunStats(args []string) int {
	suite := specs.Analyze(func() {})
	if suite == nil || suite.Arena == nil {
		fmt.Fprintln(os.Stderr, "no specs found to analyze")
		return 1
	}
	totalDescribes, totalIts, maxDepth := computeStatsArena(suite.Arena, suite.RootID, 0)
	avg := 0.0
	if totalDescribes > 0 {
		avg = float64(totalIts) / float64(totalDescribes)
	}
	fmt.Println("Suite statistics")
	fmt.Printf("Total specs: %d\n", totalIts)
	fmt.Printf("Total describe blocks: %d\n", totalDescribes)
	fmt.Printf("Max depth: %d\n", maxDepth)
	fmt.Printf("Average specs per describe: %.2f\n", avg)
	return 0
}

// RunGenerate runs test generation (delegates to coverage generate).
func RunGenerate(args []string) int {
	fmt.Fprintf(os.Stderr, "%s test generate: use '%s coverage generate' or '%s generate tests'\n", cli.ProgramName(), cli.ProgramName(), cli.ProgramName())
	return 1
}

// RunMigrate runs test migration (not implemented).
func RunMigrate(args []string) int {
	fmt.Fprintf(os.Stderr, "%s test migrate: not implemented\n", cli.ProgramName())
	return 1
}

func computeStatsArena(arena *specs.NodeArena, id int, depth int) (describes, its, maxDepth int) {
	if arena == nil || id < 0 || id >= len(arena.Nodes) {
		return 0, 0, depth
	}
	node := &arena.Nodes[id]
	if node.Type == specs.DescribeNode || node.Type == specs.WhenNode {
		describes++
	}
	if node.Type == specs.ItNode {
		its++
	}
	if depth+1 > maxDepth {
		maxDepth = depth + 1
	}
	for _, cid := range arena.Children[id] {
		d, i, md := computeStatsArena(arena, cid, depth+1)
		describes += d
		its += i
		if md > maxDepth {
			maxDepth = md
		}
	}
	return describes, its, maxDepth
}
