// executor.go provides a single allocation-free execution engine for all runners.
package runner

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablogore/go-specs/report"
	"github.com/pablogore/go-specs/specs/ctx"
	"github.com/pablogore/go-specs/specs/parallel"
	"github.com/pablogore/go-specs/specs/property"
)

// Step is a single executable step (hook or spec body). No interface; avoids boxing in hot path.
type Step func(*ctx.Context)

// Plan is a sequence of steps executed in order by RunPlan.
type Plan struct {
	Steps []Step
}

// RunPlan runs all steps in order using the given context. Allocation-free loop.
// Stops early if c.FailFast() and c.Failed(). Caller must have already set c.Reset(backend) and c.SetPathValues as needed.
func RunPlan(plan *Plan, c *ctx.Context) {
	if plan == nil || len(plan.Steps) == 0 {
		return
	}
	steps := plan.Steps
	for i := 0; i < len(steps); i++ {
		steps[i](c)
		if c.FailFast() && c.Failed() {
			return
		}
	}
}

// RunPlanWithTB obtains a context from the pool, runs the plan against tb, and returns the context to the pool.
// suiteSeed must be the run seed captured once at suite entry (e.g. ctx.GetRunSeed()); it is not read internally.
// If reporter is non-nil, SuiteStarted and SuiteFinished are emitted.
func RunPlanWithTB(plan *Plan, tb testing.TB, suiteSeed uint64, reporter report.EventReporter) {
	if plan == nil || tb == nil {
		return
	}
	backend := ctx.AsTestBackend(tb)
	defer ctx.PutTestBackend(backend)
	if reporter != nil {
		reporter.SuiteStarted(report.SuiteStartEvent{Name: "", Time: time.Now()})
		defer func() {
			n := 0
			if plan != nil {
				n = len(plan.Steps)
			}
			reporter.SuiteFinished(report.SuiteEndEvent{Name: "", Time: time.Now(), TotalSpecs: n, FailedSpecs: 0})
		}()
	}
	c := ctx.GetFromPool()
	defer func() {
		c.Reset(nil)
		ctx.PutInPool(c)
	}()
	c.Reset(backend)
	c.SetPathValues(property.PathValues{})
	c.SetSeed(suiteSeed)
	RunPlan(plan, c)
}

// RunPlanRange runs plan.Steps[start:end] with a single context from the pool. Use for parallel workers that each run a range.
// runSeed is the seed for this range (e.g. suite seed or workerSeed); captured at entry point and passed down.
func RunPlanRange(plan *Plan, start, end int, tb testing.TB, runSeed uint64) {
	if plan == nil || tb == nil {
		return
	}
	steps := plan.Steps
	if start < 0 {
		start = 0
	}
	if end > len(steps) {
		end = len(steps)
	}
	if start >= end {
		return
	}
	backend := ctx.AsTestBackend(tb)
	defer ctx.PutTestBackend(backend)
	c := ctx.GetFromPool()
	defer func() {
		c.Reset(nil)
		ctx.PutInPool(c)
	}()
	c.Reset(backend)
	c.SetPathValues(property.PathValues{})
	c.SetSeed(runSeed)
	for i := start; i < end; i++ {
		steps[i](c)
		if c.FailFast() && c.Failed() {
			return
		}
	}
}

// runWorkerSteps runs specs whose indexes it acquires via next. One context per worker; each spec runs via RunPlan.
// workerSeed = baseSeed ^ workerID so parallel runs are deterministic per worker.
func runWorkerSteps(specs []Step, backend *parallel.ParallelBackend, next *uint32, results *[]string, workerID int, baseSeed uint64) {
	c := ctx.GetFromPool()
	defer func() {
		c.Reset(nil)
		ctx.PutInPool(c)
	}()
	n := uint32(len(specs))
	workerSeed := baseSeed ^ uint64(workerID)
	var one Plan
	one.Steps = make([]Step, 1)
	for {
		i := atomic.AddUint32(next, 1) - 1
		if i >= n {
			return
		}
		idx := int(i)
		backend.SpecIndex = idx
		c.Reset(backend)
		c.SetPathValues(property.PathValues{})
		c.SetSeed(workerSeed)
		one.Steps[0] = specs[idx]
		RunPlan(&one, c)
		c.Reset(nil)
	}
}

// runWorkerStepsBatched runs specs in chunks; each iteration claims chunkSize indexes and runs each via RunPlan.
func runWorkerStepsBatched(specs []Step, backend *parallel.ParallelBackend, next *uint32, results *[]string, chunkSize uint32, workerID int, baseSeed uint64) {
	c := ctx.GetFromPool()
	defer func() {
		c.Reset(nil)
		ctx.PutInPool(c)
	}()
	n := uint32(len(specs))
	if chunkSize == 0 {
		chunkSize = 1
	}
	workerSeed := baseSeed ^ uint64(workerID)
	var one Plan
	one.Steps = make([]Step, 1)
	for {
		start := atomic.AddUint32(next, chunkSize) - chunkSize
		if start >= n {
			return
		}
		end := start + chunkSize
		if end > n {
			end = n
		}
		// Reset per-spec so c.failed from one spec never bleeds into the next
		// (Context.Step skips its body when c.failed is true).
		for i := start; i < end; i++ {
			idx := int(i)
			backend.SpecIndex = idx
			c.Reset(backend)
			c.SetPathValues(property.PathValues{})
			c.SetSeed(workerSeed)
			one.Steps[0] = specs[idx]
			RunPlan(&one, c)
			c.Reset(nil)
		}
	}
}
