// skip.go provides skipped-spec support. SkipIt and Skip() are compile-time only;
// skipped specs are not emitted to the program.
package specs

// Skip returns a SpecFn that marks the spec as skipped. Use with It: b.It("name", specs.Skip(fn)).
func Skip(fn func(*Context)) SpecFn {
	return SpecFn{Fn: fn, Skip: true}
}
