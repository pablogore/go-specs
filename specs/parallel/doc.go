// Package parallel provides the parallel execution scheduler: RunWorker, RunWorkerBatched,
// ReportFailures, ParallelBackend, and FailureReporter. It depends on specs/ctx and
// specs/property. It does not import the root specs package or specs/runner.
// Runner implementations (MinimalRunner.RunParallel, BytecodeRunner.RunParallel) use this package.
package parallel
