package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"workflow-engine/models"
)

type ExecutionEngine struct {
	httpClient *http.Client
}

func NewExecutionEngine() *ExecutionEngine {
	return &ExecutionEngine{httpClient: http.DefaultClient}
}

func (e *ExecutionEngine) Execute(dsl *models.DSL, input map[string]interface{}) (*models.ExecutionResult, error) {
	result := &models.ExecutionResult{Status: "approved"}

	for _, step := range dsl.Steps {
		switch step.Type {
		case "condition":
			passed, message, err := evaluateCondition(step, input)
			if err != nil {
				return nil, err
			}

			result.Steps = append(result.Steps, models.StepResult{
				Type:    "condition",
				Passed:  passed,
				Message: message,
			})

			if passed && step.OnPass == "reject" {
				result.Status = "rejected"
				return result, nil
			}
			if !passed && step.OnFail == "reject" {
				result.Status = "rejected"
				return result, nil
			}

		case "action":
			message, err := e.executeAction(step)
			if err != nil {
				return nil, err
			}

			result.Steps = append(result.Steps, models.StepResult{
				Type:    "action",
				Action:  step.Action,
				Message: message,
			})

			if step.Action == "reject" {
				result.Status = "rejected"
				return result, nil
			}
			if step.Action == "approve" {
				result.Status = "approved"
			}
		}
	}

	return result, nil
}

func evaluateCondition(step models.Step, input map[string]interface{}) (bool, string, error) {
	inputVal, exists := input[step.Field]
	if !exists {
		return false, fmt.Sprintf("field '%s' not found in input", step.Field), nil
	}

	inputNum, inputIsNum := toFloat64(inputVal)
	stepNum, stepIsNum := toFloat64(step.Value)

	switch step.Operator {
	case "eq":
		passed := fmt.Sprintf("%v", inputVal) == fmt.Sprintf("%v", step.Value)
		return passed, fmt.Sprintf("%s == %v: %v", step.Field, step.Value, passed), nil
	case "gt":
		if !inputIsNum || !stepIsNum {
			return false, "", fmt.Errorf("operator 'gt' requires numeric values for field '%s'", step.Field)
		}
		passed := inputNum > stepNum
		return passed, fmt.Sprintf("%s(%v) > %v: %v", step.Field, inputNum, stepNum, passed), nil
	case "lt":
		if !inputIsNum || !stepIsNum {
			return false, "", fmt.Errorf("operator 'lt' requires numeric values for field '%s'", step.Field)
		}
		passed := inputNum < stepNum
		return passed, fmt.Sprintf("%s(%v) < %v: %v", step.Field, inputNum, stepNum, passed), nil
	case "gte":
		if !inputIsNum || !stepIsNum {
			return false, "", fmt.Errorf("operator 'gte' requires numeric values for field '%s'", step.Field)
		}
		passed := inputNum >= stepNum
		return passed, fmt.Sprintf("%s(%v) >= %v: %v", step.Field, inputNum, stepNum, passed), nil
	case "lte":
		if !inputIsNum || !stepIsNum {
			return false, "", fmt.Errorf("operator 'lte' requires numeric values for field '%s'", step.Field)
		}
		passed := inputNum <= stepNum
		return passed, fmt.Sprintf("%s(%v) <= %v: %v", step.Field, inputNum, stepNum, passed), nil
	}

	return false, "", fmt.Errorf("unknown operator: %s", step.Operator)
}

func (e *ExecutionEngine) executeAction(step models.Step) (string, error) {
	switch step.Action {
	case "approve":
		return "workflow approved", nil
	case "reject":
		return "workflow rejected", nil
	case "send_email":
		template := step.Params["template"]
		return fmt.Sprintf("email sent with template '%s'", template), nil
	case "assign_role":
		role := step.Params["role"]
		return fmt.Sprintf("role '%s' assigned", role), nil
	case "webhook":
		url, err := normalizeWebhookURL(step.Params["url"])
		if err != nil {
			return "", fmt.Errorf("webhook call failed: %w", err)
		}
		payload, _ := json.Marshal(step.Params)
		resp, err := e.httpClient.Post(url, "application/json", bytes.NewBuffer(payload))
		if err != nil {
			return "", fmt.Errorf("webhook call failed: %w", err)
		}
		defer resp.Body.Close()
		return fmt.Sprintf("webhook called: %s — status %d", url, resp.StatusCode), nil
	}

	return "", fmt.Errorf("unknown action: %s", step.Action)
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	}
	return 0, false
}