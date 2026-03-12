// program.go defines the compiled execution graph: groups of (before, specs, after) for hook reuse.
// No reflection; minimal layout. Large suites (100k+ specs) share before/after slices per group.
package compiler

import (
	"sync"

	"github.com/pablogore/go-specs/specs/ctx"
)

// Step is the type of a single executable step (hook or spec body). Exported so runner can build flat plans.
type Step = func(*ctx.Context)

// step is the internal alias used by group.
type step = Step

// group is one execution unit: run before once, then all specs, then after once (reverse order).
// Before/after slices are shared across all specs in the group to reduce memory and improve locality.
// hookKey is the builder's scope key for coalescing; not used by the runner.
type group struct {
	before  []step
	specs   []step
	after   []step
	hookKey hookKey
}

// Program is a compiled execution program. Groups run in order; within a group: before once, all specs, after once (reverse).
// Built by Builder; executed by Runner.
type Program struct {
	Groups []group
}

// NumGroups returns the number of groups (for tests).
func (p *Program) NumGroups() int {
	if p == nil {
		return 0
	}
	return len(p.Groups)
}

// GroupSpecCount returns the number of specs in the group at index i (for tests).
func (p *Program) GroupSpecCount(i int) int {
	if p == nil || i < 0 || i >= len(p.Groups) {
		return 0
	}
	return len(p.Groups[i].specs)
}

// GroupBeforeCount returns the number of before hooks in the group at index i (for tests).
func (p *Program) GroupBeforeCount(i int) int {
	if p == nil || i < 0 || i >= len(p.Groups) {
		return 0
	}
	return len(p.Groups[i].before)
}

// GroupAfterCount returns the number of after hooks in the group at index i (for tests).
func (p *Program) GroupAfterCount(i int) int {
	if p == nil || i < 0 || i >= len(p.Groups) {
		return 0
	}
	return len(p.Groups[i].after)
}

// Run executes all groups in order. Stops early if c.FailFast() and c.Failed() (caller sets FailFast on context).
func (p *Program) Run(c *ctx.Context) {
	if p == nil || len(p.Groups) == 0 {
		return
	}
	for gi := range p.Groups {
		g := &p.Groups[gi]
		if c.FailFast() && c.Failed() {
			break
		}
		for _, s := range g.before {
			s(c)
			if c.FailFast() && c.Failed() {
				break
			}
		}
		if c.FailFast() && c.Failed() {
			break
		}
		for _, s := range g.specs {
			s(c)
			if c.FailFast() && c.Failed() {
				break
			}
		}
		if c.FailFast() && c.Failed() {
			break
		}
		for i := len(g.after) - 1; i >= 0; i-- {
			g.after[i](c)
			if c.FailFast() && c.Failed() {
				break
			}
		}
	}
}

// FlattenedSteps returns a single sequence of steps: for each group, before hooks in order, then all specs, then after hooks in reverse order.
// The runner can execute this with a simple indexed loop (no per-spec hook traversal).
func (p *Program) FlattenedSteps() []Step {
	if p == nil || len(p.Groups) == 0 {
		return nil
	}
	var out []Step
	for gi := range p.Groups {
		g := &p.Groups[gi]
		for _, s := range g.before {
			out = append(out, s)
		}
		for _, s := range g.specs {
			out = append(out, s)
		}
		for i := len(g.after) - 1; i >= 0; i-- {
			out = append(out, g.after[i])
		}
	}
	return out
}

// ShardProgram returns a Program containing only groups whose index gi satisfies gi%shardCount == shardIndex.
func ShardProgram(program *Program, shardIndex, shardCount int) *Program {
	if program == nil || shardCount <= 0 || shardIndex < 0 || shardIndex >= shardCount {
		return program
	}
	groups := program.Groups
	sharded := make([]group, 0, len(groups)/shardCount+1)
	for gi := range groups {
		if gi%shardCount == shardIndex {
			sharded = append(sharded, groups[gi])
		}
	}
	if len(sharded) == 0 {
		return &Program{}
	}
	return &Program{Groups: sharded}
}

// runAll returns a single step that runs the given steps in order. Used to wrap one spec's
// full sequence (beforeEach+fn+afterEach) for parallelStep.
func runAll(steps []step) step {
	return func(c *ctx.Context) {
		for _, s := range steps {
			s(c)
		}
	}
}

// parallelStep returns a single step that runs all steps in parallel (each in its own goroutine).
// Each goroutine gets its own Context from the pool so mutable state is not shared. Used by the
// builder to compile ItParallel groups.
//
// The parent backend is wrapped with a mutex so goroutines that share a non-goroutine-safe
// backend (e.g. *parallel.ParallelBackend) cannot race. For *testing.T-backed backends the
// wrapper is a no-op overhead because testing.T is already goroutine-safe.
//
// wg.Done is always called via defer so a panic in the spec body cannot deadlock wg.Wait.
func parallelStep(steps []step) step {
	return func(c *ctx.Context) {
		safeBackend := ctx.NewLockedBackend(c.Backend())
		var wg sync.WaitGroup
		for _, s := range steps {
			s := s
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx2 := ctx.GetFromPool()
				defer func() {
					ctx2.Reset(nil)
					ctx.PutInPool(ctx2)
				}()
				ctx2.Reset(safeBackend)
				s(ctx2)
			}()
		}
		wg.Wait()
	}
}
