package property

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

const (
	sampleVarName     = "sample"
	defaultSampleSeed = int64(1)
)

// ExplorationMode defines how the generator produces values.
type ExplorationMode int

const (
	CartesianMode ExplorationMode = iota
	SamplingMode
	ExplorationGuided
)

// PathGenerator produces deterministic path combinations.
type PathGenerator struct {
	vars                      []PathVar
	filters                   []PathFilter
	index                     map[string]int
	dims                      []pathDimension
	mode                      ExplorationMode
	samples                   int
	iterations                int
	rng                       *RNG
	corpus                    []PathValues
	seenSigs                  map[uint64]struct{}
	explorationSeed           int64
	exploreCoverageIterations int
	exploreSmartIterations    int
}

type pathDimension struct {
	idx      int
	values   []any
	hasRange bool
	rangeMin int
	rangeMax int
	shrinker ValueShrinker
}

func (d pathDimension) len() int {
	if d.hasRange {
		return d.rangeMax - d.rangeMin + 1
	}
	return len(d.values)
}

func (d pathDimension) valueAt(pos int) any {
	if d.hasRange {
		return d.rangeMin + pos
	}
	return d.values[pos]
}

func (d pathDimension) randomValue(rng *RNG) any {
	length := d.len()
	if length == 0 {
		return nil
	}
	if d.hasRange {
		return d.rangeMin + rng.Intn(length)
	}
	return d.values[rng.Intn(length)]
}

// NewPathGeneratorWithIntRange creates a PathGenerator with a single int dimension (for benchmarks and tests).
// Returns nil if name is empty or max < min.
func NewPathGeneratorWithIntRange(name string, min, max int) *PathGenerator {
	if name == "" || max < min {
		return nil
	}
	vars := []PathVar{IntRangeVar(name, min, max)}
	return NewPathGenerator(vars, nil, 0, 0, false, 0, 0, 0)
}

// PathValuesWith builds PathValues with the given name->value map for this generator's dimensions.
// Used by benchmarks and tests to create a specific failing input.
func (g *PathGenerator) PathValuesWith(values map[string]any) PathValues {
	if g == nil || len(g.index) == 0 {
		return PathValues{}
	}
	pv := PathValues{
		values:  make([]any, len(g.index)),
		present: make([]bool, len(g.index)),
		index:   g.index,
	}
	for name, val := range values {
		if idx, ok := g.index[name]; ok && idx < len(pv.values) {
			pv.values[idx] = val
			pv.present[idx] = true
		}
	}
	return pv
}

func NewPathGenerator(vars []PathVar, filters []PathFilter, samples int, seed int64, hasSeed bool, exploreIterations int, exploreCoverageIterations int, exploreSmartIterations int) *PathGenerator {
	cloned := append([]PathVar(nil), vars...)
	mode := CartesianMode
	if exploreIterations > 0 {
		mode = ExplorationGuided
	} else if samples > 0 {
		mode = SamplingMode
	}
	if mode == SamplingMode {
		cloned = append([]PathVar{{Name: sampleVarName}}, cloned...)
	}
	index := make(map[string]int, len(cloned))
	nextIdx := 0
	for i := range cloned {
		if cloned[i].rangeSpec != nil {
			r := *cloned[i].rangeSpec
			cloned[i].rangeSpec = &r
		}
		if len(cloned[i].Values) > 0 {
			cloned[i].Values = append([]any(nil), cloned[i].Values...)
		}
		name := cloned[i].Name
		if name == "" {
			continue
		}
		if _, exists := index[name]; exists {
			continue
		}
		index[name] = nextIdx
		nextIdx++
	}
	dims := make([]pathDimension, 0, len(cloned))
	for _, v := range cloned {
		idx, ok := index[v.Name]
		if !ok {
			continue
		}
		dim := pathDimension{idx: idx, shrinker: v.Shrinker}
		switch {
		case len(v.Values) > 0:
			dim.values = append([]any(nil), v.Values...)
		case v.rangeSpec != nil:
			dim.hasRange = true
			dim.rangeMin = v.rangeSpec.min
			dim.rangeMax = v.rangeSpec.max
		}
		if dim.len() == 0 {
			continue
		}
		dims = append(dims, dim)
	}
	var rng *RNG
	explorationSeed := defaultSampleSeed
	if hasSeed {
		explorationSeed = seed
	}
	if mode == SamplingMode || mode == ExplorationGuided {
		rng = NewRNG(uint64(explorationSeed))
	}
	seenSigs := make(map[uint64]struct{})
	if mode == ExplorationGuided {
		seenSigs = make(map[uint64]struct{})
	}
	return &PathGenerator{
		vars:                      cloned,
		filters:                   append([]PathFilter(nil), filters...),
		index:                     index,
		dims:                      dims,
		mode:                      mode,
		samples:                   samples,
		iterations:                exploreIterations,
		rng:                       rng,
		seenSigs:                  seenSigs,
		explorationSeed:           explorationSeed,
		exploreCoverageIterations: exploreCoverageIterations,
		exploreSmartIterations:    exploreSmartIterations,
	}
}

