package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"workflow-engine/models"

	goredis "github.com/redis/go-redis/v9"
)

// Executor is the dependency Pool needs — *services.WorkflowService
// satisfies it implicitly. The worker package never touches the DB or AI
// directly; it just calls the existing service method, which is also why
// a completed job lands in execution_logs for free.
type Executor interface {
	ExecuteWorkflow(workflowID string, input map[string]interface{}) (*models.ExecutionResult, error)
}

const consumerGroup = "workers"

// Pool is a fixed-size worker pool consuming a durable Redis Stream queue.
// A message only counts as done once XAck'd — a worker that crashes
// mid-job leaves its message pending, reclaimed by the next Start's
// XAutoClaim pass instead of being lost, unlike a plain in-memory channel.
type Pool struct {
	client   *goredis.Client
	stream   string
	capacity int64
	registry *Registry
	executor Executor
	wg       sync.WaitGroup
	cancel   context.CancelFunc
}

func NewPool(client *goredis.Client, stream string, capacity int64, executor Executor, registry *Registry) *Pool {
	return &Pool{
		client:   client,
		stream:   stream,
		capacity: capacity,
		registry: registry,
		executor: executor,
	}
}

// Start ensures the consumer group exists, reclaims anything left pending
// from a previous crashed run, then spawns workerCount goroutines.
func (p *Pool) Start(workerCount int) {
	setupCtx := context.Background()

	// "BUSYGROUP Consumer Group name already exists" has no typed error in
	// go-redis — string matching is the conventional way to treat it as
	// expected (the group was already created by an earlier run) rather
	// than a real failure.
	if err := p.client.XGroupCreateMkStream(setupCtx, p.stream, consumerGroup, "0").Err(); err != nil &&
		!strings.Contains(err.Error(), "BUSYGROUP") {
		slog.Error("failed to create consumer group", "stream", p.stream, "error", err.Error())
	}

	runCtx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	for i := 0; i < workerCount; i++ {
		consumerName := fmt.Sprintf("worker-%d", i)

		// Reclaim anything left pending from a previous crashed run before
		// this worker starts consuming new messages.
		p.reclaim(setupCtx, consumerName)

		p.wg.Add(1)
		go p.runWorker(runCtx, consumerName)
	}
}

func (p *Pool) reclaim(ctx context.Context, consumerName string) {
	messages, _, err := p.client.XAutoClaim(ctx, &goredis.XAutoClaimArgs{
		Stream:   p.stream,
		Group:    consumerGroup,
		MinIdle:  30 * time.Second,
		Start:    "0",
		Consumer: consumerName,
		Count:    10,
	}).Result()
	if err != nil {
		slog.Error("failed to reclaim pending messages", "consumer", consumerName, "error", err.Error())
		return
	}

	for _, msg := range messages {
		slog.Info("reclaimed pending job", "consumer", consumerName, "message_id", msg.ID)
		p.handleMessage(ctx, msg)
	}
}

func (p *Pool) runWorker(ctx context.Context, consumerName string) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		streams, err := p.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: consumerName,
			Streams:  []string{p.stream, ">"},
			Count:    1,
			Block:    200 * time.Millisecond,
		}).Result()

		if err != nil {
			if errors.Is(err, goredis.Nil) || errors.Is(err, context.Canceled) {
				continue // block timeout with no new messages, or shutting down — ctx.Done() above handles the latter
			}
			slog.Error("xreadgroup failed", "consumer", consumerName, "error", err.Error())
			time.Sleep(time.Second) // avoid a hot loop on a transient Redis issue
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				p.handleMessage(ctx, msg)
			}
		}
	}
}

func (p *Pool) handleMessage(ctx context.Context, msg goredis.XMessage) {
	jobID, _ := msg.Values["job_id"].(string)
	if jobID == "" {
		slog.Error("stream message missing job_id", "message_id", msg.ID)
		p.client.XAck(ctx, p.stream, consumerGroup, msg.ID)
		return
	}

	p.process(ctx, jobID)

	if err := p.client.XAck(ctx, p.stream, consumerGroup, msg.ID).Err(); err != nil {
		slog.Error("failed to ack message", "job_id", jobID, "message_id", msg.ID, "error", err.Error())
	}
}

func (p *Pool) process(ctx context.Context, jobID string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("worker panic recovered", "job_id", jobID, "panic", r)
			p.registry.MarkFailed(ctx, jobID, fmt.Errorf("panic: %v", r))
		}
	}()

	job, found, err := p.registry.Get(ctx, jobID)
	if err != nil || !found {
		slog.Error("job not found for processing", "job_id", jobID)
		return
	}

	slog.Info("job running", "job_id", job.ID, "workflow_id", job.WorkflowID)
	p.registry.MarkRunning(ctx, job.ID)

	result, err := p.executor.ExecuteWorkflow(job.WorkflowID, job.Input)
	if err != nil {
		slog.Error("job failed", "job_id", job.ID, "workflow_id", job.WorkflowID, "error", err.Error())
		p.registry.MarkFailed(ctx, job.ID, err)
		return
	}

	slog.Info("job completed", "job_id", job.ID, "workflow_id", job.WorkflowID, "status", result.Status)
	p.registry.MarkCompleted(ctx, job.ID, result)
}

// Submit enqueues a job without blocking; returns false if the stream is at capacity.
func (p *Pool) Submit(ctx context.Context, job *Job) (bool, error) {
	length, err := p.client.XLen(ctx, p.stream).Result()
	if err != nil {
		return false, err
	}
	if length >= p.capacity {
		return false, nil
	}

	_, err = p.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: p.stream,
		Values: map[string]interface{}{"job_id": job.ID},
	}).Result()
	if err != nil {
		return false, err
	}

	return true, nil
}

// Shutdown stops the worker goroutines and waits for in-flight jobs to finish.
func (p *Pool) Shutdown() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}
