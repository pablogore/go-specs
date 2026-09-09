package specs

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestRunGroupSpecPanicRecoveredSiblingsStillRun proves a panicking spec doesn't stop its siblings
// in the same group from running — matching the default execution path's contract.
func TestRunGroupSpecPanicRecoveredSiblingsStillRun(t *testing.T) {
	backend := &controlledBackend{}
	ctx := &Context{backend: backend}
	var ranSpec2 bool
	g := &group{
		specs: []step{
			func(*Context) { panic("boom") },
			func(*Context) { ranSpec2 = true },
		},
	}
	runGroup(ctx, g)

	if !ranSpec2 {
		t.Fatal("expected spec2 to run after spec1 panicked, but it didn't")
	}
	if len(backend.errors) != 1 || !strings.Contains(backend.errors[0], "boom") {
		t.Fatalf("expected exactly one recorded failure mentioning the panic message, got %v", backend.errors)
	}
}

// TestRunGroupAfterAlwaysRunsDespiteSpecPanic proves the group's after hooks still run when a spec
// panics, matching #61's runProgram contract for the default execution path.
func TestRunGroupAfterAlwaysRunsDespiteSpecPanic(t *testing.T) {
	backend := &controlledBackend{}
	ctx := &Context{backend: backend}
	var afterRan bool
	g := &group{
		specs: []step{func(*Context) { panic("boom") }},
		after: []step{func(*Context) { afterRan = true }},
	}
	runGroup(ctx, g)

	if !afterRan {
		t.Fatal("expected the group's after hook to run despite the spec panic")
	}
	if !ctx.failed {
		t.Fatal("expected ctx.failed to be set after an unrecovered spec panic")
	}
}

// TestRunGroupBeforePanicSkipsSpecsButRunsAfter proves the contract decided for #62: a panic in a
// before hook stops the rest of setup and skips this group's specs entirely (they can't be trusted
// to run against setup that never completed), but the group's after still runs.
func TestRunGroupBeforePanicSkipsSpecsButRunsAfter(t *testing.T) {
	backend := &controlledBackend{}
	ctx := &Context{backend: backend}
	var specRan, afterRan bool
	g := &group{
		before: []step{func(*Context) { panic("setup boom") }},
		specs:  []step{func(*Context) { specRan = true }},
		after:  []step{func(*Context) { afterRan = true }},
	}
	runGroup(ctx, g)

	if specRan {
		t.Fatal("expected specs to be skipped after a before-hook panic, but a spec ran")
	}
	if !afterRan {
		t.Fatal("expected the group's after hook to still run after a before-hook panic")
	}
	if len(backend.errors) != 1 || !strings.Contains(backend.errors[0], "setup boom") {
		t.Fatalf("expected exactly one recorded failure mentioning the panic message, got %v", backend.errors)
	}
}

// TestRunGroupAfterHookPanicDoesNotStopSiblingAfterHooks proves one panicking after hook doesn't
// prevent the remaining after hooks (for the same group) from running.
func TestRunGroupAfterHookPanicDoesNotStopSiblingAfterHooks(t *testing.T) {
	backend := &controlledBackend{}
	ctx := &Context{backend: backend}
	var secondRan bool
	// after runs in reverse order (index len-1 down to 0), so the second element here runs first.
	g := &group{
		specs: []step{func(*Context) {}},
		after: []step{
			func(*Context) { secondRan = true },
			func(*Context) { panic("after boom") },
		},
	}
	runGroup(ctx, g)

	if !secondRan {
		t.Fatal("expected the sibling after hook to run despite the first-executed one panicking")
	}
	found := false
	for _, e := range backend.errors {
		if strings.Contains(e, "after boom") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the after-hook panic to be recorded as a failure, got %v", backend.errors)
	}
}

