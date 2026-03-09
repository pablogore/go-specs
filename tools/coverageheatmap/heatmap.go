package coverage

import (
	"fmt"
)

// heatbar returns a 10-block bar where each block is 10%.
// █ = filled, ░ = empty.
func heatbar(p float64) string {
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	blocks := int(p / 10)
	bar := ""
	for i := 0; i < blocks; i++ {
		bar += "█"
	}
	for i := blocks; i < 10; i++ {
		bar += "░"
	}
	return bar
}

// PrintHeatmap prints the coverage heatmap to stdout.
// packages must be ordered (e.g. by package path); totalPct is overall coverage over included packages only.
// excluded lists package paths that were excluded from the heatmap (e.g. by -exclude); may be nil.
func PrintHeatmap(packages []PackageCoverage, totalPct float64, excluded []string) {
	const title = "Coverage Heatmap"
	fmt.Println(title)
	fmt.Println()

	// Align package names and bars by finding max package path length
	maxLen := 0
	for _, p := range packages {
		if n := len(p.Package); n > maxLen {
			maxLen = n
		}
	}
	if maxLen < 8 {
		maxLen = 8
	}

	for _, p := range packages {
		bar := heatbar(p.Pct())
		pct := p.Pct()
		line := fmt.Sprintf("%-*s  %s %5.1f%%", maxLen, p.Package, bar, pct)
		fmt.Println(line)
	}

	if len(excluded) > 0 {
		fmt.Println()
		fmt.Println("Excluded:")
		for _, pkg := range excluded {
			fmt.Println(pkg)
		}
	}

	fmt.Println()
	fmt.Printf("Total Coverage: %.1f%%\n", totalPct)
}
