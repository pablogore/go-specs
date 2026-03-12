package compiler

import (
	"strings"
	"sync"

	"github.com/pablogore/go-specs/specs/ctx"
	"github.com/pablogore/go-specs/specs/property"
)

// BytecodeCompiler emits instructions directly into an ExecutionPlan during Describe.
type BytecodeCompiler struct {
	plan       *ExecutionPlan
	nameStack  []string
	beforeStack [][]func(*ctx.Context)
	afterStack  [][]func(*ctx.Context)
	pathGen    *property.PathGenerator
	beforeFlat []func(*ctx.Context)
	afterFlat  []func(*ctx.Context)
	program    []Instruction
}

const compilerInitialSpecCap = 64
const compilerInitialInstructionsCap = 512

var bytecodeCompilerPool = sync.Pool{
	New: func() any {
		return &BytecodeCompiler{
			nameStack:   make([]string, 0, 16),
			beforeStack: make([][]func(*ctx.Context), 0, 8),
			afterStack:  make([][]func(*ctx.Context), 0, 8),
			beforeFlat:  make([]func(*ctx.Context), 0, 32),
			afterFlat:   make([]func(*ctx.Context), 0, 32),
			program:     make([]Instruction, 0, 32),
		}
	},
}

// NewBytecodeCompiler returns a new bytecode compiler. Call PushCompiler when entering Describe; PopCompiler when done.
func NewBytecodeCompiler() *BytecodeCompiler {
	c := bytecodeCompilerPool.Get().(*BytecodeCompiler)
	c.plan = newExecutionPlan(compilerInitialSpecCap)
	c.plan.Instructions = make([]Instruction, 0, compilerInitialInstructionsCap)
	c.nameStack = c.nameStack[:0]
	c.beforeStack = c.beforeStack[:0]
	c.afterStack = c.afterStack[:0]
	c.pathGen = nil
	return c
}

func (c *BytecodeCompiler) reset() {
	c.plan = nil
	c.nameStack = c.nameStack[:0]
	c.beforeStack = c.beforeStack[:0]
	c.afterStack = c.afterStack[:0]
	c.pathGen = nil
	bytecodeCompilerPool.Put(c)
}

// PushScope enters a Describe/When block.
func (c *BytecodeCompiler) PushScope(name string) {
	c.nameStack = append(c.nameStack, name)
	c.beforeStack = append(c.beforeStack, nil)
	c.afterStack = append(c.afterStack, nil)
}

// PopScope exits the current Describe/When block.
func (c *BytecodeCompiler) PopScope() {
	if n := len(c.nameStack); n > 0 {
		c.nameStack = c.nameStack[:n-1]
	}
	if n := len(c.beforeStack); n > 0 {
		c.beforeStack = c.beforeStack[:n-1]
	}
	if n := len(c.afterStack); n > 0 {
		c.afterStack = c.afterStack[:n-1]
	}
}

// AppendBefore adds a before-each hook to the current scope.
func (c *BytecodeCompiler) AppendBefore(fn func(*ctx.Context)) {
	if fn == nil {
		return
	}
	if len(c.beforeStack) == 0 {
		return
	}
	i := len(c.beforeStack) - 1
	c.beforeStack[i] = append(c.beforeStack[i], fn)
}

// AppendAfter adds an after-each hook to the current scope.
func (c *BytecodeCompiler) AppendAfter(fn func(*ctx.Context)) {
	if fn == nil {
		return
	}
	if len(c.afterStack) == 0 {
		return
	}
	i := len(c.afterStack) - 1
	c.afterStack[i] = append(c.afterStack[i], fn)
}

// SetPathGen sets the path generator for the next EmitIt.
func (c *BytecodeCompiler) SetPathGen(gen *property.PathGenerator) {
	c.pathGen = gen
}

func (c *BytecodeCompiler) fullName(itName string) string {
	if len(c.nameStack) == 0 {
		return itName
	}
	return strings.Join(c.nameStack, "/") + "/" + itName
}

func (c *BytecodeCompiler) flattenHooks() {
	c.beforeFlat = c.beforeFlat[:0]
	c.afterFlat = c.afterFlat[:0]
	for _, b := range c.beforeStack {
		c.beforeFlat = append(c.beforeFlat, b...)
	}
	for _, a := range c.afterStack {
		c.afterFlat = append(c.afterFlat, a...)
	}
}

// EmitIt appends one spec program.
func (c *BytecodeCompiler) EmitIt(name string, body func(*ctx.Context)) {
	c.flattenHooks()
	c.program = c.program[:0]
	if c.pathGen != nil {
		c.program = append(c.program, Instruction{Code: OpSetPath, Fn: nil})
	}
	for _, h := range c.beforeFlat {
		if h != nil {
			c.program = append(c.program, Instruction{Code: OpBeforeHook, Fn: h})
		}
	}
	if body != nil {
		c.program = append(c.program, Instruction{Code: OpBody, Fn: body})
	}
	for i := len(c.afterFlat) - 1; i >= 0; i-- {
		if h := c.afterFlat[i]; h != nil {
			c.program = append(c.program, Instruction{Code: OpAfterHook, Fn: h})
		}
	}
	start := len(c.plan.Instructions)
	c.plan.Instructions = append(c.plan.Instructions, c.program...)
	c.plan.ProgramStart = append(c.plan.ProgramStart, start)
	c.plan.ProgramLen = append(c.plan.ProgramLen, len(c.program))
	c.plan.Names = append(c.plan.Names, name)
	c.plan.FullNames = append(c.plan.FullNames, c.fullName(name))
	c.plan.PathGens = append(c.plan.PathGens, c.pathGen)
	c.pathGen = nil
}

// TakePlan returns the built ExecutionPlan and resets the compiler.
func (c *BytecodeCompiler) TakePlan() *ExecutionPlan {
	plan := c.plan
	c.reset()
	return plan
}

var activeCompiler struct {
	mu    sync.Mutex
	stack []*BytecodeCompiler
}

// CurrentCompiler returns the active bytecode compiler, or nil if not in compiler mode.
func CurrentCompiler() *BytecodeCompiler {
	activeCompiler.mu.Lock()
	defer activeCompiler.mu.Unlock()
	if len(activeCompiler.stack) == 0 {
		return nil
	}
	return activeCompiler.stack[len(activeCompiler.stack)-1]
}

// PushCompiler pushes the compiler onto the active stack.
func PushCompiler(c *BytecodeCompiler) {
	activeCompiler.mu.Lock()
	activeCompiler.stack = append(activeCompiler.stack, c)
	activeCompiler.mu.Unlock()
}

// PopCompiler pops the active compiler stack.
func PopCompiler() {
	activeCompiler.mu.Lock()
	if len(activeCompiler.stack) > 0 {
		activeCompiler.stack = activeCompiler.stack[:len(activeCompiler.stack)-1]
	}
	activeCompiler.mu.Unlock()
}
