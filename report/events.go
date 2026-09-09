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
	Duration    time.Duration // elapsed time between this suite's SuiteStartEvent and this event
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
//
// SpecStartEvent is always the exact event this spec's SpecStarted call sent (same Time, not
// reconstructed at finish time) — Duration is measured against it, so a producer that rebuilds
// SpecStartEvent here instead of reusing the original would silently corrupt Duration too.
type SpecResultEvent struct {
	SpecStartEvent
	Failed   bool
	Duration time.Duration // elapsed time between SpecStartEvent.Time and this event
	Message  string
}

// EventReporter consumes structured events from the spec runner.
type EventReporter interface {
	SuiteStarted(SuiteStartEvent)
	SuiteFinished(SuiteEndEvent)
	SpecStarted(SpecStartEvent)
	SpecFinished(SpecResultEvent)
}
