//go:build !js && !wasm && !lint
// +build !js,!wasm,!lint

package benchmarks

import (
	"testing"
)

// Large-scale suite benchmarks: simulate 100, 1000, 10000, 50000 specs.
// Suite generation (CreateGoSpecsSuite) runs outside the timed region; the loop measures
// execution time, memory allocations, and scaling behavior only.

func BenchmarkSuite_100(b *testing.B) {
	suite := CreateGoSpecsSuite(100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		suite(b)
	}
}

func BenchmarkSuite_1000(b *testing.B) {
	suite := CreateGoSpecsSuite(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		suite(b)
	}
}

func BenchmarkSuite_10000(b *testing.B) {
	suite := CreateGoSpecsSuite(10000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		suite(b)
	}
}

func BenchmarkSuite_50000(b *testing.B) {
	suite := CreateGoSpecsSuite(50000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		suite(b)
	}
}
