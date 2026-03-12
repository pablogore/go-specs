// arena.go defines the index-based tree used by the registry for Analyze mode.
package compiler

import (
	"github.com/pablogore/go-specs/specs/ctx"
	"github.com/pablogore/go-specs/specs/property"
)

// NodeType identifies the kind of node in the arena.
type NodeType int

const (
	SuiteNode NodeType = iota
	DescribeNode
	WhenNode
	ItNode
)

// NodeArena holds all nodes and hooks in flat slices; children are index references.
type NodeArena struct {
	Nodes       []ArenaNode
	Children    [][]int
	BeforeHooks [][]func(*ctx.Context)
	AfterHooks  [][]func(*ctx.Context)
}

// ArenaNode is one node in the arena (Describe/When/It).
type ArenaNode struct {
	Name     string
	Parent   int
	Type     NodeType
	Fn       func(*ctx.Context)
	File     string
	Line     int
	PathGen  *property.PathGenerator
}
