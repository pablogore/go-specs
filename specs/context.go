package specs

import (
	"math"
	"math/rand"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
)

// contextPool reuses Context instances in the runner to reduce allocations.
// The runner acquires with contextPool.Get().(*Context), calls Reset(backend), runs the spec,
// then Reset(nil) to release references, and contextPool.Put(ctx). Do not retain Context
// after Put; it may be reused.
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
	backend    testBackend
	T          *testing.T
	pathValues PathValues
	rng        *rand.Rand
	// coverage is set by the runner during coverage-guided exploration; assertions record edges here.
	coverage *Coverage
	// failed is set by assertions on failure; used by Runner for FailFast to stop execution.
	failed bool
	// failFast is set by Runner when FailFast is true; runner breaks after a step that set failed.
	failFast bool
}

// NewContext builds a context for the given test/bench. Use *testing.T or *testing.B.
func NewContext(tb testing.TB) *Context {
	c := &Context{backend: asTestBackend(tb)}
	if t, ok := tb.(*testing.T); ok {
		c.T = t
	}
	c.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	return c
}

// Reset clears context state for reuse (e.g. from pool). Pass nil to release references before Put.
func (c *Context) Reset(backend testBackend) {
	if c == nil {
		return
	}
	c.backend = backend
	c.pathValues = PathValues{}
	c.rng = nil
	c.T = nil
	c.coverage = nil
	c.failed = false
	if backend != nil {
		// runnableBackend wraps the subtest T; unwrap so ctx.T points to the current subtest.
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
// Called by Runner at start of Run when Runner.FailFast is true.
func (c *Context) SetFailFast(v bool) {
	if c != nil {
		c.failFast = v
	}
}

// recordFailure marks the context as failed (e.g. before Fatalf). Used for FailFast.
func (c *Context) recordFailure() {
	if c != nil {
		c.failed = true
	}
}

// SetPathValues sets the current path combination (used by path runners).
func (c *Context) SetPathValues(pv PathValues) {
	if c == nil {
		return
	}
	pv.assignTo(&c.pathValues)
}

// Path returns the current path values for this run.
func (c *Context) Path() PathValues {
	if c == nil {
		return PathValues{}
	}
	return c.pathValues
}

// RecordCoverage records an execution-path edge for coverage-guided exploration.
// Called by assertions (To, ToEqual) with a cheap hash of branch + outcome; no-op if coverage is nil.
func (c *Context) RecordCoverage(edge uint64) {
	if c == nil || c.coverage == nil {
		return
	}
	c.coverage.Hit(edge)
}

// Expect returns an expectation for the given actual value. The returned Expectation
// is reused from a pool; it is returned to the pool when To() or ToEqual() completes.
func (c *Context) Expect(actual any) *Expectation {
	e := expectationPool.Get().(*Expectation)
	e.ctx = c
	e.actual = actual
	return e
}

// expectT is the generic return type of ExpectT; holds a pooled Expectation.
type expectT[T comparable] struct{ e *Expectation }

// EqualTo asserts that actual equals expected. Zero alloc; single comparison, no type switch, no reflection.
// Helper() is only called on failure so the fast path avoids runtime.Callers().
//
// Example: specs.EqualTo(ctx, 42, 42) or specs.EqualTo(ctx, "got", "got")
func EqualTo[T comparable](c *Context, actual, expected T) {
	if c == nil || c.backend == nil {
		return
	}
	if actual == expected {
		if c.coverage != nil {
			c.RecordCoverage(coverageEdgeHash(2, actual, expected))
		}
		return
	}
	c.recordFailure()
	c.backend.Helper()
	c.backend.Fatalf("expected %v to equal %v", actual, expected)
}

// ExpectT returns a typed expectation for comparable types. Zero allocations (reuses pooled Expectation).
// ToEqual(expected) does one type assertion and direct comparison; inlineable.
//
// Example: specs.ExpectT(ctx, 42).ToEqual(42) or specs.ExpectT(ctx, true).To(specs.BeTrue())
func ExpectT[T comparable](c *Context, v T) expectT[T] {
	e := expectationPool.Get().(*Expectation)
	e.ctx = c
	e.actual = v
	return expectT[T]{e: e}
}

// ToEqual asserts that the value equals expected. No reflection; inlineable. Helper() only on failure.
func (x expectT[T]) ToEqual(expected T) {
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
		e.ctx.recordFailure()
		e.ctx.backend.Helper()
		e.ctx.backend.Fatalf("expected %v to equal %v (type mismatch)", e.actual, expected)
		return
	}
	if actual != expected {
		e.ctx.recordFailure()
		e.ctx.backend.Helper()
		e.ctx.backend.Fatalf("expected %v to equal %v", actual, expected)
		return
	}
	if e.ctx.coverage != nil {
		e.ctx.RecordCoverage(coverageEdgeHash(2, actual, expected))
	}
}

// To asserts that the value matches the matcher (interface path; use ToEqual for comparable T). Helper() only on failure.
func (x expectT[T]) To(m Matcher) {
	e := x.e
	if e == nil {
		return
	}
	defer e.release()
	if e.ctx == nil || m == nil {
		return
	}
	if m.Match(e.actual) {
		if e.ctx.coverage != nil {
			e.ctx.RecordCoverage(coverageEdgeHash(2, e.actual, nil))
		}
		return
	}
	e.ctx.backend.Helper()
	e.ctx.backend.Fatalf("%s", m.FailureMessage(e.actual))
}

// Snapshot serializes value as JSON and compares it to the stored snapshot named name.
// Snapshots are stored in __snapshots__ next to the test file. Set GO_SPECS_UPDATE_SNAPSHOTS=1 to create or update snapshots.
// Helper() is only called on failure (caller file lookup); runSnapshot failures are reported by the snapshots package.
func (c *Context) Snapshot(name string, value any) {
	if c == nil || c.backend == nil {
		return
	}
	_, callerFile, _, ok := runtime.Caller(1)
	if !ok {
		c.recordFailure()
		c.backend.Helper()
		c.backend.Fatalf("snapshot: could not get caller file")
		return
	}
	runSnapshot(c.backend, callerFile, name, value)
}

// Expectation is the result of Context.Expect(actual).
type Expectation struct {
	ctx    *Context
	actual any
}

// release returns the Expectation to the pool. Called at end of To()/ToEqual().
func (e *Expectation) release() {
	if e == nil {
		return
	}
	e.ctx = nil
	e.actual = nil
	expectationPool.Put(e)
}

// To asserts that the actual value matches the matcher. Helper() only on failure.
func (e *Expectation) To(m Matcher) {
	if e == nil {
		return
	}
	defer e.release()
	if e.ctx == nil || m == nil {
		return
	}
	if m.Match(e.actual) {
		if e.ctx.coverage != nil {
			e.ctx.RecordCoverage(coverageEdgeHash(2, e.actual, nil))
		}
		return
	}
	e.ctx.recordFailure()
	e.ctx.backend.Helper()
	e.ctx.backend.Fatalf("%s", m.FailureMessage(e.actual))
}

// ToEqual asserts that the actual value equals expected (fast path for benchmarks). Helper() only on failure.
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
				e.ctx.recordFailure()
				e.ctx.backend.Helper()
				e.ctx.backend.Fatalf("expected %v to equal %v", a, b)
			} else if e.ctx.coverage != nil {
				e.ctx.RecordCoverage(coverageEdgeHash(2, a, b))
			}
			return
		}
	case string:
		if b, ok := expected.(string); ok {
			if a != b {
				e.ctx.recordFailure()
				e.ctx.backend.Helper()
				e.ctx.backend.Fatalf("expected %v to equal %v", a, b)
			} else if e.ctx.coverage != nil {
				e.ctx.RecordCoverage(coverageEdgeHash(2, a, b))
			}
			return
		}
	case bool:
		if b, ok := expected.(bool); ok {
			if a != b {
				e.ctx.recordFailure()
				e.ctx.backend.Helper()
				e.ctx.backend.Fatalf("expected %v to equal %v", a, b)
			} else if e.ctx.coverage != nil {
				e.ctx.RecordCoverage(coverageEdgeHash(2, a, b))
			}
			return
		}
	case int64:
		if b, ok := expected.(int64); ok {
			if a != b {
				e.ctx.recordFailure()
				e.ctx.backend.Helper()
				e.ctx.backend.Fatalf("expected %v to equal %v", a, b)
			} else if e.ctx.coverage != nil {
				e.ctx.RecordCoverage(coverageEdgeHash(2, a, b))
			}
			return
		}
	case float64:
		if b, ok := expected.(float64); ok {
			if a != b {
				e.ctx.recordFailure()
				e.ctx.backend.Helper()
				e.ctx.backend.Fatalf("expected %v to equal %v", a, b)
			} else if e.ctx.coverage != nil {
				e.ctx.RecordCoverage(coverageEdgeHash(2, a, b))
			}
			return
		}
	case uint:
		if b, ok := expected.(uint); ok {
			if a != b {
				e.ctx.recordFailure()
				e.ctx.backend.Helper()
				e.ctx.backend.Fatalf("expected %v to equal %v", a, b)
			} else if e.ctx.coverage != nil {
				e.ctx.RecordCoverage(coverageEdgeHash(2, a, b))
			}
			return
		}
	}
	if !reflect.DeepEqual(e.actual, expected) {
		e.ctx.recordFailure()
		e.ctx.backend.Helper()
		e.ctx.backend.Fatalf("expected %v to equal %v", e.actual, expected)
		return
	}
	if e.ctx.coverage != nil {
		e.ctx.RecordCoverage(coverageEdgeHash(2, e.actual, expected))
	}
}

func (c *Context) randomInt64() int64 {
	if c == nil || c.rng == nil {
		return 0
	}
	return c.rng.Int63()
}

// coverageEdgeHash returns a deterministic edge ID from caller location and comparison outcome (branch sampling).
// Used for coverage-guided exploration; no allocations.
func coverageEdgeHash(skip int, actual, expected any) uint64 {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return 0
	}
	const prime = 1099511628211
	h := uint64(14695981039346656037)
	for i := 0; i < len(file); i++ {
		h ^= uint64(file[i])
		h *= prime
	}
	h ^= uint64(line)
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

// runBeforeHooks runs before-each fixtures in order.
func runBeforeHooks(ctx *Context, fixtures []Fixture) {
	for _, f := range fixtures {
		if f != nil {
			f(ctx)
		}
	}
}

// runAfterHooks runs after-each fixtures in reverse order (LIFO).
func runAfterHooks(ctx *Context, fixtures []Fixture) {
	for i := len(fixtures) - 1; i >= 0; i-- {
		if f := fixtures[i]; f != nil {
			f(ctx)
		}
	}
}

