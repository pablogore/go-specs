// sharding.go provides deterministic test sharding for CI.
package runner

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pablogore/go-specs/specs/compiler"
)

// ShardSpecs returns the subset of specs belonging to the given shard (i%total == shard).
func ShardSpecs(specs []RunSpec, shard, total int) []RunSpec {
	if total <= 0 || shard < 0 || shard >= total {
		return specs
	}
	out := make([]RunSpec, 0, len(specs)/total+1)
	for i := range specs {
		if i%total == shard {
			out = append(out, specs[i])
		}
	}
	return out
}

// ShardBCProgram returns a BCProgram containing only specs for the given shard.
func ShardBCProgram(prog compiler.BCProgram, shard, total int) compiler.BCProgram {
	return compiler.ShardBCProgram(prog, shard, total)
}

// ParseShardFlag parses -shard <shard>/<total> from args.
func ParseShardFlag(args []string) (shard, total int, ok bool) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] != "-shard" {
			continue
		}
		return parseShardPair(args[i+1])
	}
	return 0, 0, false
}

// ParseShardEnv reads shard from environment (SHARD_INDEX, SHARD_TOTAL or SHARD=2/10).
func ParseShardEnv() (shard, total int, ok bool) {
	if s := os.Getenv("SHARD"); s != "" {
		return parseShardPair(s)
	}
	idx := os.Getenv("SHARD_INDEX")
	tot := os.Getenv("SHARD_TOTAL")
	if idx == "" || tot == "" {
		return 0, 0, false
	}
	sh, err1 := strconv.Atoi(idx)
	totN, err2 := strconv.Atoi(tot)
	if err1 != nil || err2 != nil || totN <= 0 || sh < 0 || sh >= totN {
		return 0, 0, false
	}
	return sh, totN, true
}

// ParseShardString parses "N/M" into (shard, total).
func ParseShardString(s string) (shard, total int, ok bool) {
	return parseShardPair(s)
}

func parseShardPair(s string) (shard, total int, ok bool) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	sh, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	tot, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || tot <= 0 || sh < 0 || sh >= tot {
		return 0, 0, false
	}
	return sh, tot, true
}

// ShardFromArgsOrEnv returns (shard, total, true) if sharding is requested via flag or env.
func ShardFromArgsOrEnv() (shard, total int, ok bool) {
	if shard, total, ok = ParseShardFlag(os.Args); ok {
		return shard, total, true
	}
	return ParseShardEnv()
}

// FormatShardFlag returns a flag string e.g. "-shard 2/10".
func FormatShardFlag(shard, total int) string {
	return fmt.Sprintf("-shard %d/%d", shard, total)
}
