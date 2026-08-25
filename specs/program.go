// program.go defines the compiled execution graph: groups of (before, specs, after) for hook reuse.
// No reflection; minimal layout. Large suites (100k+ specs) share before/after slices per group.
package specs

import (
	"fmt"
	"sync"
)

// step is a single executable step (hook or spec body). Same signature as RunSpec.Fn.
type step func(*Context)

// group is one execution unit: run before once, then all specs, then after once (reverse order).
// Before/after slices are shared across all specs in the group to reduce memory and improve locality.
// hookKey is the builder's scope key for coalescing; not used by the runner.
type group struct {
	before  []step
	specs   []step
	after   []step
	hookKey string
}

// Program is a compiled execution program. Groups run in order; within a group: before once, all specs, after once (reverse).
// Built by Builder; executed by Runner.
type Program struct {
	Groups []group
}

// runAll returns a single step that runs the given steps in order. Used to wrap one spec's
// full sequence (beforeEach+fn+afterEach) for parallelStep.
func runAll(steps []step) step {
	return func(ctx *Context) {
		for _, s := range steps {
			s(ctx)
		}
	}
}

// parallelStep returns a single step that runs all steps in parallel (each in its own goroutine).
// Used by the builder to compile ItParallel groups. Allocations (WaitGroup, goroutines) happen
// inside the step, not in the runner loop.
//
// Each goroutine runs its own *Context, pulled from contextPool and backed by a parallelBackend
// (the same type RunParallel's worker pool uses), instead of sharing ctx: Context.failed and the
// underlying *testing.T are not safe for concurrent access, and testing.T.FailNow (used by
// Fatal/Fatalf) must only be called from the goroutine running the test. Once every goroutine has
// finished, failures are replayed on ctx from the calling goroutine, so Fatalf/FailFast still
// happen on the right goroutine.
//
// parallelBackend cannot safely expose a live *testing.T (doing so would let a spec body call
// t.Fatalf from the wrong goroutine, reintroducing the race this fixes), so child.T is nil inside
// an ItParallel body; use ctx.Expect(...) instead of ctx.T directly. A spec that panics anyway
// (e.g. by dereferencing the nil ctx.T) is recovered and reported as an ordinary spec failure
// instead of crashing the process, and pool cleanup always runs via defer.
func parallelStep(steps []step) step {
	return func(ctx *Context) {
		if len(steps) == 0 {
			return
		}
		pathValues := ctx.Path()
		results := make([]string, len(steps))
		var wg sync.WaitGroup
		for i, s := range steps {
			i, s := i, s
			wg.Add(1)
			go func() {
				defer wg.Done()
				backend := &parallelBackend{specIndex: i, results: &results}
				child := contextPool.Get().(*Context)
				child.Reset(backend)
				child.SetPathValues(pathValues)
				defer func() {
					// Only record the panic if the spec hasn't already reported a more
					// informative assertion failure (parallelBackend.Fatalf does not stop
					// execution, so a spec can fail an assertion and then go on to panic).
					if r := recover(); r != nil && results[i] == "" {
						results[i] = fmt.Sprintf("panic: %v", r)
					}
					child.Reset(nil)
					contextPool.Put(child)
				}()
				s(child)
			}()
		}
		wg.Wait()
		for _, msg := range results {
			if msg != "" {
				ctx.recordFailure()
				break
			}
		}
		reportFailures(ctx.backend, results)
	}
}
