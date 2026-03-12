package dsl

import (
	"sync"
	"testing"

	"github.com/pablogore/go-specs/report"
	"github.com/pablogore/go-specs/specs/compiler"
	"github.com/pablogore/go-specs/specs/ctx"
	"github.com/pablogore/go-specs/specs/property"
	"github.com/pablogore/go-specs/specs/runner"
)

// NewSpecForTest creates a Spec for use in tests (e.g. RNG determinism). Not for production Describe flows.
func NewSpecForTest(tb testing.TB, withReporter bool, rep report.EventReporter) *Spec {
	var backend ctx.TestBackend
	if tb != nil {
		backend = ctx.AsTestBackend(tb)
	}
	s := &Spec{tb: tb, backend: backend}
	if withReporter && rep != nil {
		s.reporter = rep
	}
	return s
}

// Spec is the DSL handle for building describe/when/it trees.
type Spec struct {
	tb       testing.TB
	backend  ctx.TestBackend
	reporter report.EventReporter
	seed     uint64
	hasSeed  bool

	// bcompiler is non-nil for the bytecode-compilation path (normal Describe / BuildSuite).
	// Storing it on the Spec instance instead of a global stack eliminates the logical race
	// condition that occurred when two goroutines built suites concurrently: each Spec now
	// carries its own compiler reference and nested DSL calls use it directly.
	bcompiler   *compiler.BytecodeCompiler
	arena       *compiler.NodeArena
	rootID      int
	plan        *compiler.ExecutionPlan
	flat        bool
	compileOnce sync.Once
	suite       *compiler.CompiledSuite
}

// childSpec returns a new Spec that inherits the current Spec's metadata and bcompiler.
// Used by Describe/When to propagate the compiler reference into nested scopes.
func (s *Spec) childSpec() *Spec {
	return &Spec{
		tb:        s.tb,
		backend:   s.backend,
		reporter:  s.reporter,
		seed:      s.seed,
		hasSeed:   s.hasSeed,
		bcompiler: s.bcompiler,
	}
}

