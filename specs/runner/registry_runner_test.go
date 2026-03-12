package runner

import (
	"testing"

	"github.com/pablogore/go-specs/specs/ctx"
	intregistry "github.com/pablogore/go-specs/specs/internal/registry"
)

func TestRegistryRunnerExecutesSpecs(t *testing.T) {
	reg := intregistry.NewRegistry()
	root := reg.Push("Suite", intregistry.NodeDescribe, nil)
	_ = root
	reg.Push("adds numbers", intregistry.NodeIt, adaptHandler(func(c *ctx.Context) {
		c.Expect(1).ToEqual(1)
	}))
	reg.Pop()
	reg.Pop()
	r := NewRegistryRunner(reg)
	r.Run(t)
}

func TestRegistryRunnerNested(t *testing.T) {
	reg := intregistry.NewRegistry()
	reg.Push("Calculator", intregistry.NodeDescribe, nil)
	reg.Push("when adding", intregistry.NodeWhen, nil)
	reg.Push("returns sum", intregistry.NodeIt, adaptHandler(func(c *ctx.Context) {
		c.Expect(2).ToEqual(2)
	}))
	reg.Pop()
	reg.Pop()
	reg.Pop()
	r := NewRegistryRunner(reg)
	r.Run(t)
}

func adaptHandler(fn func(*ctx.Context)) intregistry.Handler {
	if fn == nil {
		return nil
	}
	return func(arg any) {
		if c, ok := arg.(*ctx.Context); ok {
			fn(c)
		}
	}
}
