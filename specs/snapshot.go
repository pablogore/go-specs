package specs

import "github.com/getsyntegrity/go-specs/snapshots"

// runSnapshot compares value to the stored snapshot for name, or creates/updates it.
// callerFile is the path to the test file (from runtime.Caller(1) in Context.Snapshot).
func runSnapshot(backend testBackend, callerFile string, name string, value any) {
	snapshots.RunFromFile(backend, callerFile, name, value)
}
