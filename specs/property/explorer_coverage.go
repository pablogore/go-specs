package property

// CoverageExplorer learns from execution-path coverage (branch sampling via assertions).
type CoverageExplorer struct {
	corpus   *Corpus
	mutator  *Mutator
	coverage Coverage
	seen     Coverage
}

// NewCoverageExplorer returns an explorer with deterministic corpus and mutator (same seed).
func NewCoverageExplorer(seed int64) *CoverageExplorer {
	return &CoverageExplorer{
		corpus:  NewCorpus(seed),
		mutator: NewMutator(seed),
	}
}

// NextInput returns the next input to try: random if corpus is empty, else a mutation of a random corpus entry.
func (e *CoverageExplorer) NextInput(gen *PathGenerator) PathValues {
	if e == nil {
		return PathValues{}
	}
	if gen == nil {
		return PathValues{}
	}
	if e.corpus.Len() == 0 {
		return gen.RandomInput(e.mutator.Rng)
	}
	return e.mutator.Mutate(gen, e.corpus.Random())
}

// Feedback records coverage for the run with input p. If cov has unseen edges, p is added to the corpus
// and cov is merged into seen. Returns true if the input was added to the corpus (new coverage).
func (e *CoverageExplorer) Feedback(p PathValues, cov *Coverage) bool {
	if e == nil {
		return false
	}
	if cov == nil {
		return false
	}
	if cov.HasNewCoverage(&e.seen) {
		e.corpus.Add(p)
		e.seen.MergeFrom(cov)
		return true
	}
	return false
}

// Seen returns a copy of the accumulated coverage (for external inspection).
func (e *CoverageExplorer) Seen() Coverage {
	if e == nil {
		return Coverage{}
	}
	return e.seen
}

// CorpusLen returns the number of inputs in the corpus.
func (e *CoverageExplorer) CorpusLen() int {
	if e == nil {
		return 0
	}
	return e.corpus.Len()
}
