package specs

import (
	"sync"
	"testing"

	"github.com/getsyntegrity/go-specs/report"
)

// Spec is the DSL handle for building describe/when/it trees.
// The node tree is compiled into an ExecutionPlan once (Compile), then Run reuses it.
type Spec struct {
	tb       testing.TB
	backend  testBackend
	reporter *report.Reporter
	seed     int64
	hasSeed  bool

	// Either (arena, rootID) when built via Analyze/registry, or plan when built via bytecode compiler.
	arena   *NodeArena
	rootID  int
	plan    *ExecutionPlan // set by top-level Describe when using bytecode compiler (no arena)
	flat    bool          // if true, run all specs in one test (no subtests)
	compileOnce sync.Once
	suite   *CompiledSuite
}

// Describe starts a top-level describe block. May be called inside Analyze(fn) or directly.
// After the callback returns, the spec tree is executed automatically (if tb is non-nil).
// When top-level (no active registry), uses bytecode compiler: no NodeArena, plan built directly.
// tb may be *testing.T or *testing.B (e.g. for scaling benchmarks).
func Describe(tb testing.TB, name string, fn func(*Spec)) {
	if currentRegistry() == nil {
		describeWithCompiler(tb, name, nil, fn, false)
		return
	}
	defer ensureRegistry()()
	file, line := callerLocation(2)
	rootID, pop := enterAnalyzeNode(DescribeNode, name, file, line, nil)
	if rootID < 0 {
		return
	}
	defer pop()
	var backend testBackend
	if tb != nil {
		backend = asTestBackend(tb)
	}
	s := &Spec{tb: tb, backend: backend, arena: CurrentArena(), rootID: rootID}
	if fn != nil {
		fn(s)
	}
	if tb != nil {
		s.Compile()
		s.Run()
	}
}

// describeWithCompiler runs Describe using the bytecode compiler (no arena).
func describeWithCompiler(tb testing.TB, name string, rep *report.Reporter, fn func(*Spec), flat bool) {
	c := newBytecodeCompiler()
	c.PushScope(name)
	pushCompiler(c)
	defer func() {
		popCompiler()
		c.PopScope()
	}()
	var backend testBackend
	if tb != nil {
		backend = asTestBackend(tb)
	}
	s := &Spec{tb: tb, backend: backend, reporter: rep, flat: flat}
	if fn != nil {
		fn(s)
	}
	s.plan = c.TakePlan()
	if tb != nil {
		s.Compile()
		s.Run()
	}
}

// BuildSuite builds the spec tree and compiles it once; returns the CompiledSuite without running.
// Use for benchmarks: build once, then call suite.Run(tb) in a loop to measure execution only.
// When top-level, uses bytecode compiler (no arena).
func BuildSuite(tb testing.TB, name string, fn func(*Spec)) *CompiledSuite {
	if currentRegistry() == nil {
		c := newBytecodeCompiler()
		c.PushScope(name)
		pushCompiler(c)
		s := &Spec{tb: tb, backend: nil}
		if fn != nil {
			fn(s)
		}
		popCompiler()
		c.PopScope()
		s.plan = c.TakePlan()
		s.Compile()
		return s.suite
	}
	defer ensureRegistry()()
	file, line := callerLocation(2)
	rootID, pop := enterAnalyzeNode(DescribeNode, name, file, line, nil)
	if rootID < 0 {
		return nil
	}
	defer pop()
	s := &Spec{tb: tb, backend: nil, arena: CurrentArena(), rootID: rootID}
	if fn != nil {
		fn(s)
	}
	s.Compile()
	return s.suite
}

