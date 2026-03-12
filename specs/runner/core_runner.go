// core_runner.go runs a compiled Program. No hook resolution at runtime; no allocations in the loop.
package runner

import (
	"io"
	"testing"

	"github.com/pablogore/go-specs/report"
	"github.com/pablogore/go-specs/specs/compiler"
	"github.com/pablogore/go-specs/specs/ctx"
)

// Runner runs a compiled Program against a test backend.
type Runner struct {
	Program  *compiler.Program
	FailFast bool
	Reporter report.EventReporter
}

// NewRunner creates a runner for the given program. If reporter is nil, events are discarded (report.New(io.Discard)).
func NewRunner(program *compiler.Program, reporter report.EventReporter) *Runner {
	if reporter == nil {
		reporter = report.New(io.Discard)
	}
	return &Runner{Program: program, Reporter: reporter}
}

// NewRunnerFromProgram is an alias for NewRunner; kept for API compatibility.
func NewRunnerFromProgram(program *compiler.Program, reporter report.EventReporter) *Runner {
	return NewRunner(program, reporter)
}

// Run executes the program via a flattened plan: one indexed loop over steps (before/spec/after), no per-spec hook traversal.
func (r *Runner) Run(tb testing.TB) {
	if r == nil || r.Program == nil || tb == nil {
		return
	}
	suiteSeed := ctx.GetRunSeed()
	flat := r.Program.FlattenedSteps()
	if len(flat) == 0 {
		RunPlanWithTB(&Plan{}, tb, suiteSeed, r.Reporter)
		return
	}
	steps := make([]Step, 0, len(flat)+1)
	if r.FailFast {
		steps = append(steps, func(c *ctx.Context) { c.SetFailFast(true) })
	}
	for _, s := range flat {
		steps = append(steps, Step(s))
	}
	RunPlanWithTB(&Plan{Steps: steps}, tb, suiteSeed, r.Reporter)
}
