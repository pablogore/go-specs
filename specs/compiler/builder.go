// builder.go compiles the Describe/BeforeEach/AfterEach/It DSL into a flat Program.
// Hooks are resolved at compile time; the runner only runs steps in order.
package compiler

import "github.com/pablogore/go-specs/specs/ctx"

// SpecFn wraps a spec function with options (Focus or Skip). Pass to It: b.It("name", Focus(fn)).
type SpecFn struct {
	Fn    func(*ctx.Context)
	Skip  bool
	Focus bool
}

// scope holds hooks for one Describe level. Root scope is at index 0.
type scope struct {
	id         uint32
	beforeEach []step
	afterEach  []step
}

const maxHookKeyDepth = 32

type hookKeyLevel struct {
	scopeID   int
	beforeLen int
	afterLen  int
}

type hookKey struct {
	depth  uint8
	levels [maxHookKeyDepth]hookKeyLevel
}

type specKind int

const (
	kindNormal specKind = iota
	kindSkip
	kindFocus
	kindParallel
)

type specItem struct {
	kind    specKind
	before  []step
	spec    step
	after   []step
	steps   []step
	hookKey hookKey
}

// Builder compiles a DSL into a Program. Use NewBuilder(), then Describe/BeforeEach/AfterEach/It, then Program() or Build().
type Builder struct {
	program     *Program
	scopes      []scope
	pending     []specItem
	hasFocus    bool
	nextScopeID uint32
}

// NewBuilder creates a builder that will produce a new Program.
func NewBuilder(capacity ...int) *Builder {
	return &Builder{
		program:  &Program{Groups: nil},
		scopes:   nil,
		pending:  nil,
		hasFocus: false,
	}
}

func (b *Builder) emitBefore() []step {
	var out []step
	for i := range b.scopes {
		out = append(out, b.scopes[i].beforeEach...)
	}
	return out
}

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

func (b *Builder) emitSpecSteps(fn func(*ctx.Context)) []step {
	before := b.emitBefore()
	after := b.emitAfter()
	steps := make([]step, 0, len(before)+1+len(after))
	steps = append(steps, before...)
	steps = append(steps, step(fn))
	steps = append(steps, after...)
	return steps
}

// Describe opens a scope and runs body.
func (b *Builder) Describe(name string, body func()) {
	if body == nil {
		return
	}
	b.scopes = append(b.scopes, scope{id: b.nextScopeID})
	b.nextScopeID++
	body()
	b.scopes = b.scopes[:len(b.scopes)-1]
}

func (b *Builder) ensureScope() {
	if len(b.scopes) == 0 {
		b.scopes = append(b.scopes, scope{id: b.nextScopeID})
		b.nextScopeID++
	}
}

// BeforeEach registers a hook to run before each It in this scope.
func (b *Builder) BeforeEach(fn func(*ctx.Context)) {
	if fn == nil {
		return
	}
	b.ensureScope()
	idx := len(b.scopes) - 1
	b.scopes[idx].beforeEach = append(b.scopes[idx].beforeEach, step(fn))
}

// AfterEach registers a hook to run after each It in this scope.
func (b *Builder) AfterEach(fn func(*ctx.Context)) {
	if fn == nil {
		return
	}
	b.ensureScope()
	idx := len(b.scopes) - 1
	b.scopes[idx].afterEach = append(b.scopes[idx].afterEach, step(fn))
}

// It compiles one spec.
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
	f, ok := fn.(func(*ctx.Context))
	if !ok || f == nil {
		return
	}
	b.ensureScope()
	b.pending = append(b.pending, specItem{
		kind:    kindNormal,
		before:  b.emitBefore(),
		spec:    step(f),
		after:   b.emitAfter(),
		hookKey: b.buildHookKey(),
	})
}

// SkipIt registers a spec that is skipped at compile time.
func (b *Builder) SkipIt(name string, fn func(*ctx.Context)) {
	b.ensureScope()
	b.pending = append(b.pending, specItem{kind: kindSkip})
}

// FIt registers a focused spec.
func (b *Builder) FIt(name string, fn func(*ctx.Context)) {
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
		hookKey: b.buildHookKey(),
	})
}

// ItParallel registers a spec to run in parallel with adjacent ItParallel specs.
func (b *Builder) ItParallel(name string, fn func(*ctx.Context)) {
	if fn == nil {
		return
	}
	b.ensureScope()
	b.pending = append(b.pending, specItem{kind: kindParallel, steps: b.emitSpecSteps(fn)})
}

func (b *Builder) buildHookKey() (k hookKey) {
	if len(b.scopes) == 0 {
		return k
	}
	d := len(b.scopes)
	if d > maxHookKeyDepth {
		d = maxHookKeyDepth
	}
	k.depth = uint8(d)
	for i := 0; i < d; i++ {
		s := &b.scopes[i]
		lb, la := len(s.beforeEach), len(s.afterEach)
		if lb > 255 {
			lb = 255
		}
		if la > 255 {
			la = 255
		}
		k.levels[i] = hookKeyLevel{scopeID: int(s.id), beforeLen: lb, afterLen: la}
	}
	return k
}

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
		if curIdx >= 0 && groups[curIdx].hookKey == it.hookKey {
			groups[curIdx].specs = append(groups[curIdx].specs, it.spec)
		} else {
			groups = append(groups, group{before: it.before, specs: []step{it.spec}, after: it.after, hookKey: it.hookKey})
			curIdx = len(groups) - 1
		}
	}
	b.program.Groups = groups
}

// Program returns the compiled program.
func (b *Builder) Program() *Program {
	b.finalize()
	return b.program
}

// Build returns the compiled program. Same as Program(); kept for API compatibility.
func (b *Builder) Build() *Program {
	b.finalize()
	return b.program
}

// AddBefore registers a hook to run before each spec (flat API).
func (b *Builder) AddBefore(fn func(*ctx.Context)) {
	b.BeforeEach(fn)
}

// AddAfter registers a hook to run after each spec (flat API).
func (b *Builder) AddAfter(fn func(*ctx.Context)) {
	b.AfterEach(fn)
}

// AddSpec registers one spec (flat API). Same as It("", fn).
func (b *Builder) AddSpec(fn func(*ctx.Context)) {
	b.It("", fn)
}