// DescribeWithReporter starts a top-level describe block with a reporter.
func DescribeWithReporter(tb testing.TB, name string, rep *report.Reporter, fn func(*Spec)) {
	if currentRegistry() == nil {
		describeWithCompiler(tb, name, rep, fn, false)
		return
	}
	defer ensureRegistry()()
	file, line := callerLocation(2)
	rootID, pop := enterAnalyzeNode(DescribeNode, name, file, line, nil)
	if rootID < 0 {
		return
	}
	defer pop()
	var backend testBackend
	if tb != nil {
		backend = asTestBackend(tb)
	}
	s := &Spec{tb: tb, backend: backend, reporter: rep, arena: CurrentArena(), rootID: rootID}
	if fn != nil {
		fn(s)
	}
	if tb != nil {
		s.Compile()
		s.Run()
	}
}

// DescribeFlat runs all specs in one test (no subtests). Lower allocations than Describe.
func DescribeFlat(tb testing.TB, name string, fn func(*Spec)) {
	if currentRegistry() == nil {
		describeWithCompiler(tb, name, nil, fn, true)
		return
	}
	defer ensureRegistry()()
	file, line := callerLocation(2)
	rootID, pop := enterAnalyzeNode(DescribeNode, name, file, line, nil)
	if rootID < 0 {
		return
	}
	defer pop()
	var backend testBackend
	if tb != nil {
		backend = asTestBackend(tb)
	}
	s := &Spec{tb: tb, backend: backend, arena: CurrentArena(), rootID: rootID, flat: true}
	if fn != nil {
		fn(s)
	}
	if tb != nil {
		s.Compile()
		s.Run()
	}
}

// DescribeFlatWithReporter is like DescribeFlat with a reporter.
func DescribeFlatWithReporter(tb testing.TB, name string, rep *report.Reporter, fn func(*Spec)) {
	if currentRegistry() == nil {
		describeWithCompiler(tb, name, rep, fn, true)
		return
	}
	defer ensureRegistry()()
	file, line := callerLocation(2)
	rootID, pop := enterAnalyzeNode(DescribeNode, name, file, line, nil)
	if rootID < 0 {
		return
	}
	defer pop()
	var backend testBackend
	if tb != nil {
		backend = asTestBackend(tb)
	}
	s := &Spec{tb: tb, backend: backend, reporter: rep, arena: CurrentArena(), rootID: rootID, flat: true}
	if fn != nil {
		fn(s)
	}
	if tb != nil {
		s.Compile()
		s.Run()
	}
}

// DescribeFast runs all specs inside one test (no testing.T.Run per spec).
// Same behavior as DescribeFlat; use for maximum runner performance when subtest hierarchy is not needed.
// Avoids closure, subtest, and name allocations per spec (e.g. allocs/op ~1500–2500 vs ~8000 with Describe).
func DescribeFast(tb testing.TB, name string, fn func(*Spec)) {
	DescribeFlat(tb, name, fn)
}

// DescribeFastWithReporter is like DescribeFast with a reporter. Reporter still receives SpecStarted/SpecFinished.
func DescribeFastWithReporter(tb testing.TB, name string, rep *report.Reporter, fn func(*Spec)) {
	DescribeFlatWithReporter(tb, name, rep, fn)
}

func newSpec(tb testing.TB, withReporter bool, rep *report.Reporter) *Spec {
	var backend testBackend
	if tb != nil {
		backend = asTestBackend(tb)
	}
	s := &Spec{tb: tb, backend: backend}
	if withReporter && rep != nil {
		s.reporter = rep
	}
	return s
}

// Run runs the compiled suite. Call after Compile(); no-op if suite or tb is nil.
func (s *Spec) Run() {
	if s != nil && s.suite != nil && s.tb != nil {
		s.suite.Run(s.tb)
	}
}

// Compile builds the ExecutionPlan once. Safe to call multiple times (sync.Once).
// When s.plan is set (bytecode compiler path), uses it directly. Otherwise builds from s.arena.
func (s *Spec) Compile() {
	if s == nil {
		return
	}
	s.compileOnce.Do(func() {
		if s.plan != nil {
			s.suite = &CompiledSuite{Plan: s.plan, Arena: nil, RootID: 0}
			return
		}
		if s.arena == nil {
			return
		}
		scratch := planScratchPool.Get().(*planScratch)
		defer planScratchPool.Put(scratch)
		plan := newExecutionPlan(countSpecsArena(s.arena, s.rootID))
		buildExecutionPlanFromArena(s.arena, s.rootID, plan, scratch)
		s.suite = &CompiledSuite{Plan: plan, Arena: s.arena, RootID: s.rootID}
	})
}