// Describe starts a top-level describe block.
func Describe(tb testing.TB, name string, fn func(*Spec)) {
	if currentRegistry() == nil {
		describeWithCompiler(tb, name, nil, fn, false)
		return
	}
	defer ensureRegistry()()
	file, line := callerLocation(2)
	rootID, pop := enterAnalyzeNode(compiler.DescribeNode, name, file, line, nil)
	if rootID < 0 {
		return
	}
	defer pop()
	var backend ctx.TestBackend
	if tb != nil {
		backend = ctx.AsTestBackend(tb)
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

// describeWithCompiler is the primary execution path. The BytecodeCompiler is stored
// on the Spec and propagated to all nested DSL calls via childSpec(); the global
// activeCompiler stack is never touched, eliminating the concurrent-suite-construction race.
func describeWithCompiler(tb testing.TB, name string, rep report.EventReporter, fn func(*Spec), flat bool) {
	c := compiler.NewBytecodeCompiler()
	c.PushScope(name)
	defer c.PopScope()
	var backend ctx.TestBackend
	if tb != nil {
		backend = ctx.AsTestBackend(tb)
	}
	s := &Spec{tb: tb, backend: backend, reporter: rep, flat: flat, bcompiler: c}
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
func BuildSuite(tb testing.TB, name string, fn func(*Spec)) *compiler.CompiledSuite {
	if currentRegistry() == nil {
		c := compiler.NewBytecodeCompiler()
		c.PushScope(name)
		s := &Spec{tb: tb, backend: nil, bcompiler: c}
		if fn != nil {
			fn(s)
		}
		c.PopScope()
		s.plan = c.TakePlan()
		s.Compile()
		return s.suite
	}
	defer ensureRegistry()()
	file, line := callerLocation(2)
	rootID, pop := enterAnalyzeNode(compiler.DescribeNode, name, file, line, nil)
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
func DescribeWithReporter(tb testing.TB, name string, rep report.EventReporter, fn func(*Spec)) {
	if currentRegistry() == nil {
		describeWithCompiler(tb, name, rep, fn, false)
		return
	}
	defer ensureRegistry()()
	file, line := callerLocation(2)
	rootID, pop := enterAnalyzeNode(compiler.DescribeNode, name, file, line, nil)
	if rootID < 0 {
		return
	}
	defer pop()
	var backend ctx.TestBackend
	if tb != nil {
		backend = ctx.AsTestBackend(tb)
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

// DescribeFlat runs all specs in one test (no subtests).
func DescribeFlat(tb testing.TB, name string, fn func(*Spec)) {
	if currentRegistry() == nil {
		describeWithCompiler(tb, name, nil, fn, true)
		return
	}
	defer ensureRegistry()()
	file, line := callerLocation(2)
	rootID, pop := enterAnalyzeNode(compiler.DescribeNode, name, file, line, nil)
	if rootID < 0 {
		return
	}
	defer pop()
	var backend ctx.TestBackend
	if tb != nil {
		backend = ctx.AsTestBackend(tb)
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
func DescribeFlatWithReporter(tb testing.TB, name string, rep report.EventReporter, fn func(*Spec)) {
	if currentRegistry() == nil {
		describeWithCompiler(tb, name, rep, fn, true)
		return
	}
	defer ensureRegistry()()
	file, line := callerLocation(2)
	rootID, pop := enterAnalyzeNode(compiler.DescribeNode, name, file, line, nil)
	if rootID < 0 {
		return
	}
	defer pop()
	var backend ctx.TestBackend
	if tb != nil {
		backend = ctx.AsTestBackend(tb)
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

// DescribeFast runs all specs inside one test (same as DescribeFlat).
func DescribeFast(tb testing.TB, name string, fn func(*Spec)) {
	DescribeFlat(tb, name, fn)
}

// DescribeFastWithReporter is like DescribeFast with a reporter.
func DescribeFastWithReporter(tb testing.TB, name string, rep report.EventReporter, fn func(*Spec)) {
	DescribeFlatWithReporter(tb, name, rep, fn)
}

// Run runs the compiled suite via the runner layer.
func (s *Spec) Run() {
	if s != nil && s.suite != nil && s.tb != nil {
		runner.RunCompiledSuite(s.suite, s.tb)
	}
}

// Compile builds the ExecutionPlan once.
func (s *Spec) Compile() {
	if s == nil {
		return
	}
	s.compileOnce.Do(func() {
		if s.plan != nil {
			s.suite = &compiler.CompiledSuite{Plan: s.plan, Arena: nil, RootID: 0, Reporter: s.reporter, Seed: s.seed, HasSeed: s.hasSeed}
			return
		}
		if s.arena == nil {
			return
		}
		plan := compiler.BuildPlanFromArena(s.arena, s.rootID)
		s.suite = &compiler.CompiledSuite{Plan: plan, Arena: s.arena, RootID: s.rootID, Reporter: s.reporter, Seed: s.seed, HasSeed: s.hasSeed}
	})
}

// Describe starts a nested describe block.
func (s *Spec) Describe(name string, fn func(*Spec)) {
	if s == nil || fn == nil {
		return
	}
	if s.bcompiler != nil {
		s.bcompiler.PushScope(name)
		defer s.bcompiler.PopScope()
		fn(s.childSpec())
		return
	}
	file, line := callerLocation(2)
	_, pop := enterAnalyzeNode(compiler.DescribeNode, name, file, line, nil)
	defer pop()
	fn(s.childSpec())
}

// When starts a when block.
func (s *Spec) When(name string, fn interface{}) {
	if s == nil || fn == nil {
		return
	}
	if s.bcompiler != nil {
		s.bcompiler.PushScope(name)
		defer s.bcompiler.PopScope()
		switch f := fn.(type) {
		case func(*Spec):
			f(s.childSpec())
		case func():
			f()
		}
		return
	}
	file, line := callerLocation(2)
	_, pop := enterAnalyzeNode(compiler.WhenNode, name, file, line, nil)
	defer pop()
	switch f := fn.(type) {
	case func(*Spec):
		f(s.childSpec())
	case func():
		f()
	}
}

// It registers a spec.
func (s *Spec) It(name string, fn func(*ctx.Context)) {
	if s == nil {
		return
	}
	if s.bcompiler != nil {
		s.bcompiler.EmitIt(name, fn)
		return
	}
	file, line := callerLocation(2)
	_, pop := enterAnalyzeNode(compiler.ItNode, name, file, line, fn)
	pop()
}

// BeforeEach appends a before-each hook.
func (s *Spec) BeforeEach(fn func(*ctx.Context)) {
	if s == nil || fn == nil {
		return
	}
	if s.bcompiler != nil {
		s.bcompiler.AppendBefore(fn)
		return
	}
	AppendBeforeHook(fn)
}

// AfterEach appends an after-each hook.
func (s *Spec) AfterEach(fn func(*ctx.Context)) {
	if s == nil || fn == nil {
		return
	}
	if s.bcompiler != nil {
		s.bcompiler.AppendAfter(fn)
		return
	}
	AppendAfterHook(fn)
}

// RandomSeed sets the deterministic RNG seed for this spec subtree. Propagated to the runner so Context.SetSeed(seed) is used during execution.
func (s *Spec) RandomSeed(seed uint64) {
	if s != nil {
		s.seed = seed
		s.hasSeed = true
	}
}

func (s *Spec) runPathWithContext(name string, gen *property.PathGenerator, _ interface{}, fn func(*ctx.Context)) {
	if s == nil || fn == nil {
		return
	}
	if s.bcompiler != nil {
		s.bcompiler.SetPathGen(gen)
		s.bcompiler.EmitIt(name, fn)
		return
	}
	file, line := callerLocation(2)
	_, pop := enterAnalyzeNode(compiler.ItNode, name, file, line, fn)
	SetPathGen(gen)
	pop()
}

func parseItArgs(args []any) (ops interface{}, fn func(*ctx.Context)) {
	if len(args) == 0 {
		return nil, nil
	}
	if f, ok := args[len(args)-1].(func(*ctx.Context)); ok {
		return nil, f
	}
	return nil, nil
}
