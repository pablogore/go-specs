package dsl

import "github.com/pablogore/go-specs/specs/compiler"

// BuildProgram compiles a test suite from a function that configures the Builder.
// Returns the compiled Program; run it with runner.NewRunner(prog).Run(tb).
func BuildProgram(fn func(*compiler.Builder)) *compiler.Program {
	if fn == nil {
		return &compiler.Program{}
	}
	b := compiler.NewBuilder()
	fn(b)
	return b.Build()
}
