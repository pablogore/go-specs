// runner.go executes a compiled Program. No hook resolution at runtime; no allocations in the loop.
package specs

import (
	"runtime/debug"
	"testing"
)

// Runner runs a compiled Program against a test backend. One context from the pool, reused for every step.
type Runner struct {
	program  *Program
	FailFast bool // if true, stop after the first step that sets ctx.failed (e.g. assertion failure)
}

// NewRunner creates a runner for the given program. Program must not be nil; do not modify program.Groups after creation.
func NewRunner(program *Program) *Runner {
	return &Runner{program: program}
}

// NewRunnerFromProgram is an alias for NewRunner; kept for API compatibility.
func NewRunnerFromProgram(program *Program) *Runner {
	return NewRunner(program)
}

// Run executes all groups in order. Within each group: before once, all specs, then after once (reverse order).
// Zero allocations in the loop; deterministic.
//
// A panic in before, a spec, or an after hook is recovered instead of crashing the process — see
// runGroup for the exact contract (which specs still run, whether after still runs).
func (r *Runner) Run(tb testing.TB) {
	if r == nil || r.program == nil || tb == nil || len(r.program.Groups) == 0 {
		return
	}
	backend := asTestBackend(tb)
	defer putTestBackend(backend)
	ctx := contextPool.Get().(*Context)
	defer func() {
		ctx.Reset(nil)
		contextPool.Put(ctx)
	}()
	ctx.Reset(backend)
	ctx.SetPathValues(PathValues{})
	if r.FailFast {
		ctx.SetFailFast(true)
	}

	runGroups(ctx, r.program.Groups)
}

// runGroups runs each group in order, stopping before the next group if FailFast is set and a
// previous group left ctx failed (an ordinary assertion failure or a recovered panic — recordFailure
// marks both the same way, so this check needs no panic-specific case). Split out from Run so the
// group-iteration/FailFast contract can be tested without a real testing.TB.
func runGroups(ctx *Context, groups []group) {
	n := len(groups)
	for gi := 0; gi < n; gi++ {
		if ctx.failFast && ctx.failed {
			break
		}
		runGroup(ctx, &groups[gi])
		if ctx.failFast && ctx.failed {
			break
		}
	}
}

// runGroup runs one group's before, specs, and after against ctx.
//
// before runs as one recovered unit: a panic in any before hook stops the remaining before hooks
// in this group (later ones may depend on earlier ones' side effects) and skips this group's specs
// entirely — they can't be trusted to run meaningfully against setup that never completed.
//
// Each spec is recovered individually, so one panicking spec doesn't stop its siblings — matching
// the default execution path's contract (see execution_plan.go's runProgram).
//
// after always runs, via defer: whether before or a spec panicked, or a spec called a real
// t.Fatal/FailNow (runtime.Goexit unwinds through this defer same as a panic would), after gets a
// chance to clean up whatever before did set up. Each after hook is recovered individually, so one
// panicking after hook doesn't stop its siblings from attempting to run.
func runGroup(ctx *Context, g *group) {
	defer runAfterRecovered(ctx, g.after)

	if !runBeforeRecovered(ctx, g.before) {
		return
	}
	if ctx.failFast && ctx.failed {
		return
	}
	runSpecsRecovered(ctx, g.specs)
}

// runBeforeRecovered runs a group's before hooks in order. Returns false if a panic stopped setup
// partway through, in which case the caller must not run this group's specs.
func runBeforeRecovered(ctx *Context, before []step) (ok bool) {
	ok = true
	defer func() {
		if recovered := recover(); recovered != nil {
			ok = false
			ctx.recordFailure()
			ctx.backend.Errorf("panic in before hook: %v\n%s", recovered, debug.Stack())
		}
	}()
	for _, b := range before {
		b(ctx)
		if ctx.failFast && ctx.failed {
			return
		}
	}
	return
}

// runSpecsRecovered runs a group's specs in order, recovering each one individually.
func runSpecsRecovered(ctx *Context, specs []step) {
	for _, s := range specs {
		runStepRecovered(ctx, s, "panic")
		if ctx.failFast && ctx.failed {
			return
		}
	}
}

// runAfterRecovered runs a group's after hooks in reverse order, recovering each individually.
func runAfterRecovered(ctx *Context, after []step) {
	for i := len(after) - 1; i >= 0; i-- {
		runStepRecovered(ctx, after[i], "panic in after hook")
		if ctx.failFast && ctx.failed {
			return
		}
	}
}

// runStepRecovered runs a single step, recovering a panic so it fails just this step (recorded via
// ctx.recordFailure + ctx.backend.Errorf with message and stack trace) instead of crashing the
// process. label distinguishes a before/spec panic from an after-hook panic in the reported message.
func runStepRecovered(ctx *Context, s step, label string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			ctx.recordFailure()
			ctx.backend.Errorf("%s: %v\n%s", label, recovered, debug.Stack())
		}
	}()
	s(ctx)
}
