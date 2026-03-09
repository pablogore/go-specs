package mock

import (
	"sync"
	"testing"
)

func TestMockCallConcurrentInvocations(t *testing.T) {
	m := New()
	spy := m.Spy("invoke")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			spy.Call(v)
		}(i)
	}
	wg.Wait()
	spy.CalledTimes(t, 50)
}

func TestSpyConcurrentInvocations(t *testing.T) {
	spy := NewSpy()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			spy.Call(v)
		}(i)
	}
	wg.Wait()
	spy.CalledTimes(t, 100)
}
