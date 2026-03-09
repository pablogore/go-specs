package plan

import (
	"testing"

	intregistry "github.com/pablogore/go-specs/specs/internal/registry"
)

func TestCompileSimpleSpec(t *testing.T) {
	reg := intregistry.NewRegistry()
	reg.Push("Calculator", intregistry.NodeDescribe, nil)
	called := false
	reg.Push("adds", intregistry.NodeIt, func(arg any) {
		called = true
	})
	plan := Compile(reg.Root)
	if len(plan.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(plan.Jobs))
	}
	job := plan.Jobs[0]
	if job.Path != "Calculator/adds" {
		t.Fatalf("unexpected path: %s", job.Path)
	}
	if job.Fn == nil {
		t.Fatal("expected fn")
	}
	job.Fn(nil)
	if !called {
		t.Fatal("expected fn to run")
	}
}

func TestCompileNestedScopes(t *testing.T) {
	reg := intregistry.NewRegistry()
	reg.Push("Calculator", intregistry.NodeDescribe, nil)
	reg.Push("Add", intregistry.NodeWhen, nil)
	reg.Push("handles negatives", intregistry.NodeIt, nil)
	plan := Compile(reg.Root)
	if len(plan.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(plan.Jobs))
	}
	if plan.Jobs[0].Path != "Calculator/Add/handles negatives" {
		t.Fatalf("unexpected path: %s", plan.Jobs[0].Path)
	}
}

func TestCompileFixtureInheritance(t *testing.T) {
	reg := intregistry.NewRegistry()
	root := reg.Push("Suite", intregistry.NodeDescribe, nil)
	root.Meta.FixturesBefore = append(root.Meta.FixturesBefore, func(any) {})
	root.Meta.FixturesAfter = append(root.Meta.FixturesAfter, func(any) {})
	child := reg.Push("group", intregistry.NodeWhen, nil)
	child.Meta.FixturesBefore = append(child.Meta.FixturesBefore, func(any) {})
	child.Meta.FixturesAfter = append(child.Meta.FixturesAfter, func(any) {})
	leaf := reg.Push("leaf", intregistry.NodeIt, nil)
	leaf.Meta.FixturesBefore = append(leaf.Meta.FixturesBefore, func(any) {})
	leaf.Meta.FixturesAfter = append(leaf.Meta.FixturesAfter, func(any) {})
	plan := Compile(reg.Root)
	if len(plan.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(plan.Jobs))
	}
	job := plan.Jobs[0]
	if len(job.FixturesBefore) != 3 {
		t.Fatalf("expected 3 before fixtures, got %d", len(job.FixturesBefore))
	}
	if len(job.FixturesAfter) != 3 {
		t.Fatalf("expected 3 after fixtures, got %d", len(job.FixturesAfter))
	}
}

func TestCompileParallelFlag(t *testing.T) {
	reg := intregistry.NewRegistry()
	root := reg.Push("Suite", intregistry.NodeDescribe, nil)
	root.Meta.Parallel = true
	reg.Push("leaf", intregistry.NodeIt, nil)
	plan := Compile(reg.Root)
	if len(plan.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(plan.Jobs))
	}
	if !plan.Jobs[0].Parallel {
		t.Fatal("expected parallel flag to be true")
	}
}

func TestSpecIDAssignment(t *testing.T) {
	reg := intregistry.NewRegistry()
	reg.Push("Suite", intregistry.NodeDescribe, nil)
	reg.Push("it1", intregistry.NodeIt, nil)
	reg.Pop()
	reg.Push("it2", intregistry.NodeIt, nil)
	plan := Compile(reg.Root)
	if len(plan.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(plan.Jobs))
	}
	if plan.Jobs[0].ID != 0 || plan.Jobs[1].ID != 1 {
		t.Fatalf("unexpected IDs %+v", plan.Jobs)
	}
}

func TestExecutionPlanIndex(t *testing.T) {
	reg := intregistry.NewRegistry()
	reg.Push("Suite", intregistry.NodeDescribe, nil)
	reg.Push("spec", intregistry.NodeIt, nil)
	plan := Compile(reg.Root)
	job := plan.GetByID(plan.Jobs[0].ID)
	if job == nil || job.Path != "Suite/spec" {
		t.Fatalf("expected job lookup, got %v", job)
	}
}

func TestRunIDs(t *testing.T) {
	reg := intregistry.NewRegistry()
	reg.Push("Suite", intregistry.NodeDescribe, nil)
	reg.Push("A", intregistry.NodeIt, func(any) {})
	reg.Pop()
	called := false
	reg.Push("B", intregistry.NodeIt, func(any) { called = true })
	plan := Compile(reg.Root)
	if len(plan.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(plan.Jobs))
	}
	plan.RunIDs(t, []uint32{plan.Jobs[1].ID})
	if !called {
		t.Fatal("expected selected job to run")
	}
}
