package models

type DSL struct {
	Steps []Step `json:"steps"`
}

type Step struct {
	Type     string            `json:"type"`               // "condition" or "action"
	Field    string            `json:"field,omitempty"`    // condition only: "age", "email"
	Operator string            `json:"operator,omitempty"` // condition only: "gt", "lt", "eq", "gte", "lte"
	Value    any               `json:"value,omitempty"`    // condition only: 18, "verified"
	OnPass   string            `json:"on_pass,omitempty"`  // condition only: "next" or "reject"
	OnFail   string            `json:"on_fail,omitempty"`  // condition only: "next" or "reject"
	Action   string            `json:"action,omitempty"`   // action only: "send_email", "approve", "reject"
	Params   map[string]string `json:"params,omitempty"`   // action only: extra data
}
