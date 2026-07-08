package result

import (
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
)

const (
	CaseErrorCheckName  = "case"
	SetupErrorCheckName = "setup"
	SkippedCheckName    = "skipped"
)

func statusPassed(s evalspb.Status) bool {
	return s == evalspb.Status_PASSED
}

// RollupRunStatus aggregates per-case statuses into a run-level status.
func RollupRunStatus(statuses []evalspb.Status) evalspb.Status {
	hasFailed := false
	for _, s := range statuses {
		switch s {
		case evalspb.Status_FAILED:
			hasFailed = true
		case evalspb.Status_PASSED:
			continue
		default:
			hasFailed = true
		}
	}
	if hasFailed {
		return evalspb.Status_FAILED
	}
	return evalspb.Status_PASSED
}

// RollupCaseStatus aggregates metric leaves into one case status.
func RollupCaseStatus(metrics []execution.Metric) evalspb.Status {
	for _, m := range metrics {
		if !metricPassed(m) {
			return evalspb.Status_FAILED
		}
	}
	return evalspb.Status_PASSED
}

func metricPassed(m execution.Metric) bool {
	return statusPassed(m.Status)
}

func caseErrorMetric(err error) execution.Metric {
	return execution.Metric{
		ID:      CaseErrorCheckName,
		Status:  evalspb.Status_FAILED,
		Message: err.Error(),
	}
}

// MetricFromCheck maps a deterministic check to a wire metric.
func MetricFromCheck(c execution.Check) execution.Metric {
	return execution.Metric{
		ID:      c.ID,
		Status:  c.Status,
		Message: c.Message,
	}
}

// MetricFromCriterion maps an in-process judge criterion to a wire metric.
func MetricFromCriterion(c execution.Criterion) execution.Metric {
	score := c.Score
	m := execution.Metric{
		ID:        c.ID,
		Status:    c.Status,
		Score:     &score,
		Threshold: c.Threshold,
		Message:   c.Rationale,
	}
	if len(c.Rubric) > 0 {
		m.Rubric = make([]execution.RubricScore, len(c.Rubric))
		for i, r := range c.Rubric {
			m.Rubric[i] = execution.RubricScore{
				ID:     r.ID,
				Status: r.Status,
			}
		}
	}
	return m
}

// MetricsFromEvalLeaves converts in-process assertions and criteria to wire metrics.
func MetricsFromEvalLeaves(assertions []execution.Check, criteria []execution.Criterion) []execution.Metric {
	out := make([]execution.Metric, 0, len(assertions)+len(criteria))
	for _, a := range assertions {
		out = append(out, MetricFromCheck(a))
	}
	for _, c := range criteria {
		out = append(out, MetricFromCriterion(c))
	}
	return out
}

// SetupErrorResult records a suite setup failure on a test case.
func SetupErrorResult(name string, err error) *execution.CaseResult {
	return &execution.CaseResult{
		Name:   name,
		Status: evalspb.Status_FAILED,
		Checks: []execution.Check{{
			ID:      SetupErrorCheckName,
			Status:  evalspb.Status_FAILED,
			Message: err.Error(),
		}},
	}
}

// EvalSetupErrorResult records a suite setup failure on an eval case.
func EvalSetupErrorResult(name string, err error) *execution.CaseResult {
	metrics := []execution.Metric{caseErrorMetric(err)}
	metrics[0].ID = SetupErrorCheckName
	return &execution.CaseResult{
		Name:    name,
		Status:  evalspb.Status_FAILED,
		Metrics: metrics,
	}
}

// CaseErrorResult records a case execution failure on a test case.
func CaseErrorResult(name string, err error) *execution.CaseResult {
	return &execution.CaseResult{
		Name:   name,
		Status: evalspb.Status_FAILED,
		Checks: []execution.Check{{
			ID:      CaseErrorCheckName,
			Status:  evalspb.Status_FAILED,
			Message: err.Error(),
		}},
	}
}

// SkippedResult records a test case as NOT_EVALUATED because a preceding case
// failed. reason names the failed predecessor (or is a plain "preconditions
// not met" message).
func SkippedResult(name, reason string) *execution.CaseResult {
	return &execution.CaseResult{
		Name:   name,
		Status: evalspb.Status_NOT_EVALUATED,
		Checks: []execution.Check{{
			ID:      SkippedCheckName,
			Status:  evalspb.Status_NOT_EVALUATED,
			Message: reason,
		}},
	}
}

// EvalSkippedResult records an eval case as NOT_EVALUATED for the same reason
// as [SkippedResult]. The skipped marker lives on Metrics for eval cases.
func EvalSkippedResult(name, reason string) *execution.CaseResult {
	return &execution.CaseResult{
		Name:   name,
		Status: evalspb.Status_NOT_EVALUATED,
		Metrics: []execution.Metric{{
			ID:      SkippedCheckName,
			Status:  evalspb.Status_NOT_EVALUATED,
			Message: reason,
		}},
	}
}

// EvalCaseErrorResult records a case execution failure on an eval case.
func EvalCaseErrorResult(name string, err error) *execution.CaseResult {
	return &execution.CaseResult{
		Name:    name,
		Status:  evalspb.Status_FAILED,
		Metrics: []execution.Metric{caseErrorMetric(err)},
	}
}
