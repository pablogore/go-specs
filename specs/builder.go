// builder.go compiles the Describe/BeforeEach/AfterEach/It DSL into a flat Program.
// Hooks are resolved at compile time; the runner only runs steps in order.
package specs

import "fmt"

// scope holds hooks for one Describe level. Root scope is at index 0.
type scope struct {
	beforeEach []step
	afterEach  []step
}

// specKind describes how a spec was registered (normal, skip, focus, parallel).
type specKind int

const (
	kindNormal specKind = iota
	kindSkip
	kindFocus
	kindParallel
)

// specMeta is the metadata tracked per spec at registration (focus/skip). Used by It/FIt/SkipIt.
// The builder maps this to specItem (with before/after/hookKey) for compilation.
type specMeta struct {
	Name  string
	Fn    step
	Focus bool
	Skip  bool
}

// specItem is one registered spec (or skip). Before/after enable coalescing; hookKey identifies the scope set.
type specItem struct {
	kind    specKind
	before  []step
	spec    step   // single spec body; nil for skip
	after   []step
	steps   []step // full sequence only for kindParallel (before+fn+after flattened)
	hookKey string // from b.hookKey() for coalescing without comparing funcs
}

// Builder compiles a DSL into a Program. Use NewBuilder(), then Describe/BeforeEach/AfterEach/It, then Program() or Build().
type Builder struct {
	program  *Program
	scopes   []scope
	pending  []specItem
	hasFocus bool
}

// NewBuilder creates a builder that will produce a new Program.
// The optional capacity hint is ignored (kept for API compatibility with flat usage).
func NewBuilder(capacity ...int) *Builder {
	return &Builder{
		program:  &Program{Groups: nil},
		scopes:   nil,
		pending:  nil,
		hasFocus: false,
	}
}

// emitBefore returns current scope beforeEach in order (outer → inner).
func (b *Builder) emitBefore() []step {
	var out []step
	for i := range b.scopes {
		out = append(out, b.scopes[i].beforeEach...)
	}
	return out
}

// emitAfter returns afterEach in inner-to-outer order (innermost scope first).
// Within each scope, hooks are in declaration order. The runner iterates the
// result in reverse, so: flat case [after1, after2] => runs after2, after1 (LIFO);
// nested case [afterInner, afterOuter] => runs afterOuter, afterInner.
func (b *Builder) emitAfter() []step {
	var out []step
	for i := len(b.scopes) - 1; i >= 0; i-- {
		ae := b.scopes[i].afterEach
		for j := 0; j < len(ae); j++ {
			out = append(out, ae[j])
		}
	}
	return out
}

// emitSpecSteps returns the full step sequence for one spec (used for parallel runAll).
func (b *Builder) emitSpecSteps(fn func(*Context)) []step {
	before := b.emitBefore()
	after := b.emitAfter()
	steps := make([]step, 0, len(before)+1+len(after))
	steps = append(steps, before...)
	steps = append(steps, step(fn))
	steps = append(steps, after...)
	return steps
}

// Describe opens a scope and runs body. Nested Describes push inner scopes; hooks apply to inner It.
func (b *Builder) Describe(name string, body func()) {
	if body == nil {
		return
	}
	b.scopes = append(b.scopes, scope{})
	body()
	b.scopes = b.scopes[:len(b.scopes)-1]
}

// ensureScope ensures at least one scope exists (for flat AddBefore/AddAfter/AddSpec without Describe).
func (b *Builder) ensureScope() {
	if len(b.scopes) == 0 {
		b.scopes = append(b.scopes, scope{})
	}
}

// BeforeEach registers a hook to run before each It in this scope (and nested scopes). Prepended before the spec.
func (b *Builder) BeforeEach(fn func(*Context)) {
	if fn == nil {
		return
	}
	b.ensureScope()
	idx := len(b.scopes) - 1
	b.scopes[idx].beforeEach = append(b.scopes[idx].beforeEach, step(fn))
}

// AfterEach registers a hook to run after each It in this scope (and nested scopes). Appended after the spec.
func (b *Builder) AfterEach(fn func(*Context)) {
	if fn == nil {
		return
	}
	b.ensureScope()
	idx := len(b.scopes) - 1
	b.scopes[idx].afterEach = append(b.scopes[idx].afterEach, step(fn))
}

