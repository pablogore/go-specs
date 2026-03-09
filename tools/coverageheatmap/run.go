package coverage

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultCoverProfile = "coverage.out"

// built-in package path segments to skip when auto-detecting (case-insensitive).
var defaultSkipSegments = []string{"vendor", "testdata", "generated", "mocks", "testkit"}

func findModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			wd, _ := os.Getwd()
			return wd
		}
		dir = parent
	}
}

// detectPackages runs go list ./... and returns package paths, excluding vendor/testdata/generated/mocks/testkit
// and any path matching excludeTokens. Returns nil, nil on error.
func detectPackages(moduleRoot string, excludeTokens []string) ([]string, error) {
	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = moduleRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list ./...: %w", err)
	}
	var list []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pkg := strings.TrimSpace(line)
		if pkg == "" {
			continue
		}
		lower := strings.ToLower(pkg)
		skip := false
		for _, seg := range defaultSkipSegments {
			if strings.Contains(lower, seg) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if packageExcluded(pkg, excludeTokens) {
			continue
		}
		list = append(list, pkg)
	}
	return list, nil
}

// resolveTestPaths returns test paths: if pathFlag is non-empty, returns [pathFlag]; otherwise
// auto-detects via go list ./... and filtering. Returns (nil, nil) when no packages found (caller should print message).
func resolveTestPaths(moduleRoot, pathFlag string, excludeTokens []string) ([]string, error) {
	if pathFlag != "" {
		return []string{pathFlag}, nil
	}
	packages, err := detectPackages(moduleRoot, excludeTokens)
	if err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		return nil, nil
	}
	return packages, nil
}

func runTestsWithCoverage(moduleRoot string, testPaths []string) error {
	if len(testPaths) == 0 {
		return fmt.Errorf("no packages to test")
	}
	args := append([]string{"test", "-coverprofile=" + defaultCoverProfile}, testPaths...)
	cmd := exec.Command("go", args...)
	cmd.Dir = moduleRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go test: %w", err)
	}
	return nil
}

// runTestsAndParse runs go test with coverage and parses the profile. Returns packages, totalPct, gaps, and error.
func runTestsAndParse(moduleRoot string, testPaths []string) (packages []PackageCoverage, totalPct float64, gaps []FuncCoverage, err error) {
	coverPath := filepath.Join(moduleRoot, defaultCoverProfile)
	if err = runTestsWithCoverage(moduleRoot, testPaths); err != nil {
		return nil, 0, nil, err
	}
	packages, totalPct, err = ParseCoverage(coverPath)
	if err != nil {
		return nil, 0, nil, err
	}
	if out, e := RunCoverFunc(coverPath); e == nil {
		if funcs, e := ParseFuncCoverage(out); e == nil {
			gaps = GapsOnly(funcs)
		}
	}
	return packages, totalPct, gaps, nil
}

// RunHeatmap runs tests, parses coverage, and prints the terminal heatmap. Args may contain -path=..., -threshold=..., -package-threshold=..., -exclude=...
func RunHeatmap(args []string) error {
	fs := flag.NewFlagSet("heatmap", flag.ExitOnError)
	threshold := fs.Float64("threshold", 0, "fail if total coverage < threshold (0 = disabled)")
	pkgThreshold := fs.Float64("package-threshold", 0, "fail if any package coverage < threshold (0 = disabled)")
	testPath := fs.String("path", "", "optional go test path (default: auto-detect packages)")
	excludeFlag := fs.String("exclude", "", "comma-separated tokens: skip packages containing any (e.g. mocks,testkit,generated)")
	_ = fs.Parse(args)

	moduleRoot := findModuleRoot()
	excludeTokens := splitExclude(*excludeFlag)
	testPaths, err := resolveTestPaths(moduleRoot, *testPath, excludeTokens)
	if err != nil {
		return err
	}
	if len(testPaths) == 0 {
		fmt.Fprintln(os.Stderr, "No Go packages found in repository.")
		return nil
	}
	packages, _, _, err := runTestsAndParse(moduleRoot, testPaths)
	if err != nil {
		return err
	}
	included, excludedPkgs, totalPct := filterPackagesByExclude(packages, excludeTokens)
	PrintHeatmap(included, totalPct, excludedPkgs)
	return checkThresholds(included, totalPct, *threshold, *pkgThreshold)
}

// RunReport runs tests, parses coverage, and writes docs/COVERAGE.md. Args may contain -path=..., -threshold=..., -package-threshold=....
func RunReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	threshold := fs.Float64("threshold", 0, "fail if total coverage < threshold")
	pkgThreshold := fs.Float64("package-threshold", 0, "fail if any package coverage < threshold")
	testPath := fs.String("path", "", "optional go test path (default: auto-detect packages)")
	_ = fs.Parse(args)

	moduleRoot := findModuleRoot()
	testPaths, err := resolveTestPaths(moduleRoot, *testPath, nil)
	if err != nil {
		return err
	}
	if len(testPaths) == 0 {
		fmt.Fprintln(os.Stderr, "No Go packages found in repository.")
		return nil
	}
	packages, totalPct, gaps, err := runTestsAndParse(moduleRoot, testPaths)
	if err != nil {
		return err
	}
	if err := WriteReport(moduleRoot, packages, totalPct, *threshold, *pkgThreshold, gaps); err != nil {
		return err
	}
	return checkThresholds(packages, totalPct, *threshold, *pkgThreshold)
}

