// context_reexport re-exports context package types and functions so the public API remains specs.Context, specs.Expect, etc.
package specs

import pkgcontext "github.com/pablogore/go-specs/specs/ctx"

// Context and related types (implementation in specs/ctx).
type (
	Context    = pkgcontext.Context
	Expectation = pkgcontext.Expectation
	Fixture     = pkgcontext.Fixture
	TestBackend = pkgcontext.TestBackend
)

// Context and expectation constructors/helpers.
var (
	NewContext     = pkgcontext.NewContext
	GetRunSeed     = pkgcontext.GetRunSeed
	AsTestBackend  = pkgcontext.AsTestBackend
	PutTestBackend = pkgcontext.PutTestBackend
)

// RunBeforeHooks and RunAfterHooks (for tests / registry path).
var (
	RunBeforeHooks = pkgcontext.RunBeforeHooks
	RunAfterHooks  = pkgcontext.RunAfterHooks
	RunSnapshot    = pkgcontext.RunSnapshot
)

// Matchers (re-exported from context). The matcher type is assert.Matcher; use assert for the type or these constructors.
var (
	Equal    = pkgcontext.Equal
	NotEqual = pkgcontext.NotEqual
	BeNil    = pkgcontext.BeNil
	BeTrue   = pkgcontext.BeTrue
	BeFalse  = pkgcontext.BeFalse
	Contain  = pkgcontext.Contain
)

// EqualTo asserts that actual equals expected (generic helper).
func EqualTo[T comparable](c *Context, actual, expected T) {
	pkgcontext.EqualTo(c, actual, expected)
}

// ExpectT returns a typed expectation for comparable types.
func ExpectT[T comparable](c *Context, v T) pkgcontext.ExpectResult[T] {
	return pkgcontext.ExpectT(c, v)
}
