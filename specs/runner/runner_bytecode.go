// runner_bytecode.go runs a compiled BCProgram with Run or RunParallel.
package runner

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pablogore/go-specs/specs/compiler"
	"github.com/pablogore/go-specs/specs/ctx"
	"github.com/pablogore/go-specs/specs/parallel"
	"github.com/pablogore/go-specs/specs/property"
)

// BytecodeRunner runs a BCProgram built by BCBuilder.
type BytecodeRunner struct {
	Program compiler.BCProgram
}

// NewBytecodeRunner creates a runner for the given bytecode program.
func NewBytecodeRunner(p compiler.BCProgram) *BytecodeRunner {
	return &BytecodeRunner{Program: p}
}

// Run executes all instructions in order via the shared executor.
func (r *BytecodeRunner) Run(tb testing.TB) {
	if r == nil || tb == nil || r.Program.BCLen() == 0 {
		return
	}
	suiteSeed := ctx.GetRunSeed()
	program := r.Program
	plan := &Plan{
		Steps: []Step{func(c *ctx.Context) { program.RunAll(c) }},
	}
	RunPlanWithTB(plan, tb, suiteSeed, nil)
}

// RunParallel runs each spec (instruction range) on a worker pool.
func (r *BytecodeRunner) RunParallel(tb parallel.FailureReporter, workers int) {
	if r == nil || tb == nil || r.Program.BCLen() == 0 {
		return
	}
	nSpecs := r.Program.NumSpecs()
	if nSpecs == 0 {
		return
	}
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > nSpecs {
		workers = nSpecs
	}
	if workers <= 0 {
		workers = 1
	}
	results := make([]string, nSpecs)
	backends := make([]parallel.ParallelBackend, workers)
	for i := range backends {
		backends[i].Results = &results
		backends[i].SpecIndex = -1
	}
	baseSeed := ctx.GetRunSeed()
	var next uint32
	var wg sync.WaitGroup
	prog := &r.Program
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			runBytecodeWorker(prog, nSpecs, &backends[workerID], &next, &results, workerID, baseSeed)
		}(w)
	}
	wg.Wait()
	parallel.ReportFailures(tb, results)
}

func runBytecodeWorker(prog *compiler.BCProgram, nSpecs int, backend *parallel.ParallelBackend, next *uint32, results *[]string, workerID int, baseSeed uint64) {
	c := ctx.GetFromPool()
	defer func() {
		c.Reset(nil)
		ctx.PutInPool(c)
	}()
	starts := prog.SpecStarts
	workerSeed := baseSeed ^ uint64(workerID)
	var runRangeArgs struct {
		prog  *compiler.BCProgram
		start int
		end   int
	}
	runRangeArgs.prog = prog
	var one Plan
	one.Steps = make([]Step, 1)
	one.Steps[0] = func(c *ctx.Context) {
		runRangeArgs.prog.RunRange(c, runRangeArgs.start, runRangeArgs.end)
	}
	for {
		s := atomic.AddUint32(next, 1) - 1
		if s >= uint32(nSpecs) {
			return
		}
		si := int(s)
		runRangeArgs.start, runRangeArgs.end = starts[si], starts[si+1]
		backend.SpecIndex = si
		c.Reset(backend)
		c.SetPathValues(property.PathValues{})
		c.SetSeed(workerSeed)
		RunPlan(&one, c)
		c.Reset(nil)
	}
}