// TestRunGroupFailFastStopsRemainingSpecsButAfterStillRuns proves the contract decided for #62: a
// panic counts as ctx.failed (via ctx.recordFailure), so FailFast's existing stop-at-the-next-check
// logic naturally stops the remaining specs in the group — but after still runs regardless, via defer.
func TestRunGroupFailFastStopsRemainingSpecsButAfterStillRuns(t *testing.T) {
	backend := &controlledBackend{}
	ctx := &Context{backend: backend}
	ctx.SetFailFast(true)
	var ranSpec2, afterRan bool
	g := &group{
		specs: []step{
			func(*Context) { panic("boom") },
			func(*Context) { ranSpec2 = true },
		},
		after: []step{func(*Context) { afterRan = true }},
	}
	runGroup(ctx, g)

	if ranSpec2 {
		t.Fatal("expected FailFast to stop the remaining specs in the group after a panic")
	}
	if !afterRan {
		t.Fatal("expected the group's after hook to still run under FailFast")
	}
}

// TestRunGroupsContinuesToNextGroupAfterPanic proves issue #15's core requirement at the group-
// iteration level: a spec that panics in one group is recorded as that group's failure, and the
// next group still runs — the process doesn't crash and other groups' results aren't discarded.
func TestRunGroupsContinuesToNextGroupAfterPanic(t *testing.T) {
	backend := &controlledBackend{}
	ctx := &Context{backend: backend}
	var ranGroup1 bool
	groups := []group{
		{specs: []step{func(*Context) { panic("boom") }}},
		{specs: []step{func(*Context) { ranGroup1 = true }}},
	}
	runGroups(ctx, groups)

	if !ranGroup1 {
		t.Fatal("expected the second group to still run after the first group's spec panicked")
	}
	if len(backend.errors) != 1 || !strings.Contains(backend.errors[0], "boom") {
		t.Fatalf("expected exactly one recorded failure mentioning the panic message, got %v", backend.errors)
	}
}

// TestRunGroupsFailFastStopsSubsequentGroupsAfterPanic proves a panic interacts with FailFast the
// same way an ordinary assertion failure does: it stops progression to the next group.
func TestRunGroupsFailFastStopsSubsequentGroupsAfterPanic(t *testing.T) {
	backend := &controlledBackend{}
	ctx := &Context{backend: backend}
	ctx.SetFailFast(true)
	var ranGroup1 bool
	groups := []group{
		{specs: []step{func(*Context) { panic("boom") }}},
		{specs: []step{func(*Context) { ranGroup1 = true }}},
	}
	runGroups(ctx, groups)

	if ranGroup1 {
		t.Fatal("expected FailFast to stop the second group from running after the first group's panic")
	}
}

// TestRunnerRunRecoversPanicAcrossGroupsRealProcess is a thin end-to-end check that Runner.Run —
// the public entry point, with the real contextPool/testBackend wiring, not the internal helpers
// exercised above — turns a panicking spec into an ordinary test failure instead of crashing the
// process. Runs in a subprocess (same pattern as TestGeneratedFatalUsesRealSubtestBoundary in
// execution_plan_test.go) rather than a nested t.Run, since a nested subtest's real Errorf would
// itself mark this test failed; a subprocess crash, by contrast, would simply never print "group1
// ran" at all — the clearest possible proof the process kept going.
func TestRunnerRunRecoversPanicAcrossGroupsRealProcess(t *testing.T) {
	if os.Getenv("GO_SPECS_RUNNER_PANIC_HELPER") == "1" {
		prog := &Program{
			Groups: []group{
				{specs: []step{func(*Context) { panic("boom") }}},
				{specs: []step{func(*Context) { fmt.Println("group1 ran") }}},
			},
		}
		NewRunner(prog).Run(t)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunnerRunRecoversPanicAcrossGroupsRealProcess$")
	cmd.Env = append(os.Environ(), "GO_SPECS_RUNNER_PANIC_HELPER=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected the panicking spec to fail the test, but it passed: %s", output)
	}
	if !strings.Contains(string(output), "group1 ran") {
		t.Fatalf("expected the second group to still run (process must not crash), got: %s", output)
	}
}
