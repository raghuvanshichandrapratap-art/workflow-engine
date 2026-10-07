package handlers

import (
	"math"
	"strconv"
	"workflow-engine/services"
	"workflow-engine/worker"

	"github.com/gin-gonic/gin"
)

type WorkflowHandler struct {
	service    *services.WorkflowService
	dispatcher *worker.Dispatcher
}

func NewWorkflowHandler(service *services.WorkflowService, dispatcher *worker.Dispatcher) *WorkflowHandler {
	return &WorkflowHandler{service: service, dispatcher: dispatcher}
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

	var input map[string]interface{}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if len(input) == 0 {
		c.JSON(400, gin.H{"error": "input cannot be empty"})
		return
	}

	result, err := h.service.ExecuteWorkflow(workflowID, input)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

func (h *WorkflowHandler) ExecuteWorkflowAsync(c *gin.Context) {
	workflowID := c.Param("id")

	var input map[string]interface{}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if len(input) == 0 {
		c.JSON(400, gin.H{"error": "input cannot be empty"})
		return
	}

	job, ok := h.dispatcher.Enqueue(workflowID, input)
	if !ok {
		c.JSON(503, gin.H{"error": "job queue is full, try again shortly"})
		return
	}

	c.JSON(202, gin.H{"job_id": job.ID, "status": job.Status})
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
