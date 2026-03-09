package runner

import (
	"testing"

	"github.com/getsyntegrity/go-specs/specs"
	"github.com/getsyntegrity/go-specs/specs/internal/plan"
	intregistry "github.com/getsyntegrity/go-specs/specs/internal/registry"
)

// RunPlan executes each job in the compiled execution plan.
func RunPlan(t *testing.T, execution plan.ExecutionPlan) {
	for _, job := range execution.Jobs {
		job := job
		t.Run(job.Path, func(tt *testing.T) {
			if job.Parallel {
				tt.Parallel()
			}
			ctx := specs.NewContext(tt)
			runFixtures(ctx, job.FixturesBefore)
			if job.Fn != nil {
				job.Fn(ctx)
			}
			runAfterFixtures(ctx, job.FixturesAfter)
		})
	}
}

func runFixtures(ctx *specs.Context, fixtures []intregistry.Fixture) {
	for _, f := range fixtures {
		if f != nil {
			f(ctx)
		}
	}
}

func runAfterFixtures(ctx *specs.Context, fixtures []intregistry.Fixture) {
	for _, f := range fixtures {
		if f != nil {
			f(ctx)
		}
	}
}
