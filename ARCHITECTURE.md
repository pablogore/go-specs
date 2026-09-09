# ARCHITECTURE.md

This document describes the monorepo layout and module boundaries for `go-specs`.

---

## Monorepo Layout

The repository is a **multi-module Go workspace** (no root `go.mod`). A root `go.work` includes all modules so that `go build` and `go test` resolve local modules without publishing.

```
go-specs
├── specs        # runner + DSL (module: github.com/pablogore/go-specs/specs)
├── assert       # core assertions / matchers (module: github.com/pablogore/go-specs/assert)
├── gen          # value generators for property testing (module: github.com/pablogore/go-specs/gen)
├── snapshots    # snapshot storage and comparison (module: github.com/pablogore/go-specs/snapshots)
├── mock         # mocking utilities (module: github.com/pablogore/go-specs/mock)
├── report/      # event types and reporter (module: github.com/pablogore/go-specs/report)
├── benchmarks/  # performance benchmarks (go-specs vs Testify vs Gomega)
├── examples/    # usage examples (module: github.com/pablogore/go-specs/examples)
└── tools/
    └── specs-cli/   # CLI (module: github.com/pablogore/go-specs/tools/specs-cli)
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
- **tools/specs-cli** → specs

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

Internal code lives under the specs module and uses:

- `github.com/pablogore/go-specs/specs/internal/registry`

---

## Build and Test

From repo root (with `go.work` in effect):

- **Build:** `go build ./assert/... ./specs/... ./report/... ./mock/... ./gen/... ./snapshots/... ./benchmarks/... ./examples/... ./tools/specs-cli/...`
- **Test:** `go test ./assert/... ./specs/... ./report/... ./mock/... ./gen/... ./snapshots/... ./benchmarks/... ./examples/...`
- **Bench:** `go test ./benchmarks -run='^$' -bench=. -benchmem`
- **CLI:** `go build -o specs-cli ./tools/specs-cli`

Or use `make test`, `make bench`, `make build`.
