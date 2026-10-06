package services

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"workflow-engine/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  float64
		ok    bool
	}{
		{"float64", 18.0, 18.0, true},
		{"int", 18, 18.0, true},
		{"int64", int64(18), 18.0, true},
		{"string", "18", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestEvaluateCondition(t *testing.T) {
	tests := []struct {
		name       string
		step       models.Step
		input      map[string]interface{}
		wantPassed bool
		wantErr    bool
	}{
		{"gt passes", models.Step{Field: "age", Operator: "gt", Value: 18.0}, map[string]interface{}{"age": 25.0}, true, false},
		{"gt fails", models.Step{Field: "age", Operator: "gt", Value: 18.0}, map[string]interface{}{"age": 10.0}, false, false},
		{"lt passes", models.Step{Field: "score", Operator: "lt", Value: 50.0}, map[string]interface{}{"score": 30.0}, true, false},
		{"gte boundary passes", models.Step{Field: "score", Operator: "gte", Value: 50.0}, map[string]interface{}{"score": 50.0}, true, false},
		{"lte boundary passes", models.Step{Field: "score", Operator: "lte", Value: 50.0}, map[string]interface{}{"score": 50.0}, true, false},
		{"eq string match", models.Step{Field: "status", Operator: "eq", Value: "active"}, map[string]interface{}{"status": "active"}, true, false},
		{"eq string mismatch", models.Step{Field: "status", Operator: "eq", Value: "active"}, map[string]interface{}{"status": "inactive"}, false, false},
		{"eq bool match", models.Step{Field: "verified", Operator: "eq", Value: true}, map[string]interface{}{"verified": true}, true, false},
		{"missing field is not an error", models.Step{Field: "age", Operator: "gt", Value: 18.0}, map[string]interface{}{}, false, false},
		{"gt with non-numeric input errors", models.Step{Field: "age", Operator: "gt", Value: 18.0}, map[string]interface{}{"age": "old"}, false, true},
		{"unknown operator errors", models.Step{Field: "age", Operator: "between", Value: 18.0}, map[string]interface{}{"age": 20.0}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passed, _, err := evaluateCondition(tt.step, tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantPassed, passed)
		})
	}
}

func TestExecuteAction_NoNetwork(t *testing.T) {
	e := NewExecutionEngine()

	tests := []struct {
		name string
		step models.Step
	}{
		{"approve", models.Step{Action: "approve"}},
		{"reject", models.Step{Action: "reject"}},
		{"send_email", models.Step{Action: "send_email", Params: map[string]string{"template": "welcome"}}},
		{"assign_role", models.Step{Action: "assign_role", Params: map[string]string{"role": "admin"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := e.executeAction(tt.step)
			require.NoError(t, err)
			assert.NotEmpty(t, msg)
		})
	}

	t.Run("unknown action errors", func(t *testing.T) {
		_, err := e.executeAction(models.Step{Action: "send_sms"})
		assert.Error(t, err)
	})
}

func TestExecuteAction_Webhook_FullURL(t *testing.T) {
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := &ExecutionEngine{httpClient: server.Client()}

	msg, err := e.executeAction(models.Step{
		Action: "webhook",
		Params: map[string]string{"url": server.URL},
	})

	require.NoError(t, err)
	assert.Contains(t, msg, "status 200")
	assert.Equal(t, "POST", gotMethod)
}

// httptest.NewServer only speaks plain HTTP, so a bare (schemeless) URL — which
// normalizeWebhookURL defaults to https:// — needs a TLS test server. A direct
// server-side request's r.URL.Scheme is empty regardless of how the client
// connected, so success + r.TLS != nil is how we confirm https was actually used.
func TestExecuteAction_Webhook_BareDomainNormalizesToHTTPS(t *testing.T) {
	var sawTLS bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawTLS = r.TLS != nil
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := &ExecutionEngine{httpClient: server.Client()}
	bareURL := strings.TrimPrefix(server.URL, "https://")

	msg, err := e.executeAction(models.Step{
		Action: "webhook",
		Params: map[string]string{"url": bareURL},
	})

	require.NoError(t, err)
	assert.Contains(t, msg, "status 200")
	assert.True(t, sawTLS, "bare-domain url should have been normalized to https://")
}

func TestExecute_OnPassReject_ShortCircuits(t *testing.T) {
	e := NewExecutionEngine()
	dsl := &models.DSL{
		Steps: []models.Step{
			{Type: "condition", Field: "risk", Operator: "eq", Value: "high", OnPass: "reject", OnFail: "next"},
			{Type: "action", Action: "approve"},
		},
	}

	result, err := e.Execute(dsl, map[string]interface{}{"risk": "high"})

	require.NoError(t, err)
	assert.Equal(t, "rejected", result.Status)
	assert.Len(t, result.Steps, 1, "the approve action after a rejecting condition must not run")
}

func TestExecute_OnFailReject_ShortCircuits(t *testing.T) {
	e := NewExecutionEngine()
	dsl := &models.DSL{
		Steps: []models.Step{
			{Type: "condition", Field: "age", Operator: "gt", Value: 18.0, OnPass: "next", OnFail: "reject"},
			{Type: "action", Action: "approve"},
		},
	}

	result, err := e.Execute(dsl, map[string]interface{}{"age": 10.0})

	require.NoError(t, err)
	assert.Equal(t, "rejected", result.Status)
	assert.Len(t, result.Steps, 1)
}

func TestExecute_MultiStepChainReachesApprove(t *testing.T) {
	e := NewExecutionEngine()
	dsl := &models.DSL{
		Steps: []models.Step{
			{Type: "condition", Field: "age", Operator: "gt", Value: 18.0, OnPass: "next", OnFail: "reject"},
			{Type: "condition", Field: "country", Operator: "eq", Value: "US", OnPass: "next", OnFail: "reject"},
			{Type: "action", Action: "send_email", Params: map[string]string{"template": "welcome"}},
			{Type: "action", Action: "approve"},
		},
	}

	result, err := e.Execute(dsl, map[string]interface{}{"age": 25.0, "country": "US"})

	require.NoError(t, err)
	assert.Equal(t, "approved", result.Status)
	assert.Len(t, result.Steps, 4)
}
