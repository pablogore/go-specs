package specs

import (
	"sync"
	"testing"
)

// runnableBackendPool recycles runnableBackend to reduce allocations during execution.
var runnableBackendPool = sync.Pool{
	New: func() any { return &runnableBackend{} },
}

// testBackend abstracts the subset of testing.T / testing.B used by assertions.
// Run runs fn as a subtest (for *testing.T) or directly (for *testing.B) for IDE-friendly execution.
type testBackend interface {
	Helper()
	FailNow()
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	Log(args ...any)
	Logf(format string, args ...any)
	Name() string
	Cleanup(func())
	Run(name string, fn func(testing.TB))
}

// runnableBackend wraps testing.TB to implement testBackend and Run for subtest execution.
type runnableBackend struct {
	tb testing.TB
}

func (r *runnableBackend) Helper()                          { r.tb.Helper() }
func (r *runnableBackend) FailNow()                          { r.tb.FailNow() }
func (r *runnableBackend) Fatal(args ...any)                 { r.tb.Fatal(args...) }
func (r *runnableBackend) Fatalf(format string, args ...any) { r.tb.Fatalf(format, args...) }
func (r *runnableBackend) Error(args ...any)                 { r.tb.Error(args...) }
func (r *runnableBackend) Errorf(format string, args ...any) { r.tb.Errorf(format, args...) }
func (r *runnableBackend) Log(args ...any)                   { r.tb.Log(args...) }
func (r *runnableBackend) Logf(format string, args ...any)   { r.tb.Logf(format, args...) }
func (r *runnableBackend) Name() string                      { return r.tb.Name() }
func (r *runnableBackend) Cleanup(fn func())                { r.tb.Cleanup(fn) }

func (r *runnableBackend) Run(name string, fn func(testing.TB)) {
	if t, ok := r.tb.(*testing.T); ok {
		t.Run(name, func(t *testing.T) {
			fn(t)
		})
		return
	}
	fn(r.tb)
}

func asTestBackend(tb testing.TB) testBackend {
	if tb == nil {
		return nil
	}
	r := runnableBackendPool.Get().(*runnableBackend)
	r.tb = tb
	return r
}

// putTestBackend returns a testBackend to the pool when it is a runnableBackend. Call after execution.
func putTestBackend(b testBackend) {
	if r, ok := b.(*runnableBackend); ok {
		r.tb = nil
		runnableBackendPool.Put(r)
	}
}

func asTestingTB(tb testBackend) testing.TB {
	if r, ok := tb.(*runnableBackend); ok {
		return r.tb
	}
	if real, ok := tb.(testing.TB); ok {
		return real
	}
	panic("specs: backend does not implement testing.TB")
}

func requireTestingT(tb testBackend) *testing.T {
	if r, ok := tb.(*runnableBackend); ok {
		if t, ok := r.tb.(*testing.T); ok {
			return t
		}
	}
	panic("specs: Spec requires *testing.T; use Context directly in benchmarks")
}
