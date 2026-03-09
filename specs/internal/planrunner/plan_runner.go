package planrunner

import (
	"testing"

	"github.com/getsyntegrity/go-specs/specs"
	"github.com/getsyntegrity/go-specs/specs/internal/plan"
	intregistry "github.com/getsyntegrity/go-specs/specs/internal/registry"
)

// Run executes the compiled execution plan.
func Run(t *testing.T, execution plan.ExecutionPlan) {
	for _, job := range execution.Jobs {
		job := job
		t.Run(job.Path, func(tt *testing.T) {
			if job.Parallel {
				tt.Parallel()
			}
			if gen, ok := job.PathMeta.(*specs.PathGenerator); ok && gen != nil {
				runPathJob(tt, &job, gen)
				return
			}
			runJobOnce(tt, &job)
		})
	}
}

func runPathJob(t *testing.T, job *plan.Job, gen *specs.PathGenerator) {
	ran := false
	gen.ForEach(func(values specs.PathValues) {
		ran = true
		name := gen.FormatName(job.Path, values)
		t.Run(name, func(tt *testing.T) {
			ctx := specs.NewContext(tt)
			ctx.SetPathValues(values)
			runFixtures(ctx, job.FixturesBefore)
			if job.Fn != nil {
				job.Fn(ctx)
			}
			runAfterFixtures(ctx, job.FixturesAfter)
		})
	})
	if !ran {
		runJobOnce(t, job)
	}
}

func runJobOnce(t *testing.T, job *plan.Job) {
	ctx := specs.NewContext(t)
	runFixtures(ctx, job.FixturesBefore)
	if job.Fn != nil {
		job.Fn(ctx)
	}
	runAfterFixtures(ctx, job.FixturesAfter)
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
