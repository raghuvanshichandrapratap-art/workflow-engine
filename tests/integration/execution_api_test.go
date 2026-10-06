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

// seedWorkflowWithDSL inserts a workflow with a known-good DSL directly via
// the repo, bypassing generate-dsl entirely — these tests exercise execute
// and history, not AI output quality (that's eval/'s job).
func seedWorkflowWithDSL(t *testing.T, dsl *models.DSL) uint {
	t.Helper()

	dslBytes, err := json.Marshal(dsl)
	require.NoError(t, err)

	workflow := &models.Workflow{
		Name:        "Seeded Workflow",
		Description: "seeded directly for integration testing",
		DSL:         json.RawMessage(dslBytes),
	}

	repo := repos.NewWorkflowRepository(testDB)
	require.NoError(t, repo.Create(workflow))

	return workflow.ID
}

func simpleAgeGateDSL() *models.DSL {
	return &models.DSL{Steps: []models.Step{
		{Type: "condition", Field: "age", Operator: "gt", Value: 18.0, OnPass: "next", OnFail: "reject"},
		{Type: "action", Action: "approve"},
	}}
}

func TestExecuteWorkflow_ValidInput(t *testing.T) {
	truncateTables(t)
	id := seedWorkflowWithDSL(t, simpleAgeGateDSL())

	resp, body := doRequest(t, http.MethodPost, idPath(int(id))+"/execute", map[string]interface{}{"age": 25})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var result map[string]any
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, "approved", result["status"])
}

func TestExecuteWorkflow_RejectBranch(t *testing.T) {
	truncateTables(t)
	id := seedWorkflowWithDSL(t, simpleAgeGateDSL())

	resp, body := doRequest(t, http.MethodPost, idPath(int(id))+"/execute", map[string]interface{}{"age": 10})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var result map[string]any
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, "rejected", result["status"])
}

func TestExecuteWorkflow_EmptyInput(t *testing.T) {
	truncateTables(t)
	id := seedWorkflowWithDSL(t, simpleAgeGateDSL())

	resp, _ := doRequest(t, http.MethodPost, idPath(int(id))+"/execute", map[string]interface{}{})

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestExecuteWorkflow_NoDSL(t *testing.T) {
	truncateTables(t)

	repo := repos.NewWorkflowRepository(testDB)
	workflow := &models.Workflow{Name: "No DSL", Description: "dsl not generated yet"}
	require.NoError(t, repo.Create(workflow))

	resp, _ := doRequest(t, http.MethodPost, idPath(int(workflow.ID))+"/execute", map[string]interface{}{"age": 25})

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestExecutionHistory_Pagination(t *testing.T) {
	truncateTables(t)
	id := seedWorkflowWithDSL(t, simpleAgeGateDSL())

	logRepo := repos.NewExecutionLogRepository(testDB)
	for i := 0; i < 7; i++ {
		require.NoError(t, logRepo.Create(&models.ExecutionLog{
			WorkflowID: id,
			Status:     "approved",
			Input:      json.RawMessage(`{}`),
			Steps:      json.RawMessage(`[]`),
		}))
	}

	resp, body := doRequest(t, http.MethodGet, idPath(int(id))+"/executions?page=1&limit=5", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var page map[string]any
	require.NoError(t, json.Unmarshal(body, &page))
	assert.Equal(t, float64(7), page["total"])
	assert.Equal(t, float64(2), page["total_pages"])
	assert.Len(t, page["data"], 5)

	resp2, body2 := doRequest(t, http.MethodGet, idPath(int(id))+"/executions?page=2&limit=5", nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var page2 map[string]any
	require.NoError(t, json.Unmarshal(body2, &page2))
	assert.Len(t, page2["data"], 2)
}

func TestExecutionHistory_InvalidPageLimitClamped(t *testing.T) {
	truncateTables(t)
	id := seedWorkflowWithDSL(t, simpleAgeGateDSL())

	resp, body := doRequest(t, http.MethodGet, idPath(int(id))+"/executions?page=-1&limit=9999", nil)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var page map[string]any
	require.NoError(t, json.Unmarshal(body, &page))
	assert.Equal(t, float64(1), page["page"])
	assert.Equal(t, float64(10), page["limit"])
}
