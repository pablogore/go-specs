package ctx

import (
	"math"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pablogore/go-specs/assert"
	"github.com/pablogore/go-specs/specs/property"
	"github.com/pablogore/go-specs/snapshots"
)

// contextPool reuses Context instances in the runner to reduce allocations.
var contextPool = sync.Pool{
	New: func() any {
		return &Context{}
	},
}

// expectationPool reuses Expectation instances for Expect(...).To() / ToEqual() to reduce allocations.
var expectationPool = sync.Pool{
	New: func() any {
		return &Expectation{}
	},
}

// Fixture is a before/after hook that receives the context.
type Fixture func(*Context)

// Context is the execution context passed to It and hooks.
type Context struct {
	backend    TestBackend
	T          *testing.T
	pathValues property.PathValues
	seed       uint64
	rng        *property.RNG
	coverage   *property.Coverage
	failed     bool
	failFast   bool
}

// GetRunSeed returns the seed for this run: GO_SPECS_SEED env if set, else time-based. Use for reproducible runs.
func GetRunSeed() uint64 {
	if s := os.Getenv("GO_SPECS_SEED"); s != "" {
		if u, err := strconv.ParseUint(s, 10, 64); err == nil {
			return u
		}
	}
	return uint64(time.Now().UnixNano())
}

// NewContext builds a context for the given test/bench. Use *testing.T or *testing.B.
func NewContext(tb testing.TB) *Context {
	c := &Context{backend: AsTestBackend(tb)}
	if t, ok := tb.(*testing.T); ok {
		c.T = t
	}
	c.SetSeed(GetRunSeed())
	return c
}

// GetFromPool returns a Context from the pool for runner use. Call PutInPool when done.
func GetFromPool() *Context {
	return contextPool.Get().(*Context)
}

// PutInPool returns a Context to the pool. Call Reset(nil) before putting.
func PutInPool(c *Context) {
	contextPool.Put(c)
}

// Backend returns the current test backend (for propagating to child contexts, e.g. in parallelStep).
func (c *Context) Backend() TestBackend {
	if c == nil {
		return nil
	}
	return c.backend
}

// Reset clears context state for reuse (e.g. from pool). Pass nil to release references before PutInPool.
func (c *Context) Reset(backend TestBackend) {
	if c == nil {
		return
	}
	c.backend = backend
	c.pathValues = property.PathValues{}
	c.seed = 0
	c.rng = nil
	c.T = nil
	c.coverage = nil
	c.failed = false
	if backend != nil {
		if r, ok := backend.(*runnableBackend); ok {
			if t, ok := r.tb.(*testing.T); ok {
				c.T = t
			}
		} else if tb, ok := backend.(testing.TB); ok {
			if t, ok := tb.(*testing.T); ok {
				c.T = t
			}
		}
	}
}

// SetFailFast sets whether the runner should stop after the first failing step.
func (c *Context) SetFailFast(v bool) {
	if c != nil {
		c.failFast = v
	}
}

// FailFast returns whether the context is in fail-fast mode (stop after first failure).
func (c *Context) FailFast() bool {
	if c == nil {
		return false
	}
	return c.failFast
}

// RecordFailure marks the context as failed (used for FailFast).
func (c *Context) RecordFailure() {
	if c != nil {
		c.failed = true
	}
}

// SetSeed sets the deterministic RNG seed for this context. Same seed yields same RandomInt63 sequence.
func (c *Context) SetSeed(seed uint64) {
	if c == nil {
		return
	}
	c.seed = seed
	c.rng = property.NewRNG(seed)
}

// Seed returns the current run seed (for failure reporting: "property failed (seed=...)").
func (c *Context) Seed() uint64 {
	if c == nil {
		return 0
	}
	return c.seed
}

// SetPathValues sets the current path combination (used by path runners).
func (c *Context) SetPathValues(pv property.PathValues) {
	if c == nil {
		return
	}
	pv.AssignTo(&c.pathValues)
}

// Path returns the current path values for this run.
func (c *Context) Path() property.PathValues {
	if c == nil {
		return property.PathValues{}
	}
	return c.pathValues
}

// SetCoverage sets the coverage bitmap for coverage-guided exploration (runner only).
func (c *Context) SetCoverage(cov *property.Coverage) {
	if c != nil {
		c.coverage = cov
	}
}

// RecordCoverage records an execution-path edge for coverage-guided exploration.
func (c *Context) RecordCoverage(edge uint64) {
	if c == nil || c.coverage == nil {
		return
	}
	c.coverage.Hit(edge)
}