// PathIterator walks the Cartesian product using a single mutable index (odometer style).
// Use Iterator() for CartesianMode to avoid per-path allocations; Index() is the current position.
type PathIterator struct {
	dims  []int
	index []int
	done  bool
}

// Iterator returns a zero-allocation Cartesian iterator for CartesianMode with no filters, or nil otherwise.
// When non-nil, the caller should reuse a single PathValues and call g.FillPathValues(it.Index(), pv) each iteration.
func (g *PathGenerator) Iterator() *PathIterator {
	if g == nil || g.mode != CartesianMode || len(g.filters) > 0 {
		return nil
	}
	dims := make([]int, len(g.dims))
	for i := range g.dims {
		dims[i] = g.dims[i].len()
	}
	return &PathIterator{
		dims:  dims,
		index: make([]int, len(dims)),
	}
}

// Next advances to the next combination (odometer style). Returns false when all combinations have been visited.
func (it *PathIterator) Next() bool {
	if it == nil || it.done {
		return false
	}
	for i := len(it.index) - 1; i >= 0; i-- {
		it.index[i]++
		if it.index[i] < it.dims[i] {
			return true
		}
		it.index[i] = 0
	}
	it.done = true
	return false
}

// Index returns the current position (one index per dimension). Do not modify; valid until next Next() or iterator reuse.
func (it *PathIterator) Index() []int {
	if it == nil {
		return nil
	}
	return it.index
}

// FillPathValues fills pv from the given index using the generator's dimensions (CartesianMode).
// Reuses pv's buffers; call reset first via the generator's index map. Only call when g.mode == CartesianMode.
func (g *PathGenerator) FillPathValues(index []int, pv *PathValues) {
	if g == nil || pv == nil || len(index) != len(g.dims) {
		return
	}
	pv.reset(g.index)
	for dimIdx, dim := range g.dims {
		if dimIdx >= len(index) {
			continue
		}
		pv.present[dim.idx] = true
		pv.values[dim.idx] = dim.valueAt(index[dimIdx])
	}
}

// ForEach iterates over every allowed combination in declaration order.
// ForEach runs fn for each path combination. tb is used to report failures (e.g. sampling could not satisfy filters).
func (g *PathGenerator) ForEach(tb testing.TB, fn func(PathValues)) {
	if g == nil {
		if fn != nil {
			fn(PathValues{})
		}
		return
	}
	if len(g.vars) == 0 || len(g.index) == 0 {
		if fn != nil {
			fn(PathValues{})
		}
		return
	}
	if g.mode == ExplorationGuided {
		g.runExploration(fn)
		return
	}
	if g.mode == SamplingMode {
		g.runSamples(tb, fn)
		return
	}
	g.enumerate(fn)
}

func (g *PathGenerator) enumerate(fn func(PathValues)) {
	pv := pathValuesPool.Get().(*PathValues)
	pv.reset(g.index)
	defer pathValuesPool.Put(pv)
	if len(g.dims) == 0 {
		if g.allow(*pv) && fn != nil {
			fn(*pv)
		}
		return
	}
	indexes := make([]int, len(g.dims))
	for {
		for dimIdx, dim := range g.dims {
			pv.present[dim.idx] = true
			pv.values[dim.idx] = dim.valueAt(indexes[dimIdx])
		}
		if g.allow(*pv) && fn != nil {
			fn(*pv)
		}
		carry := len(g.dims) - 1
		for carry >= 0 {
			indexes[carry]++
			if indexes[carry] < g.dims[carry].len() {
				break
			}
			indexes[carry] = 0
			carry--
		}
		if carry < 0 {
			break
		}
	}
}

