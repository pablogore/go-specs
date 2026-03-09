package coverage

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// FuncCoverage holds per-function coverage and optional suggestions.
type FuncCoverage struct {
	Package     string   // import path package (e.g. .../internal/plan)
	Func        string   // function name
	Pct         float64  // coverage 0–100
	File        string   // source file path
	Line        int      // line number
	Suggestions []string // heuristic test suggestions
}

// Category returns A–D: A=100%, B=50–99%, C=<50%, D=0%.
func (f FuncCoverage) Category() string {
	switch {
	case f.Pct >= 100:
		return "A"
	case f.Pct >= 50:
		return "B"
	case f.Pct > 0:
		return "C"
	default:
		return "D"
	}
}

// DisplayName returns package.Func for terminal/report (short package).
func (f FuncCoverage) DisplayName() string {
	pkg := displayPackageName(f.Package)
	return pkg + "." + f.Func
}

// RunCoverFunc runs "go tool cover -func=coverPath" and returns combined stdout+stderr.
func RunCoverFunc(coverPath string) (string, error) {
	dir := filepath.Dir(coverPath)
	cmd := exec.Command("go", "tool", "cover", "-func="+coverPath)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go tool cover -func: %w", err)
	}
	return string(out), nil
}

// ParseFuncCoverage parses "go tool cover -func" output into a list of function coverages.
// Skips the "total:" line. Returns only entries with a numeric percentage.
func ParseFuncCoverage(output string) ([]FuncCoverage, error) {
	var list []FuncCoverage
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "total:") {
			continue
		}
		f, ok := parseFuncLine(line)
		if !ok {
			continue
		}
		list = append(list, f)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

// parseFuncLine parses one line: "file.go:42:\t\tFunctionName\t\t92.8%" (tabs or spaces).
// File path may contain colons (e.g. Windows). Multiple tabs may separate columns.
func parseFuncLine(line string) (FuncCoverage, bool) {
	tabParts := strings.Split(line, "\t")
	var fileLine, funcName, pctStr string
	if len(tabParts) >= 2 {
		fileLine = strings.TrimSpace(strings.TrimSuffix(tabParts[0], ":"))
		// Last part is "92.8%"; everything between first and last non-empty is function name
		for i := len(tabParts) - 1; i >= 0; i-- {
			pctStr = strings.TrimSuffix(strings.TrimSpace(tabParts[i]), "%")
			if pctStr != "" {
				if _, err := strconv.ParseFloat(pctStr, 64); err == nil {
					// Collect middle parts for function name
					var nameParts []string
					for j := 1; j < i; j++ {
						s := strings.TrimSpace(tabParts[j])
						if s != "" {
							nameParts = append(nameParts, s)
						}
					}
					funcName = strings.Join(nameParts, " ")
					break
				}
			}
		}
	}
	if fileLine == "" || funcName == "" || pctStr == "" {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return FuncCoverage{}, false
		}
		fileLine = strings.TrimSuffix(fields[0], ":")
		funcName = strings.Join(fields[1:len(fields)-1], " ")
		pctStr = strings.TrimSuffix(fields[len(fields)-1], "%")
	}
	lastColon := strings.LastIndex(fileLine, ":")
	if lastColon < 0 {
		return FuncCoverage{}, false
	}
	file := fileLine[:lastColon]
	lineStr := fileLine[lastColon+1:]
	lineNum, _ := strconv.Atoi(lineStr)
	pct, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		return FuncCoverage{}, false
	}
	pkg := filepath.Dir(file)
	return FuncCoverage{
		Package: pkg,
		Func:    funcName,
		Pct:     pct,
		File:    file,
		Line:    lineNum,
	}, true
}

// GapsOnly returns functions with coverage < 100%, with suggestions populated.
func GapsOnly(funcs []FuncCoverage) []FuncCoverage {
	var out []FuncCoverage
	for _, f := range funcs {
		if f.Pct < 100 {
			f.Suggestions = Suggest(f)
			out = append(out, f)
		}
	}
	return out
}

// Suggest returns heuristic test suggestions from function name and coverage.
func Suggest(f FuncCoverage) []string {
	name := strings.ToLower(f.Func)
	var s []string
	if f.Pct == 0 {
		s = append(s, "add tests for happy path", "error path")
	}
	if strings.Contains(name, "parse") || strings.Contains(name, "version") {
		s = append(s, "invalid format", "empty input", "malformed segments")
	}
	if strings.Contains(name, "policy") || strings.Contains(name, "evaluate") || strings.Contains(name, "rule") {
		s = append(s, "empty list", "evaluation failure", "invalid configuration")
	}
	if strings.Contains(name, "run") || strings.Contains(name, "execute") {
		s = append(s, "error path", "edge cases")
	}
	if strings.Contains(name, "registry") || strings.Contains(name, "push") || strings.Contains(name, "pop") {
		s = append(s, "empty stack", "concurrent access")
	}
	if strings.Contains(name, "compile") || strings.Contains(name, "build") {
		s = append(s, "invalid input", "empty input", "each branch")
	}
	if strings.Contains(name, "count") || strings.Contains(name, "collect") {
		s = append(s, "empty input", "multiple items")
	}
	if f.Pct > 0 && f.Pct < 100 {
		s = append(s, "error path", "missing branches")
	}
	seen := make(map[string]bool)
	var out []string
	for _, x := range s {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	if len(out) == 0 {
		out = append(out, "error path", "edge cases")
	}
	return out
}
