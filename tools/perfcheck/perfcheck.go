// Package perfcheck compares benchmark results (go test -bench -benchmem -json) against a baseline
// and reports if any benchmark regresses by more than the threshold (default 10%).
package perfcheck

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// go test -json emits one JSON object per line (streaming).
type testEvent struct {
	Action string `json:"Action"`
	Test   string `json:"Test"`
	Output string `json:"Output"`
}

// ParseGoTestJSON reads go test -bench -json output and returns benchmark name -> ns/op.
// Only lines containing "ns/op" are parsed; the first matching result per benchmark is kept.
func ParseGoTestJSON(path string) (map[string]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	benchRe := regexp.MustCompile(`^(\S+)\s+(\d+)\s+([\d.]+)\s+ns/op`)
	results := make(map[string]float64)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e testEvent
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		if e.Action != "output" || e.Output == "" {
			continue
		}
		for _, line := range strings.Split(strings.TrimSuffix(e.Output, "\n"), "\n") {
			if line == "" || !strings.Contains(line, "ns/op") {
				continue
			}
			sub := benchRe.FindStringSubmatch(line)
			if sub == nil {
				continue
			}
			name := sub[1]
			var nsOp float64
			if _, err := fmt.Sscanf(sub[3], "%f", &nsOp); err != nil {
				continue
			}
			results[name] = nsOp
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// RunCheck runs the performance check: compares current and baseline JSON files and returns an error
// if any benchmark regresses above the threshold. Args are typically [ "-threshold=0.10", "current.json", "baseline.json" ].
func RunCheck(args []string) error {
	fs := flag.NewFlagSet("perf check", flag.ContinueOnError)
	threshold := fs.Float64("threshold", 0.10, "regression threshold (fraction, e.g. 0.10 = 10%)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 2 {
		fmt.Fprintf(os.Stderr, "usage: perf check [ -threshold=0.10 ] <current.json> <baseline.json>\n")
		fmt.Fprintf(os.Stderr, "  current.json  = output of: go test -bench . -benchmem -json\n")
		fmt.Fprintf(os.Stderr, "  baseline.json = baseline from a previous run (same format)\n")
		fmt.Fprintf(os.Stderr, "  -threshold     regression fraction (default 0.10 = 10%%)\n")
		return fmt.Errorf("usage: need exactly two file arguments")
	}
	currentPath, baselinePath := rest[0], rest[1]

	current, err := ParseGoTestJSON(currentPath)
	if err != nil {
		return fmt.Errorf("read current: %w", err)
	}
	baseline, err := ParseGoTestJSON(baselinePath)
	if err != nil {
		return fmt.Errorf("read baseline: %w", err)
	}

	var regressions []string
	for name, curNs := range current {
		baseNs, ok := baseline[name]
		if !ok {
			continue
		}
		if baseNs <= 0 {
			continue
		}
		pct := (curNs - baseNs) / baseNs
		if pct > *threshold {
			regressions = append(regressions, fmt.Sprintf("%s: %.2f ns/op -> %.2f ns/op (+%.1f%%)", name, baseNs, curNs, pct*100))
		}
	}

	if len(regressions) > 0 {
		fmt.Fprintf(os.Stderr, "perfcheck: regression(s) above %.0f%% threshold:\n", *threshold*100)
		for _, r := range regressions {
			fmt.Fprintln(os.Stderr, "  ", r)
		}
		return fmt.Errorf("%d regression(s) above threshold", len(regressions))
	}
	return nil
}
