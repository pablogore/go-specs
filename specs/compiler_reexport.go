// compiler_reexport re-exports compiler package types and functions so the public API remains specs.Program, specs.Builder, etc.
package specs

import "github.com/pablogore/go-specs/specs/compiler"

// Compiler types (implementation in specs/compiler).
type (
	Program       = compiler.Program
	Builder       = compiler.Builder
	ExecutionPlan = compiler.ExecutionPlan
	CompiledSuite = compiler.CompiledSuite
	Instruction   = compiler.Instruction
	OpCode        = compiler.OpCode
	NodeArena     = compiler.NodeArena
	ArenaNode     = compiler.ArenaNode
	NodeType      = compiler.NodeType
	SpecFn        = compiler.SpecFn
)

// Compiler constants.
const (
	SuiteNode     = compiler.SuiteNode
	DescribeNode  = compiler.DescribeNode
	WhenNode      = compiler.WhenNode
	ItNode        = compiler.ItNode
	OpSetPath     = compiler.OpSetPath
	OpBeforeHook  = compiler.OpBeforeHook
	OpBody        = compiler.OpBody
	OpAfterHook   = compiler.OpAfterHook
	OpRunSpec     = compiler.OpRunSpec
	OpBeforeEach  = compiler.OpBeforeEach
	OpAfterEach   = compiler.OpAfterEach
)

// Compiler constructors and helpers.
var (
	NewBuilder = compiler.NewBuilder
)

// BuildPlanFromArena builds an ExecutionPlan from an arena (registry/Analyze path).
func BuildPlanFromArena(arena *NodeArena, rootID int) *ExecutionPlan {
	return compiler.BuildPlanFromArena(arena, rootID)
}

// ShardProgram returns a Program with only groups for the given shard (for CI).
func ShardProgram(program *Program, shardIndex, shardCount int) *Program {
	return compiler.ShardProgram(program, shardIndex, shardCount)
}
