// runner_bytecode.go runs a compiled bytecode program with a single loop (Run) or worker pool (RunParallel).
//
// No tree traversal: the program is a flat []instruction. Run uses one context from the pool;
// RunParallel uses spec boundaries (SpecStarts) so each worker runs a contiguous instruction range per spec.
// Hot path remains allocation-free; no reflection, no interface dispatch in the loop.
package specs

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// BytecodeRunner runs a BCProgram built by BCBuilder.
type BytecodeRunner struct {
	program BCProgram
}

// NewBytecodeRunner creates a runner for the given bytecode program. Program is used as-is (not copied).
func NewBytecodeRunner(p BCProgram) *BytecodeRunner {
	return &BytecodeRunner{program: p}
}

// Run executes all instructions in order. One context from the pool, reused for every instruction.
// No allocations in the loop; bounds-check elimination applies to the index loop.
func (r *BytecodeRunner) Run(tb testing.TB) {
	if r == nil || tb == nil || r.program.BCLen() == 0 {
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

	code := r.program.Code
	for i := 0; i < len(code); i++ {
		if code[i].fn != nil {
			code[i].fn(ctx)
		}
	}
}

// RunParallel runs each spec (instruction range) on a worker pool. Workers pull spec indexes via
// atomic counter; each worker reuses one Context. Failures are recorded by spec index and reported
// in order (deterministic). No allocations in the worker loop.
func (r *BytecodeRunner) RunParallel(tb failureReporter, workers int) {
	if r == nil || tb == nil || r.program.BCLen() == 0 {
		return
	}
	nSpecs := r.program.NumSpecs()
	if nSpecs == 0 {
		return
	}
	code := r.program.Code
	starts := r.program.SpecStarts
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
	backends := make([]parallelBackend, workers)
	for i := range backends {
		backends[i].results = &results
		backends[i].specIndex = -1
	}

	var next uint32
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			runBytecodeWorker(code, starts, nSpecs, &backends[workerID], &next, &results)
		}(w)
	}
	wg.Wait()

	reportFailures(tb, results)
}

// runBytecodeWorker runs spec ranges whose spec index it acquires via next. One context per worker.
// No allocations in the loop: context from pool, backend preallocated, code/starts read-only.
func runBytecodeWorker(code []instruction, starts []int, nSpecs int, backend *parallelBackend, next *uint32, results *[]string) {
	ctx := contextPool.Get().(*Context)
	defer func() {
		ctx.Reset(nil)
		contextPool.Put(ctx)
	}()

	for {
		s := atomic.AddUint32(next, 1) - 1
		if s >= uint32(nSpecs) {
			return
		}
		si := int(s)
		start := starts[si]
		end := starts[si+1]
		backend.specIndex = si
		ctx.Reset(backend)
		ctx.SetPathValues(PathValues{})
		for i := start; i < end; i++ {
			if code[i].fn != nil {
				code[i].fn(ctx)
			}
		}
		ctx.Reset(nil)
	}
}
