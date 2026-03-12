package assert

import (
	"fmt"
	"reflect"
	"strings"
)

// EqualComparable reports whether a and b are equal. Use for comparable types only; no reflection, inlineable.
func EqualComparable[T comparable](a, b T) bool {
	return a == b
}

// Matcher is the interface for assertion matchers used with Expect(...).To(m).
type Matcher interface {
	Match(actual any) bool
	FailureMessage(actual any) string
}

// MatchWithFastPath runs the matcher with direct type dispatch for built-in matchers to avoid interface calls.
// Returns (matched, failureMsg). Custom matchers fall back to matcher.Match / matcher.FailureMessage.
func MatchWithFastPath(matcher Matcher, actual any) (matched bool, failureMsg string) {
	if matcher == nil {
		return true, ""
	}
	switch m := matcher.(type) {
	case equalMatcher:
		if fastEqual(m.expected, actual) {
			return true, ""
		}
		return false, fmt.Sprintf("expected %v to equal %v", actual, m.expected)
	case notEqualMatcher:
		if !fastEqual(m.expected, actual) {
			return true, ""
		}
		return false, fmt.Sprintf("expected %v not to equal %v", actual, m.expected)
	case beNilMatcher:
		if actual == nil {
			return true, ""
		}
		if IsNilValue(actual) {
			return true, ""
		}
		return false, fmt.Sprintf("expected nil, got %v (%T)", actual, actual)
	case beTrueMatcher:
		if v, ok := actual.(bool); ok && v {
			return true, ""
		}
		return false, fmt.Sprintf("expected true, got %v (%T)", actual, actual)
	case beFalseMatcher:
		if v, ok := actual.(bool); ok && !v {
			return true, ""
		}
		return false, fmt.Sprintf("expected false, got %v (%T)", actual, actual)
	case containExpectedMatcher:
		if containMatch(m.expected, actual) {
			return true, ""
		}
		return false, fmt.Sprintf("expected %v to contain %v", actual, m.expected)
	default:
		if matcher.Match(actual) {
			return true, ""
		}
		return false, matcher.FailureMessage(actual)
	}
}

// containMatch reports whether actual (string or slice) contains expected. Used by MatchWithFastPath.
func containMatch(expected, actual any) bool {
	switch a := actual.(type) {
	case string:
		needle, ok := expected.(string)
		if !ok {
			return false
		}
		return strings.Contains(a, needle)
	case []int:
		if e, ok := expected.(int); ok {
			for _, v := range a {
				if v == e {
					return true
				}
			}
			return false
		}
	case []string:
		if e, ok := expected.(string); ok {
			for _, v := range a {
				if v == e {
					return true
				}
			}
			return false
		}
	case []float64:
		if e, ok := expected.(float64); ok {
			for _, v := range a {
				if v == e {
					return true
				}
			}
			return false
		}
	}
	rv := reflect.ValueOf(actual)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			if ValuesEqual(rv.Index(i).Interface(), expected) {
				return true
			}
		}
		return false
	}
	return false
}

// Equal returns a matcher that expects actual to equal expected. Struct-based; no closure.
func Equal(expected any) Matcher {
	return equalMatcher{expected: expected}
}

type equalMatcher struct {
	expected any
}

func (m equalMatcher) Match(actual any) bool {
	return fastEqual(m.expected, actual)
}

func (m equalMatcher) FailureMessage(actual any) string {
	return fmt.Sprintf("expected %v to equal %v", actual, m.expected)
}

// NotEqual returns a matcher that expects actual not to equal expected. Struct-based; no closure.
func NotEqual(expected any) Matcher {
	return notEqualMatcher{expected: expected}
}

type notEqualMatcher struct {
	expected any
}

func (m notEqualMatcher) Match(actual any) bool {
	return !ValuesEqual(m.expected, actual)
}

func (m notEqualMatcher) FailureMessage(actual any) string {
	return fmt.Sprintf("expected %v not to equal %v", actual, m.expected)
}

// Singleton matchers (zero allocation); no closure.
var (
	beNilSingleton   beNilMatcher
	beTrueSingleton  beTrueMatcher
	beFalseSingleton beFalseMatcher
)

// BeNil returns a matcher that expects actual to be nil. Returns singleton; zero alloc.
func BeNil() Matcher {
	return beNilSingleton
}

type beNilMatcher struct{}

func (m beNilMatcher) Match(actual any) bool {
	if actual == nil {
		return true
	}
	return IsNilValue(actual)
}

