package matchers

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestEqualErrorsUsesErrorsIs(t *testing.T) {
	baseErr := errors.New("boom")
	wrapped := fmt.Errorf("wrap: %w", baseErr)
	if !Equal(t, baseErr, wrapped) {
		t.Fatal("expected errors.Is to detect wrapped error")
	}
}

func TestEqualByteSlices(t *testing.T) {
	if !Equal(t, []byte("abc"), []byte("abc")) {
		t.Fatal("expected byte slices to be equal")
	}
	if Equal(t, []byte("abc"), []byte("abd")) {
		t.Fatal("expected byte slices to differ")
	}
}

func TestEqualTimes(t *testing.T) {
	now := time.Now()
	if !Equal(t, now, now) {
		t.Fatal("expected identical times to be equal")
	}
}

func TestEqualFallback(t *testing.T) {
	type sample struct {
		ID   int
		Data map[string]int
	}
	left := sample{ID: 1, Data: map[string]int{"a": 1}}
	right := sample{ID: 1, Data: map[string]int{"a": 1}}
	if !Equal(t, left, right) {
		t.Fatal("expected struct equality")
	}
}
