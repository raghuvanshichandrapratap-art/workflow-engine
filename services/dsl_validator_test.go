package services

import (
	"testing"

	"workflow-engine/models"

	"github.com/stretchr/testify/assert"
)

func TestValidateDSL_Valid(t *testing.T) {
	dsl := &models.DSL{
		Steps: []models.Step{
			{Type: "condition", Field: "age", Operator: "gt", Value: 18.0, OnPass: "next", OnFail: "reject"},
			{Type: "action", Action: "send_email", Params: map[string]string{"template": "welcome"}},
			{Type: "action", Action: "assign_role", Params: map[string]string{"role": "member"}},
			{Type: "action", Action: "webhook", Params: map[string]string{"url": "www.example.com/notify"}},
			{Type: "action", Action: "approve"},
		},
	}

	assert.Empty(t, ValidateDSL(dsl))
}

func TestValidateDSL_EmptySteps(t *testing.T) {
	errs := ValidateDSL(&models.DSL{})
	assert.Len(t, errs, 1)
}

func TestValidateDSL_ConditionDefects(t *testing.T) {
	tests := []struct {
		name string
		step models.Step
	}{
		{"missing field", models.Step{Type: "condition", Operator: "gt", Value: 18.0, OnPass: "next", OnFail: "reject"}},
		{"missing operator", models.Step{Type: "condition", Field: "age", Value: 18.0, OnPass: "next", OnFail: "reject"}},
		{"disallowed operator", models.Step{Type: "condition", Field: "age", Operator: "greater_than", Value: 18.0, OnPass: "next", OnFail: "reject"}},
		{"missing value", models.Step{Type: "condition", Field: "age", Operator: "gt", OnPass: "next", OnFail: "reject"}},
		{"non-numeric value for gt", models.Step{Type: "condition", Field: "age", Operator: "gt", Value: "eighteen", OnPass: "next", OnFail: "reject"}},
		{"missing on_pass", models.Step{Type: "condition", Field: "age", Operator: "gt", Value: 18.0, OnFail: "reject"}},
		{"disallowed on_pass", models.Step{Type: "condition", Field: "age", Operator: "gt", Value: 18.0, OnPass: "maybe", OnFail: "reject"}},
		{"missing on_fail", models.Step{Type: "condition", Field: "age", Operator: "gt", Value: 18.0, OnPass: "next"}},
		{"disallowed on_fail", models.Step{Type: "condition", Field: "age", Operator: "gt", Value: 18.0, OnPass: "next", OnFail: "maybe"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateDSL(&models.DSL{Steps: []models.Step{tt.step}})
			assert.NotEmpty(t, errs)
		})
	}
}

func TestValidateDSL_EqOperatorAllowsNonNumericValue(t *testing.T) {
	step := models.Step{Type: "condition", Field: "status", Operator: "eq", Value: "active", OnPass: "next", OnFail: "reject"}
	assert.Empty(t, ValidateDSL(&models.DSL{Steps: []models.Step{step}}))
}

func TestValidateDSL_UnknownStepType(t *testing.T) {
	step := models.Step{Type: "weird", Field: "x"}
	errs := ValidateDSL(&models.DSL{Steps: []models.Step{step}})
	assert.NotEmpty(t, errs)
}

func TestValidateDSL_ActionDefects(t *testing.T) {
	tests := []struct {
		name string
		step models.Step
	}{
		{"missing action name", models.Step{Type: "action"}},
		{"disallowed action", models.Step{Type: "action", Action: "send_sms"}},
		{"send_email missing template", models.Step{Type: "action", Action: "send_email"}},
		{"assign_role missing role", models.Step{Type: "action", Action: "assign_role"}},
		{"webhook missing url", models.Step{Type: "action", Action: "webhook"}},
		{"webhook malformed url", models.Step{Type: "action", Action: "webhook", Params: map[string]string{"url": "not a url!!"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateDSL(&models.DSL{Steps: []models.Step{tt.step}})
			assert.NotEmpty(t, errs)
		})
	}
}

func TestValidateDSL_ActionsThatNeedNoParams(t *testing.T) {
	tests := []models.Step{
		{Type: "action", Action: "approve"},
		{Type: "action", Action: "reject"},
	}

	for _, step := range tests {
		t.Run(step.Action, func(t *testing.T) {
			assert.Empty(t, ValidateDSL(&models.DSL{Steps: []models.Step{step}}))
		})
	}
}

func TestNormalizeWebhookURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
		want    string
	}{
		{"bare domain with path", "www.example.com/notify", false, "https://www.example.com/notify"},
		{"already has https scheme", "https://example.org/clear", false, "https://example.org/clear"},
		{"already has http scheme", "http://example.net/restock", false, "http://example.net/restock"},
		{"empty", "", true, ""},
		{"garbage", "not a url!!", true, ""},
		{"unsupported scheme", "ftp://example.com/file", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeWebhookURL(tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