// It compiles one spec: prepend all BeforeEach (outer to inner), append the spec, append all AfterEach (inner to outer).
// Order is deterministic; emitted at build time via pending.
// fn may be func(*Context) or SpecFn (e.g. It("name", Skip(fn)) or It("name", Focus(fn))).
func (b *Builder) It(name string, fn interface{}) {
	if fn == nil {
		return
	}
	if s, ok := fn.(SpecFn); ok {
		if s.Skip {
			b.SkipIt(name, s.Fn)
			return
		}
		if s.Focus {
			b.FIt(name, s.Fn)
			return
		}
		fn = s.Fn
	}
	f, ok := fn.(func(*Context))
	if !ok || f == nil {
		return
	}
	b.ensureScope()
	b.pending = append(b.pending, specItem{
		kind:    kindNormal,
		before:  b.emitBefore(),
		spec:    step(f),
		after:   b.emitAfter(),
		hookKey: b.hookKey(),
	})
}

// SkipIt registers a spec that is skipped at compile time (no steps emitted).
func (b *Builder) SkipIt(name string, fn func(*Context)) {
	b.ensureScope()
	b.pending = append(b.pending, specItem{kind: kindSkip})
}

// FIt registers a focused spec. If any spec is focused, only focused specs are compiled into the program.
func (b *Builder) FIt(name string, fn func(*Context)) {
	if fn == nil {
		return
	}
	b.ensureScope()
	b.hasFocus = true
	b.pending = append(b.pending, specItem{
		kind:    kindFocus,
		before:  b.emitBefore(),
		spec:    step(fn),
		after:   b.emitAfter(),
		hookKey: b.hookKey(),
	})
}

// ItParallel registers a spec to run in parallel with adjacent ItParallel specs; grouped into one step at build time.
func (b *Builder) ItParallel(name string, fn func(*Context)) {
	if fn == nil {
		return
	}
	b.ensureScope()
	b.pending = append(b.pending, specItem{kind: kindParallel, steps: b.emitSpecSteps(fn)})
}

// hookKey returns a key that uniquely identifies the current scope stack and hook set.
// Same scope stack (same Describe path) => same key, so specs in the same Describe coalesce.
// Uses scope pointer identity so different Describes do not coalesce.
func (b *Builder) hookKey() string {
	if len(b.scopes) == 0 {
		return ""
	}
	// Use scope pointer + lengths so we don't compare func values (Go forbids).
	var buf []byte
	for i := range b.scopes {
		s := &b.scopes[i]
		lb, la := len(s.beforeEach), len(s.afterEach)
		buf = append(buf, fmt.Sprintf("%p,%d,%d", s, lb, la)...)
	}
	return string(buf)
}

// finalize builds program.Groups from pending: focus filter, drop skip, coalesce same-hook specs, group parallel.
func (b *Builder) finalize() {
	items := b.pending
	if b.hasFocus {
		filtered := items[:0]
		for i := range items {
			if items[i].kind == kindFocus {
				filtered = append(filtered, items[i])
			}
		}
		items = filtered
	}
	var groups []group
	curIdx := -1
	for i := 0; i < len(items); i++ {
		it := items[i]
		if it.kind == kindSkip {
			continue
		}
		if it.kind == kindParallel {
			curIdx = -1
			parSteps := []step{runAll(it.steps)}
			j := i + 1
			for j < len(items) && items[j].kind == kindParallel {
				parSteps = append(parSteps, runAll(items[j].steps))
				j++
			}
			groups = append(groups, group{specs: []step{parallelStep(parSteps)}})
			i = j - 1
			continue
		}
		// normal or focus: coalesce if same hook set (same scope layout) as current group
		if curIdx >= 0 && groups[curIdx].hookKey == it.hookKey {
			groups[curIdx].specs = append(groups[curIdx].specs, it.spec)
		} else {
			groups = append(groups, group{before: it.before, specs: []step{it.spec}, after: it.after, hookKey: it.hookKey})
			curIdx = len(groups) - 1
		}
	}
	b.program.Groups = groups
}

// Program returns the compiled program. Safe to call multiple times; do not modify the returned Program's Groups.
func (b *Builder) Program() *Program {
	b.finalize()
	return b.program
}

// Build returns the compiled program. Same as Program(); kept for API compatibility with flat usage.
func (b *Builder) Build() *Program {
	b.finalize()
	return b.program
}

// AddBefore registers a hook to run before each spec (flat API, single scope). Same as BeforeEach in one implicit scope.
func (b *Builder) AddBefore(fn func(*Context)) {
	b.BeforeEach(fn)
}

// AddAfter registers a hook to run after each spec (flat API). Same as AfterEach in one implicit scope.
func (b *Builder) AddAfter(fn func(*Context)) {
	b.AfterEach(fn)
}

// AddSpec registers one spec (flat API). Same as It("", fn).
func (b *Builder) AddSpec(fn func(*Context)) {
	b.It("", fn)
}

