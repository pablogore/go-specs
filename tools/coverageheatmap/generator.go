package coverage

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GenerateTestsResult holds the paths of generated and skipped tests.
type GenerateTestsResult struct {
	Generated []string // files written
	Skipped   []string // "path (reason)"
}

// UsePathCoverage returns true when the function should get path-coverage (ctx.Step) tests.
func UsePathCoverage(f FuncCoverage) bool {
	if f.Pct >= 80 {
		return false
	}
	name := strings.ToLower(f.Func)
	// Parser/validator patterns
	for _, p := range []string{"parse", "validate", "match", "extract", "decode", "encode", "version"} {
		if strings.Contains(name, p) {
			return true
		}
	}
	// Command/IO patterns
	for _, p := range []string{"run", "execute", "create", "delete", "load", "fetch", "evaluate", "policy", "rule"} {
		if strings.Contains(name, p) {
			return true
		}
	}
	// Compile/build/count/collect
	for _, p := range []string{"compile", "build", "count", "collect"} {
		if strings.Contains(name, p) {
			return true
		}
	}
	// Low coverage suggests branches
	if f.Pct < 50 {
		return true
	}
	return false
}

// IsParserOrValidator returns true for Parse/Validate/Match/Extract/Decode/Encode-style names.
func IsParserOrValidator(name string) bool {
	n := strings.ToLower(name)
	for _, p := range []string{"parse", "validate", "match", "extract", "decode", "encode"} {
		if strings.HasPrefix(n, p) || strings.Contains(n, p) {
			return true
		}
	}
	return false
}

// IsCommandOrIO returns true for Run/Execute/Create/Delete/Load/Fetch-style names.
func IsCommandOrIO(name string) bool {
	n := strings.ToLower(name)
	for _, p := range []string{"run", "execute", "create", "delete", "load", "fetch"} {
		if strings.HasPrefix(n, p) || strings.Contains(n, p) {
			return true
		}
	}
	return false
}

// GenerateTests generates go-specs test files for gap functions. Respects exclude tokens and skips packages that already have Describe for the function.
func GenerateTests(moduleRoot string, gaps []FuncCoverage, exclude []string) (GenerateTestsResult, error) {
	modulePath, err := readModulePath(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		return GenerateTestsResult{}, err
	}
	var result GenerateTestsResult
	// Group gaps by package
	byPkg := make(map[string][]FuncCoverage)
	for _, g := range gaps {
		if excluded(g.File, g.Package, exclude) {
			result.Skipped = append(result.Skipped, fmt.Sprintf("%s (excluded)", relPath(moduleRoot, g.File)))
			continue
		}
		byPkg[g.Package] = append(byPkg[g.Package], g)
	}
	// For each package, resolve dir and filter already-tested
	for pkg, funcs := range byPkg {
		pkgDir := pkgToDir(moduleRoot, modulePath, pkg)
		if pkgDir == "" {
			continue
		}
		pkgName := filepath.Base(pkg)
		if pkgName == "" {
			pkgName = "pkg"
		}
		coverageFileName := pkgName + "_coverage_test.go"
		outPath := filepath.Join(pkgDir, coverageFileName)
		legacyPath := filepath.Join(pkgDir, "auto_generated_test.go")
		// Migrate legacy file to new name if present and target does not exist
		if stat, _ := os.Stat(legacyPath); stat != nil {
			if _, err := os.Stat(outPath); os.IsNotExist(err) {
				if err := os.Rename(legacyPath, outPath); err != nil {
					return result, fmt.Errorf("rename %s to %s: %w", legacyPath, outPath, err)
				}
			}
		}
		tested := scanDescribeInPackage(pkgDir, funcs)
		var toGen []FuncCoverage
		pkgRel := relPath(moduleRoot, pkgDir)
		for _, f := range funcs {
			if tested[f.Func] {
				result.Skipped = append(result.Skipped, fmt.Sprintf("%s (%s already tested)", pkgRel, f.Func))
				continue
			}
			toGen = append(toGen, f)
		}
		if len(toGen) == 0 {
			continue
		}
		existing, _ := os.ReadFile(outPath)
		existingStr := string(existing)
		if len(existingStr) > 0 {
			// Append new Describe blocks to existing file
			var newBlocks strings.Builder
			for _, f := range toGen {
				newBlocks.WriteString(buildTestFunc(pkg, pkgName, f))
				newBlocks.WriteString("\n")
			}
			content := strings.TrimRight(existingStr, "\n") + "\n\n" + strings.TrimRight(newBlocks.String(), "\n") + "\n"
			if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
				return result, fmt.Errorf("write %s: %w", outPath, err)
			}
		} else {
			content := buildTestFile(pkg, pkgName, toGen)
			if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
				return result, fmt.Errorf("write %s: %w", outPath, err)
			}
		}
		result.Generated = append(result.Generated, relPath(moduleRoot, outPath))
	}
	sort.Strings(result.Generated)
	sort.Strings(result.Skipped)
	return result, nil
}

