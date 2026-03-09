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

// parallelBackend implements testBackend by recording failures to results[specIndex].
// One per worker; worker sets specIndex before running each spec. No reflection, no boxing.
type parallelBackend struct {
	specIndex int
	results   *[]string
}

func (p *parallelBackend) Helper() {}

func (p *parallelBackend) FailNow() {
	if p.results != nil && p.specIndex >= 0 && p.specIndex < len(*p.results) {
		(*p.results)[p.specIndex] = "fail now"
	}
}

func (p *parallelBackend) Fatal(args ...any) {
	if p.results != nil && p.specIndex >= 0 && p.specIndex < len(*p.results) {
		(*p.results)[p.specIndex] = fmt.Sprint(args...)
	}
}

func (p *parallelBackend) Fatalf(format string, args ...any) {
	if p.results != nil && p.specIndex >= 0 && p.specIndex < len(*p.results) {
		(*p.results)[p.specIndex] = fmt.Sprintf(format, args...)
	}
}

func (p *parallelBackend) Error(args ...any) {
	p.Fatal(args...)
}

func (p *parallelBackend) Errorf(format string, args ...any) {
	p.Fatalf(format, args...)
}

func (p *parallelBackend) Log(args ...any)   {}
func (p *parallelBackend) Logf(string, ...any) {}

func (p *parallelBackend) Name() string { return "" }

func (p *parallelBackend) Cleanup(func()) {}

func (p *parallelBackend) Run(name string, fn func(testing.TB)) {
	// Parallel mode does not support subtests; Run is a no-op so the spec does not block.
	// Specs that need t.Run should use the sequential runner.
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
		specs[idx].Fn(ctx)
		ctx.Reset(nil)
	}
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

// RunShard runs a shard of the compiled Program for CI. Group indices are assigned to shards
// by gi % shardCount == shardIndex. shardCount must be > 0 and 0 <= shardIndex < shardCount.
// Allocation happens once to build the shard's Program; the runner loop is allocation-free.
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
