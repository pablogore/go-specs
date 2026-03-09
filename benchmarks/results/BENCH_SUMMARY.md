# Benchmark summary (from benchmarks/results/current.txt)

Parsed and averaged over 10 runs. Apple M4 Max, darwin/arm64.

---

## Assertions

| Framework                 | ns/op   | allocs |
| ------------------------- | ------- | ------ |
| go-specs EqualTo          | ~1 ns   | 0      |
| go-specs Expect().ToEqual  | ~7.4 ns | 0      |
| Testify Equal              | ~86 ns  | 0      |
| Gomega Expect().To(Equal)  | ~241 ns | 3      |

## Runner (1000 specs, one assertion per spec)

| Framework | ns/op    |
| --------- | -------  |
| go-specs  | ~1.7 µs  |
| Testify   | ~85 µs   |
| Gomega    | ~244 µs  |

## Hooks (100 specs, 5 nested BeforeEach + one assertion per spec)

| Framework | ns/op    |
| --------- | -------  |
| go-specs  | ~199 ns  |
| Testify   | ~8.5 µs  |
| Gomega    | ~24 µs   |

## Matcher (Expect().To(Equal) style)

| Framework        | ns/op   | allocs |
| ---------------- | ------- | ------ |
| go-specs         | ~7.8 ns | 0      |
| Gomega           | ~227 ns | 3      |

## Suite scaling (go-specs, full suite run)

| Specs  | Time     |
| -----  | -------  |
| 100    | ~201 ns  |
| 1000   | ~1.7 µs  |
| 10000  | ~17 µs   |
| 50000  | ~85 µs   |

---

### Why go-specs is fast

- **Zero allocations** — The assertion and runner hot paths allocate nothing on success (0 allocs/op in the tables above), reducing GC pressure.
- **Compiled execution plan** — Suites are compiled once into a fixed program; the runner executes steps via direct function dispatch instead of per-spec lookups or reflection.
- **No reflection** — Assertions use generics and direct comparison; the fast path avoids `reflect.DeepEqual` and runtime type switches.
- **Sequential runner loop** — The runner invokes spec and hook functions in a simple loop with direct calls; no matcher heap allocations or indirection on the hot path.