// Describe starts a nested describe block.
func (s *Spec) Describe(name string, fn func(*Spec)) {
	if s == nil || fn == nil {
		return
	}
	if c := currentCompiler(); c != nil {
		c.PushScope(name)
		defer c.PopScope()
		fn(&Spec{tb: s.tb, backend: s.backend, reporter: s.reporter, seed: s.seed, hasSeed: s.hasSeed})
		return
	}
	file, line := callerLocation(2)
	_, pop := enterAnalyzeNode(DescribeNode, name, file, line, nil)
	defer pop()
	fn(&Spec{tb: s.tb, backend: s.backend, reporter: s.reporter, seed: s.seed, hasSeed: s.hasSeed})
}

// When starts a when block. fn may be func(*Spec) or func() for legacy scope.
func (s *Spec) When(name string, fn interface{}) {
	if s == nil || fn == nil {
		return
	}
	if c := currentCompiler(); c != nil {
		c.PushScope(name)
		defer c.PopScope()
		switch f := fn.(type) {
		case func(*Spec):
			f(&Spec{tb: s.tb, backend: s.backend, reporter: s.reporter, seed: s.seed, hasSeed: s.hasSeed})
		case func():
			f()
		}
		return
	}
	file, line := callerLocation(2)
	_, pop := enterAnalyzeNode(WhenNode, name, file, line, nil)
	defer pop()
	switch f := fn.(type) {
	case func(*Spec):
		f(&Spec{tb: s.tb, backend: s.backend, reporter: s.reporter, seed: s.seed, hasSeed: s.hasSeed})
	case func():
		f()
	}
}

// It registers a spec.
func (s *Spec) It(name string, fn func(*Context)) {
	if s == nil {
		return
	}
	if c := currentCompiler(); c != nil {
		c.EmitIt(name, fn)
		return
	}
	file, line := callerLocation(2)
	_, pop := enterAnalyzeNode(ItNode, name, file, line, fn)
	pop()
}

// BeforeEach appends a before-each hook to the current node.
func (s *Spec) BeforeEach(fn func(*Context)) {
	if s == nil || fn == nil {
		return
	}
	if c := currentCompiler(); c != nil {
		c.AppendBefore(fn)
		return
	}
	AppendBeforeHook(fn)
}

// AfterEach appends an after-each hook to the current node.
func (s *Spec) AfterEach(fn func(*Context)) {
	if s == nil || fn == nil {
		return
	}
	if c := currentCompiler(); c != nil {
		c.AppendAfter(fn)
		return
	}
	AppendAfterHook(fn)
}

// RandomSeed sets the RNG seed for path/context in this spec subtree.
func (s *Spec) RandomSeed(seed int64) {
	if s != nil {
		s.seed = seed
		s.hasSeed = true
	}
}

func (s *Spec) runPathWithContext(name string, gen *PathGenerator, _ interface{}, fn func(*Context)) {
	if s == nil || fn == nil {
		return
	}
	if c := currentCompiler(); c != nil {
		c.SetPathGen(gen)
		c.EmitIt(name, fn)
		return
	}
	file, line := callerLocation(2)
	_, pop := enterAnalyzeNode(ItNode, name, file, line, fn)
	SetPathGen(gen)
	pop()
}

// parseItArgs extracts the optional last func(*Context) from args. Returns (nil, fn).
func parseItArgs(args []any) (ops interface{}, fn func(*Context)) {
	if len(args) == 0 {
		return nil, nil
	}
	if f, ok := args[len(args)-1].(func(*Context)); ok {
		return nil, f
	}
	return nil, nil
}
