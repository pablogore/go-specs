// scheduler.go implements a parallel test runner: worker pool, shared context pool, deterministic reporting.
//
// Specs are compiled into a flat []RunSpec. RunParallel distributes spec indexes via an atomic
// counter; each worker pulls an index, gets a Context from the pool, runs the spec, returns the
// context. Failures are recorded by spec index; after all workers finish, failures are reported
// in spec order (deterministic). No allocations in the worker execution loop.
package specs

import (
	"fmt"
	"sync/atomic"
	"testing"
)

// parallelAbort is the sentinel panic value FailNow/Fatal/Fatalf use, when abortOnFatal is set, to
// unwind just the current spec body — mirroring testing.T.FailNow's runtime.Goexit without ever
// touching a live *testing.T from the wrong goroutine. The recovering caller must distinguish this
// from a genuine panic (see parallelStep in program.go).
type parallelAbort struct{}

// parallelBackend implements testBackend by recording failures to results[specIndex].
// One per worker; worker sets specIndex before running each spec. No reflection, no boxing.
type parallelBackend struct {
	specIndex int
	results   *[]string
	// abortOnFatal, when true, makes FailNow/Fatal/Fatalf panic(parallelAbort{}) after recording,
	// so a fatal assertion stops the rest of the spec body — matching testing.T.FailNow's abort
	// semantics. Both parallelStep (ItParallel, program.go) and RunParallel's worker pool
	// (runWorker/runWorkerBatched, via MinimalRunner.RunParallel/RunParallelBatched) set this true.
	abortOnFatal bool
}

func (p *parallelBackend) Helper() {}

func (p *parallelBackend) FailNow() {
	if p.results != nil && p.specIndex >= 0 && p.specIndex < len(*p.results) {
		(*p.results)[p.specIndex] = "fail now"
	}
	if p.abortOnFatal {
		panic(parallelAbort{})
	}
}

func (p *parallelBackend) Fatal(args ...any) {
	if p.results != nil && p.specIndex >= 0 && p.specIndex < len(*p.results) {
		(*p.results)[p.specIndex] = fmt.Sprint(args...)
	}
	if p.abortOnFatal {
		panic(parallelAbort{})
	}
}

func (p *parallelBackend) Fatalf(format string, args ...any) {
	if p.results != nil && p.specIndex >= 0 && p.specIndex < len(*p.results) {
		(*p.results)[p.specIndex] = fmt.Sprintf(format, args...)
	}
	if p.abortOnFatal {
		panic(parallelAbort{})
	}
}

// Error/Errorf never abort, matching testing.T.Error's semantics (unlike Fatal/Fatalf/FailNow
// above, they record the failure but must not panic even when abortOnFatal is set).
func (p *parallelBackend) Error(args ...any) {
	if p.results != nil && p.specIndex >= 0 && p.specIndex < len(*p.results) {
		(*p.results)[p.specIndex] = fmt.Sprint(args...)
	}
}

func (p *parallelBackend) Errorf(format string, args ...any) {
	if p.results != nil && p.specIndex >= 0 && p.specIndex < len(*p.results) {
		(*p.results)[p.specIndex] = fmt.Sprintf(format, args...)
	}
}

func (p *parallelBackend) Log(args ...any)     {}
func (p *parallelBackend) Logf(string, ...any) {}

func (p *parallelBackend) Name() string { return "" }

func (p *parallelBackend) Cleanup(func()) {}

// Run reports subtests as unsupported instead of silently skipping them. Parallel mode (ItParallel,
// RunParallel, RunParallelBatched) has no mechanism to run a subtest body against a live testing.TB
// from a worker goroutine, so fn is never called; unlike the old no-op, that is now a fatal failure
// on this spec (via Fatalf, so it also aborts the rest of the spec body when abortOnFatal is set —
// always true for every parallelBackend constructed in this package) instead of vanishing silently.
// Specs that need t.Run should use the sequential runner (MinimalRunner.Run).
func (p *parallelBackend) Run(name string, fn func(testing.TB)) {
	p.Fatalf("t.Run(%q, ...) is not supported in parallel mode (ItParallel/RunParallel/RunParallelBatched); the subtest was not run — use the sequential runner for specs that need t.Run", name)
}

// runWorker runs specs whose indexes it acquires via next. Uses one Context from the pool for
// the whole worker lifetime; resets it per spec. Backend is the worker's dedicated parallelBackend.
// No allocations in the loop: context from pool, backend is preallocated, specs slice is read-only.
func runWorker(specs []RunSpec, backend *parallelBackend, next *uint32, results *[]string) {
	ctx := contextPool.Get().(*Context)
	defer func() {
		ctx.Reset(nil)
		contextPool.Put(ctx)
	}()

	n := uint32(len(specs))
	for {
		i := atomic.AddUint32(next, 1) - 1
		if i >= n {
			return
		}
		idx := int(i)
		backend.specIndex = idx
		ctx.Reset(backend)
		ctx.SetPathValues(PathValues{})
		runWorkerSpec(specs[idx].Fn, ctx, results, idx)
		ctx.Reset(nil)
	}
}

// runWorkerSpec runs one spec body, recovering the parallelAbort{} sentinel a fatal assertion
// panics with when backend.abortOnFatal is set (see parallelBackend.FailNow/Fatal/Fatalf) — an
// expected stop, already recorded in results[idx], not a failure to report. Any other panic is
// recorded as an ordinary spec failure instead of crashing the worker goroutine. Mirrors
// parallelStep's per-spec recover in program.go, applied per spec here too so one spec's fatal
// assertion or panic doesn't stop the worker from running the rest of its specs.
func runWorkerSpec(fn func(*Context), ctx *Context, results *[]string, idx int) {
	defer func() {
		switch r := recover(); r {
		case nil, parallelAbort{}:
		default:
			if (*results)[idx] == "" {
				(*results)[idx] = fmt.Sprintf("panic: %v", r)
			}
		}
	}()
	fn(ctx)
}

// failureReporter is the minimal interface needed to report failures (avoids requiring full testing.TB in tests).
type failureReporter interface {
	Helper()
	Fatalf(format string, args ...any)
}

// reportFailures reports the first failure in spec index order (deterministic).
func reportFailures(tb failureReporter, results []string) {
	for i, msg := range results {
		if msg != "" {
			tb.Helper()
			tb.Fatalf("spec[%d]: %s", i, msg)
			return
		}
	}
}

// RunShard runs a shard of the compiled Program for CI. Sharding operates on already-coalesced
// hook groups (specs sharing a BeforeEach/AfterEach are compiled into one group), not individual
// specs: group indices are assigned to shards by gi % shardCount == shardIndex. A suite with many
// specs under one shared hook lands its whole group on a single shard, so shard runtimes can be
// uneven when hook groups are large or unevenly sized. shardCount must be > 0 and
// 0 <= shardIndex < shardCount. Allocation happens once to build the shard's Program; the runner
// loop is allocation-free.
func RunShard(program *Program, tb testing.TB, shardIndex, shardCount int) {
	if program == nil || tb == nil {
		return
	}
	if shardCount <= 0 || shardIndex < 0 || shardIndex >= shardCount {
		NewRunner(program).Run(tb)
		return
	}
	groups := program.Groups
	sharded := make([]group, 0, len(groups)/shardCount+1)
	for gi := range groups {
		if gi%shardCount == shardIndex {
			sharded = append(sharded, groups[gi])
		}
	}
	if len(sharded) == 0 {
		return
	}
	prog := &Program{Groups: sharded}
	NewRunner(prog).Run(tb)
}
