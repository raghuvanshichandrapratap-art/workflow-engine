package services

import (
	"fmt"
	"time"

	"workflow-engine/models"
)

// PipelineTimings records how long each stage of GenerateValidatedDSL took.
// Retry fields are zero when no retry was attempted.
type PipelineTimings struct {
	LLMInitial        time.Duration
	ValidationInitial time.Duration
	LLMRetry          time.Duration
	ValidationRetry   time.Duration
	Total             time.Duration
}

// PipelineResult is the outcome of running a description through
// generate -> validate -> (retry -> validate). A bad DSL is not an error —
// FinalValid is false and ValidationErrors explains why, so callers can
// still inspect and report on it.
type PipelineResult struct {
	DSL              *models.DSL
	RawInitial       string
	RawRetry         string
	RetryAttempted   bool
	InitialValid     bool
	FinalValid       bool
	ValidationErrors []error
	Timings          PipelineTimings
}

// GenerateValidatedDSL asks the AI for a DSL, validates it, and — if invalid —
// retries once with the validation errors fed back for self-correction.
// The returned error is reserved for infra-level failures (the initial AI
// request itself failing, e.g. network or rate limit) where there's nothing
// to correct and retrying wouldn't help.
func GenerateValidatedDSL(ai *AIService, description string) (*PipelineResult, error) {
	start := time.Now()
	result := &PipelineResult{}

	llmStart := time.Now()
	dsl, raw, err := ai.GenerateDSL(description)
	result.Timings.LLMInitial = time.Since(llmStart)
	result.RawInitial = raw

	var problems []error
	switch {
	case err != nil && raw == "":
		result.Timings.Total = time.Since(start)
		return nil, fmt.Errorf("AI request failed: %w", err)
	case err != nil:
		problems = []error{err}
	default:
		validateStart := time.Now()
		problems = ValidateDSL(dsl)
		result.Timings.ValidationInitial = time.Since(validateStart)
	}

	result.DSL = dsl
	result.InitialValid = len(problems) == 0

	if result.InitialValid {
		result.FinalValid = true
		result.Timings.Total = time.Since(start)
		return result, nil
	}

	result.RetryAttempted = true

	llmRetryStart := time.Now()
	retryDSL, retryRaw, retryErr := ai.RetryDSL(description, raw, problems)
	result.Timings.LLMRetry = time.Since(llmRetryStart)
	result.RawRetry = retryRaw

	if retryErr != nil {
		result.ValidationErrors = []error{retryErr}
		result.Timings.Total = time.Since(start)
		return result, nil
	}

	validateRetryStart := time.Now()
	retryProblems := ValidateDSL(retryDSL)
	result.Timings.ValidationRetry = time.Since(validateRetryStart)

	result.DSL = retryDSL
	result.ValidationErrors = retryProblems
	result.FinalValid = len(retryProblems) == 0
	result.Timings.Total = time.Since(start)

	return result, nil
}
