package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/pablogore/go-specs/specs"
)

func main() {
	var shard string
	flag.StringVar(&shard, "shard", "", "run only shard N/M (e.g. 2/10); used by 'run' command")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [-shard N/M] <command> [args...]\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "commands:")
		fmt.Fprintln(flag.CommandLine.Output(), "  run    run tests (passes -shard to test binary when set)")
		fmt.Fprintln(flag.CommandLine.Output(), "  tree   print the spec tree")
		fmt.Fprintln(flag.CommandLine.Output(), "  stats  print basic suite statistics")
		fmt.Fprintln(flag.CommandLine.Output(), "  graph  emit graph representation")
	}
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	switch args[0] {
	case "run":
		runCmd(shard, args[1:])
	case "tree":
		runTree()
	case "stats":
		runStats()
	case "graph":
		runGraph()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		flag.Usage()
		os.Exit(1)
	}
}

// runCmd runs "go test" with optional -shard. If shard is "2/10", the test binary receives -args -shard 2/10
// so specs.ParseShardFlag(os.Args) works. Packages default to "./..." if not provided.
func runCmd(shard string, packages []string) {
	testArgs := []string{"test", "-v"}
	if len(packages) == 0 {
		packages = []string{"./..."}
	}
	testArgs = append(testArgs, packages...)
	if shard != "" {
		if _, _, ok := specs.ParseShardString(shard); !ok {
			fmt.Fprintf(os.Stderr, "invalid -shard value %q (expected N/M, e.g. 2/10)\n", shard)
			os.Exit(1)
		}
		testArgs = append(testArgs, "-args", "-shard", shard)
	}
	cmd := exec.Command("go", testArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		os.Exit(1)
	}
}

func runTree() {
	suite := specs.Analyze(func() {})
	if suite == nil || suite.Arena == nil || len(suite.Arena.Nodes) <= 1 {
		fmt.Fprintln(os.Stderr, "no specs found to display")
		os.Exit(1)
	}
	// RootID 0 is suite; print from 0 so suite name is first, then children
	specs.PrintTreeArena(suite.Arena, suite.RootID, 0, os.Stdout)
}

func runStats() {
	suite := specs.Analyze(func() {})
	if suite == nil || suite.Arena == nil {
		fmt.Fprintln(os.Stderr, "no specs found to analyze")
		os.Exit(1)
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
}

func runGraph() {
	fmt.Println("graph command not yet implemented")
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
