// shard.go provides RunShard for CI (shard of a compiled Program).
package runner

import (
	"testing"

	"github.com/pablogore/go-specs/specs/compiler"
)

// RunShard runs a shard of the compiled Program for CI.
func RunShard(program *compiler.Program, tb testing.TB, shardIndex, shardCount int) {
	if program == nil || tb == nil {
		return
	}
	prog := compiler.ShardProgram(program, shardIndex, shardCount)
	if prog != nil {
		NewRunner(prog, nil).Run(tb)
	}
}
