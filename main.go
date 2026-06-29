package main

import (
	"workflow-engine/config"
	"workflow-engine/handlers"
	"workflow-engine/repos"
	"workflow-engine/routes"
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

	logRepo := repos.NewExecutionLogRepository(db)

	aiService := services.NewAIService()

	engine := services.NewExecutionEngine()

	service := services.NewWorkflowService(repo, logRepo, aiService, engine)

	handler := handlers.NewWorkflowHandler(service)

	router := gin.Default()

	routes.RegisterRoutes(router, handler)

	router.Run()
}
