package routes

import (
	"workflow-engine/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, handler *handlers.WorkflowHandler) {
	router.Static("/static", "./static")
	router.StaticFile("/", "./static/index.html")

	router.POST("/workflow", handler.CreateWorkflow)
	router.GET("/workflow/:id", handler.GetWorkflow)
	router.PUT("/workflow/:id", handler.UpdateWorkflow)
	router.DELETE("/workflow/:id", handler.DeleteWorkflow)
	router.POST("/workflow/:id/generate-dsl", handler.GenerateDSL)
	router.POST("/workflow/:id/execute", handler.ExecuteWorkflow)
	router.GET("/workflow/:id/executions", handler.GetExecutionHistory)
}