package plan

import (
	"testing"

	"github.com/pablogore/go-specs/specs"
	intregistry "github.com/pablogore/go-specs/specs/internal/registry"
)

// GetByID returns the job associated with id.
func (p *ExecutionPlan) GetByID(id uint32) *Job {
	if p == nil || len(p.Jobs) == 0 || p.IndexByID == nil {
		return nil
	}
	return p.IndexByID[id]
}

// RunIDs executes the specified jobs using real t.Run subtests.
func (p *ExecutionPlan) RunIDs(t *testing.T, ids []uint32) {
	for _, id := range ids {
		job := p.GetByID(id)
		if job == nil {
			continue
		}
		localJob := job
		t.Run(localJob.Path, func(tt *testing.T) {
			if localJob.Parallel {
				tt.Parallel()
			}
			if gen, ok := localJob.PathMeta.(*specs.PathGenerator); ok && gen != nil {
				runPlanPathJob(tt, localJob, gen)
				return
			}
			runPlanJobOnce(tt, localJob)
		})
	}
}

func runPlanPathJob(t *testing.T, job *Job, gen *specs.PathGenerator) {
	ran := false
	gen.ForEach(t, func(values specs.PathValues) {
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
		runPlanJobOnce(t, job)
	}
}

func runPlanJobOnce(t *testing.T, job *Job) {
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
