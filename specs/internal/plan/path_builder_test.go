package plan

import "testing"

func TestPathBuilderSimple(t *testing.T) {
	path := buildPath([]string{"Calculator", "Add"})
	if path != "Calculator/Add" {
		t.Fatalf("unexpected path %s", path)
	}
}

func TestPathBuilderNested(t *testing.T) {
	parts := []string{"Calculator", "Add", "handles negatives"}
	if path := buildPath(parts); path != "Calculator/Add/handles negatives" {
		t.Fatalf("unexpected path %s", path)
	}
}

func TestPathBuilderEmpty(t *testing.T) {
	if path := buildPath(nil); path != "" {
		t.Fatalf("expected empty path, got %s", path)
	}
	if path := buildPath([]string{""}); path != "" {
		t.Fatalf("expected empty path, got %s", path)
	}
}
