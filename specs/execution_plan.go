// execution_plan.go defines the bytecode ExecutionPlan and CompiledSuite used by the compiler and runner.
package specs

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablogore/go-specs/report"
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
	s.run(tb, nil)
}

func (s *CompiledSuite) run(tb testing.TB, runCtx context.Context) []proposalControllerResult {
	if s == nil || s.Plan == nil || tb == nil || len(s.Plan.ProgramStart) == 0 {
		return nil
	}
	if runCtx == nil {
		var cancel context.CancelFunc
		runCtx, cancel = executionContext(tb)
		defer cancel()
	}
	backend := asTestBackend(tb)
	defer putTestBackend(backend)
	rep := report.New(io.Discard)
	return runPlanFlatNoSubtests(runCtx, backend, rep, s.Plan)
}

func executionContext(tb testing.TB) (context.Context, context.CancelFunc) {
	ctx := context.Background()
	if contextual, ok := tb.(interface{ Context() context.Context }); ok && contextual.Context() != nil {
		ctx = contextual.Context()
	}
	if timed, ok := tb.(interface{ Deadline() (time.Time, bool) }); ok {
		if deadline, ok := timed.Deadline(); ok {
			return context.WithDeadline(ctx, deadline)
		}
	}
	return context.WithCancel(ctx)
}

func runPlanFlatNoSubtests(runCtx context.Context, backend testBackend, rep *report.Reporter, plan *ExecutionPlan) []proposalControllerResult {
	results := make([]proposalControllerResult, 0, len(plan.ProgramStart))
	for i := 0; i < len(plan.ProgramStart); i++ {
		results = append(results, runExecutionContext(runCtx, backend, rep, plan, i))
	}
	return results
}

func runExecution(backend testBackend, rep *report.Reporter, plan *ExecutionPlan, i int) proposalControllerResult {
	return runExecutionContext(context.Background(), backend, rep, plan, i)
}

func runExecutionContext(runCtx context.Context, backend testBackend, rep *report.Reporter, plan *ExecutionPlan, i int) proposalControllerResult {
	start := plan.ProgramStart[i]
	length := plan.ProgramLen[i]
	if start+length > len(plan.Instructions) {
		return proposalControllerResult{}
	}
	program := plan.Instructions[start : start+length]
	if i < len(plan.PathGens) && plan.PathGens[i] != nil && plan.PathGens[i].mode == CartesianMode {
		gen := plan.PathGens[i]
		seq := gen.sequence()
		maxAttempts, maxAccepted, maxRejections := gen.bounds()
		return newProposalController(proposalControllerConfig{
			MaxAttempts:   maxAttempts,
			MaxAccepted:   maxAccepted,
			MaxRejections: maxRejections,
			Propose:       seq.next,
			Execute: func(candidate proposalCandidate) bool {
				return !runIsolatedCase(backend, program, candidate.Values).Failed
			},
			AdmitFeedback: func(feedback proposalFeedback) {
				seq.admitFeedback(feedback.Candidate.Values, feedback.Passed)
			},
		}).Run(runCtx)
	}
	if i < len(plan.PathGens) && plan.PathGens[i] != nil {
		// Sample and Explore/ExploreCoverage/ExploreSmart still materialize via ForEach until a
		// later change extends sequence()/bounds() to those strategies (see path_generator.go).
		paths := make([]PathValues, 0)
		plan.PathGens[i].ForEach(func(path PathValues) { paths = append(paths, path.clone()) })
		next := 0
		result := newProposalController(proposalControllerConfig{
			MaxAttempts:   len(paths),
			MaxAccepted:   len(paths),
			MaxRejections: len(paths),
			Propose: func() (PathValues, bool) {
				if next == len(paths) {
					return PathValues{}, false
				}
				path := paths[next]
				next++
				return path, true
			},
			Execute: func(candidate proposalCandidate) bool {
				return !runIsolatedCase(backend, program, candidate.Values).Failed
			},
		}).Run(runCtx)
		return result
	}
	ctx := contextPool.Get().(*Context)
	defer func() {
		ctx.Reset(nil)
		contextPool.Put(ctx)
	}()
	ctx.Reset(backend)
	ctx.SetPathValues(PathValues{})
	runProgram(program, ctx, nil)
	return proposalControllerResult{}
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

// isolatedCaseAbort lets a controlled backend stop one isolated case without
// terminating the parent test.
type isolatedCaseAbort struct{}

type isolatedCaseResult struct {
	Failed       bool
	Panic        any
	Path         PathValues
	ContextReset bool
}

// runIsolatedCase executes real generated cases in a subtest so Fatal and FailNow
// terminate only that case while preserving the parent test's failure semantics.
func runIsolatedCase(backend testBackend, program []Instruction, path PathValues) (result isolatedCaseResult) {
	if real, ok := backend.(*runnableBackend); ok {
		real.Run("generated", func(tb testing.TB) {
			caseBackend := asTestBackend(tb)
			defer putTestBackend(caseBackend)
			defer func() { result.Failed = result.Failed || tb.Failed() }()
			result = runIsolatedCaseDirect(caseBackend, program, path)
		})
		return result
	}
	return runIsolatedCaseDirect(backend, program, path)
}

func runIsolatedCaseDirect(backend testBackend, program []Instruction, path PathValues) (result isolatedCaseResult) {
	ctx := contextPool.Get().(*Context)
	ctx.Reset(backend)
	ctx.SetPathValues(path)
	result.Path = ctx.Path().clone()

	var after []Instruction
	for _, inst := range program {
		if inst.Code == OpAfterHook && inst.Fn != nil {
			after = append(after, inst)
		}
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			if _, aborted := recovered.(isolatedCaseAbort); !aborted {
				result.Panic = recovered
			}
			result.Failed = true
		}
		for _, inst := range after {
			func() {
				defer func() {
					if recovered := recover(); recovered != nil && result.Panic == nil {
						result.Panic = recovered
						result.Failed = true
					}
				}()
				inst.Fn(ctx)
			}()
		}
		result.Failed = result.Failed || ctx.failed
		ctx.Reset(nil)
		result.ContextReset = true
		contextPool.Put(ctx)
	}()

	for _, inst := range program {
		if inst.Code == OpAfterHook {
			continue
		}
		if inst.Fn != nil {
			inst.Fn(ctx)
		}
	}
	return result
}
