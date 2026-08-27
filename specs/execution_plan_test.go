package specs

import (
	"fmt"
	"testing"
)

type controlledBackend struct {
	failNow bool
	errors  []string
}

func (b *controlledBackend) Helper()               {}
func (b *controlledBackend) FailNow()              { b.failNow = true; panic(isolatedCaseAbort{}) }
func (b *controlledBackend) Fatal(args ...any)     { b.FailNow() }
func (b *controlledBackend) Fatalf(string, ...any) { b.FailNow() }
func (b *controlledBackend) Error(args ...any)     { b.errors = append(b.errors, fmt.Sprint(args...)) }
func (b *controlledBackend) Errorf(format string, args ...any) {
	b.errors = append(b.errors, fmt.Sprintf(format, args...))
}
func (b *controlledBackend) Log(...any)                   {}
func (b *controlledBackend) Logf(string, ...any)          {}
func (b *controlledBackend) Name() string                 { return "controlled" }
func (b *controlledBackend) Cleanup(func())               {}
func (b *controlledBackend) Run(string, func(testing.TB)) {}

func TestGeneratedCaseLifecycle(t *testing.T) {
	t.Run("pass resets context and reverses after hooks", func(t *testing.T) {
		backend := &controlledBackend{}
		var order []string
		result := runIsolatedCase(backend, []Instruction{
			{Code: OpBeforeHook, Fn: func(ctx *Context) { order = append(order, "before") }},
			{Code: OpBody, Fn: func(ctx *Context) { order = append(order, "body") }},
			{Code: OpAfterHook, Fn: func(ctx *Context) { order = append(order, "after-inner") }},
			{Code: OpAfterHook, Fn: func(ctx *Context) { order = append(order, "after-outer") }},
		}, PathValues{})
		if got, want := fmt.Sprint(order), "[before body after-inner after-outer]"; got != want {
			t.Fatalf("hook order = %s, want %s", got, want)
		}
		if result.Failed || result.Panic != nil {
			t.Fatalf("result = %#v, want successful case", result)
		}
		if !result.ContextReset {
			t.Fatalf("context was not reset after case")
		}
	})

	t.Run("nonfatal assertion completes after hooks", func(t *testing.T) {
		backend := &controlledBackend{}
		var afterRuns int
		result := runIsolatedCase(backend, []Instruction{
			{Code: OpBody, Fn: func(ctx *Context) { ctx.Expect(false).ToEqual(true) }},
			{Code: OpAfterHook, Fn: func(*Context) { afterRuns++ }},
		}, PathValues{})
		if !result.Failed || afterRuns != 1 {
			t.Fatalf("result = %#v, errors = %v, after runs = %d", result, backend.errors, afterRuns)
		}
	})

	t.Run("fatal preserves attribution and runs after once", func(t *testing.T) {
		backend := &controlledBackend{}
		var bodyCompleted, afterRuns bool
		result := runIsolatedCase(backend, []Instruction{
			{Code: OpBody, Fn: func(ctx *Context) { ctx.backend.FailNow(); bodyCompleted = true }},
			{Code: OpAfterHook, Fn: func(*Context) { afterRuns = true }},
		}, PathValues{})
		if !result.Failed || !backend.failNow || bodyCompleted || !afterRuns || result.Panic != nil {
			t.Fatalf("result = %#v, failNow = %t, body completed = %t, after ran = %t", result, backend.failNow, bodyCompleted, afterRuns)
		}
	})

	t.Run("panic is retained and runs after once", func(t *testing.T) {
		backend := &controlledBackend{}
		var afterRuns int
		result := runIsolatedCase(backend, []Instruction{
			{Code: OpBody, Fn: func(*Context) { panic("body panic") }},
			{Code: OpAfterHook, Fn: func(*Context) { afterRuns++ }},
		}, PathValues{})
		if got, want := result.Panic, any("body panic"); got != want || afterRuns != 1 {
			t.Fatalf("panic = %#v, after runs = %d", got, afterRuns)
		}
	})

	t.Run("shrink probe retains original values deterministically", func(t *testing.T) {
		path := PathValues{values: []any{7}, present: []bool{true}, index: map[string]int{"value": 0}}
		program := []Instruction{{Code: OpBody, Fn: func(*Context) {}}, {Code: OpAfterHook, Fn: func(*Context) {}}}
		first := runIsolatedCase(&controlledBackend{}, program, path)
		path.values[0] = 9
		second := runIsolatedCase(&controlledBackend{}, program, path)
		if first.Path.Int("value") != 7 || second.Path.Int("value") != 9 || first.Failed != second.Failed || first.Panic != second.Panic {
			t.Fatalf("first = %#v, second = %#v", first, second)
		}
	})
}

func TestOrdinaryItKeepsGoTestSemantics(t *testing.T) {
	var order []string
	Describe(t, "ordinary", func(spec *Spec) {
		spec.BeforeEach(func(*Context) { order = append(order, "before") })
		spec.AfterEach(func(*Context) { order = append(order, "after") })
		spec.It("runs", func(*Context) { order = append(order, "body") })
	})
	if got, want := fmt.Sprint(order), "[before body after]"; got != want {
		t.Fatalf("ordinary It order = %s, want %s", got, want)
	}
}
