package specs

import (
	"sort"
	"testing"
	"time"

	"github.com/pablogore/go-specs/report"
)

// TestRunnerWithReporterEmitsSuiteAndSpecEvents proves NewRunnerWithReporter's Runner reports the
// same SuiteStarted/SpecStarted/SpecFinished/SuiteFinished shape as CompiledSuite/Describe do (see
// execution_plan_reporter_test.go), for the separate Program/Runner execution model.
func TestRunnerWithReporterEmitsSuiteAndSpecEvents(t *testing.T) {
	rep := &recordingReporter{}
	prog := BuildProgram(func(b *Builder) {
		b.Describe("Suite", func() {
			b.It("passes", func(ctx *Context) {})
			b.It("fails", func(ctx *Context) { ctx.recordFailure() })
		})
	})

	NewRunnerWithReporter(prog, "Suite", rep).Run(t)

	if len(rep.suiteStarted) != 1 || rep.suiteStarted[0].Name != "Suite" {
		t.Fatalf("expected one SuiteStarted for Suite, got %+v", rep.suiteStarted)
	}
	if len(rep.specStarted) != 2 {
		t.Fatalf("expected two SpecStarted, got %+v", rep.specStarted)
	}
	if len(rep.specFinished) != 2 {
		t.Fatalf("expected two SpecFinished, got %+v", rep.specFinished)
	}
	byName := map[string]bool{}
	for _, e := range rep.specFinished {
		byName[e.Name] = e.Failed
	}
	if failed, ok := byName["passes"]; !ok || failed {
		t.Errorf("expected 'passes' to be reported as not failed, got %+v", rep.specFinished)
	}
	if failed, ok := byName["fails"]; !ok || !failed {
		t.Errorf("expected 'fails' to be reported as failed, got %+v", rep.specFinished)
	}
	if len(rep.suiteFinished) != 1 {
		t.Fatalf("expected one SuiteFinished, got %+v", rep.suiteFinished)
	}
	end := rep.suiteFinished[0]
	if end.Name != "Suite" || end.TotalSpecs != 2 || end.FailedSpecs != 1 {
		t.Fatalf("expected Suite TotalSpecs=2 FailedSpecs=1, got %+v", end)
	}
}

// TestRunnerWithReporterRecordsSpecAndSuiteDuration proves SpecFinished.Duration and
// SuiteFinished.Duration measure real elapsed time (not a placeholder zero), and that
// SpecFinished.SpecStartEvent is the exact event SpecStarted sent — the invariant Duration depends
// on, since it's computed from that event's Time, not reconstructed at finish time.
func TestRunnerWithReporterRecordsSpecAndSuiteDuration(t *testing.T) {
	const sleep = 20 * time.Millisecond
	rep := &recordingReporter{}
	prog := BuildProgram(func(b *Builder) {
		b.Describe("Suite", func() {
			b.It("slow", func(ctx *Context) { time.Sleep(sleep) })
		})
	})

	NewRunnerWithReporter(prog, "Suite", rep).Run(t)

	if len(rep.specStarted) != 1 || len(rep.specFinished) != 1 {
		t.Fatalf("expected one SpecStarted/SpecFinished pair, got started=%+v finished=%+v", rep.specStarted, rep.specFinished)
	}
	started, finished := rep.specStarted[0], rep.specFinished[0]
	if !finished.Time.Equal(started.Time) {
		t.Fatalf("expected SpecFinished.SpecStartEvent to be the exact SpecStarted event, got start=%v finish=%v", started.Time, finished.Time)
	}
	if finished.Duration < sleep {
		t.Fatalf("expected spec Duration >= %v (the spec slept that long), got %v", sleep, finished.Duration)
	}
	if len(rep.suiteFinished) != 1 {
		t.Fatalf("expected one SuiteFinished, got %+v", rep.suiteFinished)
	}
	if end := rep.suiteFinished[0]; end.Duration < sleep {
		t.Fatalf("expected suite Duration >= %v (it wraps the slow spec), got %v", sleep, end.Duration)
	}
}

