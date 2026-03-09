// parallel.go documents parallel spec execution. ItParallel is implemented in the Builder.
package specs

//
// consecutive ItParallel specs are compiled into a single parallelStep (see program.go).
//
// Example:
//
//	b.It("A", fnA)
//	b.ItParallel("B", fnB)
//	b.ItParallel("C", fnC)
//	b.It("D", fnD)
//
// Compiled program: [group A], [group with parallelStep(B,C)], [group D].
// The runner executes one step per group; the parallel step runs B and C in separate goroutines.
