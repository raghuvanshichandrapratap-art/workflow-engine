package models

type ExecutionResult struct {
	Status string       `json:"status"` // "approved" or "rejected"
	Steps  []StepResult `json:"steps"`
}

type StepResult struct {
	Type    string `json:"type"`
	Passed  bool   `json:"passed,omitempty"`
	Action  string `json:"action,omitempty"`
	Message string `json:"message"`
}