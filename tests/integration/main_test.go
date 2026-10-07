//go:build integration

// Package integration exercises the real HTTP API against a real Postgres —
// run with: go test -tags=integration ./tests/integration/...
// Requires the Postgres started by `docker compose up db` (or any Postgres
// reachable via the DB_* env vars below). Each test truncates workflows and
// execution_logs first, so this is meant for a local dev database, not one
// shared with other work.
package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"workflow-engine/config"
	"workflow-engine/handlers"
	"workflow-engine/repos"
	"workflow-engine/routes"
	"workflow-engine/services"
	"workflow-engine/worker"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

var (
	testServer *httptest.Server
	testDB     *gorm.DB
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	// go test runs with its working directory set to this package
	// (tests/integration/), not the repo root, so config.LoadEnv()'s
	// cwd-relative ".env" lookup wouldn't find it. Load it explicitly from
	// the repo root instead; ignore the error so CI environments that set
	// DB_* vars directly, without a .env file, still work.
	_ = godotenv.Load("../../.env")

	testDB = config.ConnectDB(
		envOr("DB_HOST", "localhost"),
		envOr("DB_USER", "postgres"),
		envOr("DB_PASSWORD", "postgres"),
		envOr("DB_NAME", "workflow_engine"),
		envOr("DB_PORT", "5432"),
	)

	workflowRepo := repos.NewWorkflowRepository(testDB)
	logRepo := repos.NewExecutionLogRepository(testDB)
	engine := services.NewExecutionEngine()
	// ai is nil: these tests seed DSLs directly and never call generate-dsl —
	// AI output quality is eval/'s job, not this suite's.
	svc := services.NewWorkflowService(workflowRepo, logRepo, nil, engine)

	registry := worker.NewRegistry()
	pool := worker.NewPool(50, svc, registry)
	pool.Start(2)
	dispatcher := worker.NewDispatcher(pool, registry)

	workflowHandler := handlers.NewWorkflowHandler(svc, dispatcher)
	jobHandler := handlers.NewJobHandler(dispatcher)

	router := gin.New()
	routes.RegisterRoutes(router, workflowHandler, jobHandler)

	testServer = httptest.NewServer(router)

	code := m.Run()

	testServer.Close()
	pool.Shutdown()
	// Leave the DB clean afterward.
	testDB.Exec("TRUNCATE TABLE execution_logs, workflows RESTART IDENTITY CASCADE")

	os.Exit(code)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func truncateTables(t *testing.T) {
	t.Helper()
	if err := testDB.Exec("TRUNCATE TABLE execution_logs, workflows RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
}

func doRequest(t *testing.T, method, path string, body any) (*http.Response, []byte) {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, testServer.URL+path, reqBody)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := testServer.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	return resp, respBody
}
