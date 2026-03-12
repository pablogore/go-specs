// compiled_suite_run.go executes a CompiledSuite (plan produced by the compiler).
package runner

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pablogore/go-specs/report"
	"github.com/pablogore/go-specs/specs/compiler"
	"github.com/pablogore/go-specs/specs/ctx"
	"github.com/pablogore/go-specs/specs/property"
)

// RunCompiledSuite executes all specs in the suite's plan. Call this from the runner layer; the compiler only produces the suite data.
func RunCompiledSuite(suite *compiler.CompiledSuite, tb testing.TB) {
	if suite == nil || suite.Plan == nil || tb == nil || len(suite.Plan.ProgramStart) == 0 {
		return
	}
	backend := ctx.AsTestBackend(tb)
	defer ctx.PutTestBackend(backend)
	rep := suite.Reporter
	if rep == nil {
		rep = report.New(io.Discard)
	}
	suiteSeed := ctx.GetRunSeed()
	if suite.HasSeed {
		suiteSeed = suite.Seed
	}
	runPlanFlatNoSubtests(backend, rep, suite.Plan, suiteSeed)
}

func runPlanFlatNoSubtests(backend ctx.TestBackend, rep report.EventReporter, plan *compiler.ExecutionPlan, suiteSeed uint64) {
	suiteName := ""
	if plan != nil && len(plan.Names) > 0 && len(plan.FullNames) > 0 {
		suiteName = plan.FullNames[0]
		if idx := strings.Index(suiteName, "/"); idx >= 0 {
			suiteName = suiteName[:idx]
		}
	}
	rep.SuiteStarted(report.SuiteStartEvent{Name: suiteName, Time: time.Now()})
	var failedCount int
	defer func() {
		n := 0
		if plan != nil {
			n = len(plan.ProgramStart)
		}
		rep.SuiteFinished(report.SuiteEndEvent{Name: suiteName, Time: time.Now(), TotalSpecs: n, FailedSpecs: failedCount})
	}()
	for i := 0; i < len(plan.ProgramStart); i++ {
		runExecution(backend, rep, plan, i, &failedCount, suiteSeed)
	}
}

func runExecution(backend ctx.TestBackend, rep report.EventReporter, plan *compiler.ExecutionPlan, i int, failedCount *int, suiteSeed uint64) {
	start := plan.ProgramStart[i]
	length := plan.ProgramLen[i]
	if start+length > len(plan.Instructions) {
		return
	}
	name := ""
	path := []string(nil)
	if i < len(plan.Names) {
		name = plan.Names[i]
	}
	if i < len(plan.FullNames) {
		path = strings.Split(plan.FullNames[i], "/")
	}
	rep.SpecStarted(report.SpecStartEvent{Name: name, Path: path, Time: time.Now()})
	program := plan.Instructions[start : start+length]
	c := ctx.GetFromPool()
	defer func() {
		if c.Failed() {
			*failedCount++
		}
		rep.SpecFinished(report.SpecResultEvent{
			SpecStartEvent: report.SpecStartEvent{Name: name, Path: path, Time: time.Now()},
			Failed:         c.Failed(),
			Message:        "",
		})
		c.Reset(nil)
		ctx.PutInPool(c)
	}()
	c.Reset(backend)
	c.SetPathValues(property.PathValues{})
	c.SetSeed(suiteSeed)
	runProgram(program, c, nil)
}

func runProgram(program []compiler.Instruction, c *ctx.Context, path *property.PathValues) {
	if path != nil {
		c.SetPathValues(*path)
	}
	for _, inst := range program {
		if inst.Fn != nil {
			inst.Fn(c)
		}
	}
}
