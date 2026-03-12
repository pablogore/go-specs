// dsl_reexport re-exports the DSL layer so the public API remains specs.Describe, specs.It, etc.
package specs

import (
	"github.com/pablogore/go-specs/specs/compiler"
	"github.com/pablogore/go-specs/specs/dsl"
)

// DSL types.
type (
	Spec        = dsl.Spec
	SuiteTree   = dsl.SuiteTree
	Node        = dsl.Node
	PathBuilder = dsl.PathBuilder
	PathSpec    = dsl.PathSpec
)

// Top-level Describe / BuildSuite.
var (
	Describe                 = dsl.Describe
	DescribeWithReporter     = dsl.DescribeWithReporter
	DescribeFlat             = dsl.DescribeFlat
	DescribeFlatWithReporter = dsl.DescribeFlatWithReporter
	DescribeFast             = dsl.DescribeFast
	DescribeFastWithReporter = dsl.DescribeFastWithReporter
	BuildSuite               = dsl.BuildSuite
)

// BuildProgram compiles a suite from a builder callback.
func BuildProgram(fn func(*Builder)) *Program {
	if fn == nil {
		return &Program{}
	}
	return dsl.BuildProgram(func(b *compiler.Builder) { fn(b) })
}

// Focus and Skip (spec modifiers).
var (
	Focus = dsl.Focus
	Skip  = dsl.Skip
)

// NewSpecForTest creates a Spec for tests (e.g. RNG determinism).
var NewSpecForTest = dsl.NewSpecForTest

// Analyze / registry.
var (
	Analyze           = dsl.Analyze
	CurrentSuite      = dsl.CurrentSuite
	CurrentArena      = dsl.CurrentArena
	AppendBeforeHook  = dsl.AppendBeforeHook
	AppendAfterHook   = dsl.AppendAfterHook
	SetPathGen        = dsl.SetPathGen
	PrintTree         = dsl.PrintTree
	PrintTreeArena    = dsl.PrintTreeArena
	Walk              = dsl.Walk
)

// SetCaptureCallerLocation enables or disables file/line capture for arena nodes.
// Call once from TestMain before any Describe or Analyze invocation.
func SetCaptureCallerLocation(v bool) { dsl.SetCaptureCallerLocation(v) }
