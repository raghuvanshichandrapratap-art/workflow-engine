package worker

import (
	"context"
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

// Enqueue creates a job and submits it. The bool return is false when the
// queue was full — the job is still recorded, already marked failed, so a
// client that calls Get on the returned ID sees a coherent reason why.
func (d *Dispatcher) Enqueue(ctx context.Context, workflowID string, input map[string]interface{}) (*Job, bool, error) {
	job, err := d.registry.Create(ctx, workflowID, input)
	if err != nil {
		return nil, false, err
	}
	slog.Info("job queued", "job_id", job.ID, "workflow_id", workflowID)

	submitted, err := d.pool.Submit(ctx, job)
	if err != nil {
		return job, false, err
	}

	if !submitted {
		logger.Metrics.QueueErrors.Add(1)
		slog.Warn("job queue full", "job_id", job.ID, "workflow_id", workflowID)
		failErr := fmt.Errorf("job queue is full")
		if markErr := d.registry.MarkFailed(ctx, job.ID, failErr); markErr != nil {
			slog.Error("failed to mark job failed", "job_id", job.ID, "error", markErr.Error())
		}

		// Create's returned struct and MarkFailed's internally-fetched copy
		// are separate deserializations now that storage is Redis, not a
		// shared map entry — re-fetch so the caller sees the real status.
		if fresh, ok, getErr := d.registry.Get(ctx, job.ID); getErr == nil && ok {
			job = fresh
		} else {
			job.Status = JobFailed
			job.Error = failErr.Error()
		}
		return job, false, nil
	}

	return job, true, nil
}

func (d *Dispatcher) Get(ctx context.Context, jobID string) (*Job, bool, error) {
	return d.registry.Get(ctx, jobID)
}
