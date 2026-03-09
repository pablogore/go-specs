package runner

import (
	"testing"

	"github.com/pablogore/go-specs/specs/internal/plan"
	intregistry "github.com/pablogore/go-specs/specs/internal/registry"
)

func TestRunPlanExecutesSpecs(t *testing.T) {
	called := false
	job := plan.Job{
		Path: "Suite/spec",
		Fn: func(arg any) {
			called = true
		},
	}
	RunPlan(t, plan.ExecutionPlan{Jobs: []plan.Job{job}})
	if !called {
		t.Fatal("expected job to be executed")
	}
}

func TestRunPlanFixtureOrder(t *testing.T) {
	order := make([]string, 0, 4)
	job := plan.Job{
		Path: "suite/spec",
		FixturesBefore: []intregistry.Fixture{
			func(any) { order = append(order, "before-parent") },
			func(any) { order = append(order, "before-child") },
		},
		Fn: func(any) { order = append(order, "spec") },
		FixturesAfter: []intregistry.Fixture{
			func(any) { order = append(order, "after-child") },
			func(any) { order = append(order, "after-parent") },
		},
	}
	RunPlan(t, plan.ExecutionPlan{Jobs: []plan.Job{job}})
	expected := []string{"before-parent", "before-child", "spec", "after-child", "after-parent"}
	if len(order) != len(expected) {
		t.Fatalf("unexpected order %v", order)
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Fatalf("unexpected order %v", order)
		}
	}
}

func TestRunPlanParallelSpecs(t *testing.T) {
	job := plan.Job{
		Path:     "suite/parallel",
		Parallel: true,
	}
	RunPlan(t, plan.ExecutionPlan{Jobs: []plan.Job{job}})
}
