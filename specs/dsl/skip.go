package dsl

import (
	"github.com/pablogore/go-specs/specs/compiler"
	"github.com/pablogore/go-specs/specs/ctx"
)

// Skip returns a SpecFn that marks the spec as skipped.
func Skip(fn func(*ctx.Context)) compiler.SpecFn {
	return compiler.SpecFn{Fn: fn, Skip: true}
}
