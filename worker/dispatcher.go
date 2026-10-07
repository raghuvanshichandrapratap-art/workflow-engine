package worker

import (
	"fmt"
	"log/slog"

	"workflow-engine/logger"
)

// Dispatcher is the single dependency handlers hold — it combines the pool
// (where to submit work) and the registry (where to look up status).
type Dispatcher struct {
	pool     *Pool
	registry *Registry
}

func NewDispatcher(pool *Pool, registry *Registry) *Dispatcher {
	return &Dispatcher{pool: pool, registry: registry}
}

// Enqueue creates a job and submits it. The bool return is false only when
// the queue was full — the job is still recorded, already marked failed,
// so a client that calls Get on the returned ID sees a coherent reason why.
func (d *Dispatcher) Enqueue(workflowID string, input map[string]interface{}) (*Job, bool) {
	job := d.registry.Create(workflowID, input)
	slog.Info("job queued", "job_id", job.ID, "workflow_id", workflowID)

	if !d.pool.Submit(job) {
		logger.Metrics.QueueErrors.Add(1)
		slog.Warn("job queue full", "job_id", job.ID, "workflow_id", workflowID)
		d.registry.MarkFailed(job.ID, fmt.Errorf("job queue is full"))
		return job, false
	}

	return job, true
}

func (d *Dispatcher) Get(jobID string) (*Job, bool) {
	return d.registry.Get(jobID)
}
