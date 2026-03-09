# perfcheck

Performance regression check for Go benchmarks. Parses `go test -bench -benchmem -json` output and fails CI if any benchmark regresses by more than a threshold (default 10%).

## Usage

```bash
# 1. Capture baseline (e.g. on main after a known-good run)
go test ./benchmarks -bench . -benchmem -json -count=1 > baseline.json

# 2. In CI (or before merge), run benchmarks and compare
go test ./benchmarks -bench . -benchmem -json -count=1 > bench.json
go run ./tools/perfcheck/main.go bench.json baseline.json
# Exit 0 = no regression; exit 1 = at least one benchmark regressed >10%
```

## Options

- **-threshold** (default `0.10`): Regression threshold as a fraction. Example: `-threshold=0.10` means fail if current ns/op is more than 10% higher than baseline.

```bash
go run ./tools/perfcheck/main.go -threshold=0.05 bench.json baseline.json
```

## Example

- Baseline: `BenchmarkAssertion_GoSpecs-16` = 75 ns/op  
- Current: 95 ns/op  
- Regression: (95 − 75) / 75 = +26.7%  
- With default threshold 10%, **CI fails** (exit 1).

## CI integration

1. Store `baseline.json` in the repo (or as a CI artifact from a pinned run).
2. In your pipeline: run benchmarks with `-json`, then run `perfcheck current.json baseline.json`.
3. If perfcheck exits non-zero, fail the job.
