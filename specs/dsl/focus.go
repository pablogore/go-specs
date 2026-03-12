package dsl

import (
	"github.com/pablogore/go-specs/specs/compiler"
	"github.com/pablogore/go-specs/specs/ctx"
)

// Focus returns a SpecFn that marks the spec as focused.
func Focus(fn func(*ctx.Context)) compiler.SpecFn {
	return compiler.SpecFn{Fn: fn, Focus: true}
}
