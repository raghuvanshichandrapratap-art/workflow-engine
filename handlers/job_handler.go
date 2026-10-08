package handlers

import (
	"workflow-engine/worker"

	"github.com/gin-gonic/gin"
)

type JobHandler struct {
	dispatcher *worker.Dispatcher
}

func NewJobHandler(dispatcher *worker.Dispatcher) *JobHandler {
	return &JobHandler{dispatcher: dispatcher}
}

func (h *JobHandler) GetJob(c *gin.Context) {
	jobID := c.Param("job_id")

	job, ok, err := h.dispatcher.Get(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(404, gin.H{"error": "job not found"})
		return
	}

	c.JSON(200, job)
}
