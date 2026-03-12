//go:build !lint

package runner

import (
	"testing"

	"github.com/pablogore/go-specs/specs/ctx"
	intregistry "github.com/pablogore/go-specs/specs/internal/registry"
	"github.com/pablogore/go-specs/specs/property"
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
			if gen, ok := node.Path.(*property.PathGenerator); ok && gen != nil {
				runRegistryPath(tt, node, gen)
				return
			}
			runRegistryLeaf(tt, node, property.PathValues{})
		})
	default:
		t.Run(node.Name, func(tt *testing.T) {
			for _, child := range node.Children {
				r.walk(child, tt)
			}
		})
	}
}

func runRegistryLeaf(t *testing.T, node *intregistry.Node, values property.PathValues) {
	c := ctx.NewContext(t)
	c.SetPathValues(values)
	if node.Fn != nil {
		node.Fn(c)
	}
}

func runRegistryPath(t *testing.T, node *intregistry.Node, gen *property.PathGenerator) {
	ran := false
	gen.ForEach(t, func(values property.PathValues) {
		ran = true
		name := gen.FormatName(node.Name, values)
		t.Run(name, func(tt *testing.T) {
			runRegistryLeaf(tt, node, values)
		})
	})
	if !ran {
		runRegistryLeaf(t, node, property.PathValues{})
	}
}
