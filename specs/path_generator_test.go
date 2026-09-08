package specs

import (
	"reflect"
	"testing"
)

// collectSequence drains a pathSequence, formatting each candidate the same way ForEach's
// caller would (via FormatPathValuesForReport) so it can be compared against ForEach's output.
func collectSequence(g *PathGenerator) []string {
	seq := g.sequence()
	var got []string
	for {
		pv, ok := seq.next()
		if !ok {
			break
		}
		got = append(got, g.FormatPathValuesForReport(pv))
	}
	return got
}

func collectForEach(g *PathGenerator) []string {
	var want []string
	g.ForEach(func(pv PathValues) { want = append(want, g.FormatPathValuesForReport(pv)) })
	return want
}

func TestPathSequenceCartesianMatchesForEachOrder(t *testing.T) {
	gen := newPathGenerator([]PathVar{
		{Name: "x", Values: []any{1, 2, 3}},
		{Name: "y", Values: []any{"a", "b"}},
	}, nil, 0, 0, false, 0, 0, 0)

	got, want := collectSequence(gen), collectForEach(gen)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sequence order = %v, want %v (must match ForEach exactly, including the first combination)", got, want)
	}
	if len(got) != 6 {
		t.Fatalf("len(got) = %d, want 6 (3x2 combinations)", len(got))
	}
}

func TestPathSequenceCartesianWithFiltersMatchesForEachOrder(t *testing.T) {
	gen := newPathGenerator([]PathVar{
		{Name: "x", Values: []any{1, 2, 3, 4}},
	}, []PathFilter{func(pv PathValues) bool {
		v, _ := pv.lookup("x")
		return v.(int)%2 == 0
	}}, 0, 0, false, 0, 0, 0)

	got, want := collectSequence(gen), collectForEach(gen)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered sequence order = %v, want %v", got, want)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (only even x)", len(got))
	}
}

func TestPathSequenceTrivialGeneratorYieldsOneEmptyValue(t *testing.T) {
	gen := newPathGenerator(nil, nil, 0, 0, false, 0, 0, 0)
	seq := gen.sequence()
	if _, ok := seq.next(); !ok {
		t.Fatal("expected exactly one candidate for a generator with no vars")
	}
	if _, ok := seq.next(); ok {
		t.Fatal("expected exhaustion after the single trivial candidate")
	}
}

func TestPathSequenceAdmitFeedbackIsNoOpForCartesian(t *testing.T) {
	gen := newPathGenerator([]PathVar{{Name: "x", Values: []any{1, 2}}}, nil, 0, 0, false, 0, 0, 0)
	seq := gen.sequence()
	before, _ := seq.next()
	seq.admitFeedback(before, true)
	seq.admitFeedback(before, false)
	after, ok := seq.next()
	if !ok {
		t.Fatal("expected a second candidate")
	}
	if reflect.DeepEqual(before, after) {
		t.Fatal("second candidate must differ from the first regardless of admitFeedback calls")
	}
}

func TestPathGeneratorBoundsIsConservativeUpperBound(t *testing.T) {
	gen := newPathGenerator([]PathVar{
		{Name: "x", Values: []any{1, 2, 3}},
		{Name: "y", Values: []any{"a", "b"}},
	}, []PathFilter{func(pv PathValues) bool {
		v, _ := pv.lookup("x")
		return v.(int) == 1
	}}, 0, 0, false, 0, 0, 0)

	maxAttempts, maxAccepted, maxRejections := gen.bounds()
	if maxAttempts != 6 || maxAccepted != 6 || maxRejections != 6 {
		t.Fatalf("bounds = (%d, %d, %d), want (6, 6, 6) — the raw product, ignoring filters", maxAttempts, maxAccepted, maxRejections)
	}
	actual := len(collectSequence(gen))
	if actual >= maxAccepted {
		t.Fatalf("actual accepted candidates = %d, want fewer than the conservative bound %d (filters reduce the real count)", actual, maxAccepted)
	}
}

func TestPathSequencePanicsForUnsupportedModes(t *testing.T) {
	sample := newPathGenerator([]PathVar{{Name: "x", Values: []any{1, 2, 3}}}, nil, 2, 0, false, 0, 0, 0)
	assertPanics(t, func() { sample.sequence() })
	assertPanics(t, func() { sample.bounds() })

	explore := newPathGenerator([]PathVar{{Name: "x", Values: []any{1, 2, 3}}}, nil, 0, 0, false, 5, 0, 0)
	assertPanics(t, func() { explore.sequence() })
	assertPanics(t, func() { explore.bounds() })
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic")
		}
	}()
	fn()
}
