# BENCHMARKS.md

This document explains how benchmarks are structured in go-specs.

---

# Benchmark Philosophy

Benchmarks measure **subsystems independently**.

The framework has three primary cost centers:

1. Assertions
2. Runner execution
3. Path exploration

---

# Benchmark Suites

All benchmarks live in a single directory:

```
benchmarks/
```

Files:

| File                             | Benchmark                          |
| -------------------------------- | ---------------------------------- |
| assertion_bench_test.go          | single assertion cost (go-specs, Testify, Gomega) |
| matcher_bench_test.go            | matcher / Expect().To() performance |
| runner_bench_test.go            | N specs, one assertion per spec   |
| hooks_bench_test.go             | before-each + assertion per spec  |
| large_suite_bench_test.go       | scaling (100, 1000, 10000, 50000)  |
| minimal_and_buildsuite_bench_test.go | BuildSuite runner, MinimalRunner, parallel, nested hooks |

See [benchmarks/README.md](benchmarks/README.md) for categories and scripts.

---

# Running Benchmarks

```
go test ./benchmarks -run='^$' -bench=. -benchmem
```

---

# Expected Performance

Typical expectations:

| Operation        | ns/op     |
| ---------------- | --------- |
| Assertion        | 80–150    |
| Runner           | 10–40 µs  |
| Path exploration | 50–200 µs |

---

# Important Rule

Do not benchmark:

```
suite construction
DSL parsing
registry initialization
```

inside `b.N` loops.

These must happen before `b.ResetTimer()`.
