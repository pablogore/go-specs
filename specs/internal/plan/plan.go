package plan

import (
	intregistry "github.com/pablogore/go-specs/specs/internal/registry"
)

// ExecutionPlan is a flattened list of spec jobs to run.
type ExecutionPlan struct {
	Jobs      []Job
	IndexByID map[uint32]*Job
}

// Job represents a single spec execution unit.
type Job struct {
	ID             uint32
	Path           string
	Fn             intregistry.Handler
	FixturesBefore []intregistry.Fixture
	FixturesAfter  []intregistry.Fixture
	Parallel       bool
	PathMeta       any
}

// Compile builds an execution plan from a registry tree.
func Compile(root *intregistry.Node) ExecutionPlan {
	plan := ExecutionPlan{}
	if root == nil {
		return plan
	}
	count := CountSpecs(root)
	jobs := make([]Job, 0, count)
	ancestors := make([]*intregistry.Node, 0, 8)
	path := make([]string, 0, 8)
	for _, child := range root.Children {
		compileNode(child, &ancestors, &path, &jobs)
	}
	plan.Jobs = jobs
	plan.IndexByID = make(map[uint32]*Job, len(plan.Jobs))
	for i := range plan.Jobs {
		plan.IndexByID[plan.Jobs[i].ID] = &plan.Jobs[i]
	}
	return plan
}

func compileNode(node *intregistry.Node, ancestors *[]*intregistry.Node, path *[]string, jobs *[]Job) {
	if node == nil {
		return
	}
	*ancestors = append(*ancestors, node)
	pushed := false
	if node.Name != "" && !(node.Parent == nil && node.Name == "root") {
		*path = append(*path, node.Name)
		pushed = true
	}
	if node.Type == intregistry.NodeIt {
		job := Job{
			ID:             node.ID,
			Path:           buildPath(*path),
			Fn:             node.Fn,
			FixturesBefore: collectFixturesBefore(*ancestors),
			FixturesAfter:  collectFixturesAfter(*ancestors),
			Parallel:       collectParallel(*ancestors),
			PathMeta:       node.Path,
		}
		*jobs = append(*jobs, job)
		goto cleanup
	}
	for _, child := range node.Children {
		compileNode(child, ancestors, path, jobs)
	}
cleanup:
	if pushed {
		*path = (*path)[:len(*path)-1]
	}
	*ancestors = (*ancestors)[:len(*ancestors)-1]
}

func collectFixturesBefore(nodes []*intregistry.Node) []intregistry.Fixture {
	estimated := 0
	for _, node := range nodes {
		estimated += len(node.Meta.FixturesBefore)
	}
	fixtures := make([]intregistry.Fixture, 0, estimated)
	for _, node := range nodes {
		if len(node.Meta.FixturesBefore) == 0 {
			continue
		}
		fixtures = append(fixtures, node.Meta.FixturesBefore...)
	}
	return fixtures
}

func collectFixturesAfter(nodes []*intregistry.Node) []intregistry.Fixture {
	estimated := 0
	for _, node := range nodes {
		estimated += len(node.Meta.FixturesAfter)
	}
	fixtures := make([]intregistry.Fixture, 0, estimated)
	for i := len(nodes) - 1; i >= 0; i-- {
		meta := nodes[i].Meta
		for j := len(meta.FixturesAfter) - 1; j >= 0; j-- {
			fixtures = append(fixtures, meta.FixturesAfter[j])
		}
	}
	return fixtures
}

func collectParallel(nodes []*intregistry.Node) bool {
	for _, node := range nodes {
		if node.Meta.Parallel {
			return true
		}
	}
	return false
}

// CountSpecs counts the number of It nodes in the registry.
func CountSpecs(root *intregistry.Node) int {
	if root == nil {
		return 0
	}
	count := 0
	for _, child := range root.Children {
		count += countItNodes(child)
	}
	return count
}

func countItNodes(node *intregistry.Node) int {
	if node == nil {
		return 0
	}
	total := 0
	if node.Type == intregistry.NodeIt {
		total++
	}
	for _, child := range node.Children {
		total += countItNodes(child)
	}
	return total
}
