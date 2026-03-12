# ARCHITECTURE.md

This document describes the monorepo layout and module boundaries for `go-specs`.

---

## Monorepo Layout

The repository is a **multi-module Go workspace** (no root `go.mod`). A root `go.work` includes all modules so that `go build` and `go test` resolve local modules without publishing.

```
go-specs
├── specs        # runner + DSL (module: github.com/pablogore/go-specs/specs)
├── assert       # core assertions and matchers (module: github.com/pablogore/go-specs/assert)
├── gen          # value generators for property testing (module: github.com/pablogore/go-specs/gen)
├── snapshots    # snapshot storage and comparison (module: github.com/pablogore/go-specs/snapshots)
├── mock         # mocking utilities (module: github.com/pablogore/go-specs/mock)
├── report/      # event types and reporter (module: github.com/pablogore/go-specs/report)
├── benchmarks/  # performance benchmarks (go-specs vs Testify vs Gomega)
├── examples/    # usage examples (module: github.com/pablogore/go-specs/examples)
└── cmd/
    └── specs-ci/   # CLI (binary: specs-cli)
```

---

## Module Dependencies

- **specs** → assert, report, snapshots
- **assert** → (none)
- **report** → (none)
- **gen** → (none)
- **snapshots** → (none)
- **mock** → (none)
- **benchmarks** → specs
- **examples** → specs, mock
- **cmd/specs-ci** → specs

No cycles: assert, gen, snapshots, and mock do not depend on specs or runner.

---

## Import Paths

Public import paths are unchanged for compatibility:

- `github.com/pablogore/go-specs/specs`
- `github.com/pablogore/go-specs/assert`
- `github.com/pablogore/go-specs/report`
- `github.com/pablogore/go-specs/mock`
- `github.com/pablogore/go-specs/gen/generators`
- `github.com/pablogore/go-specs/snapshots`

Internal and runner code live under the specs module and use:

- `github.com/pablogore/go-specs/specs/internal/registry`
- `github.com/pablogore/go-specs/specs/internal/plan`
- `github.com/pablogore/go-specs/specs/runner`

---

## Build and Test

From repo root (with `go.work` in effect):

- **Build:** `go build ./assert/... ./specs/... ./report/... ./mock/... ./gen/... ./snapshots/... ./benchmarks/... ./examples/... ./cmd/specs-ci/...`
- **Test:** `go test ./assert/... ./specs/... ./report/... ./mock/... ./gen/... ./snapshots/... ./benchmarks/... ./examples/...`
- **Bench:** `go test ./benchmarks -run='^$' -bench=. -benchmem`
- **CLI:** `go build -o specs-cli ./cmd/specs-ci`

Or use `make test`, `make bench`, `make build`.
