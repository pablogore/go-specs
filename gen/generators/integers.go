package generators

import "math"

// Integers returns a deterministic slice of adversarial integer inputs for testing.
// Order: zero, -1, 1, math.MinInt, math.MaxInt, near boundaries, -100, 100.
func Integers() []int {
	return []int{
		0,
		-1,
		1,
		math.MinInt,
		math.MaxInt,
		math.MinInt + 1,
		math.MaxInt - 1,
		-100,
		100,
	}
}
