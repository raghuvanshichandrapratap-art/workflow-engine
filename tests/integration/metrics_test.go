//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func getMetrics(t *testing.T) map[string]float64 {
	t.Helper()

	resp, body := doRequest(t, http.MethodGet, "/metrics", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var snap map[string]float64
	require.NoError(t, json.Unmarshal(body, &snap))
	return snap
}

func TestMetrics_ExecuteWorkflow_IncrementsStartedAndSucceeded(t *testing.T) {
	truncateTables(t)
	id := seedWorkflowWithDSL(t, simpleAgeGateDSL())

	before := getMetrics(t)

	resp, _ := doRequest(t, http.MethodPost, idPath(int(id))+"/execute", map[string]interface{}{"age": 25})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	after := getMetrics(t)

	require.Equal(t, before["executions_started"]+1, after["executions_started"])
	require.Equal(t, before["executions_succeeded"]+1, after["executions_succeeded"])
	require.Equal(t, before["executions_failed"], after["executions_failed"])
}

func TestMetrics_ExecuteWorkflow_IncrementsFailedOnNoDSL(t *testing.T) {
	truncateTables(t)

	resp, createBody := doRequest(t, http.MethodPost, "/workflow", map[string]string{
		"name":        "No DSL Yet",
		"description": "metrics failure test",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createBody, &created))
	id := int(created["ID"].(float64))

	before := getMetrics(t)

	doRequest(t, http.MethodPost, idPath(id)+"/execute", map[string]interface{}{"age": 25})

	after := getMetrics(t)

	require.Equal(t, before["executions_started"]+1, after["executions_started"])
	require.Equal(t, before["executions_failed"]+1, after["executions_failed"])
	require.Equal(t, before["executions_succeeded"], after["executions_succeeded"])
}
