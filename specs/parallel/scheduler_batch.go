// scheduler_batch.go implements batched work distribution for the parallel scheduler.
package parallel

import (
	"sync/atomic"

	"github.com/pablogore/go-specs/specs/ctx"
	"github.com/pablogore/go-specs/specs/property"
)

// DefaultChunkSize is the default batch size for RunParallelBatched when chunkSize <= 0.
const DefaultChunkSize = 16

// RunWorkerBatched runs specs in chunks; each iteration claims chunkSize indexes at a time.
func RunWorkerBatched(specs []Spec, backend *ParallelBackend, next *uint32, results *[]string, chunkSize uint32) {
	c := ctx.GetFromPool()
	defer func() {
		c.Reset(nil)
		ctx.PutInPool(c)
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
		// Reset per-spec so c.failed from one spec never bleeds into the next.
		for i := start; i < end; i++ {
			idx := int(i)
			backend.SpecIndex = idx
			c.Reset(backend)
			c.SetPathValues(property.PathValues{})
			specs[idx].Fn(c)
			c.Reset(nil)
		}
	}
}
