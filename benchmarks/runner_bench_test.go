//go:build !js && !wasm && !lint
// +build !js,!wasm,!lint

package benchmarks

import (
	"testing"
)

// Runner benchmarks: full execution of N specs, one assertion per spec. Suite created before ResetTimer; loop runs the returned function.
// go-specs target: 0 allocs/op in the run loop. Fixed size (SuiteSize1000) for fair comparison.

func BenchmarkRunner_GoSpecs(b *testing.B) {
	suite := CreateGoSpecsSuite(SuiteSize1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		suite(b)
	}
}

func BenchmarkRunner_Testify(b *testing.B) {
	suite := CreateTestifySuite(SuiteSize1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		suite(b)
	}
}

func BenchmarkRunner_Gomega(b *testing.B) {
	suite := CreateGomegaSuite(SuiteSize1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		suite(b)
	}
}
