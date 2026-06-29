package handlers

import (
	"workflow-engine/services"

	"github.com/gin-gonic/gin"
)

type WorkflowHandler struct {
	service *services.WorkflowService
}

func NewWorkflowHandler(service *services.WorkflowService) *WorkflowHandler {
	return &WorkflowHandler{service: service}
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

func (h *WorkflowHandler) GetExecutionHistory(c *gin.Context) {
	workflowID := c.Param("id")

	logs, err := h.service.GetExecutionHistory(workflowID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, logs)
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
