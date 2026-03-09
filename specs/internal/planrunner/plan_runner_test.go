package planrunner

import (
	"testing"

	"github.com/getsyntegrity/go-specs/specs/internal/plan"
	intregistry "github.com/getsyntegrity/go-specs/specs/internal/registry"
)

func TestRunExecutesFixturesAndSpec(t *testing.T) {
	called := make([]string, 0, 3)
	job := plan.Job{
		Path: "Suite/spec",
		Fn: func(arg any) {
			called = append(called, "fn")
		},
		FixturesBefore: []intregistry.Fixture{func(any) { called = append(called, "before") }},
		FixturesAfter:  []intregistry.Fixture{func(any) { called = append(called, "after") }},
	}
	Run(t, plan.ExecutionPlan{Jobs: []plan.Job{job}})
	if len(called) != 3 {
		t.Fatalf("expected 3 callbacks, got %v", called)
	}
	if called[0] != "before" || called[1] != "fn" || called[2] != "after" {
		t.Fatalf("unexpected order %v", called)
	}
}
