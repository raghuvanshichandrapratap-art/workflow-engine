package eval

import (
	"fmt"
	"sort"
	"strings"
)

func BuildReport(results []CaseResult) Report {
	report := Report{ByCategory: make(map[string]*CategoryStats)}
	report.TotalCases = len(results)

	var sumLLMInitial, sumValidationInitial, sumLLMRetry, sumValidationRetry, sumTotal float64
	var retryLatencyCount int

	for _, r := range results {
		cat := report.ByCategory[r.Category]
		if cat == nil {
			cat = &CategoryStats{}
			report.ByCategory[r.Category] = cat
		}
		cat.Total++

		if r.InfraError != "" {
			report.InfraErrors++
			continue
		}
		report.Graded++

		if r.InitialValid {
			report.InitialValidCount++
		} else {
			report.RetryRequiredCount++
			if r.FinalValid {
				report.RetryPassedCount++
			}
		}

		if r.FinalValid {
			report.ValidDSLCount++
			cat.ValidDSL++
		}

		if r.BehaviorChecked {
			report.BehaviorGraded++
			if r.SemanticallyCorrect {
				report.SemanticCorrectCount++
				cat.SemanticCorrect++
			}
			if r.ExecutedOK {
				report.ExecutedOKCount++
			}
		}

		sumLLMInitial += r.Timings.LLMInitial
		sumValidationInitial += r.Timings.ValidationInitial
		sumTotal += r.Timings.Total
		if r.RetryAttempted {
			sumLLMRetry += r.Timings.LLMRetry
			sumValidationRetry += r.Timings.ValidationRetry
			retryLatencyCount++
		}
	}

	if graded := float64(report.Graded); graded > 0 {
		report.ValidDSLRate = float64(report.ValidDSLCount) / graded
		report.SemanticCorrectRateOfTotal = float64(report.SemanticCorrectCount) / graded
		report.InitialValidRate = float64(report.InitialValidCount) / graded
		report.AvgLLMInitialMS = sumLLMInitial / graded
		report.AvgValidationInitialMS = sumValidationInitial / graded
		report.AvgTotalMS = sumTotal / graded
	}
	if report.BehaviorGraded > 0 {
		behaviorGraded := float64(report.BehaviorGraded)
		report.SemanticCorrectRateOfGradable = float64(report.SemanticCorrectCount) / behaviorGraded
		report.ExecutedOKRateOfGradable = float64(report.ExecutedOKCount) / behaviorGraded
	}
	if report.RetryRequiredCount > 0 {
		report.RetrySuccessRate = float64(report.RetryPassedCount) / float64(report.RetryRequiredCount)
	}
	if retryLatencyCount > 0 {
		report.AvgLLMRetryMS = sumLLMRetry / float64(retryLatencyCount)
		report.AvgValidationRetryMS = sumValidationRetry / float64(retryLatencyCount)
	}

	return report
}

func RenderMarkdown(report Report) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Evaluation Report\n\n")
	fmt.Fprintf(&b, "- Total cases: %d\n", report.TotalCases)
	fmt.Fprintf(&b, "- Infra errors (excluded from all rates below): %d\n", report.InfraErrors)
	fmt.Fprintf(&b, "- Graded cases: %d\n", report.Graded)
	fmt.Fprintf(&b, "- Behavior-gradable cases (excludes invalid DSLs and ambiguous-description cases): %d\n\n", report.BehaviorGraded)

	fmt.Fprintf(&b, "## Pipeline Metrics\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| Initial generation pass rate | %d/%d (%.1f%%) |\n", report.InitialValidCount, report.Graded, report.InitialValidRate*100)
	fmt.Fprintf(&b, "| Retry required | %d |\n", report.RetryRequiredCount)
	fmt.Fprintf(&b, "| Retry passed (of required) | %d/%d (%.1f%%) |\n", report.RetryPassedCount, report.RetryRequiredCount, report.RetrySuccessRate*100)
	fmt.Fprintf(&b, "| Valid DSL rate (final, after retry) | %d/%d (%.1f%%) |\n", report.ValidDSLCount, report.Graded, report.ValidDSLRate*100)
	fmt.Fprintf(&b, "| Semantically correct (of total graded) | %d/%d (%.1f%%) |\n", report.SemanticCorrectCount, report.Graded, report.SemanticCorrectRateOfTotal*100)
	fmt.Fprintf(&b, "| Semantically correct (of behavior-gradable) | %d/%d (%.1f%%) |\n", report.SemanticCorrectCount, report.BehaviorGraded, report.SemanticCorrectRateOfGradable*100)
	fmt.Fprintf(&b, "| Executed without runtime error (of behavior-gradable) | %d/%d (%.1f%%) |\n\n", report.ExecutedOKCount, report.BehaviorGraded, report.ExecutedOKRateOfGradable*100)

	fmt.Fprintf(&b, "## Average Latency (ms)\n\n")
	fmt.Fprintf(&b, "| Stage | Avg ms |\n|---|---|\n")
	fmt.Fprintf(&b, "| LLM — initial call | %.1f |\n", report.AvgLLMInitialMS)
	fmt.Fprintf(&b, "| Validation — initial | %.3f |\n", report.AvgValidationInitialMS)
	fmt.Fprintf(&b, "| LLM — retry call (of retried cases) | %.1f |\n", report.AvgLLMRetryMS)
	fmt.Fprintf(&b, "| Validation — retry (of retried cases) | %.3f |\n", report.AvgValidationRetryMS)
	fmt.Fprintf(&b, "| Total per case | %.1f |\n\n", report.AvgTotalMS)

	fmt.Fprintf(&b, "## By Category\n\n")
	fmt.Fprintf(&b, "| Category | Total | Valid DSL | Semantically Correct |\n|---|---|---|---|\n")

	categories := make([]string, 0, len(report.ByCategory))
	for c := range report.ByCategory {
		categories = append(categories, c)
	}
	sort.Strings(categories)

	for _, c := range categories {
		s := report.ByCategory[c]
		fmt.Fprintf(&b, "| %s | %d | %d | %d |\n", c, s.Total, s.ValidDSL, s.SemanticCorrect)
	}

	return b.String()
}
