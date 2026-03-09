package generators

import "testing"

func TestGeneratorsProvideDeterministicData(t *testing.T) {
	if got := Integers(); len(got) == 0 || got[0] != 0 {
		t.Fatalf("unexpected integers: %v", got)
	}
	if got := Strings(); len(got) < 2 || got[0] != "" || got[1] != " " {
		t.Fatalf("unexpected strings: %v", got[:min(2, len(got))])
	}
	if got := Bytes(); len(got) == 0 || got[0] != nil {
		t.Fatalf("unexpected bytes: %v", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
