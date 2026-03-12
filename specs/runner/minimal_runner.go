// minimal_runner.go implements a high-performance runner: slice of specs, single loop, optional parallel.
package runner

import (
	"runtime"
	"sync"
	"testing"

	"github.com/pablogore/go-specs/specs/ctx"
	"github.com/pablogore/go-specs/specs/parallel"
)

// RunSpec is a single runnable test (name + func).
type RunSpec struct {
	Name string
	Fn   func(*ctx.Context)
}

// MinimalRunner runs a flat list of specs with a single context.
type MinimalRunner struct {
	specs []RunSpec
}

// NewMinimalRunner creates a runner with preallocated capacity.
func NewMinimalRunner(capacity int) *MinimalRunner {
	if capacity <= 0 {
		capacity = 64
	}
	return &MinimalRunner{specs: make([]RunSpec, 0, capacity)}
}

// NewMinimalRunnerFromSpecs creates a runner that runs the given specs (slice is copied).
func NewMinimalRunnerFromSpecs(specs []RunSpec) *MinimalRunner {
	if len(specs) == 0 {
		return &MinimalRunner{specs: nil}
	}
	copied := make([]RunSpec, len(specs))
	copy(copied, specs)
	return &MinimalRunner{specs: copied}
}

// Add registers one spec.
func (r *MinimalRunner) Add(name string, fn func(*ctx.Context)) {
	if r == nil || fn == nil {
		return
	}
	r.specs = append(r.specs, RunSpec{Name: name, Fn: fn})
}

// RunBatchSize is the number of specs per inner loop batch.
const RunBatchSize = 8

// DefaultChunkSize is the default batch size for RunParallelBatched when chunkSize <= 0 (re-exported from parallel).
const DefaultChunkSize = parallel.DefaultChunkSize

// Run executes all specs in order via the shared executor.
func (r *MinimalRunner) Run(tb testing.TB) {
	if r == nil || tb == nil || len(r.specs) == 0 {
		return
	}
	suiteSeed := ctx.GetRunSeed()
	steps := make([]Step, len(r.specs))
	for i := range r.specs {
		steps[i] = r.specs[i].Fn
	}
	RunPlanWithTB(&Plan{Steps: steps}, tb, suiteSeed, nil)
}

// RunParallel runs specs across workers; each worker runs specs via RunPlan.
func (r *MinimalRunner) RunParallel(tb parallel.FailureReporter, workers int) {
	if r == nil || tb == nil || len(r.specs) == 0 {
		return
	}
	specs := r.specs
	n := len(specs)
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > n {
		workers = n
	}
	if workers <= 0 {
		workers = 1
	}
	steps := make([]Step, n)
	for i := range specs {
		steps[i] = specs[i].Fn
	}
	results := make([]string, n)
	backends := make([]parallel.ParallelBackend, workers)
	for i := range backends {
		backends[i].Results = &results
		backends[i].SpecIndex = -1
	}
	var next uint32
	var wg sync.WaitGroup
	baseSeed := ctx.GetRunSeed()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			runWorkerSteps(steps, &backends[w], &next, &results, w, baseSeed)
		}(w)
	}
	wg.Wait()
	parallel.ReportFailures(tb, results)
}

// RunParallelBatched runs specs in parallel with chunked work distribution; workers use RunPlan.
func (r *MinimalRunner) RunParallelBatched(tb parallel.FailureReporter, workers int, chunkSize int) {
	if r == nil || tb == nil || len(r.specs) == 0 {
		return
	}
	specs := r.specs
	n := len(specs)
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > n {
		workers = n
	}
	if workers <= 0 {
		workers = 1
	}
	cs := uint32(chunkSize)
	if cs <= 1 {
		r.RunParallel(tb, workers)
		return
	}
	steps := make([]Step, n)
	for i := range specs {
		steps[i] = specs[i].Fn
	}
	results := make([]string, n)
	backends := make([]parallel.ParallelBackend, workers)
	for i := range backends {
		backends[i].Results = &results
		backends[i].SpecIndex = -1
	}
	var next uint32
	var wg sync.WaitGroup
	baseSeed := ctx.GetRunSeed()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			runWorkerStepsBatched(steps, &backends[w], &next, &results, cs, w, baseSeed)
		}(w)
	}
	wg.Wait()
	parallel.ReportFailures(tb, results)
}
