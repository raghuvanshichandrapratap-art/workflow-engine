package logger

import "sync/atomic"

// Counters is a small set of atomic counters for the execution lifecycle.
// NewCounters gives tests an isolated instance; Metrics is the global
// singleton production code increments and GET /metrics reads from.
type Counters struct {
	ExecutionsStarted   atomic.Int64
	ExecutionsSucceeded atomic.Int64
	ExecutionsFailed    atomic.Int64
	QueueErrors         atomic.Int64
}

func NewCounters() *Counters {
	return &Counters{}
}

var Metrics = NewCounters()

type Snapshot struct {
	ExecutionsStarted   int64 `json:"executions_started"`
	ExecutionsSucceeded int64 `json:"executions_succeeded"`
	ExecutionsFailed    int64 `json:"executions_failed"`
	QueueErrors         int64 `json:"queue_errors"`
}

func (c *Counters) Snapshot() Snapshot {
	return Snapshot{
		ExecutionsStarted:   c.ExecutionsStarted.Load(),
		ExecutionsSucceeded: c.ExecutionsSucceeded.Load(),
		ExecutionsFailed:    c.ExecutionsFailed.Load(),
		QueueErrors:         c.QueueErrors.Load(),
	}
}
