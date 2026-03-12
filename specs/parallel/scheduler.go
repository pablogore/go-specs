// scheduler.go implements parallel execution: ParallelBackend, RunWorker, ReportFailures.
package parallel

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pablogore/go-specs/specs/ctx"
	"github.com/pablogore/go-specs/specs/property"
)

// Spec is a single runnable step (Fn only). Used by RunWorker; runner converts RunSpec to this.
type Spec struct {
	Fn func(*ctx.Context)
}

// ParallelBackend implements ctx.TestBackend by recording failures to results[specIndex].
// Create one per worker; set specIndex before running each spec.
type ParallelBackend struct {
	SpecIndex int
	Results   *[]string
}

func (p *ParallelBackend) Helper() {}

func (p *ParallelBackend) FailNow() {
	if p.Results != nil && p.SpecIndex >= 0 && p.SpecIndex < len(*p.Results) {
		(*p.Results)[p.SpecIndex] = "fail now"
	}
}

func (p *ParallelBackend) Fatal(args ...any) {
	if p.Results != nil && p.SpecIndex >= 0 && p.SpecIndex < len(*p.Results) {
		(*p.Results)[p.SpecIndex] = fmt.Sprint(args...)
	}
}

func (p *ParallelBackend) Fatalf(format string, args ...any) {
	if p.Results != nil && p.SpecIndex >= 0 && p.SpecIndex < len(*p.Results) {
		(*p.Results)[p.SpecIndex] = fmt.Sprintf(format, args...)
	}
}

func (p *ParallelBackend) Error(args ...any)   { p.Fatal(args...) }
func (p *ParallelBackend) Errorf(format string, args ...any) { p.Fatalf(format, args...) }
func (p *ParallelBackend) Log(args ...any)      {}
func (p *ParallelBackend) Logf(string, ...any)  {}
func (p *ParallelBackend) Name() string         { return "" }
func (p *ParallelBackend) Cleanup(func())       {}

func (p *ParallelBackend) Run(name string, fn func(testing.TB)) {
	panic("t.Run is not supported in parallel specs")
}

// RunWorker runs specs whose indexes it acquires via next. One context per worker.
func RunWorker(specs []Spec, backend *ParallelBackend, next *uint32, results *[]string) {
	c := ctx.GetFromPool()
	defer func() {
		c.Reset(nil)
		ctx.PutInPool(c)
	}()
	n := uint32(len(specs))
	for {
		i := atomic.AddUint32(next, 1) - 1
		if i >= n {
			return
		}
		idx := int(i)
		backend.SpecIndex = idx
		c.Reset(backend)
		c.SetPathValues(property.PathValues{})
		specs[idx].Fn(c)
		c.Reset(nil)
	}
}

// FailureReporter is the minimal interface for reporting failures (e.g. testing.T).
type FailureReporter interface {
	Helper()
	Fatalf(format string, args ...any)
}

// ReportFailures collects all failures from results and fails the test once with a combined report.
func ReportFailures(tb FailureReporter, results []string) {
	var failures []string
	for i, r := range results {
		if r != "" {
			failures = append(failures, fmt.Sprintf("spec[%d]: %s", i, r))
		}
	}
	if len(failures) > 0 {
		tb.Helper()
		tb.Fatalf("parallel specs failed (%d failures):\n\n%s\n", len(failures), strings.Join(failures, "\n\n"))
	}
}