func excluded(filePath, pkgPath string, exclude []string) bool {
	lower := strings.ToLower(filePath + " " + pkgPath)
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

func relPath(moduleRoot, absPath string) string {
	rel, err := filepath.Rel(moduleRoot, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

func readModulePath(goModPath string) (string, error) {
	f, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), sc.Err()
		}
	}
	return "", fmt.Errorf("no module directive in %s", goModPath)
}

func pkgToDir(moduleRoot, modulePath, pkg string) string {
	rel := strings.TrimPrefix(pkg, modulePath)
	rel = strings.TrimPrefix(rel, "/")
	if rel == pkg {
		return ""
	}
	return filepath.Join(moduleRoot, filepath.FromSlash(rel))
}

func scanDescribeInPackage(pkgDir string, funcs []FuncCoverage) map[string]bool {
	tested := make(map[string]bool)
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return tested
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(pkgDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		for _, f := range funcs {
			// Describe(t, "FunctionName") or Describe(t, "FuncName")
			if strings.Contains(content, `Describe(t, "`+f.Func+`")`) || strings.Contains(content, "Describe(t, `"+f.Func+"`)") {
				tested[f.Func] = true
			}
		}
	}
	return tested
}

func buildTestFile(pkg, pkgName string, funcs []FuncCoverage) string {
	var b strings.Builder
	b.WriteString("// Code generated by specs coverage generator.\n")
	b.WriteString("// Coverage-focused tests.\n")
	b.WriteString("// Safe to edit.\n\n")
	b.WriteString("package " + pkgName + "_test\n\n")
	b.WriteString("import (\n\t\"testing\"\n\n\t\"github.com/pablogore/go-specs/specs\"\n)\n\n")
	for _, f := range funcs {
		b.WriteString(buildTestFunc(pkg, pkgName, f))
		b.WriteString("\n")
	}
	return b.String()
}

func buildTestFunc(pkg, pkgName string, f FuncCoverage) string {
	testName := sanitizeTestName(f.Func)
	if UsePathCoverage(f) {
		return buildPathCoverageTest(pkg, pkgName, f, testName)
	}
	return buildSimpleTest(pkg, pkgName, f, testName)
}

func sanitizeTestName(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	if s == "" {
		return "Func"
	}
	// Test name must start with uppercase (e.g. TestRun_..., not Testrun_...)
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] = b[0] - ('a' - 'A')
	}
	return string(b)
}

