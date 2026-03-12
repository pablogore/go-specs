package dsl

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pablogore/go-specs/specs/compiler"
	"github.com/pablogore/go-specs/specs/ctx"
	"github.com/pablogore/go-specs/specs/property"
)

// Node is the legacy pointer-based node type (retained for reference / SuiteTree).
type Node struct {
	Name        string
	Type        compiler.NodeType
	Children    []*Node
	Parent      *Node
	Fn          func(*ctx.Context)
	File        string
	Line        int
	PathGen     *property.PathGenerator
	BeforeHooks []func(*ctx.Context)
	AfterHooks  []func(*ctx.Context)
	BeforeAll   []func(*ctx.Context)
	AfterAll    []func(*ctx.Context)
}

type registry struct {
	mu    sync.Mutex
	arena *compiler.NodeArena
	stack []int
}

type registryStack struct {
	mu    sync.Mutex
	stack []*registry
}

var activeRegistries registryStack
var analyzeLock sync.Mutex

const initialArenaCap = 4096

func newRegistry() *registry {
	arena := &compiler.NodeArena{
		Nodes:       make([]compiler.ArenaNode, 0, initialArenaCap),
		Children:    make([][]int, 0, initialArenaCap),
		BeforeHooks: make([][]func(*ctx.Context), 0, initialArenaCap),
		AfterHooks:  make([][]func(*ctx.Context), 0, initialArenaCap),
	}
	arena.Nodes = append(arena.Nodes, compiler.ArenaNode{Name: "suite", Type: compiler.SuiteNode, Parent: -1})
	arena.Children = append(arena.Children, nil)
	arena.BeforeHooks = append(arena.BeforeHooks, nil)
	arena.AfterHooks = append(arena.AfterHooks, nil)
	return &registry{arena: arena, stack: []int{0}}
}

func (r *registry) currentSuite() *SuiteTree {
	r.mu.Lock()
	defer r.mu.Unlock()
	return &SuiteTree{Arena: r.arena, RootID: 0}
}

func (r *registry) enterNode(nodeType compiler.NodeType, name, file string, line int, fn func(*ctx.Context)) (int, func()) {
	r.mu.Lock()
	parentID := r.stack[len(r.stack)-1]
	id := len(r.arena.Nodes)
	r.arena.Nodes = append(r.arena.Nodes, compiler.ArenaNode{
		Name: name, Parent: parentID, Type: nodeType, Fn: fn,
		File: file, Line: line,
	})
	r.arena.Children = append(r.arena.Children, nil)
	r.arena.BeforeHooks = append(r.arena.BeforeHooks, nil)
	r.arena.AfterHooks = append(r.arena.AfterHooks, nil)
	r.arena.Children[parentID] = append(r.arena.Children[parentID], id)
	r.stack = append(r.stack, id)
	r.mu.Unlock()
	return id, func() {
		r.mu.Lock()
		if len(r.stack) > 1 {
			r.stack = r.stack[:len(r.stack)-1]
		}
		r.mu.Unlock()
	}
}

func (r *registry) appendBeforeHook(fn func(*ctx.Context)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.stack) == 0 {
		return
	}
	id := r.stack[len(r.stack)-1]
	r.arena.BeforeHooks[id] = append(r.arena.BeforeHooks[id], fn)
}

func (r *registry) appendAfterHook(fn func(*ctx.Context)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.stack) == 0 {
		return
	}
	id := r.stack[len(r.stack)-1]
	r.arena.AfterHooks[id] = append(r.arena.AfterHooks[id], fn)
}

func (r *registry) setPathGen(gen *property.PathGenerator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.stack) == 0 {
		return
	}
	id := r.stack[len(r.stack)-1]
	if id < len(r.arena.Nodes) {
		r.arena.Nodes[id].PathGen = gen
	}
}

func pushRegistry(r *registry) func() {
	activeRegistries.mu.Lock()
	activeRegistries.stack = append(activeRegistries.stack, r)
	activeRegistries.mu.Unlock()
	return func() {
		activeRegistries.mu.Lock()
		if len(activeRegistries.stack) > 0 {
			activeRegistries.stack = activeRegistries.stack[:len(activeRegistries.stack)-1]
		}
		activeRegistries.mu.Unlock()
	}
}