// RunBadge runs tests, parses coverage, and writes docs/coverage.svg. Args may contain -path=....
func RunBadge(args []string) error {
	fs := flag.NewFlagSet("badge", flag.ExitOnError)
	testPath := fs.String("path", "", "optional go test path (default: auto-detect packages)")
	_ = fs.Parse(args)

	moduleRoot := findModuleRoot()
	testPaths, err := resolveTestPaths(moduleRoot, *testPath, nil)
	if err != nil {
		return err
	}
	if len(testPaths) == 0 {
		fmt.Fprintln(os.Stderr, "No Go packages found in repository.")
		return nil
	}
	packages, totalPct, _, err := runTestsAndParse(moduleRoot, testPaths)
	if err != nil {
		return err
	}
	_ = packages
	if err := GenerateBadge(moduleRoot, totalPct); err != nil {
		return err
	}
	fmt.Println("Badge generated: docs/coverage.svg")
	return nil
}

// RunAnalysis runs tests, parses coverage, and prints coverage gaps and optional test suggestions.
// Args may contain -path=..., -suggest-tests, -generate-tests, -exclude=...
func RunAnalysis(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	testPath := fs.String("path", "", "optional go test path (default: auto-detect packages)")
	suggestTests := fs.Bool("suggest-tests", false, "print detailed test suggestions")
	generateTests := fs.Bool("generate-tests", false, "generate go-specs test files for uncovered code")
	excludeFlag := fs.String("exclude", "", "comma-separated tokens: skip file paths containing any (e.g. mocks,testkit,generated)")
	_ = fs.Parse(args)

	moduleRoot := findModuleRoot()
	excludeTokens := splitExclude(*excludeFlag)
	testPaths, err := resolveTestPaths(moduleRoot, *testPath, excludeTokens)
	if err != nil {
		return err
	}
	if len(testPaths) == 0 {
		fmt.Fprintln(os.Stderr, "No Go packages found in repository.")
		return nil
	}
	_, _, gaps, err := runTestsAndParse(moduleRoot, testPaths)
	if err != nil {
		return err
	}
	if len(gaps) > 0 {
		fmt.Println()
		fmt.Println("Coverage Gaps")
		for _, g := range gaps {
			fmt.Printf("%s\t%.0f%%\n", g.DisplayName(), g.Pct)
		}
		fmt.Println()
		if *suggestTests {
			for _, g := range gaps {
				fmt.Printf("Missing test cases for %s:\n", g.Func)
				for _, s := range g.Suggestions {
					fmt.Printf("  - %s\n", s)
				}
			}
			fmt.Println()
		}
		if *generateTests {
			exclude := splitExclude(*excludeFlag)
			genResult, genErr := GenerateTests(moduleRoot, gaps, exclude)
			if genErr != nil {
				return genErr
			}
			fmt.Println("Generated tests:")
			for _, p := range genResult.Generated {
				fmt.Println("  " + p)
			}
			if len(genResult.Skipped) > 0 {
				fmt.Println("Skipped:")
				for _, p := range genResult.Skipped {
					fmt.Println("  " + p)
				}
			}
		}
		fmt.Println("Test suggestions generated.")
	}
	return nil
}

func splitExclude(v string) []string {
	if v == "" {
		return nil
	}
	var out []string
	for _, s := range strings.Split(v, ",") {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// packageExcluded returns true if pkgPath contains any of the exclude tokens (case-insensitive).
func packageExcluded(pkgPath string, exclude []string) bool {
	lower := strings.ToLower(pkgPath)
	for _, tok := range exclude {
		if tok == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(strings.TrimSpace(tok))) {
			return true
		}
	}
	return false
}

// filterPackagesByExclude returns included packages, excluded package paths, and total coverage % over included only.
func filterPackagesByExclude(packages []PackageCoverage, exclude []string) (included []PackageCoverage, excludedPkgs []string, totalPct float64) {
	if len(exclude) == 0 {
		var totalStmts, coveredStmts int64
		for _, p := range packages {
			totalStmts += p.Total
			coveredStmts += p.Covered
		}
		if totalStmts > 0 {
			totalPct = 100 * float64(coveredStmts) / float64(totalStmts)
		}
		return packages, nil, totalPct
	}
	var totalStmts, coveredStmts int64
	for _, p := range packages {
		if packageExcluded(p.Package, exclude) {
			excludedPkgs = append(excludedPkgs, p.Package)
			continue
		}
		included = append(included, p)
		totalStmts += p.Total
		coveredStmts += p.Covered
	}
	if totalStmts > 0 {
		totalPct = 100 * float64(coveredStmts) / float64(totalStmts)
	}
	return included, excludedPkgs, totalPct
}

func checkThresholds(packages []PackageCoverage, totalPct, threshold, pkgThreshold float64) error {
	if pkgThreshold > 0 {
		for _, p := range packages {
			if p.Pct() < pkgThreshold {
				fmt.Fprintf(os.Stderr, "FAIL: package %s below threshold (%.0f%%)\n", p.Package, pkgThreshold)
			}
		}
	}
	if threshold > 0 && totalPct < threshold {
		fmt.Fprintf(os.Stderr, "FAIL: total coverage %.1f%% below threshold (%.0f%%)\n", totalPct, threshold)
	}
	var err error
	if pkgThreshold > 0 {
		for _, p := range packages {
			if p.Pct() < pkgThreshold {
				fmt.Fprintf(os.Stderr, "FAIL: package %s below threshold (%.0f%%)\n", p.Package, pkgThreshold)
				if err == nil {
					err = fmt.Errorf("package %s below threshold (%.0f%%)", p.Package, pkgThreshold)
				}
			}
		}
	}
	if threshold > 0 && totalPct < threshold {
		fmt.Fprintf(os.Stderr, "FAIL: total coverage %.1f%% below threshold (%.0f%%)\n", totalPct, threshold)
		err = fmt.Errorf("total coverage %.1f%% below threshold (%.0f%%)", totalPct, threshold)
	}
	return err
}
