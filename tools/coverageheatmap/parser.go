package coverage

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// PackageCoverage holds total and covered statement counts for a package.
type PackageCoverage struct {
	Package string
	Total   int64
	Covered int64
}

// Pct returns coverage percentage (0-100). Returns 0 if Total is 0.
func (p PackageCoverage) Pct() float64 {
	if p.Total == 0 {
		return 0
	}
	return 100 * float64(p.Covered) / float64(p.Total)
}

// ParseCoverage reads a Go coverage profile and returns coverage per package.
// Profile format: first line "mode: set|count|atomic", then lines
// "file:startLine.startCol,endLine.endCol numStmts count".
func ParseCoverage(path string) ([]PackageCoverage, float64, error) {
	// Aggregate by package: total statements and covered statements (count > 0)
	type agg struct {
		total   int64
		covered int64
	}
	byPkg := make(map[string]*agg)

	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if first {
			first = false
			if strings.HasPrefix(line, "mode:") {
				continue
			}
		}
		// Format: "file:start.end,end.end numStmts count"
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}
		fileRange := parts[0]
		numStmts, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		count, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			continue
		}
		// Filename is the part before the last colon (handles Windows paths).
		idx := strings.LastIndex(fileRange, ":")
		if idx < 0 {
			continue
		}
		filePath := fileRange[:idx]
		pkg := filepath.Dir(filePath)
		if byPkg[pkg] == nil {
			byPkg[pkg] = &agg{}
		}
		byPkg[pkg].total += numStmts
		if count > 0 {
			byPkg[pkg].covered += numStmts
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("reading coverage: %w", err)
	}

	// Build ordered slice and total coverage
	var list []PackageCoverage
	var totalStmts, coveredStmts int64
	for pkg, a := range byPkg {
		list = append(list, PackageCoverage{Package: pkg, Total: a.total, Covered: a.covered})
		totalStmts += a.total
		coveredStmts += a.covered
	}
	sortPackages(list)
	totalPct := 0.0
	if totalStmts > 0 {
		totalPct = 100 * float64(coveredStmts) / float64(totalStmts)
	}
	return list, totalPct, nil
}

// sortPackages orders by package path for stable output.
func sortPackages(list []PackageCoverage) {
	// simple insertion sort by package name
	for i := 1; i < len(list); i++ {
		p := list[i]
		j := i
		for j > 0 && list[j-1].Package > p.Package {
			list[j] = list[j-1]
			j--
		}
		list[j] = p
	}
}
