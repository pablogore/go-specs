// program.go defines the compiled execution graph: groups of (before, specs, after) for hook reuse.
// No reflection; minimal layout. Large suites (100k+ specs) share before/after slices per group.
package specs

import "sync"

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
func parallelStep(steps []step) step {
	return func(ctx *Context) {
		var wg sync.WaitGroup
		for _, s := range steps {
			s := s
			wg.Add(1)
			go func() {
				s(ctx)
				wg.Done()
			}()
		}
		wg.Wait()
	}
}
