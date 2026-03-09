//go:build !lint

package runner

import (
	"testing"

	"github.com/getsyntegrity/go-specs/specs"
	intregistry "github.com/getsyntegrity/go-specs/specs/internal/registry"
)

// RegistryRunner executes specs recorded in a registry tree.
type RegistryRunner struct {
	registry *intregistry.Registry
}

// NewRegistryRunner returns a runner that walks the given registry.
func NewRegistryRunner(reg *intregistry.Registry) *RegistryRunner {
	return &RegistryRunner{registry: reg}
}

// Run traverses the registry tree and executes each spec.
func (r *RegistryRunner) Run(t *testing.T) {
	if r == nil || r.registry == nil || r.registry.Root == nil {
		return
	}
	for _, child := range r.registry.Root.Children {
		r.walk(child, t)
	}
}

func (r *RegistryRunner) walk(node *intregistry.Node, t *testing.T) {
	if node == nil {
		return
	}
	switch node.Type {
	case intregistry.NodeIt:
		t.Run(node.Name, func(tt *testing.T) {
			if gen, ok := node.Path.(*specs.PathGenerator); ok && gen != nil {
				runRegistryPath(tt, node, gen)
				return
			}
			runRegistryLeaf(tt, node, specs.PathValues{})
		})
	default:
		t.Run(node.Name, func(tt *testing.T) {
			for _, child := range node.Children {
				r.walk(child, tt)
			}
		})
	}
}

func runRegistryLeaf(t *testing.T, node *intregistry.Node, values specs.PathValues) {
	ctx := specs.NewContext(t)
	ctx.SetPathValues(values)
	if node.Fn != nil {
		node.Fn(ctx)
	}
}

func runRegistryPath(t *testing.T, node *intregistry.Node, gen *specs.PathGenerator) {
	ran := false
	gen.ForEach(func(values specs.PathValues) {
		ran = true
		name := gen.FormatName(node.Name, values)
		t.Run(name, func(tt *testing.T) {
			runRegistryLeaf(tt, node, values)
		})
	})
	if !ran {
		runRegistryLeaf(t, node, specs.PathValues{})
	}
}
