//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"workflow-engine/repos"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pollJobUntilTerminal(t *testing.T, jobID string) map[string]any {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, body := doRequest(t, http.MethodGet, "/jobs/"+jobID, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var job map[string]any
		require.NoError(t, json.Unmarshal(body, &job))

		if job["status"] == "completed" || job["status"] == "failed" {
			return job
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("job never reached a terminal state")
	return nil
}

func TestExecuteWorkflowAsync_CompletesAndLogsHistory(t *testing.T) {
	truncateTables(t)
	id := seedWorkflowWithDSL(t, simpleAgeGateDSL())

	resp, body := doRequest(t, http.MethodPost, idPath(int(id))+"/execute-async", map[string]interface{}{"age": 25})
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	var accepted map[string]any
	require.NoError(t, json.Unmarshal(body, &accepted))
	jobID, ok := accepted["job_id"].(string)
	require.True(t, ok, "response must include a job_id")
	assert.Equal(t, "queued", accepted["status"])

	job := pollJobUntilTerminal(t, jobID)
	require.Equal(t, "completed", job["status"])

	result, ok := job["result"].(map[string]any)
	require.True(t, ok, "completed job must include its result")
	assert.Equal(t, "approved", result["status"])

	logRepo := repos.NewExecutionLogRepository(testDB)
	logs, total, err := logRepo.GetByWorkflowID(strconv.Itoa(int(id)), 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total, "the async execution should have been logged just like a synchronous one")
	require.Len(t, logs, 1)
	assert.Equal(t, "approved", logs[0].Status)
}

func TestExecuteWorkflowAsync_RejectBranch(t *testing.T) {
	truncateTables(t)
	id := seedWorkflowWithDSL(t, simpleAgeGateDSL())

	_, body := doRequest(t, http.MethodPost, idPath(int(id))+"/execute-async", map[string]interface{}{"age": 10})

	var accepted map[string]any
	require.NoError(t, json.Unmarshal(body, &accepted))
	jobID := accepted["job_id"].(string)

	job := pollJobUntilTerminal(t, jobID)
	require.Equal(t, "completed", job["status"])

	result := job["result"].(map[string]any)
	assert.Equal(t, "rejected", result["status"])
}

func TestExecuteWorkflowAsync_EmptyInput(t *testing.T) {
	truncateTables(t)
	id := seedWorkflowWithDSL(t, simpleAgeGateDSL())

	resp, _ := doRequest(t, http.MethodPost, idPath(int(id))+"/execute-async", map[string]interface{}{})

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJob_NotFound(t *testing.T) {
	truncateTables(t)

	resp, _ := doRequest(t, http.MethodGet, "/jobs/does-not-exist", nil)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
