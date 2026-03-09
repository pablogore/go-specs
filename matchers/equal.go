package matchers

import (
	"errors"
	"reflect"
	"testing"
)

// Equal reports whether a and b are equal. Used as a test helper.
// For error values, uses errors.Is for wrapped error comparison.
// Otherwise uses reflect.DeepEqual.
func Equal(t *testing.T, a, b any) bool {
	t.Helper()
	return equal(a, b)
}

func equal(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	errA, aIsErr := a.(error)
	errB, bIsErr := b.(error)
	if aIsErr && bIsErr {
		return errors.Is(errA, errB) || errors.Is(errB, errA)
	}
	return reflect.DeepEqual(a, b)
}
