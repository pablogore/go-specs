# SKILLS.md

This file describes common high-value tasks in `go-specs` and the preferred implementation strategy.

---

## Skill: Optimize Matchers

### Goal
Reduce allocations and time spent in expectations/matchers.

### Preferred Approach
- keep public assertion API stable
- add typed fast paths first
- use reflection only as fallback
- replace per-call matcher closures with reusable structs or direct methods

### Validate
```bash
go test ./...
go test -race ./...
go test ./benchmarks -run='^$' -bench=. -benchmem
```
