package ctx

import (
	"sync"
	"testing"
)

// runnableBackendPool recycles runnableBackend to reduce allocations during execution.
var runnableBackendPool = sync.Pool{
	New: func() any { return &runnableBackend{} },
}

// TestBackend abstracts the subset of testing.T / testing.B used by assertions.
// Run runs fn as a subtest (for *testing.T) or directly (for *testing.B) for IDE-friendly execution.
type TestBackend interface {
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

// runnableBackend wraps testing.TB to implement TestBackend and Run for subtest execution.
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
func (r *runnableBackend) Logf(format string, args ...any)    { r.tb.Logf(format, args...) }
func (r *runnableBackend) Name() string                      { return r.tb.Name() }
func (r *runnableBackend) Cleanup(fn func())                 { r.tb.Cleanup(fn) }

func (r *runnableBackend) Run(name string, fn func(testing.TB)) {
	if t, ok := r.tb.(*testing.T); ok {
		t.Run(name, func(t *testing.T) {
			fn(t)
		})
		return
	}
	fn(r.tb)
}

// AsTestBackend wraps testing.TB as TestBackend. Call PutTestBackend when done.
func AsTestBackend(tb testing.TB) TestBackend {
	if tb == nil {
		return nil
	}
	r := runnableBackendPool.Get().(*runnableBackend)
	r.tb = tb
	return r
}

// PutTestBackend returns a TestBackend to the pool when it is a runnableBackend. Call after execution.
func PutTestBackend(b TestBackend) {
	if r, ok := b.(*runnableBackend); ok {
		r.tb = nil
		runnableBackendPool.Put(r)
	}
}

// NewLockedBackend wraps b so every method call is serialised under a mutex.
// Use in parallelStep when goroutines must share a single backend that is not
// goroutine-safe (e.g. *parallel.ParallelBackend). runnableBackend/testing.T
// is already goroutine-safe, but wrapping it is harmless.
// Fatal/Fatalf/FailNow unlock via defer so runtime.Goexit() from testing.T
// cannot leave the mutex permanently locked.
func NewLockedBackend(b TestBackend) TestBackend {
	if b == nil {
		return nil
	}
	return &lockedBackend{b: b}
}

type lockedBackend struct {
	mu sync.Mutex
	b  TestBackend
}

func (l *lockedBackend) Helper() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.Helper()
}

func (l *lockedBackend) FailNow() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.FailNow()
}

func (l *lockedBackend) Fatal(args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.Fatal(args...)
}

func (l *lockedBackend) Fatalf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.Fatalf(format, args...)
}

func (l *lockedBackend) Error(args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.Error(args...)
}

func (l *lockedBackend) Errorf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.Errorf(format, args...)
}

func (l *lockedBackend) Log(args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.Log(args...)
}

func (l *lockedBackend) Logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.Logf(format, args...)
}

func (l *lockedBackend) Name() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Name()
}

// Cleanup and Run are forwarded without the lock:
//   - In *ParallelBackend these are no-ops / panics; locking adds nothing.
//   - In runnableBackend they delegate to testing.T which is goroutine-safe.
func (l *lockedBackend) Cleanup(fn func()) { l.b.Cleanup(fn) }

func (l *lockedBackend) Run(name string, fn func(testing.TB)) { l.b.Run(name, fn) }

// AsTestingTB unwraps TestBackend to testing.TB. If the backend cannot be unwrapped, fails the test and returns nil.
func AsTestingTB(tb TestBackend) testing.TB {
	if r, ok := tb.(*runnableBackend); ok {
		return r.tb
	}
	if real, ok := tb.(testing.TB); ok {
		return real
	}
	tb.Helper()
	tb.Fatalf("specs: backend does not implement testing.TB (type=%T)", tb)
	return nil
}

// RequireTestingT returns *testing.T from the backend; panics if not available.
func RequireTestingT(tb TestBackend) *testing.T {
	if r, ok := tb.(*runnableBackend); ok {
		if t, ok := r.tb.(*testing.T); ok {
			return t
		}
	}
	panic("specs: Spec requires *testing.T; use Context directly in benchmarks")
}