func buildPathCoverageTest(pkg, pkgName string, f FuncCoverage, testName string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("func Test%s_AutoCoverage(t *testing.T) {\n", testName))
	b.WriteString(fmt.Sprintf("\tspecs.Describe(t, %q, func(s *specs.Spec) {\n", f.Func))
	b.WriteString("\t\ts.It(\"covers execution paths\", func(ctx *specs.Context) {\n")
	if IsParserOrValidator(f.Func) {
		b.WriteString("\t\t\tcases := []struct {\n\t\t\t\tname string\n\t\t\t\tinput string\n\t\t\t\texpectError bool\n\t\t\t}{\n")
		b.WriteString("\t\t\t\t{\"valid input\", \"\", false},\n")
		b.WriteString("\t\t\t\t{\"invalid input\", \"invalid\", true},\n")
		b.WriteString("\t\t\t\t{\"empty input\", \"\", true},\n")
		b.WriteString("\t\t\t}\n")
		b.WriteString("\t\t\tfor _, tc := range cases {\n")
		b.WriteString("\t\t\t\tctx.Step(tc.name, func() {\n")
		b.WriteString("\t\t\t\t\tvar err error\n")
		b.WriteString("\t\t\t\t\t// TODO: call " + f.Func + " with tc.input and set err\n")
		b.WriteString("\t\t\t\t\tif tc.expectError {\n")
		b.WriteString("\t\t\t\t\t\tctx.Expect(err != nil).To(specs.BeTrue())\n\t\t\t\t\t\treturn\n\t\t\t\t\t}\n")
		b.WriteString("\t\t\t\t\tctx.Expect(err).To(specs.BeNil())\n")
		b.WriteString("\t\t\t\t})\n\t\t\t}\n")
	} else if IsCommandOrIO(f.Func) {
		b.WriteString("\t\t\tcases := []struct {\n\t\t\t\tname string\n\t\t\t\tsimulateError bool\n\t\t\t}{\n")
		b.WriteString("\t\t\t\t{\"success\", false},\n")
		b.WriteString("\t\t\t\t{\"failure\", true},\n")
		b.WriteString("\t\t\t}\n")
		b.WriteString("\t\t\tfor _, tc := range cases {\n")
		b.WriteString("\t\t\t\tctx.Step(tc.name, func() {\n")
		b.WriteString("\t\t\t\t\tvar err error\n")
		b.WriteString("\t\t\t\t\t// TODO: setup dependencies; call " + f.Func + "; set err when tc.simulateError\n")
		b.WriteString("\t\t\t\t\tif tc.simulateError {\n")
		b.WriteString("\t\t\t\t\t\tctx.Expect(err != nil).To(specs.BeTrue())\n\t\t\t\t\t\treturn\n\t\t\t\t\t}\n")
		b.WriteString("\t\t\t\t\tctx.Expect(err).To(specs.BeNil())\n")
		b.WriteString("\t\t\t\t})\n\t\t\t}\n")
	} else {
		b.WriteString("\t\t\tcases := []struct {\n\t\t\t\tname string\n\t\t\t\texpectError bool\n\t\t\t}{\n")
		b.WriteString("\t\t\t\t{\"happy path\", false},\n")
		b.WriteString("\t\t\t\t{\"error path\", true},\n")
		b.WriteString("\t\t\t\t{\"edge case\", false},\n")
		b.WriteString("\t\t\t}\n")
		b.WriteString("\t\t\tfor _, tc := range cases {\n")
		b.WriteString("\t\t\t\tctx.Step(tc.name, func() {\n")
		b.WriteString("\t\t\t\t\tvar err error\n")
		b.WriteString("\t\t\t\t\t// TODO: setup dependencies and call " + f.Func + "\n")
		b.WriteString("\t\t\t\t\tif tc.expectError {\n")
		b.WriteString("\t\t\t\t\t\tctx.Expect(err != nil).To(specs.BeTrue())\n\t\t\t\t\t\treturn\n")
		b.WriteString("\t\t\t\t\t}\n")
		b.WriteString("\t\t\t\t\tctx.Expect(err).To(specs.BeNil())\n")
		b.WriteString("\t\t\t\t})\n\t\t\t}\n")
	}
	b.WriteString("\t\t})\n\t})\n}\n")
	return b.String()
}

func buildSimpleTest(pkg, pkgName string, f FuncCoverage, testName string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("func Test%s_AutoCoverage(t *testing.T) {\n", testName))
	b.WriteString(fmt.Sprintf("\tspecs.Describe(t, %q, func(s *specs.Spec) {\n", f.Func))
	b.WriteString("\t\ts.It(\"returns expected value\", func(ctx *specs.Context) {\n")
	b.WriteString("\t\t\tvar result, expected interface{}\n")
	b.WriteString("\t\t\t// TODO: call " + f.Func + " and set result and expected\n")
	b.WriteString("\t\t\tctx.Expect(result).ToEqual(expected)\n")
	b.WriteString("\t\t})\n\t})\n}\n")
	return b.String()
}
