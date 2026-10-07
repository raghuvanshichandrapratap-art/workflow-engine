package worker

import (
	"sync"
	"time"

	"workflow-engine/logger"
	"workflow-engine/models"
)

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
)

type Job struct {
	ID         string                 `json:"id"`
	WorkflowID string                 `json:"workflow_id"`
	Input      map[string]interface{} `json:"input"`
	Status     JobStatus              `json:"status"`
	Result     *models.ExecutionResult `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	StartedAt  *time.Time             `json:"started_at,omitempty"`
	FinishedAt *time.Time             `json:"finished_at,omitempty"`
}

// Registry is the concurrency-safe job store. One mutex guards the whole
// map, and every status transition goes through a Registry method rather
// than mutating a Job's fields directly from outside the lock. Get returns
// a copy, not the internal pointer, so a caller can never read a field
// while a worker is mid-write to it.
type Registry struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

func NewRegistry() *Registry {
	return &Registry{jobs: make(map[string]*Job)}
}

func (r *Registry) Create(workflowID string, input map[string]interface{}) *Job {
	r.mu.Lock()
	defer r.mu.Unlock()

	job := &Job{
		ID:         logger.NewID(),
		WorkflowID: workflowID,
		Input:      input,
		Status:     JobQueued,
		CreatedAt:  time.Now(),
	}
	r.jobs[job.ID] = job
	return job
}

func (r *Registry) MarkRunning(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if job, ok := r.jobs[id]; ok {
		now := time.Now()
		job.Status = JobRunning
		job.StartedAt = &now
	}
}

func (r *Registry) MarkCompleted(id string, result *models.ExecutionResult) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if job, ok := r.jobs[id]; ok {
		now := time.Now()
		job.Status = JobCompleted
		job.Result = result
		job.FinishedAt = &now
	}
}

func (r *Registry) MarkFailed(id string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if job, ok := r.jobs[id]; ok {
		now := time.Now()
		job.Status = JobFailed
		job.Error = err.Error()
		job.FinishedAt = &now
	}
}

func (r *Registry) Get(id string) (*Job, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	job, ok := r.jobs[id]
	if !ok {
		return nil, false
	}
	cp := *job
	return &cp, true
}
