package specs

import (
	"strings"
	"sync"
	"testing"

	"github.com/pablogore/go-specs/report"
)

// loggingReporter implements EventReporter and records events for tests.
type loggingReporter struct {
	mu     sync.Mutex
	events []string
}

func (r *loggingReporter) SuiteStarted(e report.SuiteStartEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, "SuiteStarted:"+e.Name)
}

func (r *loggingReporter) SuiteFinished(e report.SuiteEndEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, "SuiteFinished")
}

func (r *loggingReporter) SpecStarted(e report.SpecStartEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, "SpecStarted:"+e.Name)
}

func (r *loggingReporter) SpecFinished(e report.SpecResultEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, "SpecFinished:"+e.Name)
}

func (r *loggingReporter) Events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.events))
	copy(out, r.events)
	return out
}

// TestRunnerReporterReceivesEvents confirms a custom reporter receives suite events when using Runner.
func TestRunnerReporterReceivesEvents(t *testing.T) {
	rep := &loggingReporter{}
	b := NewBuilder()
	b.Describe("suite", func() {
		b.It("spec one", func(ctx *Context) { EqualTo(ctx, 1, 1) })
		b.It("spec two", func(ctx *Context) { EqualTo(ctx, 2, 2) })
	})
	prog := b.Build()
	r := NewRunner(prog, rep)
	r.Run(t)
	events := rep.Events()
	var gotSuiteStart, gotSuiteFinish bool
	for _, e := range events {
		if e == "SuiteStarted:" || e == "SuiteStarted:suite" {
			gotSuiteStart = true
		}
		if e == "SuiteFinished" {
			gotSuiteFinish = true
		}
	}
	if !gotSuiteStart {
		t.Errorf("reporter did not receive SuiteStarted, got: %v", events)
	}
	if !gotSuiteFinish {
		t.Errorf("reporter did not receive SuiteFinished, got: %v", events)
	}
	if len(events) < 2 {
		t.Errorf("expected at least SuiteStarted and SuiteFinished, got %d: %v", len(events), events)
	}
}

// TestDescribeWithReporterReceivesSpecEvents confirms a custom reporter receives spec-level events via DSL.
func TestDescribeWithReporterReceivesSpecEvents(t *testing.T) {
	rep := &loggingReporter{}
	DescribeWithReporter(t, "my_suite", rep, func(s *Spec) {
		s.It("first", func(ctx *Context) { EqualTo(ctx, 1, 1) })
		s.It("second", func(ctx *Context) { EqualTo(ctx, 2, 2) })
	})
	events := rep.Events()
	var gotSuiteStart, gotSpecStart, gotSpecFinish, gotSuiteFinish bool
	for _, e := range events {
		switch e {
		case "SuiteStarted:my_suite":
			gotSuiteStart = true
		case "SuiteFinished":
			gotSuiteFinish = true
		default:
			if strings.HasPrefix(e, "SpecStarted") {
				gotSpecStart = true
			}
			if strings.HasPrefix(e, "SpecFinished") {
				gotSpecFinish = true
			}
		}
	}
	if !gotSuiteStart {
		t.Errorf("reporter did not receive SuiteStarted, got: %v", events)
	}
	if !gotSuiteFinish {
		t.Errorf("reporter did not receive SuiteFinished, got: %v", events)
	}
	if !gotSpecStart {
		t.Errorf("reporter did not receive any SpecStarted, got: %v", events)
	}
	if !gotSpecFinish {
		t.Errorf("reporter did not receive any SpecFinished, got: %v", events)
	}
}

