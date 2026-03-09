package mock

// Mock holds named spies for verification.
type Mock struct {
	spies map[string]*Spy
}

// New returns a new Mock.
func New() *Mock {
	return &Mock{
		spies: map[string]*Spy{},
	}
}

// Spy returns the spy for the given name, creating it if needed.
func (m *Mock) Spy(name string) *Spy {
	if m == nil {
		return nil
	}
	if s, ok := m.spies[name]; ok {
		return s
	}
	s := NewSpy()
	m.spies[name] = s
	return s
}
