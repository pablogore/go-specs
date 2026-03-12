// execution_plan.go defines the bytecode ExecutionPlan and CompiledSuite (data only). Execution is in specs/runner.
package compiler

import (
	"strings"
	"sync"

	"github.com/pablogore/go-specs/report"
	"github.com/pablogore/go-specs/specs/ctx"
	"github.com/pablogore/go-specs/specs/property"
)

// ExecutionPlan holds the flat instruction stream and per-spec metadata for the runner.
type ExecutionPlan struct {
	Instructions []Instruction
	ProgramStart []int
	ProgramLen   []int
	Names        []string
	FullNames    []string
	PathGens     []*property.PathGenerator
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
		PathGens:     make([]*property.PathGenerator, 0, estimatedSpecs),
	}
}

type planScratch struct {
	beforeFlat []func(*ctx.Context)
	afterFlat  []func(*ctx.Context)
	program    []Instruction
	path       []string
}

var planScratchPool = sync.Pool{
	New: func() any {
		return &planScratch{
			beforeFlat: make([]func(*ctx.Context), 0, 32),
			afterFlat:  make([]func(*ctx.Context), 0, 32),
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

// BuildPlanFromArena builds an ExecutionPlan from an arena (registry/Analyze path). Exported for root specs.
func BuildPlanFromArena(arena *NodeArena, rootID int) *ExecutionPlan {
	if arena == nil || rootID < 0 {
		return nil
	}
	plan := newExecutionPlan(countSpecsArena(arena, rootID))
	scratch := planScratchPool.Get().(*planScratch)
	defer planScratchPool.Put(scratch)
	buildExecutionPlanFromArena(arena, rootID, plan, scratch)
	return plan
}

// CompiledSuite holds the compiled plan and optional arena reference (data only).
// Execute with runner.RunCompiledSuite(suite, tb). If Reporter is nil, the runner uses report.New(io.Discard).
// If HasSeed is true, Seed is used for Context.SetSeed (deterministic RNG); otherwise ctx.GetRunSeed() is used.
type CompiledSuite struct {
	Plan     *ExecutionPlan
	Arena    *NodeArena
	RootID   int
	Reporter report.EventReporter
	Seed     uint64
	HasSeed  bool
}
