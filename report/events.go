package report

import "time"

// SuiteStartEvent is emitted when a suite (root Describe) begins.
type SuiteStartEvent struct {
	Name string
	Time time.Time
}

// SuiteEndEvent is emitted when a suite finishes executing.
type SuiteEndEvent struct {
	Name        string
	Time        time.Time
	TotalSpecs  int
	FailedSpecs int
}

// SpecStartEvent captures the start of an individual spec (It/Then).
type SpecStartEvent struct {
	Name string
	Path []string
	Time time.Time
}

// SpecResultEvent captures the result of an individual spec.
type SpecResultEvent struct {
	SpecStartEvent
	Failed  bool
	Message string
}

// EventReporter consumes structured events from the spec runner.
type EventReporter interface {
	SuiteStarted(SuiteStartEvent)
	SuiteFinished(SuiteEndEvent)
	SpecStarted(SpecStartEvent)
	SpecFinished(SpecResultEvent)
}
