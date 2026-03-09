package mock

import "reflect"

// ArgMatcher matches a single argument in call verification.
type ArgMatcher interface {
	Match(v any) bool
}

// Any returns a matcher that matches any value.
func Any() ArgMatcher {
	return anyMatcher{}
}

type anyMatcher struct{}

func (anyMatcher) Match(v any) bool {
	return true
}

// Equal returns a matcher that matches values equal to expected (reflect.DeepEqual).
func Equal(expected any) ArgMatcher {
	return &equalMatcher{expected: expected}
}

type equalMatcher struct {
	expected any
}

func (m *equalMatcher) Match(v any) bool {
	return reflect.DeepEqual(v, m.expected)
}
