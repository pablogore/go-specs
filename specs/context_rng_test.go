package specs

import (
	"os"
	"testing"
)

func TestContextRNGDeterminism(t *testing.T) {
	// Deterministic run: same seed yields same RandomInt63 sequence.
	os.Setenv("GO_SPECS_SEED", "12345")
	defer os.Unsetenv("GO_SPECS_SEED")
	s := NewSpecForTest(t, false, nil)
	first := generateRNGSequence(t, s)
	second := generateRNGSequence(t, s)
	if len(first) != len(second) {
		t.Fatalf("sequence length mismatch: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("rng sequence mismatch at %d: %d vs %d", i, first[i], second[i])
		}
	}
}

func generateRNGSequence(t *testing.T, s *Spec) []int64 {
	values := make([]int64, 10)
	s.Describe("seq", func(spec *Spec) {
		spec.It("reads", func(ctx *Context) {
			for i := range values {
				values[i] = ctx.RandomInt63()
			}
		})
	})
	return append([]int64(nil), values...)
}

// TestRandomSeedDeterministic verifies that Spec.RandomSeed(seed) propagates to the runner
// so that Context uses the given seed and RandomInt63() is deterministic across runs.
func TestRandomSeedDeterministic(t *testing.T) {
	// First value from xorshift64* with seed 42 (from specs/property/rng.go).
	const seed42FirstInt63 = int64(3127509542104846800)

	Describe(t, "seed", func(s *Spec) {
		s.RandomSeed(42)

		s.It("deterministic", func(ctx *Context) {
			v := ctx.RandomInt63()
			ctx.Expect(v).To(Equal(seed42FirstInt63))
		})
	})
}