func currentRegistry() *registry {
	activeRegistries.mu.Lock()
	defer activeRegistries.mu.Unlock()
	if len(activeRegistries.stack) == 0 {
		return nil
	}
	return activeRegistries.stack[len(activeRegistries.stack)-1]
}

func ensureRegistry() func() {
	if currentRegistry() != nil {
		return func() {}
	}
	return pushRegistry(newRegistry())
}

// CurrentArena returns the current registry's arena, or nil if none is active.
func CurrentArena() *compiler.NodeArena {
	reg := currentRegistry()
	if reg == nil {
		return nil
	}
	reg.mu.Lock()
	a := reg.arena
	reg.mu.Unlock()
	return a
}

// AppendBeforeHook appends a before-each hook to the current node.
func AppendBeforeHook(fn func(*ctx.Context)) {
	reg := currentRegistry()
	if reg != nil {
		reg.appendBeforeHook(fn)
	}
}

// AppendAfterHook appends an after-each hook to the current node.
func AppendAfterHook(fn func(*ctx.Context)) {
	reg := currentRegistry()
	if reg != nil {
		reg.appendAfterHook(fn)
	}
}

// SetPathGen sets the PathGenerator on the current node.
func SetPathGen(gen *property.PathGenerator) {
	reg := currentRegistry()
	if reg != nil {
		reg.setPathGen(gen)
	}
}

// Analyze builds the spec tree by running fn and returns the SuiteTree.
func Analyze(fn func()) *SuiteTree {
	analyzeLock.Lock()
	defer analyzeLock.Unlock()
	reg := newRegistry()
	pop := pushRegistry(reg)
	if fn != nil {
		fn()
	}
	pop()
	return reg.currentSuite()
}

// CurrentSuite returns the current registry's suite, or nil.
func CurrentSuite() *SuiteTree {
	it := currentRegistry()
	if it == nil {
		return nil
	}
	return it.currentSuite()
}

func enterAnalyzeNode(nodeType compiler.NodeType, name, file string, line int, fn func(*ctx.Context)) (int, func()) {
	reg := currentRegistry()
	if reg == nil {
		return -1, func() {}
	}
	return reg.enterNode(nodeType, name, file, line, fn)
}

// PrintTree prints the pointer-based node tree (legacy).
func PrintTree(node *Node, depth int, w io.Writer) {
	if node == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	if w == nil {
		w = io.Discard
	}
	_, _ = fmt.Fprintf(w, "%s%s\n", indent, node.Name)
	for _, child := range node.Children {
		PrintTree(child, depth+1, w)
	}
}

// PrintTreeArena prints the arena-based tree from rootID.
func PrintTreeArena(arena *compiler.NodeArena, rootID int, depth int, w io.Writer) {
	if arena == nil || rootID < 0 || rootID >= len(arena.Nodes) {
		return
	}
	if w == nil {
		w = io.Discard
	}
	indent := strings.Repeat("  ", depth)
	_, _ = fmt.Fprintf(w, "%s%s\n", indent, arena.Nodes[rootID].Name)
	for _, cid := range arena.Children[rootID] {
		PrintTreeArena(arena, cid, depth+1, w)
	}
}

// Walk visits every node in the pointer-based tree.
func Walk(node *Node, fn func(*Node)) {
	if node == nil || fn == nil {
		return
	}
	fn(node)
	for _, child := range node.Children {
		Walk(child, fn)
	}
}

// captureCallerLocation is accessed concurrently (test goroutines vs TestMain),
// so it must be read/written atomically.
var captureCallerLocation atomic.Bool

// SetCaptureCallerLocation enables or disables file/line capture for arena nodes.
// Call from TestMain before any Describe or Analyze invocation.
func SetCaptureCallerLocation(v bool) { captureCallerLocation.Store(v) }

func callerLocation(skip int) (string, int) {
	if !captureCallerLocation.Load() {
		return "", 0
	}
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "", 0
	}
	return file, line
}
