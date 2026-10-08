package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"workflow-engine/logger"
	"workflow-engine/models"

	goredis "github.com/redis/go-redis/v9"
)

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
)

type Job struct {
	ID         string                  `json:"id"`
	WorkflowID string                  `json:"workflow_id"`
	Input      map[string]interface{}  `json:"input"`
	Status     JobStatus               `json:"status"`
	Result     *models.ExecutionResult `json:"result,omitempty"`
	Error      string                  `json:"error,omitempty"`
	CreatedAt  time.Time               `json:"created_at"`
	StartedAt  *time.Time              `json:"started_at,omitempty"`
	FinishedAt *time.Time              `json:"finished_at,omitempty"`
}

// Registry is the durable job store, backed by Redis. A job is only ever
// mutated by the single worker goroutine processing it, so the read-modify-
// write shape of the Mark* methods never races against a concurrent writer —
// only concurrent readers (HTTP polling via Get), which a plain Redis SET
// already makes safe (a reader sees either the old or the new value, never
// a partial write). Keys expire after ttl so completed jobs don't accumulate
// forever.
type Registry struct {
	client *goredis.Client
	ttl    time.Duration
}

func NewRegistry(client *goredis.Client, ttl time.Duration) *Registry {
	return &Registry{client: client, ttl: ttl}
}

func jobKey(id string) string {
	return "job:" + id
}

func (r *Registry) Create(ctx context.Context, workflowID string, input map[string]interface{}) (*Job, error) {
	job := &Job{
		ID:         logger.NewID(),
		WorkflowID: workflowID,
		Input:      input,
		Status:     JobQueued,
		CreatedAt:  time.Now(),
	}

	if err := r.save(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *Registry) MarkRunning(ctx context.Context, id string) error {
	return r.update(ctx, id, func(job *Job) {
		now := time.Now()
		job.Status = JobRunning
		job.StartedAt = &now
	})
}

func (r *Registry) MarkCompleted(ctx context.Context, id string, result *models.ExecutionResult) error {
	return r.update(ctx, id, func(job *Job) {
		now := time.Now()
		job.Status = JobCompleted
		job.Result = result
		job.FinishedAt = &now
	})
}

func (r *Registry) MarkFailed(ctx context.Context, id string, failErr error) error {
	return r.update(ctx, id, func(job *Job) {
		now := time.Now()
		job.Status = JobFailed
		job.Error = failErr.Error()
		job.FinishedAt = &now
	})
}

func (r *Registry) Get(ctx context.Context, id string) (*Job, bool, error) {
	raw, err := r.client.Get(ctx, jobKey(id)).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var job Job
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		return nil, false, fmt.Errorf("failed to parse stored job %s: %w", id, err)
	}
	return &job, true, nil
}

func (r *Registry) update(ctx context.Context, id string, mutate func(*Job)) error {
	job, found, err := r.Get(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("job %s not found", id)
	}

	mutate(job)
	return r.save(ctx, job)
}

func (r *Registry) save(ctx context.Context, job *Job) error {
	raw, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to serialize job %s: %w", job.ID, err)
	}
	return r.client.Set(ctx, jobKey(job.ID), raw, r.ttl).Err()
}
