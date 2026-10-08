//go:build integration

// Package integration exercises the real HTTP API against real Postgres and
// Redis — run with: go test -tags=integration ./tests/integration/...
// Requires both started by `docker compose up db redis` (or reachable via
// the DB_*/REDIS_ADDR env vars below). Each test truncates workflows and
// execution_logs and clears job/idempotency Redis keys first, so this is
// meant for a local dev database, not one shared with other work.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"workflow-engine/config"
	"workflow-engine/handlers"
	"workflow-engine/redis"
	"workflow-engine/repos"
	"workflow-engine/routes"
	"workflow-engine/services"
	"workflow-engine/worker"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	testServer      *httptest.Server
	testDB          *gorm.DB
	testRedisClient *goredis.Client
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

	var err error
	testRedisClient, err = redis.NewClient(envOr("REDIS_ADDR", "localhost:6379"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	registry := worker.NewRegistry(testRedisClient, time.Hour)
	pool := worker.NewPool(testRedisClient, "test:jobs:stream", 50, svc, registry)
	pool.Start(2)
	dispatcher := worker.NewDispatcher(pool, registry)

	idempotency := redis.NewIdempotency(testRedisClient, time.Hour)

	workflowHandler := handlers.NewWorkflowHandler(svc, dispatcher, idempotency)
	jobHandler := handlers.NewJobHandler(dispatcher)

	router := gin.New()
	routes.RegisterRoutes(router, workflowHandler, jobHandler)

	testServer = httptest.NewServer(router)

	code := m.Run()

	testServer.Close()
	pool.Shutdown()
	// Leave the DB and Redis clean afterward.
	testDB.Exec("TRUNCATE TABLE execution_logs, workflows RESTART IDENTITY CASCADE")
	testRedisClient.FlushDB(context.Background())

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
	if err := clearRedisTestKeys(context.Background()); err != nil {
		t.Fatalf("failed to clear redis test keys: %v", err)
	}
}

// clearRedisTestKeys removes job and idempotency keys only — not a full
// FlushDB, since that would also delete the job stream's consumer group
// (created once in TestMain), breaking every test after the first.
func clearRedisTestKeys(ctx context.Context) error {
	for _, pattern := range []string{"job:*", "idempotency:*"} {
		keys, err := testRedisClient.Keys(ctx, pattern).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := testRedisClient.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
	}
	return nil
}

// doRequest issues an HTTP request against the test server. Extra headers
// are optional and variadic (each a [name, value] pair) so existing call
// sites that don't need them are unaffected.
func doRequest(t *testing.T, method, path string, body any, headers ...[2]string) (*http.Response, []byte) {
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
	for _, h := range headers {
		req.Header.Set(h[0], h[1])
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
