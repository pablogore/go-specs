# Coverage Report

Generated automatically by tools/coverageheatmap.

## Coverage Heatmap

| Package | Coverage |
|--------|--------|
| internal/plan | ███████░░░ 78% |
| internal/planrunner | █████░░░░░ 52% |
| internal/registry | ░░░░░░░░░░ 0% |
| internal/runner | ██████████ 100% |

## Total Coverage

63.9%

## Threshold

No thresholds set.

## Coverage Gaps

Functions below 100% coverage:

| Function | Coverage | Suggested tests |
|--------|--------|--------|
| internal/plan.GetByID | 67% | error path, missing branches |
| internal/plan.RunIDs | 67% | error path, edge cases, missing branches |
| internal/plan.runPlanPathJob | 0% | add tests for happy path, error path, edge cases |
| internal/plan.runFixtures | 33% | error path, edge cases, missing branches |
| internal/plan.runAfterFixtures | 33% | error path, edge cases, missing branches |
| internal/plan.Compile | 93% | invalid input, empty input, each branch, error path, missing branches |
| internal/plan.compileNode | 94% | invalid input, empty input, each branch, error path, missing branches |
| internal/plan.CountSpecs | 83% | empty input, multiple items, error path, missing branches |
| internal/plan.countItNodes | 88% | empty input, multiple items, error path, missing branches |
| internal/planrunner.Run | 67% | error path, edge cases, missing branches |
| internal/planrunner.runPathJob | 0% | add tests for happy path, error path, edge cases |
| internal/registry.NewRegistry | 0% | add tests for happy path, error path, empty stack, concurrent access |
| internal/registry.Push | 0% | add tests for happy path, error path, empty stack, concurrent access |
| internal/registry.Pop | 0% | add tests for happy path, error path, empty stack, concurrent access |
| internal/registry.Current | 0% | add tests for happy path, error path |
| internal/registry.Attach | 0% | add tests for happy path, error path |
| internal/registry.attachLocked | 0% | add tests for happy path, error path |
