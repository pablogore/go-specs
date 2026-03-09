package specs

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRunParallel_AllPass(t *testing.T) {
	r := NewMinimalRunner(8)
	for i := 0; i < 10; i++ {
		r.Add("spec", func(ctx *Context) { EqualTo(ctx, 1, 1) })
	}
	r.RunParallel(t, 4)
}

func TestRunParallel_DeterministicReport(t *testing.T) {
	// Fail at spec index 2; report should be for spec[2] (first failure in order).
	r := NewMinimalRunner(8)
	r.Add("s0", func(ctx *Context) { EqualTo(ctx, 1, 1) })
	r.Add("s1", func(ctx *Context) { EqualTo(ctx, 1, 1) })
	r.Add("s2", func(ctx *Context) { EqualTo(ctx, 1, 2) }) // fail
	r.Add("s3", func(ctx *Context) { EqualTo(ctx, 1, 1) })

	var reported string
	fake := &fakeReporter{fatalf: func(format string, args ...any) { reported = fmt.Sprintf(format, args...) }}
	r.RunParallel(fake, 2)
	if reported == "" {
		t.Error("expected failure to be reported")
	}
	if !strings.Contains(reported, "spec[2]") {
		t.Errorf("expected report to mention spec[2], got %q", reported)
	}
}

func TestRunParallel_WorkerReusesContext(t *testing.T) {
	var count int32
	r := NewMinimalRunner(8)
	for i := 0; i < 20; i++ {
		r.Add("spec", func(ctx *Context) {
			atomic.AddInt32(&count, 1)
			EqualTo(ctx, 1, 1)
		})
	}
	r.RunParallel(t, 4)
	if count != 20 {
		t.Errorf("expected 20 runs, got %d", count)
	}
}

func TestRunParallelBatched_AllPass(t *testing.T) {
	r := NewMinimalRunner(32)
	for i := 0; i < 64; i++ {
		r.Add("spec", func(ctx *Context) { EqualTo(ctx, 1, 1) })
	}
	r.RunParallelBatched(t, 4, 16)
}

func TestRunParallelBatched_DeterministicReport(t *testing.T) {
	r := NewMinimalRunner(8)
	r.Add("s0", func(ctx *Context) { EqualTo(ctx, 1, 1) })
	r.Add("s1", func(ctx *Context) { EqualTo(ctx, 1, 2) }) // fail at 1
	r.Add("s2", func(ctx *Context) { EqualTo(ctx, 1, 1) })
	var reported string
	fake := &fakeReporter{fatalf: func(format string, args ...any) { reported = fmt.Sprintf(format, args...) }}
	r.RunParallelBatched(fake, 2, 4)
	if reported == "" {
		t.Error("expected failure to be reported")
	}
	if !strings.Contains(reported, "spec[1]") {
		t.Errorf("expected report to mention spec[1], got %q", reported)
	}
}

// fakeReporter implements failureReporter for tests (captures Fatalf instead of failing).
type fakeReporter struct {
	fatalf func(string, ...any)
}

func (f *fakeReporter) Helper() {}
func (f *fakeReporter) Fatalf(format string, args ...any) {
	if f.fatalf != nil {
		f.fatalf(format, args...)
	}
}

