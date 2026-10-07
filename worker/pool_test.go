package worker

import (
	"errors"
	"testing"
	"time"

	"workflow-engine/logger"
	"workflow-engine/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeExecutor struct {
	fn func(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error)
}

func (f *fakeExecutor) ExecuteWorkflow(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error) {
	return f.fn(workflowID, input)
}

func waitForTerminal(t *testing.T, registry *Registry, jobID string) *Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := registry.Get(jobID)
		require.True(t, ok)
		if job.Status == JobCompleted || job.Status == JobFailed {
			return job
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("job never reached a terminal state")
	return nil
}

func TestPool_HappyPath(t *testing.T) {
	registry := NewRegistry()
	executor := &fakeExecutor{fn: func(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error) {
		return &models.ExecutionResult{Status: "approved"}, nil
	}}
	pool := NewPool(10, executor, registry)
	pool.Start(2)
	defer pool.Shutdown()

	job := registry.Create("1", map[string]interface{}{"age": 25})
	require.True(t, pool.Submit(job))

	final := waitForTerminal(t, registry, job.ID)
	assert.Equal(t, JobCompleted, final.Status)
	assert.Equal(t, "approved", final.Result.Status)
	assert.NotNil(t, final.StartedAt)
	assert.NotNil(t, final.FinishedAt)
}

func TestPool_FailurePath(t *testing.T) {
	registry := NewRegistry()
	executor := &fakeExecutor{fn: func(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error) {
		return nil, errors.New("workflow has no DSL")
	}}
	pool := NewPool(10, executor, registry)
	pool.Start(1)
	defer pool.Shutdown()

	job := registry.Create("2", map[string]interface{}{})
	require.True(t, pool.Submit(job))

	final := waitForTerminal(t, registry, job.ID)
	assert.Equal(t, JobFailed, final.Status)
	assert.Contains(t, final.Error, "workflow has no DSL")
}

func TestPool_PanicRecovery(t *testing.T) {
	registry := NewRegistry()
	callCount := 0
	executor := &fakeExecutor{fn: func(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error) {
		callCount++
		if callCount == 1 {
			panic("boom")
		}
		return &models.ExecutionResult{Status: "approved"}, nil
	}}
	pool := NewPool(10, executor, registry)
	pool.Start(1)
	defer pool.Shutdown()

	job1 := registry.Create("3", nil)
	require.True(t, pool.Submit(job1))
	final1 := waitForTerminal(t, registry, job1.ID)
	assert.Equal(t, JobFailed, final1.Status)
	assert.Contains(t, final1.Error, "panic")

	// The pool must still be alive and processing after a panic in one job.
	job2 := registry.Create("3", nil)
	require.True(t, pool.Submit(job2))
	final2 := waitForTerminal(t, registry, job2.ID)
	assert.Equal(t, JobCompleted, final2.Status)
}

func TestPool_QueueFullRejectsSubmit(t *testing.T) {
	registry := NewRegistry()
	executor := &fakeExecutor{fn: func(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error) {
		return &models.ExecutionResult{Status: "approved"}, nil
	}}
	// No Start() call — nothing drains the channel, so capacity 1 fills deterministically.
	pool := NewPool(1, executor, registry)
	defer pool.Shutdown()

	job1 := registry.Create("4", nil)
	job2 := registry.Create("4", nil)

	assert.True(t, pool.Submit(job1))
	assert.False(t, pool.Submit(job2))
}

func TestDispatcher_Enqueue_MarksFailedWhenQueueFull(t *testing.T) {
	registry := NewRegistry()
	executor := &fakeExecutor{fn: func(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error) {
		return &models.ExecutionResult{Status: "approved"}, nil
	}}
	pool := NewPool(1, executor, registry) // not started
	defer pool.Shutdown()
	dispatcher := NewDispatcher(pool, registry)

	// logger.Metrics is a shared global singleton, so assert the delta this
	// test causes rather than an absolute value — other tests in the same
	// binary run may also touch it.
	before := logger.Metrics.Snapshot().QueueErrors

	_, ok1 := dispatcher.Enqueue("5", nil)
	require.True(t, ok1)

	job2, ok2 := dispatcher.Enqueue("5", nil)
	assert.False(t, ok2)
	assert.Equal(t, JobFailed, job2.Status)
	assert.Contains(t, job2.Error, "queue is full")

	after := logger.Metrics.Snapshot().QueueErrors
	assert.Equal(t, int64(1), after-before, "exactly one QueueErrors increment expected from the full-queue Enqueue")
}
