package coverage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteReport generates docs/COVERAGE.md with the coverage heatmap, threshold info, and optional coverage gaps.
// Overwrites the file each run. Creates docs/ if needed. If gaps is non-nil, appends a Coverage Gaps section.
func WriteReport(moduleRoot string, packages []PackageCoverage, totalPct, threshold, pkgThreshold float64, gaps []FuncCoverage) error {
	docsDir := filepath.Join(moduleRoot, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return fmt.Errorf("create docs dir: %w", err)
	}
	outPath := filepath.Join(docsDir, "COVERAGE.md")
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer f.Close()

	var b strings.Builder

	// Header
	b.WriteString("# Coverage Report\n\n")
	b.WriteString("Generated automatically by tools/coverageheatmap.\n\n")

	// Coverage Heatmap table
	b.WriteString("## Coverage Heatmap\n\n")
	b.WriteString("| Package | Coverage |\n")
	b.WriteString("|--------|--------|\n")
	for _, p := range packages {
		displayPkg := displayPackageName(p.Package)
		bar := heatbar(p.Pct())
		pct := p.Pct()
		b.WriteString(fmt.Sprintf("| %s | %s %.0f%% |\n", displayPkg, bar, pct))
	}
	b.WriteString("\n")

	// Total Coverage
	b.WriteString("## Total Coverage\n\n")
	b.WriteString(fmt.Sprintf("%.1f%%\n\n", totalPct))

	// Threshold section
	b.WriteString("## Threshold\n\n")
	if threshold > 0 || pkgThreshold > 0 {
		if threshold > 0 {
			b.WriteString(fmt.Sprintf("Total threshold: %.0f%%\n", threshold))
		}
		if pkgThreshold > 0 {
			b.WriteString(fmt.Sprintf("Package threshold: %.0f%%\n", pkgThreshold))
		}
		b.WriteString("\n")

		// Packages below threshold
		var below []PackageCoverage
		if pkgThreshold > 0 {
			for _, p := range packages {
				if p.Pct() < pkgThreshold {
					below = append(below, p)
				}
			}
		}
		if len(below) > 0 {
			b.WriteString("Packages below threshold must be marked:\n\n")
			b.WriteString("| Package | Coverage | Status |\n")
			b.WriteString("|--------|--------|--------|\n")
			for _, p := range below {
				displayPkg := displayPackageName(p.Package)
				b.WriteString(fmt.Sprintf("| %s | %.0f%% | ⚠ below threshold |\n", displayPkg, p.Pct()))
			}
		}
	} else {
		b.WriteString("No thresholds set.\n")
	}

	// Coverage Gaps (functions below 100%)
	if len(gaps) > 0 {
		b.WriteString("\n## Coverage Gaps\n\n")
		b.WriteString("Functions below 100% coverage:\n\n")
		b.WriteString("| Function | Coverage | Suggested tests |\n")
		b.WriteString("|--------|--------|--------|\n")
		for _, g := range gaps {
			sug := strings.Join(g.Suggestions, ", ")
			b.WriteString(fmt.Sprintf("| %s | %.0f%% | %s |\n", g.DisplayName(), g.Pct, sug))
		}
	}

	_, err = f.WriteString(b.String())
	if err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

// displayPackageName shortens package path for display (e.g. .../internal/plan -> internal/plan).
func displayPackageName(pkg string) string {
	if idx := strings.Index(pkg, "internal/"); idx >= 0 {
		return pkg[idx:]
	}
	return pkg
}
