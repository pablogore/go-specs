package specs

import (
	"sync"
	"testing"
)

// SnapshotMatcher defines the function signature used for snapshot assertions.
type SnapshotMatcher func(testing.TB, any)

var (
	defaultSnapshotMatcher SnapshotMatcher
	snapshotMatcherLock    sync.RWMutex
)

// RegisterSnapshotMatcher allows extensions to provide snapshot matching. Pass nil to clear the handler.
func RegisterSnapshotMatcher(fn SnapshotMatcher) {
	snapshotMatcherLock.Lock()
	defer snapshotMatcherLock.Unlock()
	defaultSnapshotMatcher = fn
}

func currentSnapshotMatcher() SnapshotMatcher {
	snapshotMatcherLock.RLock()
	defer snapshotMatcherLock.RUnlock()
	return defaultSnapshotMatcher
}

func enforceSnapshotMatcher(t testing.TB, handler SnapshotMatcher, actual any) {
	if handler == nil {
		t.Fatalf("snapshot matcher not registered; import github.com/pablogore/go-specs/snapshots and register if using custom matcher")
		return
	}
	handler(t, actual)
}
