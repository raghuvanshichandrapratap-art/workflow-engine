package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"workflow-engine/config"
	"workflow-engine/handlers"
	"workflow-engine/logger"
	"workflow-engine/redis"
	"workflow-engine/repos"
	"workflow-engine/routes"
	"workflow-engine/services"
	"workflow-engine/worker"

	"github.com/gin-gonic/gin"
)

func main() {

	err := config.LoadEnv()
	if err != nil {
		panic("Failed to load environment variables")
	}

	logger.Init(config.GetEnv("LOG_LEVEL"))

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

	redisAddr := config.GetEnv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient, err := redis.NewClient(redisAddr)
	if err != nil {
		log.Fatal(err)
	}

	jobTTL := time.Duration(config.GetEnvInt("JOB_TTL_SECONDS", 86400)) * time.Second
	idempotencyTTL := time.Duration(config.GetEnvInt("IDEMPOTENCY_TTL_SECONDS", 86400)) * time.Second

	registry := worker.NewRegistry(redisClient, jobTTL)
	workerCount := config.GetEnvInt("WORKER_COUNT", 4)
	queueCapacity := config.GetEnvInt("JOB_QUEUE_CAPACITY", 50)
	pool := worker.NewPool(redisClient, "jobs:stream", int64(queueCapacity), service, registry)
	pool.Start(workerCount)
	dispatcher := worker.NewDispatcher(pool, registry)

	idempotency := redis.NewIdempotency(redisClient, idempotencyTTL)

	workflowHandler := handlers.NewWorkflowHandler(service, dispatcher, idempotency)
	jobHandler := handlers.NewJobHandler(dispatcher)

	router := gin.Default()

	routes.RegisterRoutes(router, workflowHandler, jobHandler)

	port := config.GetEnv("PORT")
	if port == "" {
		port = "5050"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	// Stop accepting new jobs and let already-queued ones finish, now that
	// the HTTP server has stopped taking new requests.
	pool.Shutdown()

	log.Println("shutdown complete")
}
