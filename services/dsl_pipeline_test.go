package services

import (
	"fmt"
	"testing"
	"time"

	"workflow-engine/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAI is a scripted AIGenerator test double — each call to GenerateDSL or
// RetryDSL returns the next entry in its queue, so a test can simulate any
// sequence of initial/retry outcomes without touching the real Groq API.
type fakeAI struct {
	generateCalls int
	retryCalls    int
	generateFn    func() (*models.DSL, string, time.Duration, error)
	retryFn       func() (*models.DSL, string, time.Duration, error)
}

func (f *fakeAI) GenerateDSL(description string) (*models.DSL, string, time.Duration, error) {
	f.generateCalls++
	return f.generateFn()
}

func (f *fakeAI) RetryDSL(description, previousRaw string, validationErrs []error) (*models.DSL, string, time.Duration, error) {
	f.retryCalls++
	return f.retryFn()
}

func validDSL() *models.DSL {
	return &models.DSL{Steps: []models.Step{
		{Type: "condition", Field: "age", Operator: "gt", Value: 18.0, OnPass: "next", OnFail: "reject"},
		{Type: "action", Action: "approve"},
	}}
}

func invalidDSL() *models.DSL {
	return &models.DSL{Steps: []models.Step{
		{Type: "condition", Field: "age", Operator: "between", Value: 18.0, OnPass: "next", OnFail: "reject"},
	}}
}

func TestGenerateValidatedDSL_InitialValid_NoRetry(t *testing.T) {
	ai := &fakeAI{
		generateFn: func() (*models.DSL, string, time.Duration, error) {
			return validDSL(), `{"steps":[...]}`, 0, nil
		},
		retryFn: func() (*models.DSL, string, time.Duration, error) {
			t.Fatal("RetryDSL should not be called when the initial DSL is valid")
			return nil, "", 0, nil
		},
	}

	result, err := GenerateValidatedDSL(ai, "some description")

	require.NoError(t, err)
	assert.True(t, result.InitialValid)
	assert.True(t, result.FinalValid)
	assert.False(t, result.RetryAttempted)
	assert.Equal(t, 1, ai.generateCalls)
	assert.Equal(t, 0, ai.retryCalls)
}

func TestGenerateValidatedDSL_InvalidThenRetrySucceeds(t *testing.T) {
	ai := &fakeAI{
		generateFn: func() (*models.DSL, string, time.Duration, error) {
			return invalidDSL(), `{"steps":[bad]}`, 0, nil
		},
		retryFn: func() (*models.DSL, string, time.Duration, error) {
			return validDSL(), `{"steps":[fixed]}`, 0, nil
		},
	}

	result, err := GenerateValidatedDSL(ai, "some description")

	require.NoError(t, err)
	assert.False(t, result.InitialValid)
	assert.True(t, result.RetryAttempted)
	assert.True(t, result.FinalValid)
	assert.Equal(t, 1, ai.generateCalls)
	assert.Equal(t, 1, ai.retryCalls)
}

func TestGenerateValidatedDSL_BothInvalid(t *testing.T) {
	ai := &fakeAI{
		generateFn: func() (*models.DSL, string, time.Duration, error) {
			return invalidDSL(), `{"steps":[bad]}`, 0, nil
		},
		retryFn: func() (*models.DSL, string, time.Duration, error) {
			return invalidDSL(), `{"steps":[still bad]}`, 0, nil
		},
	}

	result, err := GenerateValidatedDSL(ai, "some description")

	require.NoError(t, err)
	assert.False(t, result.InitialValid)
	assert.True(t, result.RetryAttempted)
	assert.False(t, result.FinalValid)
	assert.NotEmpty(t, result.ValidationErrors)
}

func TestGenerateValidatedDSL_InfraErrorSkipsRetry(t *testing.T) {
	ai := &fakeAI{
		generateFn: func() (*models.DSL, string, time.Duration, error) {
			return nil, "", 0, fmt.Errorf("connection refused")
		},
		retryFn: func() (*models.DSL, string, time.Duration, error) {
			t.Fatal("RetryDSL should not be called after an infra-level failure")
			return nil, "", 0, nil
		},
	}

	result, err := GenerateValidatedDSL(ai, "some description")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 1, ai.generateCalls)
	assert.Equal(t, 0, ai.retryCalls)
}

func TestGenerateValidatedDSL_InvalidJSONIsNotInfraError(t *testing.T) {
	// raw != "" distinguishes "AI replied with bad JSON" (retryable) from
	// "the API call itself failed" (not retryable).
	ai := &fakeAI{
		generateFn: func() (*models.DSL, string, time.Duration, error) {
			return nil, `not json`, 0, fmt.Errorf("AI returned invalid DSL: unexpected token")
		},
		retryFn: func() (*models.DSL, string, time.Duration, error) {
			return validDSL(), `{"steps":[fixed]}`, 0, nil
		},
	}

	result, err := GenerateValidatedDSL(ai, "some description")

	require.NoError(t, err)
	assert.True(t, result.RetryAttempted)
	assert.True(t, result.FinalValid)
	assert.Equal(t, 1, ai.retryCalls)
}

func TestGenerateValidatedDSL_TotalTimingIsSane(t *testing.T) {
	ai := &fakeAI{
		generateFn: func() (*models.DSL, string, time.Duration, error) {
			time.Sleep(time.Millisecond)
			return validDSL(), `{"steps":[...]}`, 0, nil
		},
	}

	result, err := GenerateValidatedDSL(ai, "some description")

	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Timings.Total, result.Timings.LLMInitial)
	assert.Greater(t, result.Timings.LLMInitial, time.Duration(0))
}
