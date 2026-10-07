package worker

import (
	"fmt"
	"log/slog"
	"sync"

	"workflow-engine/models"
)

// Executor is the dependency Pool needs — *services.WorkflowService
// satisfies it implicitly. The worker package never touches the DB or AI
// directly; it just calls the existing service method, which is also why
// a completed job lands in execution_logs for free.
type Executor interface {
	ExecuteWorkflow(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error)
}

// Pool is a fixed-size worker pool consuming a bounded in-memory job queue.
type Pool struct {
	jobs     chan *Job
	registry *Registry
	executor Executor
	wg       sync.WaitGroup
}

func NewPool(queueCapacity int, executor Executor, registry *Registry) *Pool {
	return &Pool{
		jobs:     make(chan *Job, queueCapacity),
		registry: registry,
		executor: executor,
	}
}

// Start spawns workerCount goroutines, each consuming jobs until the queue
// channel is closed (via Shutdown).
func (p *Pool) Start(workerCount int) {
	for i := 0; i < workerCount; i++ {
		p.wg.Add(1)
		go p.runWorker()
	}
}

func (p *Pool) runWorker() {
	defer p.wg.Done()
	for job := range p.jobs {
		p.process(job)
	}
}

func (p *Pool) process(job *Job) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("worker panic recovered", "job_id", job.ID, "workflow_id", job.WorkflowID, "panic", r)
			p.registry.MarkFailed(job.ID, fmt.Errorf("panic: %v", r))
		}
	}()

	slog.Info("job running", "job_id", job.ID, "workflow_id", job.WorkflowID)
	p.registry.MarkRunning(job.ID)

	result, err := p.executor.ExecuteWorkflow(job.WorkflowID, job.Input)
	if err != nil {
		slog.Error("job failed", "job_id", job.ID, "workflow_id", job.WorkflowID, "error", err.Error())
		p.registry.MarkFailed(job.ID, err)
		return
	}

	slog.Info("job completed", "job_id", job.ID, "workflow_id", job.WorkflowID, "status", result.Status)
	p.registry.MarkCompleted(job.ID, result)
}

// Submit enqueues a job without blocking; returns false if the queue is full.
func (p *Pool) Submit(job *Job) bool {
	select {
	case p.jobs <- job:
		return true
	default:
		return false
	}
}

// Shutdown stops accepting new work implicitly (closing the channel means
// any further Submit would panic — callers must stop calling Submit first,
// e.g. by closing the HTTP listener before calling Shutdown) and waits for
// already-queued jobs to drain.
func (p *Pool) Shutdown() {
	close(p.jobs)
	p.wg.Wait()
}