// TestParallelStepWithObserverRecordsPerGoroutineDuration proves each ItParallel spec's Duration is
// measured against its own goroutine's started event, not shared or reused across specs.
//
// This only asserts a lower bound on the deliberately slow spec and that every spec's reported
// SpecFinished.SpecStartEvent.Time matches its own SpecStarted.Time exactly — never an upper bound
// on the fast siblings. An upper-bound/exact-timing assertion on "fast1"/"fast2" (e.g. Duration <
// sleep) would be flaky under a loaded CI runner, where even an empty spec body can take longer than
// a short sleep to get scheduled between SpecStarted and SpecFinished.
func TestParallelStepWithObserverRecordsPerGoroutineDuration(t *testing.T) {
	const sleep = 20 * time.Millisecond
	rep := &recordingReporter{}
	obs := &reporterObserver{rep: rep}
	backend := &capturingBackend{}
	ctx := &Context{backend: backend, execObserver: obs}

	run := parallelStep([]step{
		runAll([]step{func(*Context) {}}),
		runAll([]step{func(*Context) { time.Sleep(sleep) }}),
		runAll([]step{func(*Context) {}}),
	}, []string{"fast1", "slow", "fast2"})
	run(ctx)

	if len(rep.specStarted) != 3 || len(rep.specFinished) != 3 {
		t.Fatalf("expected three SpecStarted/SpecFinished, got started=%+v finished=%+v", rep.specStarted, rep.specFinished)
	}
	startByName := map[string]time.Time{}
	for _, e := range rep.specStarted {
		startByName[e.Name] = e.Time
	}
	for _, e := range rep.specFinished {
		start, ok := startByName[e.Name]
		if !ok || !e.Time.Equal(start) {
			t.Fatalf("expected %q's SpecFinished.SpecStartEvent to be its own SpecStarted event, got start=%v finish=%v", e.Name, start, e.Time)
		}
		if e.Duration < 0 {
			t.Fatalf("expected non-negative Duration for %q, got %v", e.Name, e.Duration)
		}
		if e.Name == "slow" && e.Duration < sleep {
			t.Fatalf("expected 'slow' Duration >= %v, got %v", sleep, e.Duration)
		}
	}
}

// TestRunnerNilReporterMatchesReporterFailFastSemantics proves the ctx.failed-per-spec-reset fix
// (required unconditionally, not gated on Reporter != nil — see runSpecsRecovered) does not change
// FailFast behavior: with FailFast set, a failing spec still stops every later spec, whether or not
// a Reporter is attached.
func TestRunnerNilReporterMatchesReporterFailFastSemantics(t *testing.T) {
	t.Run("without reporter", func(t *testing.T) {
		var ranSecond bool
		prog := BuildProgram(func(b *Builder) {
			b.Describe("Suite", func() {
				b.It("first", func(ctx *Context) { ctx.recordFailure() })
				b.It("second", func(ctx *Context) { ranSecond = true })
			})
		})
		r := NewRunner(prog)
		r.FailFast = true
		r.Run(t)
		if ranSecond {
			t.Error("expected FailFast to stop before the second spec")
		}
	})

	t.Run("with reporter", func(t *testing.T) {
		var ranSecond bool
		prog := BuildProgram(func(b *Builder) {
			b.Describe("Suite", func() {
				b.It("first", func(ctx *Context) { ctx.recordFailure() })
				b.It("second", func(ctx *Context) { ranSecond = true })
			})
		})
		rep := &recordingReporter{}
		r := NewRunnerWithReporter(prog, "Suite", rep)
		r.FailFast = true
		r.Run(t)
		if ranSecond {
			t.Error("expected FailFast to stop before the second spec")
		}
		if len(rep.specFinished) != 1 || rep.specFinished[0].Name != "first" || !rep.specFinished[0].Failed {
			t.Fatalf("expected only 'first' to be reported, as failed, got %+v", rep.specFinished)
		}
	})
}

// TestParallelStepWithObserverReportsEachSpecIndividually proves each real ItParallel spec is
// reported individually (one SpecStarted/SpecFinished pair per spec, not one for the whole opaque
// group), with Failed derived from that spec's own result (results[i], the same per-index tracking
// reportFailures already uses), not the group's aggregate ctx.failed. Exercises parallelStep
// directly against a capturingBackend, like program_test.go's other parallelStep tests, so a
// (deliberately) failing spec here never reaches a real *testing.T's Fatalf/Goexit.
func TestParallelStepWithObserverReportsEachSpecIndividually(t *testing.T) {
	rep := &recordingReporter{}
	obs := &reporterObserver{rep: rep}
	backend := &capturingBackend{}
	ctx := &Context{backend: backend, execObserver: obs}

	run := parallelStep([]step{
		runAll([]step{func(*Context) {}}),
		runAll([]step{func(ctx *Context) { ctx.backend.Error("boom") }}),
		runAll([]step{func(*Context) {}}),
	}, []string{"p1", "p2", "p3"})
	run(ctx)

	if !backend.failed {
		t.Fatal("expected the group's failure (from p2) to still be reported on the outer backend")
	}
	if len(rep.specStarted) != 3 {
		t.Fatalf("expected three SpecStarted (one per real ItParallel spec), got %+v", rep.specStarted)
	}
	if len(rep.specFinished) != 3 {
		t.Fatalf("expected three SpecFinished, got %+v", rep.specFinished)
	}
	var names []string
	byName := map[string]bool{}
	for _, e := range rep.specFinished {
		names = append(names, e.Name)
		byName[e.Name] = e.Failed
	}
	sort.Strings(names)
	if got := names; len(got) != 3 || got[0] != "p1" || got[1] != "p2" || got[2] != "p3" {
		t.Fatalf("expected p1/p2/p3 each reported once, got %v", names)
	}
	if byName["p1"] || byName["p3"] {
		t.Errorf("expected p1/p3 to be reported as not failed, got %+v", rep.specFinished)
	}
	if !byName["p2"] {
		t.Errorf("expected p2 to be reported as failed, got %+v", rep.specFinished)
	}
	if obs.total != 3 || obs.failed != 1 {
		t.Fatalf("expected observer to tally total=3 failed=1, got total=%d failed=%d", obs.total, obs.failed)
	}
}

