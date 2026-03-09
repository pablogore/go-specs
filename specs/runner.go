// runner.go executes a compiled Program. No hook resolution at runtime; no allocations in the loop.
package specs

import "testing"

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

	groups := r.program.Groups
	n := len(groups)
	for gi := 0; gi < n; gi++ {
		if ctx.failFast && ctx.failed {
			break
		}
		g := &groups[gi]
		before := g.before
		specs := g.specs
		after := g.after

		nb := len(before)
		for i := 0; i < nb; i++ {
			before[i](ctx)
			if ctx.failFast && ctx.failed {
				break
			}
		}
		if ctx.failFast && ctx.failed {
			break
		}
		ns := len(specs)
		for i := 0; i < ns; i++ {
			specs[i](ctx)
			if ctx.failFast && ctx.failed {
				break
			}
		}
		if ctx.failFast && ctx.failed {
			break
		}
		na := len(after)
		for i := na - 1; i >= 0; i-- {
			after[i](ctx)
			if ctx.failFast && ctx.failed {
				break
			}
		}
	}
}
