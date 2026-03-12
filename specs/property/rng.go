// rng.go provides a fast deterministic RNG for reproducible property/fuzz runs.
// Uses xorshift64*: no allocations, no reflection, no global state.
package property

// RNG is a deterministic PRNG. Same seed always produces the same sequence.
type RNG struct {
	state uint64
}

// NewRNG returns an RNG seeded with seed. Seed 0 is valid (splitmix64 mix).
func NewRNG(seed uint64) *RNG {
	if seed == 0 {
		seed = 0x9e3779b97f4a7c15 // splitmix64-style default
	}
	return &RNG{state: seed}
}

// Next returns the next 64-bit value and advances state. Allocation-free.
func (r *RNG) Next() uint64 {
	x := r.state
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	r.state = x
	return x * 0x2545f4914f6cdd1d
}

// Intn returns a uniform value in [0, n). Returns 0 if n <= 0. Allocation-free.
func (r *RNG) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.Next() % uint64(n))
}

// Int63 returns a non-negative int64 in [0, 1<<63). Allocation-free.
func (r *RNG) Int63() int64 {
	return int64(r.Next() >> 1)
}

// Float64 returns a float64 in [0, 1). Allocation-free.
func (r *RNG) Float64() float64 {
	return float64(r.Next()>>11) / (1 << 53)
}
