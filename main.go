package main

import (
	"workflow-engine/config"
	"workflow-engine/handlers"
	"workflow-engine/repos"
	"workflow-engine/services"

	"github.com/gin-gonic/gin"
)

func main() {

	err := config.LoadEnv()
	if err != nil {
		panic("Failed to load environment variables")
	}

	db := config.ConnectDB(
		config.GetEnv("DB_HOST"),
		config.GetEnv("DB_USER"),
		config.GetEnv("DB_PASSWORD"),
		config.GetEnv("DB_NAME"),
		config.GetEnv("DB_PORT"),
	)

	repo := repos.NewWorkflowRepository(db)

	service := services.NewWorkflowService(repo)

	handler := handlers.NewWorkflowHandler(service)

	router := gin.Default()

	router.POST("/workflow", handler.CreateWorkflow)

	router.GET("/workflow/:id", handler.GetWorkflow)

	router.PUT("/workflow/:id", handler.UpdateWorkflow)

	router.DELETE("/workflow/:id", handler.DeleteWorkflow)

	router.Run() // listens on 0.0.0.0:8080 by default
}
