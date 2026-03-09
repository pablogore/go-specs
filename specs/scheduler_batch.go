// scheduler_batch.go implements cache-aware batched work distribution for the parallel runner.
//
// Workers claim chunks of spec indexes (e.g. 16 at a time) instead of one, reducing atomic
// contention on the shared counter. No allocations in the worker loop; deterministic
// reporting is unchanged (results by spec index, reported in order).
package specs

import (
	"sync/atomic"
)

// DefaultChunkSize is the default batch size when using RunParallelBatched with chunkSize <= 0.
// Tuned to reduce atomic contention while keeping chunks cache-friendly (multiple RunSpecs
// in one or two cache lines).
const DefaultChunkSize = 16

// runWorkerBatched runs specs in chunks. Each iteration claims [start, end) via
// atomic.AddUint32(next, chunkSize); then runs specs[start:end] with one context Reset per chunk
// (not per spec), reducing backend setup overhead. No allocations in the loop.
func runWorkerBatched(specs []RunSpec, backend *parallelBackend, next *uint32, results *[]string, chunkSize uint32) {
	ctx := contextPool.Get().(*Context)
	defer func() {
		ctx.Reset(nil)
		contextPool.Put(ctx)
	}()

	n := uint32(len(specs))
	if chunkSize == 0 {
		chunkSize = 1
	}
	for {
		start := atomic.AddUint32(next, chunkSize) - chunkSize
		if start >= n {
			return
		}
		end := start + chunkSize
		if end > n {
			end = n
		}
		// One Reset per chunk; reuse context for all specs in the chunk.
		ctx.Reset(backend)
		ctx.SetPathValues(PathValues{})
		for i := start; i < end; i++ {
			idx := int(i)
			backend.specIndex = idx
			specs[idx].Fn(ctx)
		}
		ctx.Reset(nil)
	}
}
