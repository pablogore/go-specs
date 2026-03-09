// execution_plan.go defines the bytecode ExecutionPlan and CompiledSuite used by the compiler and runner.
package specs

import (
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/getsyntegrity/go-specs/report"
)

// ExecutionPlan holds the flat instruction stream and per-spec metadata for the runner.
type ExecutionPlan struct {
	Instructions []Instruction
	ProgramStart []int
	ProgramLen   []int
	Names        []string
	FullNames    []string
	PathGens     []*PathGenerator
}

func newExecutionPlan(estimatedSpecs int) *ExecutionPlan {
	if estimatedSpecs <= 0 {
		estimatedSpecs = 64
	}
	return &ExecutionPlan{
		Instructions: make([]Instruction, 0, estimatedSpecs*8),
		ProgramStart: make([]int, 0, estimatedSpecs),
		ProgramLen:   make([]int, 0, estimatedSpecs),
		Names:        make([]string, 0, estimatedSpecs),
		FullNames:    make([]string, 0, estimatedSpecs),
		PathGens:     make([]*PathGenerator, 0, estimatedSpecs),
	}
}

type planScratch struct {
	beforeFlat []func(*Context)
	afterFlat  []func(*Context)
	program    []Instruction
	path       []string
}

var planScratchPool = sync.Pool{
	New: func() any {
		return &planScratch{
			beforeFlat: make([]func(*Context), 0, 32),
			afterFlat:  make([]func(*Context), 0, 32),
			program:    make([]Instruction, 0, 32),
			path:       make([]string, 0, 8),
		}
	},
}

func countSpecsArena(arena *NodeArena, rootID int) int {
	if arena == nil || rootID < 0 || rootID >= len(arena.Nodes) {
		return 0
	}
	n := 0
	if arena.Nodes[rootID].Type == ItNode {
		n = 1
	}
	for _, cid := range arena.Children[rootID] {
		n += countSpecsArena(arena, cid)
	}
	return n
}

func buildExecutionPlanFromArena(arena *NodeArena, rootID int, plan *ExecutionPlan, scratch *planScratch) {
	if arena == nil || plan == nil || scratch == nil {
		return
	}
	scratch.path = scratch.path[:0]
	buildExecutionPlanFromArenaRec(arena, rootID, plan, scratch)
}

func buildExecutionPlanFromArenaRec(arena *NodeArena, nodeID int, plan *ExecutionPlan, scratch *planScratch) {
	if arena == nil || nodeID < 0 || nodeID >= len(arena.Nodes) {
		return
	}
	node := &arena.Nodes[nodeID]
	name := node.Name
	if name != "" && node.Type != SuiteNode {
		scratch.path = append(scratch.path, name)
	}
	if node.Type == ItNode {
		scratch.beforeFlat = scratch.beforeFlat[:0]
		scratch.afterFlat = scratch.afterFlat[:0]
		ancestorIDs := collectAncestorIDs(arena, node.Parent)
		for _, id := range ancestorIDs {
			scratch.beforeFlat = append(scratch.beforeFlat, arena.BeforeHooks[id]...)
			scratch.afterFlat = append(scratch.afterFlat, arena.AfterHooks[id]...)
		}
		scratch.beforeFlat = append(scratch.beforeFlat, arena.BeforeHooks[nodeID]...)
		scratch.afterFlat = append(scratch.afterFlat, arena.AfterHooks[nodeID]...)
		scratch.program = scratch.program[:0]
		if node.PathGen != nil {
			scratch.program = append(scratch.program, Instruction{Code: OpSetPath, Fn: nil})
		}
		for _, h := range scratch.beforeFlat {
			if h != nil {
				scratch.program = append(scratch.program, Instruction{Code: OpBeforeHook, Fn: h})
			}
		}
		if node.Fn != nil {
			scratch.program = append(scratch.program, Instruction{Code: OpBody, Fn: node.Fn})
		}
		for i := len(scratch.afterFlat) - 1; i >= 0; i-- {
			if h := scratch.afterFlat[i]; h != nil {
				scratch.program = append(scratch.program, Instruction{Code: OpAfterHook, Fn: h})
			}
		}
		start := len(plan.Instructions)
		plan.Instructions = append(plan.Instructions, scratch.program...)
		plan.ProgramStart = append(plan.ProgramStart, start)
		plan.ProgramLen = append(plan.ProgramLen, len(scratch.program))
		plan.Names = append(plan.Names, name)
		plan.FullNames = append(plan.FullNames, strings.Join(scratch.path, "/"))
		plan.PathGens = append(plan.PathGens, node.PathGen)
	}
	for _, cid := range arena.Children[nodeID] {
		buildExecutionPlanFromArenaRec(arena, cid, plan, scratch)
	}
	if name != "" && node.Type != SuiteNode && len(scratch.path) > 0 {
		scratch.path = scratch.path[:len(scratch.path)-1]
	}
}

// collectAncestorIDs returns ancestor IDs from root to the given node (inclusive), so that hooks are in declaration order.
func collectAncestorIDs(arena *NodeArena, nodeID int) []int {
	if arena == nil || nodeID < 0 {
		return nil
	}
	var ids []int
	for id := nodeID; id >= 0 && id < len(arena.Nodes); id = arena.Nodes[id].Parent {
		ids = append(ids, id)
	}
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
	return ids
}

// CompiledSuite holds the compiled plan and optional arena reference. Run executes the plan.
type CompiledSuite struct {
	Plan   *ExecutionPlan
	Arena  *NodeArena
	RootID int
}

// Run executes all specs in the plan. Uses one context from the pool per spec (or per path iteration).
func (s *CompiledSuite) Run(tb testing.TB) {
	if s == nil || s.Plan == nil || tb == nil || len(s.Plan.ProgramStart) == 0 {
		return
	}
	backend := asTestBackend(tb)
	defer putTestBackend(backend)
	rep := report.New(io.Discard)
	runPlanFlatNoSubtests(backend, rep, s.Plan)
}

func runPlanFlatNoSubtests(backend testBackend, rep *report.Reporter, plan *ExecutionPlan) {
	for i := 0; i < len(plan.ProgramStart); i++ {
		runExecution(backend, rep, plan, i)
	}
}

func runExecution(backend testBackend, rep *report.Reporter, plan *ExecutionPlan, i int) {
	start := plan.ProgramStart[i]
	length := plan.ProgramLen[i]
	if start+length > len(plan.Instructions) {
		return
	}
	program := plan.Instructions[start : start+length]
	ctx := contextPool.Get().(*Context)
	defer func() {
		ctx.Reset(nil)
		contextPool.Put(ctx)
	}()
	ctx.Reset(backend)
	ctx.SetPathValues(PathValues{})
	runProgram(program, ctx, nil)
}

func runProgram(program []Instruction, ctx *Context, path *PathValues) {
	if path != nil {
		ctx.SetPathValues(*path)
	}
	for _, inst := range program {
		if inst.Fn != nil {
			inst.Fn(ctx)
		}
	}
}
