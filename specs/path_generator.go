package specs

import (
	"fmt"
	"math/rand"
	"runtime"
	"strings"
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

// explorationStrategy selects which candidate-generation strategy runExploration uses in
// ExplorationGuided mode. Set from which of .Explore/.ExploreCoverage/.ExploreSmart was called.
type explorationStrategy int

const (
	strategyPlain    explorationStrategy = iota // PathGenerator.mutate; PathSpec.Explore
	strategyCoverage                            // CoverageExplorer; PathSpec.ExploreCoverage
	strategySmart                               // SmartExplorer; PathSpec.ExploreSmart
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
	rng                       *rand.Rand
	corpus                    []PathValues
	seenSigs                  map[uint64]struct{}
	explorationSeed           int64
	exploreCoverageIterations int
	exploreSmartIterations    int
	strategy                  explorationStrategy
	coverageExplorer          *CoverageExplorer
	smartExplorer             *SmartExplorer
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

func (d pathDimension) randomValue(rng *rand.Rand) any {
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
	rs := intRange{min: min, max: max}
	vars := []PathVar{{Name: name, rangeSpec: &rs}}
	return newPathGenerator(vars, nil, 0, 0, false, 0, 0, 0)
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

func newPathGenerator(vars []PathVar, filters []PathFilter, samples int, seed int64, hasSeed bool, exploreIterations int, exploreCoverageIterations int, exploreSmartIterations int) *PathGenerator {
	cloned := append([]PathVar(nil), vars...)
	mode := CartesianMode
	iterations := 0
	strategy := strategyPlain
	switch {
	// Precedence when more than one is set on the same PathSpec (not the intended usage, but
	// deterministic beats silently picking one arbitrarily): Explore, then ExploreCoverage, then
	// ExploreSmart, then Sample.
	case exploreIterations > 0:
		mode = ExplorationGuided
		iterations = exploreIterations
		strategy = strategyPlain
	case exploreCoverageIterations > 0:
		mode = ExplorationGuided
		iterations = exploreCoverageIterations
		strategy = strategyCoverage
	case exploreSmartIterations > 0:
		mode = ExplorationGuided
		iterations = exploreSmartIterations
		strategy = strategySmart
	case samples > 0:
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
		dim := pathDimension{idx: idx, shrinker: v.shrinker}
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
	var rng *rand.Rand
	explorationSeed := defaultSampleSeed
	if hasSeed {
		explorationSeed = seed
	}
	if mode == SamplingMode || mode == ExplorationGuided {
		rng = rand.New(rand.NewSource(explorationSeed))
	}
	seenSigs := make(map[uint64]struct{})
	if mode == ExplorationGuided {
		seenSigs = make(map[uint64]struct{})
	}
	var coverageExplorer *CoverageExplorer
	var smartExplorer *SmartExplorer
	switch strategy {
	case strategyCoverage:
		coverageExplorer = NewCoverageExplorer(explorationSeed)
	case strategySmart:
		smartExplorer = NewSmartExplorer(explorationSeed)
	}
	return &PathGenerator{
		vars:                      cloned,
		filters:                   append([]PathFilter(nil), filters...),
		index:                     index,
		dims:                      dims,
		mode:                      mode,
		samples:                   samples,
		iterations:                iterations,
		rng:                       rng,
		seenSigs:                  seenSigs,
		explorationSeed:           explorationSeed,
		exploreCoverageIterations: exploreCoverageIterations,
		exploreSmartIterations:    exploreSmartIterations,
		strategy:                  strategy,
		coverageExplorer:          coverageExplorer,
		smartExplorer:             smartExplorer,
	}
}

// PathIterator walks the Cartesian product using a single mutable index (odometer style).
// Use Iterator() for CartesianMode to avoid per-path allocations; Index() is the current position.
type PathIterator struct {
	dims    []int
	index   []int
	started bool
	done    bool
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
// The first call yields the zero index (the first combination) without advancing it; every
// call after that advances the odometer before reporting whether a combination remains.
func (it *PathIterator) Next() bool {
	if it == nil || it.done {
		return false
	}
	if !it.started {
		it.started = true
		for _, d := range it.dims {
			if d == 0 {
				it.done = true
				return false
			}
		}
		return true
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

// pathSequence drives a PathGenerator one candidate at a time, replacing the ForEach-into-slice
// materialization that runExecutionContext used to do before the bounded controller ever saw a
// candidate. next() must do generation work only — it never depends on, or is influenced by, the
// result of executing a previously proposed candidate. admitFeedback reports that result back to
// the generator afterward, so guided strategies can update corpus/novelty state only once the
// real case has actually run.
//
// CartesianMode, SamplingMode, and all three ExplorationGuided strategies (Explore/ExploreCoverage/
// ExploreSmart) are supported.
type pathSequence struct {
	g       *PathGenerator
	done    bool
	trivial bool // no vars, or every var collapsed to zero dims: yields exactly one empty PathValues

	it *PathIterator // Cartesian, no filters

	indexes []int // Cartesian with filters: odometer state, mirrors enumerate()
	started bool

	// Sample: mirrors runSamples' state one call to next() at a time instead of looping to
	// completion up front. sampleAttempts/sampleMaxAttempts preserve runSamples' samples*100
	// retry budget and panic message; the budget and the filter-retry loop are both internal to
	// nextSample and never surface to the proposalController as separate Propose calls.
	sampleIdx         int
	hasSampleVar      bool
	sampleExecuted    int
	sampleAttempts    int
	sampleMaxAttempts int

	// Explore (plain strategy only): mirrors runPlainExploration's state one call to next() at a
	// time. exploreCorpus/exploreSeenSigs are local to this sequence rather than g.corpus/
	// g.seenSigs — those remain owned by the ForEach/runPlainExploration path so a direct ForEach
	// call and an incremental sequence() walk never share (and corrupt) the same corpus. The
	// filter-retry loop is internal to nextExplore, same as nextSample.
	exploreSeeded   bool
	exploreExecuted int
	exploreCorpus   []PathValues
	exploreSeenSigs map[uint64]struct{}

	// ExploreCoverage/ExploreSmart: mirrors runGuidedExploration's state one call to next() at a
	// time. Unlike exploreSeenSigs above, guidedSeenSigs is the only local copy needed — NextInput
	// is a fixed method on CoverageExplorer/SmartExplorer that always reads that explorer's own
	// e.corpus field, so corpus growth must write directly into g.coverageExplorer.corpus /
	// g.smartExplorer.corpus (the same real corpus runGuidedExploration grows) rather than a local
	// copy; there is no g.corpus-equivalent field to avoid contaminating for these two strategies.
	// guidedSeenSigs stays local so a direct ForEach call and an incremental sequence() walk never
	// share (and corrupt) the same signature set — same discipline as exploreSeenSigs vs g.seenSigs.
	guidedExecuted int
	guidedSeenSigs map[uint64]struct{}
}

// sequence returns a pathSequence for g. Panics if g uses a mode sequence() doesn't support yet.
func (g *PathGenerator) sequence() *pathSequence {
	seq := &pathSequence{g: g}
	if g == nil || len(g.vars) == 0 || len(g.index) == 0 {
		seq.trivial = true
		return seq
	}
	switch g.mode {
	case CartesianMode:
		if len(g.filters) == 0 {
			seq.it = g.Iterator()
			return seq
		}
		seq.indexes = make([]int, len(g.dims))
		return seq
	case SamplingMode:
		seq.sampleIdx, seq.hasSampleVar = g.index[sampleVarName]
		seq.sampleMaxAttempts = g.samples * 100
		if seq.sampleMaxAttempts < g.samples {
			seq.sampleMaxAttempts = g.samples
		}
		if g.samples <= 0 {
			seq.done = true
		}
		return seq
	case ExplorationGuided:
		switch g.strategy {
		case strategyPlain:
			seq.exploreCorpus = make([]PathValues, 0, g.iterations/10+1)
			seq.exploreSeenSigs = make(map[uint64]struct{})
		case strategyCoverage, strategySmart:
			seq.guidedSeenSigs = make(map[uint64]struct{})
		}
		if g.iterations <= 0 {
			seq.done = true
		}
		return seq
	default:
		panic("specs: PathGenerator.sequence supports CartesianMode/SamplingMode/ExplorationGuided only")
	}
}

// next proposes the next candidate, or (PathValues{}, false) once the space is exhausted.
func (s *pathSequence) next() (PathValues, bool) {
	if s == nil || s.done {
		return PathValues{}, false
	}
	if s.trivial {
		s.done = true
		return PathValues{}, true
	}
	if s.it != nil {
		if !s.it.Next() {
			s.done = true
			return PathValues{}, false
		}
		var pv PathValues
		s.g.FillPathValues(s.it.Index(), &pv)
		return pv, true
	}
	if s.g.mode == SamplingMode {
		return s.nextSample()
	}
	if s.g.mode == ExplorationGuided {
		if s.g.strategy == strategyPlain {
			return s.nextExplore()
		}
		return s.nextGuided()
	}
	return s.nextFiltered()
}

// nextSample mirrors runSamples: draw a random value per dimension, retry internally against
// filters up to the same samples*100 attempt budget (panicking with the same message if that
// budget is exhausted), and number the synthetic "sample" dimension 1-indexed. Each call yields
// at most one filter-accepted candidate; the internal retries never surface to the
// proposalController as separate Propose calls.
func (s *pathSequence) nextSample() (PathValues, bool) {
	g := s.g
	if s.sampleExecuted >= g.samples {
		s.done = true
		return PathValues{}, false
	}
	for {
		s.sampleAttempts++
		if s.sampleAttempts > s.sampleMaxAttempts {
			panic("specs: sampling could not satisfy filters; reduce sample count or relax filters")
		}
		var pv PathValues
		pv.reset(g.index)
		for _, dim := range g.dims {
			pv.present[dim.idx] = true
			pv.values[dim.idx] = dim.randomValue(g.rng)
		}
		if s.hasSampleVar {
			pv.present[s.sampleIdx] = true
			pv.values[s.sampleIdx] = s.sampleExecuted + 1
		}
		if !g.allow(pv) {
			continue
		}
		s.sampleExecuted++
		return pv, true
	}
}

// nextExplore mirrors runPlainExploration: seed the corpus with one random candidate (if it
// passes filters), then on each call either mutate a random corpus entry (70% of the time, once
// the corpus is non-empty) or draw a fully random candidate, retrying internally against filters.
// captureSignature() is called from this fixed call site for every candidate in the sequence, so
// — same as runPlainExploration calling it from its own fixed loop line — it yields the same
// signature every time within one sequence, meaning exploreCorpus only ever grows by the seed
// candidate plus the first accepted candidate from this loop. That's an existing, documented
// heuristic limitation (see runGuidedExploration's doc comment), not something introduced here;
// this method preserves it exactly rather than changing observable behavior.
func (s *pathSequence) nextExplore() (PathValues, bool) {
	g := s.g
	if !s.exploreSeeded {
		s.exploreSeeded = true
		var seed PathValues
		seed.reset(g.index)
		for i := range g.dims {
			seed.present[g.dims[i].idx] = true
			seed.values[g.dims[i].idx] = g.dims[i].randomValue(g.rng)
		}
		if g.allow(seed) {
			s.exploreCorpus = append(s.exploreCorpus, seed.clone())
		}
	}
	if s.exploreExecuted >= g.iterations {
		s.done = true
		return PathValues{}, false
	}
	for {
		var candidate PathValues
		if len(s.exploreCorpus) > 0 && g.rng.Float64() > 0.3 {
			candidate = g.mutate(s.exploreCorpus[g.rng.Intn(len(s.exploreCorpus))])
		} else {
			candidate = g.randomInput()
		}
		if !g.allow(candidate) {
			continue
		}
		s.exploreExecuted++
		sig := captureSignature()
		if _, seen := s.exploreSeenSigs[sig]; !seen {
			s.exploreSeenSigs[sig] = struct{}{}
			s.exploreCorpus = append(s.exploreCorpus, candidate.clone())
		}
		return candidate, true
	}
}

// nextGuided mirrors runGuidedExploration exactly for strategyCoverage/strategySmart: draw a
// candidate from the CoverageExplorer's/SmartExplorer's own NextInput, retry internally against
// filters, and on acceptance grow that explorer's own corpus via the same captureSignature()
// call-site novelty proxy runGuidedExploration already uses — see that method's doc comment for why
// the proxy, not real per-iteration coverage, still drives corpus growth here. Unlike nextExplore,
// there is no separate seed step: runGuidedExploration doesn't have one either, so growth caps at
// exactly one corpus entry (the first accepted candidate) rather than two.
func (s *pathSequence) nextGuided() (PathValues, bool) {
	g := s.g
	if s.guidedExecuted >= g.iterations {
		s.done = true
		return PathValues{}, false
	}
	var nextInput func(*PathGenerator) PathValues
	var corpus *Corpus
	switch g.strategy {
	case strategyCoverage:
		nextInput = g.coverageExplorer.NextInput
		corpus = g.coverageExplorer.corpus
	case strategySmart:
		nextInput = g.smartExplorer.NextInput
		corpus = g.smartExplorer.corpus
	}
	for {
		candidate := nextInput(g)
		if !g.allow(candidate) {
			continue
		}
		s.guidedExecuted++
		sig := captureSignature()
		if _, seen := s.guidedSeenSigs[sig]; !seen {
			s.guidedSeenSigs[sig] = struct{}{}
			corpus.Add(candidate)
		}
		return candidate, true
	}
}

// nextFiltered mirrors enumerate()'s odometer, yielding one allowed combination per call instead
// of visiting the whole space up front.
func (s *pathSequence) nextFiltered() (PathValues, bool) {
	g := s.g
	if len(g.dims) == 0 {
		if s.started {
			s.done = true
			return PathValues{}, false
		}
		s.started = true
		var pv PathValues
		pv.reset(g.index)
		if !g.allow(pv) {
			s.done = true
			return PathValues{}, false
		}
		return pv, true
	}
	for {
		if s.started {
			if !s.advance() {
				s.done = true
				return PathValues{}, false
			}
		}
		s.started = true
		var pv PathValues
		pv.reset(g.index)
		for dimIdx, dim := range g.dims {
			pv.present[dim.idx] = true
			pv.values[dim.idx] = dim.valueAt(s.indexes[dimIdx])
		}
		if g.allow(pv) {
			return pv, true
		}
	}
}

func (s *pathSequence) advance() bool {
	for carry := len(s.indexes) - 1; carry >= 0; carry-- {
		s.indexes[carry]++
		if s.indexes[carry] < s.g.dims[carry].len() {
			return true
		}
		s.indexes[carry] = 0
	}
	return false
}

// admitFeedback reports the real outcome of executing candidate back to the generator. It is a
// no-op for all five supported mode/strategy combinations: none has feedback-dependent state today.
// Sample's candidates are drawn independently of prior results, same as runSamples always was.
// Explore/ExploreCoverage/ExploreSmart's corpus growth is driven by captureSignature()'s call-site
// novelty proxy inside nextExplore/nextGuided, not by whether the candidate passed — so
// admitFeedback has nothing to do for any of them, even though it now runs after the real case (see
// nextExplore's/nextGuided's doc comments). Genuinely reacting to passed for ExploreCoverage/
// ExploreSmart would require wiring real per-iteration coverage into Feedback (see
// runGuidedExploration's doc comment) — deliberately out of scope for this migration; see #43.
func (s *pathSequence) admitFeedback(candidate PathValues, passed bool) {}

// bounds returns conservative (never-underestimating) MaxAttempts/MaxAccepted/MaxRejections
// upper bounds for proposalControllerConfig, computed without generating a single candidate.
// Panics if g uses a mode sequence()/bounds() doesn't support yet.
func (g *PathGenerator) bounds() (maxAttempts, maxAccepted, maxRejections int) {
	if g == nil || len(g.vars) == 0 || len(g.index) == 0 {
		return 1, 1, 1
	}
	switch g.mode {
	case CartesianMode:
		total := 1
		for _, dim := range g.dims {
			total *= dim.len()
		}
		return total, total, total
	case SamplingMode:
		// next() emits at most g.samples candidates (nextSample's own retry budget is internal
		// and never surfaces as separate Propose calls), so g.samples is an exact bound, not just
		// a conservative one.
		if g.samples <= 0 {
			return 0, 0, 0
		}
		return g.samples, g.samples, g.samples
	case ExplorationGuided:
		// Same reasoning as SamplingMode for all three strategies: nextExplore's/nextGuided's
		// filter retries are internal and never surface as separate Propose calls, so g.iterations
		// is exact.
		if g.iterations <= 0 {
			return 0, 0, 0
		}
		return g.iterations, g.iterations, g.iterations
	default:
		panic("specs: PathGenerator.bounds supports CartesianMode/SamplingMode/ExplorationGuided only")
	}
}

// ForEach iterates over every allowed combination in declaration order.
func (g *PathGenerator) ForEach(fn func(PathValues)) {
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
		g.runSamples(fn)
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

func (g *PathGenerator) runSamples(fn func(PathValues)) {
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
			panic("specs: sampling could not satisfy filters; reduce sample count or relax filters")
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

// runExploration dispatches to the candidate-generation strategy selected by .Explore
// (strategyPlain, this generator's own mutate), .ExploreCoverage (strategyCoverage,
// CoverageExplorer), or .ExploreSmart (strategySmart, SmartExplorer) — whichever of those was
// called on the PathSpec (see newPathGenerator).
func (g *PathGenerator) runExploration(fn func(PathValues)) {
	if g.iterations <= 0 || g.rng == nil {
		return
	}
	switch g.strategy {
	case strategyCoverage:
		g.runGuidedExploration(fn, g.coverageExplorer.NextInput, g.coverageExplorer.corpus)
	case strategySmart:
		g.runGuidedExploration(fn, g.smartExplorer.NextInput, g.smartExplorer.corpus)
	default:
		g.runPlainExploration(fn)
	}
}

// runGuidedExploration drives CoverageExplorer/SmartExplorer for candidate generation. It backs
// ForEach()/runExploration() — the compatibility path exercised directly by callers like
// explore_mode_selection_test.go that construct a PathGenerator and call ForEach without going
// through the top-level Describe/runExecutionContext dispatch. That dispatch path is fully
// incremental now for all three ExplorationGuided strategies (see PathGenerator.sequence(),
// nextExplore for strategyPlain, nextGuided for strategyCoverage/strategySmart) and no longer calls
// this method at all.
//
// Real coverage-guided corpus growth needs ctx.coverage populated with genuine assertion-level edge
// data on every iteration, which requires wiring the runner to set it per path iteration — not yet
// done (see nextGuided's doc comment). So, same as nextGuided, this method's corpus growth still
// uses the call-site-signature novelty heuristic (captureSignature) as an honest, documented proxy —
// good enough to give NextInput something to mutate from instead of always falling back to fully
// random input, but not genuine coverage-guided selection.
func (g *PathGenerator) runGuidedExploration(fn func(PathValues), nextInput func(*PathGenerator) PathValues, corpus *Corpus) {
	executed := 0
	for executed < g.iterations {
		candidate := nextInput(g)
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
			corpus.Add(candidate)
		}
	}
}

func (g *PathGenerator) runPlainExploration(fn func(PathValues)) {
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
func (g *PathGenerator) RandomInput(rng *rand.Rand) PathValues {
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

// mutate returns a clone of input with one random dimension mutated: int gets +1/-1/mid-range (if
// ranged)/a random discrete value/full re-randomization, bool gets negated, and any other type is
// fully re-randomized via dim.randomValue. Unlike Mutator.Mutate (mutator.go), every dimension type
// has an explicit mutation path — bool and discrete values aren't left unsupported. That doesn't
// guarantee the result differs from the input, though: the discrete-value and dim.randomValue paths
// pick from the dimension's full value set without excluding the current value, so re-picking the
// same value by chance is possible (unlike, say, bool's unconditional negation). It also lacks
// Mutator's richer int operators (bit-flip, ×2/÷2, clamping).
//
// This is PathSpec.Explore's mutation strategy (via runExploration below); ExploreCoverage/
// ExploreSmart use Mutator.Mutate instead. See Mutator.Mutate's doc comment and #9 for why these
// are two independently-evolved strategies rather than one shared implementation.
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

func (g *PathGenerator) shrinkValues(current PathValues, check func(PathValues) bool) PathValues {
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
