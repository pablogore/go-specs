package specs

// mutation_strategy_test.go locks in the documented divergence between the two independently-
// reachable mutation strategies for property-based exploration (see #9, and the doc comments on
// Mutator.Mutate and PathGenerator.mutate): Mutator.Mutate (used by PathSpec.ExploreCoverage/
// ExploreSmart) is a no-op for a bool dimension, while PathGenerator.mutate (used by
// PathSpec.Explore) unconditionally negates a bool dimension. With a single bool dimension, the
// dimension pick is deterministic (there's nothing else to pick), so this needs no seeding tricks
// to be reliable.

import "testing"

func TestMutationStrategies_DivergeOnBoolDimension(t *testing.T) {
	// exploreIterations > 0 puts the generator in ExplorationGuided mode, which is what allocates
	// g.rng — PathGenerator.mutate (unlike Mutator.Mutate, which carries its own rng) uses it.
	gen := newPathGenerator([]PathVar{{Name: "flag", Values: []any{true, false}}}, nil, 0, 1, true, 1, 0, 0)
	input := gen.PathValuesWith(map[string]any{"flag": true})

	out := NewMutator(1).Mutate(gen, input)
	if out.values[0] != true {
		t.Errorf("Mutator.Mutate: expected the bool dimension to stay unchanged (documented no-op for non-int dimensions), got %v", out.values[0])
	}

	genMutated := gen.mutate(input)
	if genMutated.values[0] != false {
		t.Errorf("PathGenerator.mutate: expected the bool dimension to be negated to false, got %v", genMutated.values[0])
	}
}
