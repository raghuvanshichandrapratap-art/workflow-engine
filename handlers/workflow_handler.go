package handlers

import (
	"encoding/json"
	"math"
	"strconv"
	"time"
	"workflow-engine/redis"
	"workflow-engine/services"
	"workflow-engine/worker"

	"github.com/gin-gonic/gin"
)

type WorkflowHandler struct {
	service     *services.WorkflowService
	dispatcher  *worker.Dispatcher
	idempotency *redis.Idempotency
}

func NewWorkflowHandler(service *services.WorkflowService, dispatcher *worker.Dispatcher, idempotency *redis.Idempotency) *WorkflowHandler {
	return &WorkflowHandler{service: service, dispatcher: dispatcher, idempotency: idempotency}
}

type CreateWorkflowRequest struct {
	Name        string `json:"name"        binding:"required"`
	Description string `json:"description" binding:"required"`
}

func (h *WorkflowHandler) CreateWorkflow(c *gin.Context) {

	var req CreateWorkflowRequest

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	workflow, err := h.service.CreateWorkflow(req.Name, req.Description)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, workflow)
}

func (h *WorkflowHandler) GetWorkflow(c *gin.Context) {
	workflowID := c.Param("id")

	workflow, err := h.service.GetWorkflow(workflowID)

	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, workflow)
}

func (h *WorkflowHandler) UpdateWorkflow(c *gin.Context) {
	workflowID := c.Param("id")

	var req CreateWorkflowRequest

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	workflow, err := h.service.UpdateWorkflow(workflowID, req.Name, req.Description)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, workflow)
}

func (h *WorkflowHandler) GenerateDSL(c *gin.Context) {
	workflowID := c.Param("id")

	workflow, err := h.service.GenerateDSL(workflowID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, workflow)
}

func (h *WorkflowHandler) ExecuteWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	ctx := c.Request.Context()
	idempotencyKey := c.GetHeader("Idempotency-Key")

	var input map[string]interface{}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if len(input) == 0 {
		c.JSON(400, gin.H{"error": "input cannot be empty"})
		return
	}

	if idempotencyKey != "" {
		cached, found, err := h.idempotency.GetResult(ctx, idempotencyKey)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if found {
			if redis.IsProcessing(cached) {
				c.JSON(409, gin.H{"error": "a request with this idempotency key is already being processed"})
				return
			}
			c.Data(200, "application/json", []byte(cached))
			return
		}

		claimed, err := h.idempotency.Claim(ctx, idempotencyKey)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !claimed {
			c.JSON(409, gin.H{"error": "a request with this idempotency key is already being processed"})
			return
		}
	}

	result, err := h.service.ExecuteWorkflow(workflowID, input)
	if err != nil {
		if idempotencyKey != "" {
			// Don't poison the key on a transient failure — let a retry through.
			h.idempotency.Delete(ctx, idempotencyKey)
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if idempotencyKey != "" {
		if resultJSON, marshalErr := json.Marshal(result); marshalErr == nil {
			h.idempotency.StoreResult(ctx, idempotencyKey, string(resultJSON))
		}
	}

	c.JSON(200, result)
}

func (h *WorkflowHandler) ExecuteWorkflowAsync(c *gin.Context) {
	workflowID := c.Param("id")
	ctx := c.Request.Context()
	idempotencyKey := c.GetHeader("Idempotency-Key")

	var input map[string]interface{}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if len(input) == 0 {
		c.JSON(400, gin.H{"error": "input cannot be empty"})
		return
	}

	if idempotencyKey != "" {
		if job, replayed := h.replayAsyncJob(c, idempotencyKey); replayed {
			c.JSON(202, gin.H{"job_id": job.ID, "status": job.Status})
			return
		}

		claimed, err := h.idempotency.Claim(ctx, idempotencyKey)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if !claimed {
			// Someone else is mid-enqueue for this key — briefly poll for
			// them to finish storing the job id before giving up.
			for i := 0; i < 3; i++ {
				time.Sleep(50 * time.Millisecond)
				if job, replayed := h.replayAsyncJob(c, idempotencyKey); replayed {
					c.JSON(202, gin.H{"job_id": job.ID, "status": job.Status})
					return
				}
			}
			c.JSON(409, gin.H{"error": "a request with this idempotency key is already being processed"})
			return
		}
	}

	job, ok, err := h.dispatcher.Enqueue(ctx, workflowID, input)
	if err != nil {
		if idempotencyKey != "" {
			h.idempotency.Delete(ctx, idempotencyKey)
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		if idempotencyKey != "" {
			h.idempotency.Delete(ctx, idempotencyKey)
		}
		c.JSON(503, gin.H{"error": "job queue is full, try again shortly"})
		return
	}

	if idempotencyKey != "" {
		h.idempotency.StoreResult(ctx, idempotencyKey, job.ID)
	}

	c.JSON(202, gin.H{"job_id": job.ID, "status": job.Status})
}

// replayAsyncJob checks whether idempotencyKey already maps to a job and,
// if so, returns that job instead of enqueueing a new one.
func (h *WorkflowHandler) replayAsyncJob(c *gin.Context, idempotencyKey string) (*worker.Job, bool) {
	ctx := c.Request.Context()

	jobID, found, err := h.idempotency.GetResult(ctx, idempotencyKey)
	if err != nil || !found || redis.IsProcessing(jobID) {
		return nil, false
	}

	job, ok, err := h.dispatcher.Get(ctx, jobID)
	if err != nil || !ok {
		return nil, false
	}
	return job, true
}

func (h *WorkflowHandler) GetExecutionHistory(c *gin.Context) {
	workflowID := c.Param("id")

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}

	logs, total, err := h.service.GetExecutionHistory(workflowID, page, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"data":        logs,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int(math.Ceil(float64(total) / float64(limit))),
	})
}

func (h *WorkflowHandler) DeleteWorkflow(c *gin.Context) {
	workflowID := c.Param("id")

	err := h.service.DeleteWorkflow(workflowID)

	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}

	c.Status(204)
}
