package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"workflow-engine/logger"
	"workflow-engine/models"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testStream = "test:jobs:stream"

type fakeExecutor struct {
	fn func(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error)
}

func (f *fakeExecutor) ExecuteWorkflow(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error) {
	return f.fn(workflowID, input)
}

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	return NewRegistry(client, time.Hour)
}

func waitForTerminal(t *testing.T, ctx context.Context, registry *Registry, jobID string) *Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, ok, err := registry.Get(ctx, jobID)
		require.NoError(t, err)
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
	ctx := context.Background()
	registry := newTestRegistry(t)
	executor := &fakeExecutor{fn: func(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error) {
		return &models.ExecutionResult{Status: "approved"}, nil
	}}
	pool := NewPool(registry.client, testStream, 10, executor, registry)
	pool.Start(2)
	defer pool.Shutdown()

	job, err := registry.Create(ctx, "1", map[string]interface{}{"age": 25})
	require.NoError(t, err)
	submitted, err := pool.Submit(ctx, job)
	require.NoError(t, err)
	require.True(t, submitted)

	final := waitForTerminal(t, ctx, registry, job.ID)
	assert.Equal(t, JobCompleted, final.Status)
	assert.Equal(t, "approved", final.Result.Status)
	assert.NotNil(t, final.StartedAt)
	assert.NotNil(t, final.FinishedAt)
}

func TestPool_FailurePath(t *testing.T) {
	ctx := context.Background()
	registry := newTestRegistry(t)
	executor := &fakeExecutor{fn: func(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error) {
		return nil, errors.New("workflow has no DSL")
	}}
	pool := NewPool(registry.client, testStream, 10, executor, registry)
	pool.Start(1)
	defer pool.Shutdown()

	job, err := registry.Create(ctx, "2", map[string]interface{}{})
	require.NoError(t, err)
	submitted, err := pool.Submit(ctx, job)
	require.NoError(t, err)
	require.True(t, submitted)

	final := waitForTerminal(t, ctx, registry, job.ID)
	assert.Equal(t, JobFailed, final.Status)
	assert.Contains(t, final.Error, "workflow has no DSL")
}

func TestPool_PanicRecovery(t *testing.T) {
	ctx := context.Background()
	registry := newTestRegistry(t)
	callCount := 0
	executor := &fakeExecutor{fn: func(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error) {
		callCount++
		if callCount == 1 {
			panic("boom")
		}
		return &models.ExecutionResult{Status: "approved"}, nil
	}}
	pool := NewPool(registry.client, testStream, 10, executor, registry)
	pool.Start(1)
	defer pool.Shutdown()

	job1, err := registry.Create(ctx, "3", nil)
	require.NoError(t, err)
	submitted, err := pool.Submit(ctx, job1)
	require.NoError(t, err)
	require.True(t, submitted)

	final1 := waitForTerminal(t, ctx, registry, job1.ID)
	assert.Equal(t, JobFailed, final1.Status)
	assert.Contains(t, final1.Error, "panic")

	// The pool must still be alive and processing after a panic in one job.
	job2, err := registry.Create(ctx, "3", nil)
	require.NoError(t, err)
	submitted, err = pool.Submit(ctx, job2)
	require.NoError(t, err)
	require.True(t, submitted)

	final2 := waitForTerminal(t, ctx, registry, job2.ID)
	assert.Equal(t, JobCompleted, final2.Status)
}

func TestPool_QueueFullRejectsSubmit(t *testing.T) {
	ctx := context.Background()
	registry := newTestRegistry(t)
	executor := &fakeExecutor{fn: func(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error) {
		return &models.ExecutionResult{Status: "approved"}, nil
	}}
	// No Start() call — nothing consumes the stream, so XLen grows and
	// capacity 1 fills deterministically.
	pool := NewPool(registry.client, testStream, 1, executor, registry)
	defer pool.Shutdown()

	job1, err := registry.Create(ctx, "4", nil)
	require.NoError(t, err)
	job2, err := registry.Create(ctx, "4", nil)
	require.NoError(t, err)

	submitted1, err := pool.Submit(ctx, job1)
	require.NoError(t, err)
	assert.True(t, submitted1)

	submitted2, err := pool.Submit(ctx, job2)
	require.NoError(t, err)
	assert.False(t, submitted2)
}

func TestDispatcher_Enqueue_MarksFailedWhenQueueFull(t *testing.T) {
	ctx := context.Background()
	registry := newTestRegistry(t)
	executor := &fakeExecutor{fn: func(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error) {
		return &models.ExecutionResult{Status: "approved"}, nil
	}}
	pool := NewPool(registry.client, testStream, 1, executor, registry) // not started
	defer pool.Shutdown()
	dispatcher := NewDispatcher(pool, registry)

	// logger.Metrics is a shared global singleton, so assert the delta this
	// test causes rather than an absolute value — other tests in the same
	// binary run may also touch it.
	before := logger.Metrics.Snapshot().QueueErrors

	_, ok1, err := dispatcher.Enqueue(ctx, "5", nil)
	require.NoError(t, err)
	require.True(t, ok1)

	job2, ok2, err := dispatcher.Enqueue(ctx, "5", nil)
	require.NoError(t, err)
	assert.False(t, ok2)
	assert.Equal(t, JobFailed, job2.Status)
	assert.Contains(t, job2.Error, "queue is full")

	after := logger.Metrics.Snapshot().QueueErrors
	assert.Equal(t, int64(1), after-before, "exactly one QueueErrors increment expected from the full-queue Enqueue")
}