func (g *PathGenerator) runSamples(tb testing.TB, fn func(PathValues)) {
	if g.samples <= 0 {
		return
	}
	pv := pathValuesPool.Get().(*PathValues)
	pv.reset(g.index)
	defer pathValuesPool.Put(pv)
	sampleIdx, hasSampleVar := g.index[sampleVarName]
	if hasSampleVar {
		pv.present[sampleIdx] = true
	}
	maxAttempts := g.samples * 100
	if maxAttempts < g.samples {
		maxAttempts = g.samples
	}
	executed := 0
	attempts := 0
	for executed < g.samples {
		attempts++
		if attempts > maxAttempts {
			tb.Helper()
			tb.Fatalf("specs: sampling could not satisfy filters after %d attempts", attempts)
		}
		for _, dim := range g.dims {
			pv.present[dim.idx] = true
			pv.values[dim.idx] = dim.randomValue(g.rng)
		}
		if hasSampleVar {
			pv.values[sampleIdx] = executed + 1
		}
		if !g.allow(*pv) {
			continue
		}
		if fn != nil {
			fn(*pv)
		}
		executed++
	}
}

func (g *PathGenerator) runExploration(fn func(PathValues)) {
	if g.iterations <= 0 || g.rng == nil {
		return
	}
	pv := PathValues{
		values:  make([]any, len(g.index)),
		present: make([]bool, len(g.index)),
		index:   g.index,
	}
	g.corpus = make([]PathValues, 0, g.iterations/10+1)
	for i := range g.dims {
		pv.present[g.dims[i].idx] = true
		pv.values[g.dims[i].idx] = g.dims[i].randomValue(g.rng)
	}
	if g.allow(pv) {
		g.corpus = append(g.corpus, pv.clone())
	}
	executed := 0
	for executed < g.iterations {
		var candidate PathValues
		if len(g.corpus) > 0 && g.rng.Float64() > 0.3 {
			candidate = g.mutate(g.corpus[g.rng.Intn(len(g.corpus))])
		} else {
			candidate = g.randomInput()
		}
		if !g.allow(candidate) {
			continue
		}
		if fn != nil {
			fn(candidate)
		}
		executed++
		sig := captureSignature()
		if _, seen := g.seenSigs[sig]; !seen {
			g.seenSigs[sig] = struct{}{}
			g.corpus = append(g.corpus, candidate.clone())
		}
	}
}

func (g *PathGenerator) randomInput() PathValues {
	return g.RandomInput(g.rng)
}

// RandomInput returns a random PathValues using the given RNG (for coverage exploration).
func (g *PathGenerator) RandomInput(rng *RNG) PathValues {
	if g == nil {
		return PathValues{}
	}
	if rng == nil {
		return PathValues{}
	}
	pv := PathValues{
		values:  make([]any, len(g.index)),
		present: make([]bool, len(g.index)),
		index:   g.index,
	}
	for _, dim := range g.dims {
		pv.present[dim.idx] = true
		pv.values[dim.idx] = dim.randomValue(rng)
	}
	return pv
}

func (g *PathGenerator) mutate(input PathValues) PathValues {
	mutated := input.clone()
	if len(g.dims) == 0 {
		return mutated
	}
	dimIdx := g.rng.Intn(len(g.dims))
	dim := g.dims[dimIdx]
	mutated.present[dim.idx] = true
	switch v := mutated.values[dim.idx].(type) {
	case int:
		choice := g.rng.Intn(4)
		switch choice {
		case 0:
			mutated.values[dim.idx] = v + 1
		case 1:
			mutated.values[dim.idx] = v - 1
		case 2:
			if dim.hasRange && dim.rangeMax > dim.rangeMin {
				mid := dim.rangeMin + (dim.rangeMax-dim.rangeMin)/2
				mutated.values[dim.idx] = mid
			} else if len(dim.values) > 1 {
				mutated.values[dim.idx] = dim.values[(g.rng.Intn(len(dim.values)))].(int)
			}
		default:
			mutated.values[dim.idx] = dim.randomValue(g.rng)
		}
	case bool:
		mutated.values[dim.idx] = !v
	default:
		mutated.values[dim.idx] = dim.randomValue(g.rng)
	}
	return mutated
}

func captureSignature() uint64 {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(4, pcs)
	var hash uint64
	for i := 0; i < n; i++ {
		hash ^= uint64(pcs[i])
		hash *= 31
	}
	return hash
}

func (g *PathGenerator) allow(values PathValues) bool {
	for _, filter := range g.filters {
		if filter == nil {
			continue
		}
		if !filter(values) {
			return false
		}
	}
	return true
}

// NumDims returns the number of path dimensions (for use by Shrinker).
func (g *PathGenerator) NumDims() int {
	if g == nil {
		return 0
	}
	return len(g.dims)
}

