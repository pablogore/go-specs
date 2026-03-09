package specs

import "testing"

func TestSuiteTree(t *testing.T) {
	suite := Analyze(func() {
		Describe(nil, "Calculator", func(s *Spec) {
			s.When("adding numbers", func(child *Spec) {
				child.It("adds correctly", func(ctx *Context) {})
				child.It("handles negatives", func(ctx *Context) {})
			})
		})
	})
	got := suite.Tree()
	want := "Calculator\n  adding numbers\n    adds correctly\n    handles negatives"
	if got != want {
		t.Fatalf("unexpected tree:\n%s", got)
	}
}

func TestSuiteWalkDeterministicOrder(t *testing.T) {
	suite := Analyze(func() {
		Describe(nil, "Calculator", func(s *Spec) {
			s.When("adding numbers", func(child *Spec) {
				child.It("adds correctly", func(ctx *Context) {})
				child.It("handles negatives", func(ctx *Context) {})
			})
		})
	})
	var names []string
	suite.Walk(func(id int) {
		names = append(names, suite.Arena.Nodes[id].Name)
	})
	want := []string{"suite", "Calculator", "adding numbers", "adds correctly", "handles negatives"}
	if len(names) != len(want) {
		t.Fatalf("unexpected walk length: got %d want %d", len(names), len(want))
	}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("walk order mismatch at %d: got %s want %s", i, names[i], name)
		}
	}
}

func TestDescribeFastRunsSpecs(t *testing.T) {
	var run []string
	DescribeFast(t, "FastSuite", func(s *Spec) {
		s.It("first", func(ctx *Context) {
			run = append(run, "first")
			ctx.Expect(1).ToEqual(1)
		})
		s.It("second", func(ctx *Context) {
			run = append(run, "second")
			ctx.Expect(2).ToEqual(2)
		})
	})
	if len(run) != 2 || run[0] != "first" || run[1] != "second" {
		t.Fatalf("DescribeFast: expected run [first second], got %v", run)
	}
}

func TestDescribeFlatRunsSpecs(t *testing.T) {
	var run []string
	DescribeFlat(t, "FlatSuite", func(s *Spec) {
		s.It("first", func(ctx *Context) {
			run = append(run, "first")
			ctx.Expect(1).ToEqual(1)
		})
		s.It("second", func(ctx *Context) {
			run = append(run, "second")
			ctx.Expect(2).ToEqual(2)
		})
	})
	if len(run) != 2 || run[0] != "first" || run[1] != "second" {
		t.Fatalf("DescribeFlat: expected run [first second], got %v", run)
	}
}

func TestDescribeFlatHookOrder(t *testing.T) {
	var order []string
	DescribeFlat(t, "FlatHooks", func(s *Spec) {
		s.BeforeEach(func(_ *Context) { order = append(order, "before:outer") })
		s.AfterEach(func(_ *Context) { order = append(order, "after:outer") })
		s.Describe("inner", func(s2 *Spec) {
			s2.BeforeEach(func(_ *Context) { order = append(order, "before:inner") })
			s2.AfterEach(func(_ *Context) { order = append(order, "after:inner") })
			s2.It("leaf", func(_ *Context) { order = append(order, "test") })
		})
	})
	want := []string{"before:outer", "before:inner", "test", "after:inner", "after:outer"}
	if len(order) != len(want) {
		t.Fatalf("flat hook order: got %d steps want %d: %v", len(order), len(want), order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("flat hook order at %d: got %s want %s (full %v)", i, order[i], want[i], order)
		}
	}
}
