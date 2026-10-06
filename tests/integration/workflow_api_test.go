//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateWorkflow_Valid(t *testing.T) {
	truncateTables(t)

	resp, body := doRequest(t, http.MethodPost, "/workflow", map[string]string{
		"name":        "Employee Onboarding",
		"description": "Check if age is above 18, if not reject. If yes approve.",
	})

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created map[string]any
	require.NoError(t, json.Unmarshal(body, &created))
	assert.Equal(t, "Employee Onboarding", created["Name"])
	assert.NotNil(t, created["ID"])
}

func TestCreateWorkflow_MissingRequiredField(t *testing.T) {
	truncateTables(t)

	resp, _ := doRequest(t, http.MethodPost, "/workflow", map[string]string{
		"name": "Missing Description",
	})

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetWorkflow_Found(t *testing.T) {
	truncateTables(t)

	_, createBody := doRequest(t, http.MethodPost, "/workflow", map[string]string{
		"name":        "Test Workflow",
		"description": "some description",
	})
	var created map[string]any
	require.NoError(t, json.Unmarshal(createBody, &created))
	id := int(created["ID"].(float64))

	resp, body := doRequest(t, http.MethodGet, idPath(id), nil)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got map[string]any
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, "Test Workflow", got["Name"])
}

func TestGetWorkflow_NotFound(t *testing.T) {
	truncateTables(t)

	resp, _ := doRequest(t, http.MethodGet, "/workflow/99999", nil)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdateWorkflow(t *testing.T) {
	truncateTables(t)

	_, createBody := doRequest(t, http.MethodPost, "/workflow", map[string]string{
		"name":        "Original Name",
		"description": "original description",
	})
	var created map[string]any
	require.NoError(t, json.Unmarshal(createBody, &created))
	id := int(created["ID"].(float64))

	resp, body := doRequest(t, http.MethodPut, idPath(id), map[string]string{
		"name":        "Updated Name",
		"description": "updated description",
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var updated map[string]any
	require.NoError(t, json.Unmarshal(body, &updated))
	assert.Equal(t, "Updated Name", updated["Name"])
}

func TestDeleteWorkflow(t *testing.T) {
	truncateTables(t)

	_, createBody := doRequest(t, http.MethodPost, "/workflow", map[string]string{
		"name":        "To Delete",
		"description": "will be deleted",
	})
	var created map[string]any
	require.NoError(t, json.Unmarshal(createBody, &created))
	id := int(created["ID"].(float64))

	deleteResp, _ := doRequest(t, http.MethodDelete, idPath(id), nil)
	require.Equal(t, http.StatusNoContent, deleteResp.StatusCode)

	getResp, _ := doRequest(t, http.MethodGet, idPath(id), nil)
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
}

func idPath(id int) string {
	return "/workflow/" + strconv.Itoa(id)
}
