package services

import (
	"fmt"
	"net/url"
	"workflow-engine/models"
)

var allowedOperators = map[string]bool{
	"gt": true, "lt": true, "eq": true, "gte": true, "lte": true,
}

var numericOperators = map[string]bool{
	"gt": true, "lt": true, "gte": true, "lte": true,
}

var allowedOnBranch = map[string]bool{
	"next": true, "reject": true,
}

var allowedActions = map[string]bool{
	"approve": true, "reject": true, "send_email": true, "assign_role": true, "webhook": true,
}

var requiredActionParams = map[string]string{
	"send_email":  "template",
	"assign_role": "role",
	"webhook":     "url",
}

func ValidateDSL(dsl *models.DSL) []error {
	var errs []error

	if dsl == nil || len(dsl.Steps) == 0 {
		return []error{fmt.Errorf("DSL must contain at least one step")}
	}

	for i, step := range dsl.Steps {
		switch step.Type {
		case "condition":
			errs = append(errs, validateCondition(i, step)...)
		case "action":
			errs = append(errs, validateAction(i, step)...)
		default:
			errs = append(errs, fmt.Errorf("step %d: type must be 'condition' or 'action', got %q", i, step.Type))
		}
	}

	return errs
}

func validateCondition(i int, step models.Step) []error {
	var errs []error

	if step.Field == "" {
		errs = append(errs, fmt.Errorf("step %d: condition missing 'field'", i))
	}

	if step.Operator == "" {
		errs = append(errs, fmt.Errorf("step %d: condition missing 'operator'", i))
	} else if !allowedOperators[step.Operator] {
		errs = append(errs, fmt.Errorf("step %d: operator %q is not allowed (must be gt, lt, eq, gte, lte)", i, step.Operator))
	}

	if step.Value == nil {
		errs = append(errs, fmt.Errorf("step %d: condition missing 'value'", i))
	} else if numericOperators[step.Operator] {
		if _, ok := toFloat64(step.Value); !ok {
			errs = append(errs, fmt.Errorf("step %d: operator %q requires a numeric 'value', got %v", i, step.Operator, step.Value))
		}
	}

	if step.OnPass == "" {
		errs = append(errs, fmt.Errorf("step %d: condition missing 'on_pass'", i))
	} else if !allowedOnBranch[step.OnPass] {
		errs = append(errs, fmt.Errorf("step %d: on_pass %q is not allowed (must be next or reject)", i, step.OnPass))
	}

	if step.OnFail == "" {
		errs = append(errs, fmt.Errorf("step %d: condition missing 'on_fail'", i))
	} else if !allowedOnBranch[step.OnFail] {
		errs = append(errs, fmt.Errorf("step %d: on_fail %q is not allowed (must be next or reject)", i, step.OnFail))
	}

	return errs
}

func validateAction(i int, step models.Step) []error {
	var errs []error

	if step.Action == "" {
		errs = append(errs, fmt.Errorf("step %d: action missing 'action'", i))
		return errs
	}

	if !allowedActions[step.Action] {
		errs = append(errs, fmt.Errorf("step %d: action %q is not allowed (must be approve, reject, send_email, assign_role, webhook)", i, step.Action))
		return errs
	}

	requiredParam, needsParam := requiredActionParams[step.Action]
	if needsParam && step.Params[requiredParam] == "" {
		errs = append(errs, fmt.Errorf("step %d: action %q requires params.%s", i, step.Action, requiredParam))
		return errs
	}

	if step.Action == "webhook" {
		if _, err := normalizeWebhookURL(step.Params["url"]); err != nil {
			errs = append(errs, fmt.Errorf("step %d: webhook params.url %q is invalid: %w", i, step.Params["url"], err))
		}
	}

	return errs
}

// normalizeWebhookURL accepts both full URLs ("https://api.example.com/hook")
// and bare hosts ("www.example.com"), defaulting to https when no scheme is given.
// Used by both validation and execution so a URL that passes validation is
// guaranteed to be postable.
func normalizeWebhookURL(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("url is empty")
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		parsed, err = url.Parse("https://" + raw)
	}
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("not a valid url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported url scheme %q", parsed.Scheme)
	}

	return parsed.String(), nil
}