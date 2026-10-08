//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"workflow-engine/models"
	"workflow-engine/repos"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteWorkflow_IdempotencyKey_ReplaysCachedResult(t *testing.T) {
	truncateTables(t)
	id := seedWorkflowWithDSL(t, simpleAgeGateDSL())

	header := [2]string{"Idempotency-Key", "test-key-sync-1"}

	resp1, body1 := doRequest(t, http.MethodPost, idPath(int(id))+"/execute", map[string]interface{}{"age": 25}, header)
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	resp2, body2 := doRequest(t, http.MethodPost, idPath(int(id))+"/execute", map[string]interface{}{"age": 25}, header)
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	assert.JSONEq(t, string(body1), string(body2), "a retried request with the same key must replay the exact cached result")

	// Only one execution should have actually run.
	historyResp, historyBody := doRequest(t, http.MethodGet, idPath(int(id))+"/executions?page=1&limit=10", nil)
	require.Equal(t, http.StatusOK, historyResp.StatusCode)
	var history map[string]any
	require.NoError(t, json.Unmarshal(historyBody, &history))
	assert.Equal(t, float64(1), history["total"], "a duplicate request with the same idempotency key must not create a second execution_logs row")
}

func TestExecuteWorkflow_NoIdempotencyKey_RunsTwice(t *testing.T) {
	truncateTables(t)
	id := seedWorkflowWithDSL(t, simpleAgeGateDSL())

	doRequest(t, http.MethodPost, idPath(int(id))+"/execute", map[string]interface{}{"age": 25})
	doRequest(t, http.MethodPost, idPath(int(id))+"/execute", map[string]interface{}{"age": 25})

	historyResp, historyBody := doRequest(t, http.MethodGet, idPath(int(id))+"/executions?page=1&limit=10", nil)
	require.Equal(t, http.StatusOK, historyResp.StatusCode)
	var history map[string]any
	require.NoError(t, json.Unmarshal(historyBody, &history))
	assert.Equal(t, float64(2), history["total"], "without an idempotency key, two requests must run independently")
}

func TestExecuteWorkflowAsync_IdempotencyKey_ReturnsSameJob(t *testing.T) {
	truncateTables(t)
	id := seedWorkflowWithDSL(t, simpleAgeGateDSL())

	header := [2]string{"Idempotency-Key", "test-key-async-1"}

	resp1, body1 := doRequest(t, http.MethodPost, idPath(int(id))+"/execute-async", map[string]interface{}{"age": 25}, header)
	require.Equal(t, http.StatusAccepted, resp1.StatusCode)
	var accepted1 map[string]any
	require.NoError(t, json.Unmarshal(body1, &accepted1))

	resp2, body2 := doRequest(t, http.MethodPost, idPath(int(id))+"/execute-async", map[string]interface{}{"age": 25}, header)
	require.Equal(t, http.StatusAccepted, resp2.StatusCode)
	var accepted2 map[string]any
	require.NoError(t, json.Unmarshal(body2, &accepted2))

	assert.Equal(t, accepted1["job_id"], accepted2["job_id"], "a retried async request with the same key must return the same job, not enqueue a second one")

	jobID := accepted1["job_id"].(string)
	pollJobUntilTerminal(t, jobID)

	historyResp, historyBody := doRequest(t, http.MethodGet, idPath(int(id))+"/executions?page=1&limit=10", nil)
	require.Equal(t, http.StatusOK, historyResp.StatusCode)
	var history map[string]any
	require.NoError(t, json.Unmarshal(historyBody, &history))
	assert.Equal(t, float64(1), history["total"], "only one job should have actually been enqueued and executed")
}

func TestExecuteWorkflow_FailedAttempt_ReleasesIdempotencyKey(t *testing.T) {
	truncateTables(t)

	// Seeded without a DSL on purpose, so the first /execute fails — then a
	// DSL is attached directly via the repo (no live AI call, consistent
	// with the rest of this suite) before retrying with the same key.
	repo := repos.NewWorkflowRepository(testDB)
	workflow := &models.Workflow{Name: "No DSL Yet", Description: "idempotency release test"}
	require.NoError(t, repo.Create(workflow))

	header := [2]string{"Idempotency-Key", "test-key-release-1"}

	failResp, _ := doRequest(t, http.MethodPost, idPath(int(workflow.ID))+"/execute", map[string]interface{}{"age": 25}, header)
	assert.Equal(t, http.StatusInternalServerError, failResp.StatusCode)

	dslBytes, err := json.Marshal(simpleAgeGateDSL())
	require.NoError(t, err)
	workflow.DSL = dslBytes
	require.NoError(t, repo.Update(workflow))

	// A retry with the same key, now that the underlying problem is fixed,
	// must actually run rather than getting stuck on a stale claim.
	retryResp, _ := doRequest(t, http.MethodPost, idPath(int(workflow.ID))+"/execute", map[string]interface{}{"age": 25}, header)
	assert.Equal(t, http.StatusOK, retryResp.StatusCode)
}
