# ARCHITECTURE.md

This document describes the monorepo layout and module boundaries for `go-specs`.

---

## Monorepo Layout

The repository is a **multi-module Go workspace** (no root `go.mod`). A root `go.work` includes all modules so that `go build` and `go test` resolve local modules without publishing.

```
go-specs
├── specs        # runner + DSL (module: github.com/getsyntegrity/go-specs/specs)
├── assert       # core assertions / matchers (module: github.com/getsyntegrity/go-specs/assert)
├── matchers     # test helper matchers (module: github.com/getsyntegrity/go-specs/matchers)
├── gen          # value generators for property testing (module: github.com/getsyntegrity/go-specs/gen)
├── snapshots    # snapshot storage and comparison (module: github.com/getsyntegrity/go-specs/snapshots)
├── mock         # mocking utilities (module: github.com/getsyntegrity/go-specs/mock)
├── report/      # event types and reporter (module: github.com/getsyntegrity/go-specs/report)
├── benchmarks/  # performance benchmarks (go-specs vs Testify vs Gomega)
├── examples/    # usage examples (module: github.com/getsyntegrity/go-specs/examples)
└── tools/
    └── specs-cli/   # CLI (module: github.com/getsyntegrity/go-specs/tools/specs-cli)
```

---

## Module Dependencies

- **specs** → assert, report, snapshots
- **assert** → (none)
- **report** → (none)
- **matchers** → (none)
- **gen** → (none)
- **snapshots** → (none)
- **mock** → (none)
- **benchmarks** → specs
- **examples** → specs, mock
- **tools/specs-cli** → specs

No cycles: assert, matchers, gen, snapshots, and mock do not depend on specs or runner.

---

## Import Paths

Public import paths are unchanged for compatibility:

- `github.com/getsyntegrity/go-specs/specs`
- `github.com/getsyntegrity/go-specs/assert`
- `github.com/getsyntegrity/go-specs/report`
- `github.com/getsyntegrity/go-specs/matchers`
- `github.com/getsyntegrity/go-specs/mock`
- `github.com/getsyntegrity/go-specs/gen/generators`
- `github.com/getsyntegrity/go-specs/snapshots`

Internal and runner code live under the specs module and use:

- `github.com/getsyntegrity/go-specs/specs/internal/registry`
- `github.com/getsyntegrity/go-specs/specs/internal/plan`
- `github.com/getsyntegrity/go-specs/specs/runner`

---

## Build and Test

From repo root (with `go.work` in effect):

- **Build:** `go build ./assert/... ./specs/... ./report/... ./mock/... ./matchers/... ./gen/... ./snapshots/... ./benchmarks/... ./examples/... ./tools/specs-cli/...`
- **Test:** `go test ./assert/... ./specs/... ./report/... ./mock/... ./matchers/... ./gen/... ./snapshots/... ./benchmarks/... ./examples/...`
- **Bench:** `go test ./benchmarks -run='^$' -bench=. -benchmem`
- **CLI:** `go build -o specs-cli ./tools/specs-cli`

Or use `make test`, `make bench`, `make build`.
