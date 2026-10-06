package eval

import (
	"time"

	"workflow-engine/models"
	"workflow-engine/services"
)

// RunCase runs one TestCase through the real generate->validate->retry
// pipeline, then — if the final DSL is valid and behavior grading isn't
// skipped — executes it against every SampleInput and compares outcomes.
func RunCase(ai *services.AIService, engine *services.ExecutionEngine, tc TestCase) CaseResult {
	result := CaseResult{ID: tc.ID, Category: tc.Category, Description: tc.Description}

	pipelineResult, err := services.GenerateValidatedDSL(ai, tc.Description)
	if err != nil {
		result.InfraError = err.Error()
		return result
	}

	result.InitialValid = pipelineResult.InitialValid
	result.RetryAttempted = pipelineResult.RetryAttempted
	result.FinalValid = pipelineResult.FinalValid
	result.ValidationErrors = errsToStrings(pipelineResult.ValidationErrors)
	result.Timings = TimingsMS{
		LLMInitial:           msFloat(pipelineResult.Timings.LLMInitial),
		RateLimitWaitInitial: msFloat(pipelineResult.Timings.RateLimitWaitInitial),
		ValidationInitial:    msFloat(pipelineResult.Timings.ValidationInitial),
		LLMRetry:             msFloat(pipelineResult.Timings.LLMRetry),
		RateLimitWaitRetry:   msFloat(pipelineResult.Timings.RateLimitWaitRetry),
		ValidationRetry:      msFloat(pipelineResult.Timings.ValidationRetry),
		Total:                msFloat(pipelineResult.Timings.Total),
	}

	if !result.FinalValid || tc.SkipBehaviorCheck {
		return result
	}

	result.BehaviorChecked = true
	executedOK := true
	semanticallyCorrect := true

	for _, sample := range tc.SampleInputs {
		sr := SampleResult{
			Input:           sample.Input,
			ExpectedStatus:  sample.ExpectedStatus,
			ExpectedActions: sample.ExpectedActions,
		}

		execResult, execErr := engine.Execute(pipelineResult.DSL, sample.Input)
		if execErr != nil {
			sr.ExecutionError = execErr.Error()
			executedOK = false
			semanticallyCorrect = false
		} else {
			sr.ActualStatus = execResult.Status
			sr.ActualActions = actionsFromSteps(execResult.Steps)
			sr.Match = sr.ActualStatus == sample.ExpectedStatus && sameSet(sr.ActualActions, sample.ExpectedActions)
			if !sr.Match {
				semanticallyCorrect = false
			}
		}

		result.SampleResults = append(result.SampleResults, sr)
	}

	result.ExecutedOK = executedOK
	result.SemanticallyCorrect = semanticallyCorrect

	return result
}

func msFloat(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

func errsToStrings(errs []error) []string {
	if len(errs) == 0 {
		return nil
	}
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Error()
	}
	return out
}

func actionsFromSteps(steps []models.StepResult) []string {
	var actions []string
	for _, s := range steps {
		if s.Type == "action" && s.Action != "" {
			actions = append(actions, s.Action)
		}
	}
	return actions
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		counts[v]--
	}
	for _, c := range counts {
		if c != 0 {
			return false
		}
	}
	return true
}