// recordCoverageEdge records an edge identified by the caller's program counter and value hashes.
func recordCoverageEdge(c *Context, actual, expected any) {
	if c == nil || c.coverage == nil {
		return
	}
	var pcs [1]uintptr
	n := runtime.Callers(2, pcs[:])
	pc := uintptr(0)
	if n > 0 {
		pc = pcs[0]
	}
	c.coverage.Hit(coverageEdgeHashPC(pc, actual, expected))
}

// Step runs the given function as a named step for path-coverage style specs.
func (c *Context) Step(name string, fn func()) {
	if c == nil || c.failed {
		return
	}
	fn()
}

// Expect returns an expectation for the given actual value.
func (c *Context) Expect(actual any) *Expectation {
	e := expectationPool.Get().(*Expectation)
	e.ctx = c
	e.actual = actual
	return e
}

// ExpectResult is the generic return type of ExpectT (exported for specs re-export).
type ExpectResult[T comparable] struct{ e *Expectation }

// EqualTo asserts that actual equals expected. Zero alloc; single comparison.
func EqualTo[T comparable](c *Context, actual, expected T) {
	if c == nil || c.backend == nil {
		return
	}
	if actual == expected {
		recordCoverageEdge(c, actual, expected)
		return
	}
	c.RecordFailure()
	c.backend.Helper()
	c.backend.Fatalf("expected %v to equal %v", actual, expected)
}

// ExpectT returns a typed expectation for comparable types.
func ExpectT[T comparable](c *Context, v T) ExpectResult[T] {
	e := expectationPool.Get().(*Expectation)
	e.ctx = c
	e.actual = v
	return ExpectResult[T]{e: e}
}

// ToEqual asserts that the value equals expected.
func (x ExpectResult[T]) ToEqual(expected T) {
	e := x.e
	if e == nil {
		return
	}
	defer e.release()
	if e.ctx == nil || e.ctx.backend == nil {
		return
	}
	actual, ok := e.actual.(T)
	if !ok {
		e.ctx.RecordFailure()
		e.ctx.backend.Helper()
		e.ctx.backend.Fatalf("expected %v to equal %v (type mismatch)", e.actual, expected)
		return
	}
	if actual != expected {
		e.ctx.RecordFailure()
		e.ctx.backend.Helper()
		e.ctx.backend.Fatalf("expected %v to equal %v", actual, expected)
		return
	}
	recordCoverageEdge(e.ctx, actual, expected)
}

// To asserts that the value matches the matcher. Uses fast-path dispatch for built-in matchers.
func (x ExpectResult[T]) To(m assert.Matcher) {
	e := x.e
	if e == nil {
		return
	}
	defer e.release()
	if e.ctx == nil || m == nil {
		return
	}
	matched, failureMsg := assert.MatchWithFastPath(m, e.actual)
	if matched {
		recordCoverageEdge(e.ctx, e.actual, nil)
		return
	}
	e.ctx.RecordFailure()
	e.ctx.backend.Helper()
	msg := failureMsg
	if e.ctx.Seed() != 0 {
		msg += "\nproperty failed (seed=" + strconv.FormatUint(e.ctx.Seed(), 10) + ")"
	}
	e.ctx.backend.Fatalf("%s", msg)
}

// Snapshot serializes value as JSON and compares it to the stored snapshot named name.
func (c *Context) Snapshot(name string, value any) {
	if c == nil || c.backend == nil {
		return
	}
	_, callerFile, _, ok := runtime.Caller(1)
	if !ok {
		c.RecordFailure()
		c.backend.Helper()
		c.backend.Fatalf("snapshot: could not get caller file")
		return
	}
	RunSnapshot(c.backend, callerFile, name, value)
}

// RunSnapshot compares value to the stored snapshot for name (exported for tests).
func RunSnapshot(backend TestBackend, callerFile, name string, value any) {
	snapshots.RunFromFile(backend, callerFile, name, value)
}

// Expectation is the result of Context.Expect(actual).
type Expectation struct {
	ctx    *Context
	actual any
}

func (e *Expectation) release() {
	if e == nil {
		return
	}
	e.ctx = nil
	e.actual = nil
	expectationPool.Put(e)
}

// To asserts that the actual value matches the matcher. Uses fast-path dispatch for built-in matchers.
func (e *Expectation) To(m assert.Matcher) {
	if e == nil {
		return
	}
	defer e.release()
	if e.ctx == nil || m == nil {
		return
	}
	matched, failureMsg := assert.MatchWithFastPath(m, e.actual)
	if matched {
		recordCoverageEdge(e.ctx, e.actual, nil)
		return
	}
	e.ctx.RecordFailure()
	e.ctx.backend.Helper()
	msg := failureMsg
	if e.ctx.Seed() != 0 {
		msg += "\nproperty failed (seed=" + strconv.FormatUint(e.ctx.Seed(), 10) + ")"
	}
	e.ctx.backend.Fatalf("%s", msg)
}

