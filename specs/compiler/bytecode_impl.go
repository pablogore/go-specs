package compiler

import "github.com/pablogore/go-specs/specs/ctx"

// bcInstruction is a single bytecode step: run fn(ctx). Used in BCProgram/BCBuilder.
type bcInstruction struct {
	fn func(*ctx.Context)
}

// BCProgram is a flat bytecode program: Code is the instruction stream, SpecStarts[i] is the start
// index of spec i (SpecStarts[NumSpecs()] == len(Code)).
type BCProgram struct {
	Code       []bcInstruction
	SpecStarts []int
}

// BCLen returns the number of instructions.
func (p BCProgram) BCLen() int {
	return len(p.Code)
}

// NumSpecs returns the number of specs (len(SpecStarts)-1).
func (p BCProgram) NumSpecs() int {
	if len(p.SpecStarts) < 2 {
		return 0
	}
	return len(p.SpecStarts) - 1
}

// BCBuilder builds a BCProgram from AddBefore/AddAfter/AddSpec.
type BCBuilder struct {
	before []func(*ctx.Context)
	after  []func(*ctx.Context)
	code   []bcInstruction
	starts []int
}

// NewBCBuilder creates a bytecode builder with optional capacity hint.
func NewBCBuilder(capacity int) *BCBuilder {
	if capacity <= 0 {
		capacity = 64
	}
	return &BCBuilder{
		before: nil,
		after:  nil,
		code:   make([]bcInstruction, 0, capacity*4),
		starts: make([]int, 0, capacity+1),
	}
}

// AddBefore adds a before-each hook.
func (b *BCBuilder) AddBefore(fn func(*ctx.Context)) {
	if fn == nil {
		return
	}
	b.before = append(b.before, fn)
}

// AddAfter adds an after-each hook (run in LIFO order per spec).
func (b *BCBuilder) AddAfter(fn func(*ctx.Context)) {
	if fn == nil {
		return
	}
	b.after = append(b.after, fn)
}

// AddSpec adds one spec: emits before hooks, then spec body, then after hooks (reverse).
func (b *BCBuilder) AddSpec(fn func(*ctx.Context)) {
	if fn == nil {
		return
	}
	b.starts = append(b.starts, len(b.code))
	for _, h := range b.before {
		b.code = append(b.code, bcInstruction{fn: h})
	}
	b.code = append(b.code, bcInstruction{fn: fn})
	for i := len(b.after) - 1; i >= 0; i-- {
		b.code = append(b.code, bcInstruction{fn: b.after[i]})
	}
}

// BuildBC returns the compiled bytecode program.
func (b *BCBuilder) BuildBC() BCProgram {
	b.starts = append(b.starts, len(b.code))
	return BCProgram{
		Code:       b.code,
		SpecStarts: b.starts,
	}
}

// RunAll runs all instructions in order with the given context.
func (p BCProgram) RunAll(c *ctx.Context) {
	for i := 0; i < len(p.Code); i++ {
		if p.Code[i].fn != nil {
			p.Code[i].fn(c)
		}
	}
}

// RunRange runs instructions [start, end) with the given context.
func (p BCProgram) RunRange(c *ctx.Context, start, end int) {
	for i := start; i < end; i++ {
		if p.Code[i].fn != nil {
			p.Code[i].fn(c)
		}
	}
}

// ShardBCProgram returns a BCProgram containing only specs whose index s satisfies s%total == shard.
func ShardBCProgram(prog BCProgram, shard, total int) BCProgram {
	if total <= 0 || shard < 0 || shard >= total {
		return prog
	}
	nSpecs := prog.NumSpecs()
	if nSpecs == 0 {
		return prog
	}
	code := prog.Code
	starts := prog.SpecStarts
	var newCode []bcInstruction
	newStarts := make([]int, 0, nSpecs/total+2)
	newStarts = append(newStarts, 0)
	for s := 0; s < nSpecs; s++ {
		if s%total != shard {
			continue
		}
		start, end := starts[s], starts[s+1]
		for j := start; j < end; j++ {
			newCode = append(newCode, code[j])
		}
		newStarts = append(newStarts, len(newCode))
	}
	if len(newStarts) == 1 {
		return BCProgram{Code: nil, SpecStarts: nil}
	}
	return BCProgram{Code: newCode, SpecStarts: newStarts}
}