// ValueIndex returns the PathValues index for dimension dim. Returns -1 if dim is out of range.
func (g *PathGenerator) ValueIndex(dim int) int {
	if g == nil || dim < 0 || dim >= len(g.dims) {
		return -1
	}
	return g.dims[dim].idx
}

// DimensionBounds returns the min and max (inclusive) for dimension dim when it has an int range.
// If hasRange is false, the dimension uses discrete values (no single min/max for mutation clamping).
func (g *PathGenerator) DimensionBounds(dim int) (min, max int, hasRange bool) {
	if g == nil || dim < 0 || dim >= len(g.dims) {
		return 0, 0, false
	}
	d := g.dims[dim]
	if !d.hasRange {
		return 0, 0, false
	}
	return d.rangeMin, d.rangeMax, true
}

// ForEachShrinkCandidate calls fn for each candidate PathValues with dimension dimIdx
// shrunk via the dimension's value shrinker. Only candidates that pass the generator's
// filters are passed to fn. If fn returns false, iteration stops. pv is not modified.
func (g *PathGenerator) ForEachShrinkCandidate(pv PathValues, dimIdx int, fn func(PathValues) bool) {
	if g == nil || dimIdx < 0 || dimIdx >= len(g.dims) || fn == nil {
		return
	}
	dim := g.dims[dimIdx]
	val := pv.values[dim.idx]
	var vs ValueShrinker
	if dim.shrinker != nil {
		vs = dim.shrinker
	} else {
		vs = g.defaultShrinkerFor(val)
	}
	if vs == nil {
		return
	}
	for _, cand := range vs.Shrink(val) {
		try := pv.clone()
		try.values[dim.idx] = cand
		try.present[dim.idx] = true
		if !g.allow(try) {
			continue
		}
		if !fn(try) {
			return
		}
	}
}

// FormatPathValuesForReport returns a multiline string for reporting minimal failing input (e.g. "x = 1\ny = 0").
func (g *PathGenerator) FormatPathValuesForReport(values PathValues) string {
	if g == nil || values.len() == 0 {
		return ""
	}
	var b strings.Builder
	first := true
	for _, v := range g.vars {
		val, ok := values.lookup(v.Name)
		if !ok {
			continue
		}
		if !first {
			b.WriteByte('\n')
		}
		first = false
		b.WriteString(v.Name)
		b.WriteString(" = ")
		if val == nil {
			b.WriteString("<nil>")
			continue
		}
		b.WriteString(toString(val))
	}
	return b.String()
}

// FormatName appends combination metadata to the base spec name.
func (g *PathGenerator) FormatName(base string, values PathValues) string {
	if g == nil || values.len() == 0 {
		return base
	}
	var b strings.Builder
	b.WriteString(base)
	b.WriteString(" [")
	first := true
	for _, v := range g.vars {
		val, ok := values.lookup(v.Name)
		if !ok {
			continue
		}
		if !first {
			b.WriteByte(' ')
		}
		first = false
		b.WriteString(v.Name)
		b.WriteByte('=')
		if val == nil {
			b.WriteString("<nil>")
			continue
		}
		b.WriteString(toString(val))
	}
	b.WriteByte(']')
	return b.String()
}

func toString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	default:
		return strings.TrimSpace(strings.ReplaceAll(fmt.Sprintf("%v", value), "\n", " "))
	}
}

// ShrinkValues reduces current toward a minimal failing case (exported for tests).
func (g *PathGenerator) ShrinkValues(current PathValues, check func(PathValues) bool) PathValues {
	best := current.clone()
	improved := true
	for improved {
		improved = false
		for _, dim := range g.dims {
			val := best.values[dim.idx]
			var shrinker ValueShrinker
			if dim.shrinker != nil {
				shrinker = dim.shrinker
			} else {
				shrinker = g.defaultShrinkerFor(val)
			}
			if shrinker == nil {
				continue
			}
			candidates := shrinker.Shrink(val)
			for _, cand := range candidates {
				try := best.clone()
				try.values[dim.idx] = cand
				try.present[dim.idx] = true
				if g.allow(try) && !check(try) {
					best = try
					improved = true
					break
				}
			}
			if improved {
				break
			}
		}
	}
	return best
}

func (g *PathGenerator) defaultShrinkerFor(value any) ValueShrinker {
	switch value.(type) {
	case int, int64:
		return DefaultIntShrinker
	case bool:
		return DefaultBoolShrinker
	case float64:
		return DefaultFloatShrinker
	}
	return nil
}
