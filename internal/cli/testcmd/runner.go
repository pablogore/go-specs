package testcmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// go test -v output: "ok \tpath\ttime" or "ok \tpath\t(cached)" or "FAIL\tpath\ttime" or "? \tpath\t[reason]"

// pkgResult is one package's result with per-package spec counts.
type pkgResult struct {
	path         string
	status       string // "ok", "FAIL", "skip", "no_tests"
	time         string
	reason       string // for skip: e.g. "[no test files]"
	skippedCount int    // number of --- SKIP: in this package (for "ok" packages)
	// Per-package spec execution counts (from --- PASS:/FAIL:/SKIP: lines, including subtests)
	testsRun    int
	testsPassed int
	testsFailed int
	testsSkipped int
}

// runResult holds aggregated go test output.
type runResult struct {
	packages      []pkgResult
	specs         int   // total executed = passed + failed + skipped
	failed        int   // number of failed test cases (--- FAIL:)
	failedPackages int  // number of packages with failing tests (FAIL\t)
	skipped       int
	noTests       int   // packages with [no test files]
	totalSec      float64
}

func parseGoTestOutput(out []byte) runResult {
	var r runResult
	var pendingPassed, pendingFailed, pendingSkips int
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		// Package result lines: tab-separated "status\tpath\ttime_or_reason"
		if strings.HasPrefix(line, "ok\t") || strings.HasPrefix(line, "ok ") {
			fields := strings.SplitN(line, "\t", 3)
			if len(fields) >= 2 {
				path := strings.TrimSpace(fields[1])
				timeStr := ""
				if len(fields) >= 3 {
					timeStr = strings.TrimSpace(fields[2])
					if strings.HasSuffix(timeStr, "s") && !strings.HasPrefix(timeStr, "(") {
						r.totalSec += parseDuration(timeStr)
					}
				}
				run := pendingPassed + pendingFailed + pendingSkips
				r.packages = append(r.packages, pkgResult{
					path: path, status: "ok", time: timeStr,
					skippedCount:  pendingSkips,
					testsRun:      run,
					testsPassed:   pendingPassed,
					testsFailed:   pendingFailed,
					testsSkipped:  pendingSkips,
				})
				pendingPassed, pendingFailed, pendingSkips = 0, 0, 0
			}
			continue
		}
		if strings.HasPrefix(line, "FAIL\t") {
			fields := strings.SplitN(line, "\t", 3)
			if len(fields) >= 2 {
				path := strings.TrimSpace(fields[1])
				timeStr := ""
				if len(fields) >= 3 {
					timeStr = strings.TrimSpace(fields[2])
					r.totalSec += parseDuration(timeStr)
				}
				run := pendingPassed + pendingFailed + pendingSkips
				r.packages = append(r.packages, pkgResult{
					path: path, status: "FAIL", time: timeStr,
					skippedCount:  pendingSkips,
					testsRun:      run,
					testsPassed:   pendingPassed,
					testsFailed:   pendingFailed,
					testsSkipped:  pendingSkips,
				})
				r.failedPackages++
				pendingPassed, pendingFailed, pendingSkips = 0, 0, 0
			}
			continue
		}
		if strings.HasPrefix(line, "?\t") || strings.HasPrefix(line, "? ") {
			fields := strings.SplitN(line, "\t", 3)
			if len(fields) >= 2 {
				path := strings.TrimSpace(fields[1])
				reason := ""
				if len(fields) >= 3 {
					reason = strings.TrimSpace(fields[2])
				}
				if strings.Contains(strings.ToLower(reason), "no test files") {
					r.packages = append(r.packages, pkgResult{path: path, status: "no_tests", reason: reason})
					r.noTests++
				} else {
					r.packages = append(r.packages, pkgResult{path: path, status: "skip", time: "", reason: reason})
				}
			}
			continue
		}
		// Test result lines: count --- PASS:/FAIL:/SKIP: (including indented subtests)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--- PASS:") {
			r.specs++
			pendingPassed++
		} else if strings.HasPrefix(trimmed, "--- FAIL:") {
			r.specs++
			r.failed++
			pendingFailed++
		} else if strings.HasPrefix(trimmed, "--- SKIP:") {
			r.specs++
			r.skipped++
			pendingSkips++
		}
	}
	return r
}

func parseDuration(s string) float64 {
	s = strings.TrimSuffix(s, "s")
	var sec float64
	_, _ = fmt.Sscanf(s, "%f", &sec)
	return sec
}

func shortPath(path string) string {
	// Use last two path segments if long (e.g. .../internal/plan -> internal/plan)
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return path
}

// ANSI color codes (only used when colorsEnabled).
const (
	ansiReset  = "\033[0m"
	ansiGreen  = "\033[32m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiGray   = "\033[90m"
)

func colorsEnabled() bool {
	return os.Getenv("NO_COLOR") == ""
}

func colorLine(icon, path, color string) string {
	if !colorsEnabled() {
		return icon + " " + path
	}
	return color + icon + " " + path + ansiReset
}

func printFormattedOutput(r runResult, elapsed time.Duration) {
	fmt.Println("Running specs...")
	fmt.Println()
	for _, p := range r.packages {
		name := shortPath(p.path)
		switch p.status {
		case "ok":
			if p.skippedCount > 0 {
				line := fmt.Sprintf("⚠ %s (%d skipped)", name, p.skippedCount)
				if colorsEnabled() {
					line = ansiYellow + line + ansiReset
				}
				fmt.Println(line)
			} else {
				fmt.Println(colorLine("✓", name, ansiGreen))
			}
		case "FAIL":
			fmt.Println(colorLine("✗", name, ansiRed))
		case "no_tests":
			fmt.Println(colorLine("∅", name, ansiGray))
		case "skip":
			line := "⚠ " + name
			if colorsEnabled() {
				line = ansiYellow + line + ansiReset
			}
			fmt.Println(line)
		}
	}
	fmt.Println()
	fmt.Println("Summary")
	fmt.Println("----------------")
	const labelWidth = 10
	fmt.Printf("%-*s %d\n", labelWidth, "Packages:", len(r.packages))
	fmt.Printf("%-*s %d\n", labelWidth, "Specs:", r.specs)
	fmt.Printf("%-*s %d\n", labelWidth, "Failed:", r.failedPackages)
	fmt.Printf("%-*s %d\n", labelWidth, "Skipped:", r.skipped)
	fmt.Printf("%-*s %d\n", labelWidth, "No tests:", r.noTests)
	fmt.Printf("%-*s %s\n", labelWidth, "Time:", elapsed.Round(time.Millisecond))
}

// runGoTest runs go test with the given args. If verbose, stdout/stderr are streamed to os.Stdout/Stderr
// and also captured. Returns combined output, exit code, and any error.
func runGoTest(args []string, verbose bool) (out []byte, exitCode int, err error) {
	cmd := exec.Command("go", args...)
	cmd.Stdin = os.Stdin
	if verbose {
		var buf bytes.Buffer
		cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
		cmd.Stderr = io.MultiWriter(os.Stderr, &buf)
		err := cmd.Run()
		out = buf.Bytes()
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				return out, exit.ExitCode(), nil
			}
			return out, 1, err
		}
		return out, 0, nil
	}
	out, err = cmd.CombinedOutput()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return out, exit.ExitCode(), nil
		}
		return out, 1, err
	}
	return out, 0, nil
}