// TestRunnerWithReporterItParallelSharesRunnerObserver proves a passing ItParallel group, run
// through the real Runner (not parallelStep directly), reports through the same observer instance
// as the surrounding sequential specs — so SuiteEndEvent.TotalSpecs counts both kinds together.
func TestRunnerWithReporterItParallelSharesRunnerObserver(t *testing.T) {
	rep := &recordingReporter{}
	prog := BuildProgram(func(b *Builder) {
		b.Describe("Suite", func() {
			b.It("sequential", func(ctx *Context) {})
			b.ItParallel("p1", func(ctx *Context) {})
			b.ItParallel("p2", func(ctx *Context) {})
		})
	})

	NewRunnerWithReporter(prog, "Suite", rep).Run(t)

	if len(rep.specStarted) != 3 {
		t.Fatalf("expected three SpecStarted (1 sequential + 2 parallel), got %+v", rep.specStarted)
	}
	if len(rep.suiteFinished) != 1 {
		t.Fatalf("expected one SuiteFinished, got %+v", rep.suiteFinished)
	}
	end := rep.suiteFinished[0]
	if end.TotalSpecs != 3 || end.FailedSpecs != 0 {
		t.Fatalf("expected TotalSpecs=3 FailedSpecs=0, got %+v", end)
	}
}

// TestRunShardWithReporterCountsOnlyShardSpecs proves SuiteEndEvent.TotalSpecs for a shard reflects
// only the specs that shard actually executed, not the full Program's total — the reporter describes
// the concrete execution that happened, not the unsharded program.
func TestRunShardWithReporterCountsOnlyShardSpecs(t *testing.T) {
	// Built as four distinct groups directly (not via the Builder), so this test isn't at the mercy
	// of finalize()'s hook-based coalescing merging same-hookKey Describes (all hookless here) into
	// one group — RunShard shards by group index, so it needs four real groups to split across.
	noop := func(*Context) {}
	prog := &Program{Groups: []group{
		{specs: []step{noop}, names: []string{"a"}},
		{specs: []step{noop}, names: []string{"b"}},
		{specs: []step{noop}, names: []string{"c"}},
		{specs: []step{noop}, names: []string{"d"}},
	}}

	rep := &recordingReporter{}
	RunShardWithReporter(prog, t, 0, 2, "Shard0", rep)

	if len(rep.suiteFinished) != 1 {
		t.Fatalf("expected one SuiteFinished, got %+v", rep.suiteFinished)
	}
	end := rep.suiteFinished[0]
	if end.TotalSpecs != 2 {
		t.Fatalf("expected shard 0 of 2 to report TotalSpecs=2 (half of 4), got %+v", end)
	}
	if len(rep.specStarted) != 2 {
		t.Fatalf("expected two SpecStarted for this shard, got %+v", rep.specStarted)
	}
}

// TestRunnerNilReporterNoObserver proves a Runner without a Reporter never installs an
// execObserver, so it takes zero extra branches beyond the ordinary nil check — same invariant as
// #12-A: no reporter, no observable difference in execution.
func TestRunnerNilReporterNoObserver(t *testing.T) {
	var sawObserver bool
	prog := BuildProgram(func(b *Builder) {
		b.Describe("Suite", func() {
			b.It("spec", func(ctx *Context) { sawObserver = ctx.execObserver != nil })
		})
	})
	NewRunner(prog).Run(t)
	if sawObserver {
		t.Error("expected ctx.execObserver to be nil without a Reporter")
	}
}

var _ report.EventReporter = (*recordingReporter)(nil)
