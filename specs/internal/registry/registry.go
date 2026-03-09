package registry

import "sync"

type NodeType int

const (
	NodeDescribe NodeType = iota
	NodeWhen
	NodeIt
)

type Handler func(any)

type Fixture func(any)

type ReporterHook func(any)

type ScopeMeta struct {
	FixturesBefore []Fixture
	FixturesAfter  []Fixture
	Snapshot       interface{}
	Parallel       bool
	ReporterHooks  []ReporterHook
}

type Node struct {
	Name     string
	Type     NodeType
	ID       uint32
	Fn       Handler
	Meta     ScopeMeta
	Parent   *Node
	Children []*Node
	Path     any
}

type Registry struct {
	mu     sync.Mutex
	Root   *Node
	stack  []*Node
	nextID uint32
}

func NewRegistry() *Registry {
	root := &Node{Name: "root", Type: NodeDescribe}
	return &Registry{Root: root, stack: []*Node{root}}
}

func (r *Registry) Push(name string, typ NodeType, fn Handler) *Node {
	r.mu.Lock()
	defer r.mu.Unlock()
	parent := r.stack[len(r.stack)-1]
	node := r.attachLocked(parent, name, typ, fn)
	r.stack = append(r.stack, node)
	return node
}

func (r *Registry) Pop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.stack) > 1 {
		r.stack = r.stack[:len(r.stack)-1]
	}
}

func (r *Registry) Current() *Node {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.stack) == 0 {
		return nil
	}
	return r.stack[len(r.stack)-1]
}

func (r *Registry) Attach(parent *Node, name string, typ NodeType, fn Handler) *Node {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attachLocked(parent, name, typ, fn)
}

func (r *Registry) attachLocked(parent *Node, name string, typ NodeType, fn Handler) *Node {
	if parent == nil {
		parent = r.Root
	}
	node := &Node{Name: name, Type: typ, Fn: fn, Parent: parent}
	if typ == NodeIt {
		node.ID = r.nextID
		r.nextID++
	}
	parent.Children = append(parent.Children, node)
	return node
}
