// block_runner.go compiles specs into blocks and runs them with fewer outer-loop iterations.
package runner

import (
	"testing"

	"github.com/pablogore/go-specs/specs/ctx"
)

// DefaultBlockSize is the default number of specs per block when blockSize <= 0.
const DefaultBlockSize = 8

// SpecBlock describes a contiguous range of spec funcs (start, count).
type SpecBlock struct {
	Start int
	Count int
}

// CompileBlocks compiles specs into blocks. Returns fns and block descriptors.
func CompileBlocks(specs []RunSpec, blockSize int) (fns []func(*ctx.Context), blocks []SpecBlock) {
	n := len(specs)
	if n == 0 {
		return nil, nil
	}
	if blockSize <= 0 {
		blockSize = DefaultBlockSize
	}
	fns = make([]func(*ctx.Context), n)
	for i := range specs {
		fns[i] = specs[i].Fn
	}
	numBlocks := (n + blockSize - 1) / blockSize
	blocks = make([]SpecBlock, 0, numBlocks)
	for i := 0; i < n; i += blockSize {
		count := min(blockSize, n-i)
		blocks = append(blocks, SpecBlock{Start: i, Count: count})
	}
	return fns, blocks
}

// BlockRunner runs specs via precompiled blocks.
type BlockRunner struct {
	fns    []func(*ctx.Context)
	blocks []SpecBlock
}

// NewBlockRunner creates a runner for the given blocks.
func NewBlockRunner(fns []func(*ctx.Context), blocks []SpecBlock) *BlockRunner {
	return &BlockRunner{fns: fns, blocks: blocks}
}

// Run executes all blocks in order via the shared executor.
func (r *BlockRunner) Run(tb testing.TB) {
	if r == nil || tb == nil || len(r.blocks) == 0 {
		return
	}
	suiteSeed := ctx.GetRunSeed()
	fns := r.fns
	blocks := r.blocks
	plan := &Plan{
		Steps: []Step{func(c *ctx.Context) {
			for bi := 0; bi < len(blocks); bi++ {
				b := blocks[bi]
				for i := 0; i < b.Count; i++ {
					fns[b.Start+i](c)
				}
			}
		}},
	}
	RunPlanWithTB(plan, tb, suiteSeed, nil)
}

// NumSpecs returns the total number of specs.
func (r *BlockRunner) NumSpecs() int {
	if r == nil {
		return 0
	}
	return len(r.fns)
}

// NumBlocks returns the number of blocks.
func (r *BlockRunner) NumBlocks() int {
	if r == nil {
		return 0
	}
	return len(r.blocks)
}