func (m beNilMatcher) FailureMessage(actual any) string {
	return fmt.Sprintf("expected nil, got %v (%T)", actual, actual)
}

// BeTrue returns a matcher that expects actual to be the bool true. Returns singleton; zero alloc.
func BeTrue() Matcher {
	return beTrueSingleton
}

type beTrueMatcher struct{}

func (m beTrueMatcher) Match(actual any) bool {
	v, ok := actual.(bool)
	return ok && v
}

func (m beTrueMatcher) FailureMessage(actual any) string {
	return fmt.Sprintf("expected true, got %v (%T)", actual, actual)
}

// BeFalse returns a matcher that expects actual to be the bool false. Returns singleton; zero alloc.
func BeFalse() Matcher {
	return beFalseSingleton
}

type beFalseMatcher struct{}

func (m beFalseMatcher) Match(actual any) bool {
	v, ok := actual.(bool)
	return ok && !v
}

func (m beFalseMatcher) FailureMessage(actual any) string {
	return fmt.Sprintf("expected false, got %v (%T)", actual, actual)
}

// Contain returns a matcher that expects actual (string or slice) to contain expected. Struct-based; no closure.
func Contain(expected any) Matcher {
	return containExpectedMatcher{expected: expected}
}

type containExpectedMatcher struct {
	expected any
}

func (m containExpectedMatcher) Match(actual any) bool {
	switch a := actual.(type) {
	case string:
		needle, ok := m.expected.(string)
		if !ok {
			return false
		}
		return strings.Contains(a, needle)
	case []int:
		if e, ok := m.expected.(int); ok {
			for _, v := range a {
				if v == e {
					return true
				}
			}
			return false
		}
	case []string:
		if e, ok := m.expected.(string); ok {
			for _, v := range a {
				if v == e {
					return true
				}
			}
			return false
		}
	case []float64:
		if e, ok := m.expected.(float64); ok {
			for _, v := range a {
				if v == e {
					return true
				}
			}
			return false
		}
	}
	rv := reflect.ValueOf(actual)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			if ValuesEqual(rv.Index(i).Interface(), m.expected) {
				return true
			}
		}
		return false
	}
	return false
}

func (m containExpectedMatcher) FailureMessage(actual any) string {
	return fmt.Sprintf("expected %v to contain %v", actual, m.expected)
}

// fastEqual compares a and b with fast paths for common types, then falls back to reflect.DeepEqual.
func fastEqual(a, b any) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if eq, handled := fastEqualComparable(a, b); handled {
		return eq
	}
	return reflect.DeepEqual(a, b)
}

// ValuesEqual reports whether expected and actual are equal (for use by other packages).
func ValuesEqual(expected, actual any) bool {
	return fastEqual(expected, actual)
}

// IsNilValue reports whether value is nil or a nil pointer/slice/map/etc.
func IsNilValue(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Func, reflect.Interface, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}

func fastEqualComparable(expected, actual any) (bool, bool) {
	switch a := actual.(type) {
	case bool:
		if b, ok := expected.(bool); ok {
			return a == b, true
		}
	case string:
		if b, ok := expected.(string); ok {
			return a == b, true
		}
	case int:
		if b, ok := expected.(int); ok {
			return a == b, true
		}
	case int8:
		if b, ok := expected.(int8); ok {
			return a == b, true
		}
	case int16:
		if b, ok := expected.(int16); ok {
			return a == b, true
		}
	case int32:
		if b, ok := expected.(int32); ok {
			return a == b, true
		}
	case int64:
		if b, ok := expected.(int64); ok {
			return a == b, true
		}
	case uint:
		if b, ok := expected.(uint); ok {
			return a == b, true
		}
	case uint8:
		if b, ok := expected.(uint8); ok {
			return a == b, true
		}
	case uint16:
		if b, ok := expected.(uint16); ok {
			return a == b, true
		}
	case uint32:
		if b, ok := expected.(uint32); ok {
			return a == b, true
		}
	case uint64:
		if b, ok := expected.(uint64); ok {
			return a == b, true
		}
	case uintptr:
		if b, ok := expected.(uintptr); ok {
			return a == b, true
		}
	case float32:
		if b, ok := expected.(float32); ok {
			return a == b, true
		}
	case float64:
		if b, ok := expected.(float64); ok {
			return a == b, true
		}
	case complex64:
		if b, ok := expected.(complex64); ok {
			return a == b, true
		}
	case complex128:
		if b, ok := expected.(complex128); ok {
			return a == b, true
		}
	}
	return false, false
}
