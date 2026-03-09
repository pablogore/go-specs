package specs

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pablogore/go-specs/assert"
)

type MatchResult struct {
	OK      bool
	Message string
}

type nativeMatcher interface {
	Match(actual any) MatchResult
}

type matcherFunc func(actual any) MatchResult

func (f matcherFunc) Match(actual any) MatchResult {
	return f(actual)
}

func nilMatcher() nativeMatcher {
	return matcherFunc(func(actual any) MatchResult {
		if assert.IsNilValue(actual) {
			return MatchResult{OK: true}
		}
		return MatchResult{Message: fmt.Sprintf("expected value to be nil, got %v (%T)", actual, actual)}
	})
}

func boolMatcher(expected bool) nativeMatcher {
	return matcherFunc(func(actual any) MatchResult {
		val, ok := actual.(bool)
		if !ok {
			return MatchResult{Message: fmt.Sprintf("expected bool, got %T", actual)}
		}
		if val == expected {
			return MatchResult{OK: true}
		}
		return MatchResult{Message: fmt.Sprintf("expected %t, got %t", expected, val)}
	})
}

func errorMatcher() nativeMatcher {
	return matcherFunc(func(actual any) MatchResult {
		err, ok := actual.(error)
		if !ok {
			return MatchResult{Message: fmt.Sprintf("expected error, got %T", actual)}
		}
		if err == nil {
			return MatchResult{Message: "expected error to be non-nil"}
		}
		return MatchResult{OK: true}
	})
}

func noErrorMatcher() nativeMatcher {
	return matcherFunc(func(actual any) MatchResult {
		err, ok := actual.(error)
		if !ok {
			return MatchResult{Message: fmt.Sprintf("expected error, got %T", actual)}
		}
		if err == nil {
			return MatchResult{OK: true}
		}
		return MatchResult{Message: fmt.Sprintf("expected no error, got %v", err)}
	})
}

func matchErrorMatcher(substr string) nativeMatcher {
	return matcherFunc(func(actual any) MatchResult {
		err, ok := actual.(error)
		if !ok {
			return MatchResult{Message: fmt.Sprintf("expected error, got %T", actual)}
		}
		if err == nil {
			return MatchResult{Message: "expected error, got nil"}
		}
		if substr == "" || strings.Contains(err.Error(), substr) {
			return MatchResult{OK: true}
		}
		return MatchResult{Message: fmt.Sprintf("expected error %q to contain %q", err.Error(), substr)}
	})
}

func containMatcher(expected any) nativeMatcher {
	return matcherFunc(func(actual any) MatchResult {
		switch a := actual.(type) {
		case string:
			needle, ok := expected.(string)
			if !ok {
				return MatchResult{Message: fmt.Sprintf("expected string substring, got %T", expected)}
			}
			if strings.Contains(a, needle) {
				return MatchResult{OK: true}
			}
			return MatchResult{Message: fmt.Sprintf("expected %q to contain %q", a, needle)}
		}
		rv := reflect.ValueOf(actual)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < rv.Len(); i++ {
				if assert.ValuesEqual(rv.Index(i).Interface(), expected) {
					return MatchResult{OK: true}
				}
			}
			return MatchResult{Message: fmt.Sprintf("expected %v to contain %v", actual, expected)}
		}
		return MatchResult{Message: fmt.Sprintf("ToContain requires string, slice, or array, got %T", actual)}
	})
}

