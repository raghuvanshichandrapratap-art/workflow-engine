package routes

import (
	"workflow-engine/handlers"
	"workflow-engine/logger"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, workflowHandler *handlers.WorkflowHandler, jobHandler *handlers.JobHandler) {
	router.Static("/static", "./static")
	router.StaticFile("/", "./static/index.html")

	router.GET("/metrics", func(c *gin.Context) {
		c.JSON(200, logger.Metrics.Snapshot())
	})

	router.POST("/workflow", workflowHandler.CreateWorkflow)
	router.GET("/workflow/:id", workflowHandler.GetWorkflow)
	router.PUT("/workflow/:id", workflowHandler.UpdateWorkflow)
	router.DELETE("/workflow/:id", workflowHandler.DeleteWorkflow)
	router.POST("/workflow/:id/generate-dsl", workflowHandler.GenerateDSL)
	router.POST("/workflow/:id/execute", workflowHandler.ExecuteWorkflow)
	router.POST("/workflow/:id/execute-async", workflowHandler.ExecuteWorkflowAsync)
	router.GET("/workflow/:id/executions", workflowHandler.GetExecutionHistory)

	router.GET("/jobs/:job_id", jobHandler.GetJob)
}
