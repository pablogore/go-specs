// runner_reexport re-exports runner package types and functions so the public API remains specs.NewRunner, specs.RunSpec, etc.
package specs

import (
	"testing"

	"github.com/pablogore/go-specs/specs/runner"
)

// Runner types and constructors.
type (
	Runner       = runner.Runner
	MinimalRunner = runner.MinimalRunner
	BlockRunner   = runner.BlockRunner
	BytecodeRunner = runner.BytecodeRunner
	RunSpec      = runner.RunSpec
	SpecBlock    = runner.SpecBlock
)

var (
	NewRunner             = runner.NewRunner
	NewRunnerFromProgram  = runner.NewRunnerFromProgram
	NewMinimalRunner      = runner.NewMinimalRunner
	NewMinimalRunnerFromSpecs = runner.NewMinimalRunnerFromSpecs
	NewBlockRunner        = runner.NewBlockRunner
	NewBytecodeRunner     = runner.NewBytecodeRunner
	CompileBlocks         = runner.CompileBlocks
)

// Sharding and parallel execution.
var (
	ShardSpecs        = runner.ShardSpecs
	ParseShardFlag    = runner.ParseShardFlag
	ParseShardEnv     = runner.ParseShardEnv
	ParseShardString  = runner.ParseShardString
	ShardFromArgsOrEnv = runner.ShardFromArgsOrEnv
	FormatShardFlag   = runner.FormatShardFlag
)

const (
	DefaultBlockSize = runner.DefaultBlockSize
	RunBatchSize     = runner.RunBatchSize
	DefaultChunkSize = runner.DefaultChunkSize
)

// RunShard runs a shard of the compiled Program for CI.
func RunShard(program *Program, tb testing.TB, shardIndex, shardCount int) {
	runner.RunShard(program, tb, shardIndex, shardCount)
}

// RunCompiledSuite executes the compiled suite (runner layer). Use instead of suite.Run(tb).
func RunCompiledSuite(suite *CompiledSuite, tb testing.TB) {
	runner.RunCompiledSuite(suite, tb)
}
