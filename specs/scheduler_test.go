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

// TestRunParallel_FatalAssertionStopsSpecBody verifies a fatal assertion (EqualTo failing, which
// calls parallelBackend.Fatalf) stops the rest of that spec's body — same as a sequential It, and
// same as ItParallel since #24. Before #38's fix, RunParallel's worker pool left abortOnFatal
// false, so Fatalf recorded the failure but returned normally and code after it kept running.
func TestRunParallel_FatalAssertionStopsSpecBody(t *testing.T) {
	r := NewMinimalRunner(1)
	var ranAfterFailure bool
	r.Add("s0", func(ctx *Context) {
		EqualTo(ctx, 1, 2)
		ranAfterFailure = true // must not run: the assertion above is fatal
	})

	var reported string
	fake := &fakeReporter{fatalf: func(format string, args ...any) { reported = fmt.Sprintf(format, args...) }}
	r.RunParallel(fake, 1)

	if reported == "" {
		t.Fatal("expected a failure to be reported")
	}
	if !strings.Contains(reported, "expected 1 to equal 2") {
		t.Errorf("expected the assertion failure message, got %q", reported)
	}
	if ranAfterFailure {
		t.Error("expected code after the fatal assertion to be skipped, but it ran")
	}
}

// TestRunParallel_FatalAssertionDoesNotStopOtherSpecs verifies a spec's fatal assertion aborts only
// that spec, not the worker running it — the worker must still run the specs after it. Uses a
// single worker so spec order relative to the failing one is deterministic.
func TestRunParallel_FatalAssertionDoesNotStopOtherSpecs(t *testing.T) {
	r := NewMinimalRunner(3)
	var laterRan int32
	r.Add("s0", func(ctx *Context) { EqualTo(ctx, 1, 2) }) // fails, aborts
	r.Add("s1", func(ctx *Context) { atomic.AddInt32(&laterRan, 1); EqualTo(ctx, 1, 1) })
	r.Add("s2", func(ctx *Context) { atomic.AddInt32(&laterRan, 1); EqualTo(ctx, 1, 1) })

	fake := &fakeReporter{fatalf: func(string, ...any) {}}
	r.RunParallel(fake, 1)

	if laterRan != 2 {
		t.Errorf("expected the 2 specs after the failing one to still run, got %d", laterRan)
	}
}

// TestRunParallelBatched_FatalAssertionStopsSpecBody mirrors
// TestRunParallel_FatalAssertionStopsSpecBody for the batched worker loop (runWorkerBatched),
// including that the chunk loop continues to the next spec in the same chunk after an abort.
func TestRunParallelBatched_FatalAssertionStopsSpecBody(t *testing.T) {
	r := NewMinimalRunner(4)
	var ranAfterFailure bool
	var laterRan int32
	r.Add("s0", func(ctx *Context) {
		EqualTo(ctx, 1, 2)
		ranAfterFailure = true // must not run: the assertion above is fatal
	})
	r.Add("s1", func(ctx *Context) { atomic.AddInt32(&laterRan, 1); EqualTo(ctx, 1, 1) })

	var reported string
	fake := &fakeReporter{fatalf: func(format string, args ...any) { reported = fmt.Sprintf(format, args...) }}
	r.RunParallelBatched(fake, 1, 4) // 1 worker, chunkSize 4 so both specs land in the same chunk

	if reported == "" {
		t.Fatal("expected a failure to be reported")
	}
	if ranAfterFailure {
		t.Error("expected code after the fatal assertion to be skipped, but it ran")
	}
	if laterRan != 1 {
		t.Errorf("expected the spec after the failing one in the same chunk to still run, got %d", laterRan)
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
