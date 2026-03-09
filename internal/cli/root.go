package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Runner runs a subcommand with the given args. Returns exit code (0 = success).
type Runner func(args []string) int

// Root is the root command dispatcher (kubectl-style).
type Root struct {
	// groups: "coverage" -> "heatmap" -> runner
	groups map[string]map[string]Runner
	// topLevel: "run" -> runner (for specs-ci run, specs-ci doctor, etc.)
	topLevel map[string]Runner
}

// NewRoot returns a new root CLI.
func NewRoot() *Root {
	return &Root{
		groups:   make(map[string]map[string]Runner),
		topLevel: make(map[string]Runner),
	}
}

// RegisterGroup registers a command group (e.g. "coverage") with subcommands.
func (r *Root) RegisterGroup(name string, subcommands map[string]Runner) {
	r.groups[name] = subcommands
}

// RegisterTopLevel registers a top-level command (e.g. "run", "doctor").
func (r *Root) RegisterTopLevel(name string, fn Runner) {
	r.topLevel[name] = fn
}

// ProgramName returns the executable name from os.Args[0] for help and errors.
// Use so both "specs" and "specs-ci" show the correct branding.
func ProgramName() string {
	if len(os.Args) > 0 {
		return filepath.Base(os.Args[0])
	}
	return "specs-ci"
}

// Name returns the executable name (same as ProgramName). Enables both "specs" and "specs-ci".
func (r *Root) Name() string {
	return ProgramName()
}

// Execute parses os.Args and runs the appropriate command. Returns exit code.
func (r *Root) Execute() int {
	args := os.Args[1:]
	if len(args) == 0 {
		r.PrintHelp()
		return 1
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "help", "-h", "--help":
		r.PrintHelp()
		return 0
	}
	if fn, ok := r.topLevel[cmd]; ok {
		return fn(rest)
	}
	if sub, ok := r.groups[cmd]; ok {
		if len(rest) < 1 {
			fmt.Fprintf(os.Stderr, "%s %s requires a subcommand\n", r.Name(), cmd)
			r.printGroupHelp(cmd, sub)
			return 2
		}
		subcmd := rest[0]
		subArgs := rest[1:]
		if fn, ok := sub[subcmd]; ok {
			return fn(subArgs)
		}
		fmt.Fprintf(os.Stderr, "%s %s: unknown subcommand %q\n", r.Name(), cmd, subcmd)
		r.printGroupHelp(cmd, sub)
		return 2
	}
	fmt.Fprintf(os.Stderr, "%s: unknown command %q\n", r.Name(), cmd)
	r.PrintHelp()
	return 2
}

func (r *Root) printGroupHelp(group string, sub map[string]Runner) {
	fmt.Fprintf(os.Stderr, "  %s %s <subcommand> [args...]\n", r.Name(), group)
	fmt.Fprintf(os.Stderr, "  subcommands: %s\n", strings.Join(sortedKeys(sub), ", "))
}

func sortedKeys(m map[string]Runner) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// sort for stable help
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// PrintHelp prints the main help message. Uses executable name from os.Args[0] so
// both "specs" and "specs-ci" show the correct branding.
func (r *Root) PrintHelp() {
	name := r.Name()
	fmt.Printf("%s – tools for go-specs projects\n", name)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s run              run tests (same as test run)\n", name)
	fmt.Printf("  %s test run\n", name)
	fmt.Printf("  %s test tree\n", name)
	fmt.Printf("  %s test stats\n", name)
	fmt.Printf("  %s coverage analyze\n", name)
	fmt.Printf("  %s coverage heatmap\n", name)
	fmt.Printf("  %s coverage report\n", name)
	fmt.Printf("  %s coverage badge\n", name)
	fmt.Printf("  %s coverage diff\n", name)
	fmt.Printf("  %s coverage generate\n", name)
	fmt.Printf("  %s coverage enforce\n", name)
	fmt.Printf("  %s bench run\n", name)
	fmt.Printf("  %s bench report\n", name)
	fmt.Printf("  %s doctor repo\n", name)
	fmt.Printf("  %s generate tests\n", name)
	fmt.Printf("  %s migrate tests\n", name)
}
