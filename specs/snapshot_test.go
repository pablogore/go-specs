package specs

import (
	"fmt"
	"strings"
	"testing"
)

func TestSnapshotMatchPasses(t *testing.T) {
	Describe(t, "SnapshotMatch", func(s *Spec) {
		s.It("matches stored snapshot", func(ctx *Context) {
			ctx.Snapshot("create-user", map[string]any{
				"id":   123,
				"name": "alice",
			})
		})
	})
}

func TestSnapshotMissingFails(t *testing.T) {
	fake := &fakeSnapshotBackend{}
	RunSnapshot(fake, "test.go", "nonexistent-key", 42)
	if fake.fatalfMsg == "" {
		t.Fatal("expected Fatalf when snapshot key is missing")
	}
	if fake.fatalfMsg != "" && !strings.Contains(fake.fatalfMsg, "missing") {
		t.Errorf("expected message about missing snapshot, got: %s", fake.fatalfMsg)
	}
}

type fakeSnapshotBackend struct {
	fatalfMsg string
}

func (f *fakeSnapshotBackend) Helper() {}

func (f *fakeSnapshotBackend) FailNow() {}

func (f *fakeSnapshotBackend) Fatal(args ...any) {
	f.fatalfMsg = fmt.Sprint(args...)
}

func (f *fakeSnapshotBackend) Fatalf(format string, args ...any) {
	f.fatalfMsg = fmt.Sprintf(format, args...)
}

func (f *fakeSnapshotBackend) Error(args ...any) {}

func (f *fakeSnapshotBackend) Errorf(format string, args ...any) {}

func (f *fakeSnapshotBackend) Log(args ...any) {}

func (f *fakeSnapshotBackend) Logf(format string, args ...any) {}

func (f *fakeSnapshotBackend) Name() string { return "" }

func (f *fakeSnapshotBackend) Cleanup(func()) {}

func (f *fakeSnapshotBackend) Run(name string, fn func(testing.TB)) { fn(nil) }
