package eval

// SampleInput is one (input, expected outcome) pair a test case is graded against.
type SampleInput struct {
	Input           map[string]interface{} `json:"input"`
	ExpectedStatus  string                 `json:"expected_status"`
	ExpectedActions []string               `json:"expected_actions"`
}

// TestCase is one entry in eval/dataset.json.
type TestCase struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	// Description is the natural-language workflow description fed to the AI,
	// exactly as a user would type it.
	Description string `json:"description"`
	// SkipBehaviorCheck marks deliberately vague descriptions where there's no
	// single correct DSL — only schema validity is graded, not behavior.
	SkipBehaviorCheck bool          `json:"skip_behavior_check,omitempty"`
	SampleInputs      []SampleInput `json:"sample_inputs,omitempty"`
}

// SampleResult is the graded outcome of running one SampleInput through the
// generated DSL.
type SampleResult struct {
	Input           map[string]interface{} `json:"input"`
	ExpectedStatus  string                 `json:"expected_status"`
	ActualStatus    string                 `json:"actual_status,omitempty"`
	ExpectedActions []string               `json:"expected_actions,omitempty"`
	ActualActions   []string               `json:"actual_actions,omitempty"`
	ExecutionError  string                 `json:"execution_error,omitempty"`
	Match           bool                   `json:"match"`
}

// TimingsMS mirrors services.PipelineTimings in milliseconds, for JSON output.
type TimingsMS struct {
	LLMInitial        float64 `json:"llm_initial_ms"`
	ValidationInitial float64 `json:"validation_initial_ms"`
	LLMRetry          float64 `json:"llm_retry_ms,omitempty"`
	ValidationRetry   float64 `json:"validation_retry_ms,omitempty"`
	Total             float64 `json:"total_ms"`
}

// CaseResult is the full graded outcome for one TestCase.
type CaseResult struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Description string `json:"description"`

	// InfraError is set only when the initial AI request itself failed
	// (network, rate limit, etc.) — nothing else below is graded in that case.
	InfraError string `json:"infra_error,omitempty"`

	InitialValid     bool     `json:"initial_valid"`
	RetryAttempted   bool     `json:"retry_attempted"`
	FinalValid       bool     `json:"final_valid"`
	ValidationErrors []string `json:"validation_errors,omitempty"`

	// BehaviorChecked is false for infra errors, invalid DSLs, and
	// SkipBehaviorCheck cases — SemanticallyCorrect/ExecutedOK are meaningless then.
	BehaviorChecked     bool           `json:"behavior_checked"`
	SampleResults       []SampleResult `json:"sample_results,omitempty"`
	SemanticallyCorrect bool           `json:"semantically_correct"`
	ExecutedOK          bool           `json:"executed_ok"`

	Timings TimingsMS `json:"timings_ms"`
}

// CategoryStats aggregates CaseResult counts for one category.
type CategoryStats struct {
	Total           int `json:"total"`
	ValidDSL        int `json:"valid_dsl"`
	SemanticCorrect int `json:"semantic_correct"`
}

// Report is the aggregate result of a full eval run.
type Report struct {
	TotalCases     int `json:"total_cases"`
	InfraErrors    int `json:"infra_errors"`
	Graded         int `json:"graded"`          // TotalCases - InfraErrors
	BehaviorGraded int `json:"behavior_graded"` // Graded minus invalid/SkipBehaviorCheck cases

	ValidDSLCount int     `json:"valid_dsl_count"`
	ValidDSLRate  float64 `json:"valid_dsl_rate"`

	SemanticCorrectCount          int     `json:"semantic_correct_count"`
	SemanticCorrectRateOfTotal    float64 `json:"semantic_correct_rate_of_total"`
	SemanticCorrectRateOfGradable float64 `json:"semantic_correct_rate_of_gradable"`

	InitialValidCount int     `json:"initial_valid_count"`
	InitialValidRate  float64 `json:"initial_valid_rate"`

	RetryRequiredCount int     `json:"retry_required_count"`
	RetryPassedCount   int     `json:"retry_passed_count"`
	RetrySuccessRate   float64 `json:"retry_success_rate"`

	ExecutedOKCount          int     `json:"executed_ok_count"`
	ExecutedOKRateOfGradable float64 `json:"executed_ok_rate_of_gradable"`

	AvgLLMInitialMS        float64 `json:"avg_llm_initial_ms"`
	AvgValidationInitialMS float64 `json:"avg_validation_initial_ms"`
	AvgLLMRetryMS          float64 `json:"avg_llm_retry_ms"`
	AvgValidationRetryMS   float64 `json:"avg_validation_retry_ms"`
	AvgTotalMS             float64 `json:"avg_total_ms"`

	ByCategory map[string]*CategoryStats `json:"by_category"`
}