// ToEqual asserts that the actual value equals expected.
func (e *Expectation) ToEqual(expected any) {
	if e == nil {
		return
	}
	defer e.release()
	if e.ctx == nil {
		return
	}
	switch a := e.actual.(type) {
	case int:
		if b, ok := expected.(int); ok {
			if a != b {
				e.ctx.RecordFailure()
				e.ctx.backend.Helper()
				e.ctx.backend.Fatalf("expected %v to equal %v", a, b)
			} else {
				recordCoverageEdge(e.ctx, a, b)
			}
			return
		}
	case string:
		if b, ok := expected.(string); ok {
			if a != b {
				e.ctx.RecordFailure()
				e.ctx.backend.Helper()
				e.ctx.backend.Fatalf("expected %v to equal %v", a, b)
			} else {
				recordCoverageEdge(e.ctx, a, b)
			}
			return
		}
	case bool:
		if b, ok := expected.(bool); ok {
			if a != b {
				e.ctx.RecordFailure()
				e.ctx.backend.Helper()
				e.ctx.backend.Fatalf("expected %v to equal %v", a, b)
			} else {
				recordCoverageEdge(e.ctx, a, b)
			}
			return
		}
	case int64:
		if b, ok := expected.(int64); ok {
			if a != b {
				e.ctx.RecordFailure()
				e.ctx.backend.Helper()
				e.ctx.backend.Fatalf("expected %v to equal %v", a, b)
			} else {
				recordCoverageEdge(e.ctx, a, b)
			}
			return
		}
	case float64:
		if b, ok := expected.(float64); ok {
			if a != b {
				e.ctx.RecordFailure()
				e.ctx.backend.Helper()
				e.ctx.backend.Fatalf("expected %v to equal %v", a, b)
			} else {
				recordCoverageEdge(e.ctx, a, b)
			}
			return
		}
	case uint:
		if b, ok := expected.(uint); ok {
			if a != b {
				e.ctx.RecordFailure()
				e.ctx.backend.Helper()
				e.ctx.backend.Fatalf("expected %v to equal %v", a, b)
			} else {
				recordCoverageEdge(e.ctx, a, b)
			}
			return
		}
	}
	if !reflect.DeepEqual(e.actual, expected) {
		e.ctx.RecordFailure()
		e.ctx.backend.Helper()
		e.ctx.backend.Fatalf("expected %v to equal %v", e.actual, expected)
		return
	}
	recordCoverageEdge(e.ctx, e.actual, expected)
}

// RandomInt63 returns a random int64 from the context RNG (for path exploration). Deterministic when seed is set.
func (c *Context) RandomInt63() int64 {
	if c == nil || c.rng == nil {
		return 0
	}
	return c.rng.Int63()
}

// Failed returns whether the context has recorded a failure (FailFast).
func (c *Context) Failed() bool {
	if c == nil {
		return false
	}
	return c.failed
}

func coverageEdgeHashPC(pc uintptr, actual, expected any) uint64 {
	const prime = 1099511628211
	h := uint64(14695981039346656037)
	h ^= uint64(pc)
	h *= prime
	h ^= valueHash(actual)
	h *= prime
	h ^= valueHash(expected)
	h *= prime
	return h
}

func valueHash(v any) uint64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int:
		return uint64(x)
	case int64:
		return uint64(x)
	case int32:
		return uint64(x)
	case uint:
		return uint64(x)
	case uint64:
		return x
	case uint32:
		return uint64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		h := uint64(len(x))
		for i := 0; i < len(x) && i < 8; i++ {
			h = h*31 + uint64(x[i])
		}
		return h
	case float64:
		return math.Float64bits(x)
	default:
		return 0xabad1dea
	}
}

// RunBeforeHooks runs before-each fixtures in order.
func RunBeforeHooks(ctx *Context, fixtures []Fixture) {
	for _, f := range fixtures {
		if f != nil {
			f(ctx)
		}
	}
}

// RunAfterHooks runs after-each fixtures in reverse order (LIFO).
func RunAfterHooks(ctx *Context, fixtures []Fixture) {
	for i := len(fixtures) - 1; i >= 0; i-- {
		if f := fixtures[i]; f != nil {
			f(ctx)
		}
	}
}
