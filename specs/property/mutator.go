package property

// Mutator applies fast, deterministic mutations to PathValues for exploration.
type Mutator struct {
	Rng *RNG
}

// NewMutator returns a mutator with a deterministic RNG (seed for reproducibility).
func NewMutator(seed int64) *Mutator {
	return &Mutator{Rng: NewRNG(uint64(seed))}
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// MutateInt returns a mutation of v: +1, -1, ×2, ÷2, bit flip, or random nearby.
func (m *Mutator) MutateInt(v, min, max int) int {
	if m == nil || m.Rng == nil {
		return clampInt(v, min, max)
	}
	const numOps = 6
	op := m.Rng.Intn(numOps)
	var out int
	switch op {
	case 0:
		out = v + 1
	case 1:
		out = v - 1
	case 2:
		out = v * 2
	case 3:
		if v != 0 {
			out = v / 2
		} else {
			out = 0
		}
	case 4:
		bit := uint(m.Rng.Intn(32))
		out = v ^ (1 << bit)
	case 5:
		delta := m.Rng.Intn(11) - 5
		out = v + delta
	default:
		out = v
	}
	return clampInt(out, min, max)
}

// Mutate returns a clone of p with one random dimension mutated.
func (m *Mutator) Mutate(gen *PathGenerator, p PathValues) PathValues {
	if m == nil || m.Rng == nil || gen == nil {
		return p.clone()
	}
	nd := gen.NumDims()
	if nd == 0 {
		return p.clone()
	}
	dim := m.Rng.Intn(nd)
	idx := gen.ValueIndex(dim)
	if idx < 0 || idx >= len(p.values) {
		return p.clone()
	}
	out := p.clone()
	val := p.values[idx]
	min, max, hasRange := gen.DimensionBounds(dim)

	switch v := val.(type) {
	case int:
		if hasRange {
			out.values[idx] = m.MutateInt(v, min, max)
		} else {
			out.values[idx] = v
		}
		out.present[idx] = true
	case int64:
		if hasRange {
			mi, ma := int(min), int(max)
			out.values[idx] = int64(m.MutateInt(int(v), mi, ma))
		} else {
			out.values[idx] = v
		}
		out.present[idx] = true
	default:
	}
	return out
}
