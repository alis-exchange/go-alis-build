package result

import (
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
)

const (
	// CaseErrorCheckName is the check id recorded when a case panics.
	CaseErrorCheckName = "case"
	// SetupErrorCheckName is the check id recorded when suite setup fails.
	SetupErrorCheckName = "setup"
	// SkippedCheckName is the check id recorded when a case is not evaluated.
	SkippedCheckName = "skipped"
)

// statusPassed reports whether s is the wire PASSED enum value.
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

// metricPassed reports whether a single metric leaf passed rollup.
func metricPassed(m execution.Metric) bool {
	return statusPassed(m.Status)
}

// caseErrorMetric builds a FAILED metric leaf for a panicked or errored case.
// The caller may override ID (for example to SetupErrorCheckName).
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
// Per-rubric Rationale is preserved so downstream mappers can emit it on the
// wire; see [MetricsProto].
func MetricFromCriterion(c execution.Criterion) execution.Metric {
	m := execution.Metric{
		ID:        c.ID,
		Status:    c.Status,
		Score:     new(c.Score),
		Threshold: c.Threshold,
		Message:   c.Rationale,
	}
	if len(c.Rubric) > 0 {
		m.Rubric = make([]execution.RubricScore, len(c.Rubric))
		for i, r := range c.Rubric {
			m.Rubric[i] = execution.RubricScore{
				ID:        r.ID,
				Status:    r.Status,
				Rationale: r.Rationale,
			}
		}
	}
	return m
}

// MetricsProto converts a slice of internal [execution.Metric] into the wire
// proto shape emitted on [alis.evals.v1.AgentEvalResults.Case.Metric].
//
// Shared between the ADK adapter and the runner-level mapper so both paths
// into the wire stay in lock-step; without this indirection a schema addition
// (e.g. the [execution.RubricScore.Rationale] rollout) has to be applied
// twice and can silently drift.
//
// Threshold is emitted only when Score is set: without an observed score there
// is no comparison baseline to report on the wire.
//
// Empty [execution.RubricScore.Rationale] values are elided (the proto's
// Rationale is proto3-optional) so readers can distinguish "not provided"
// from an explicit empty string; the same convention applies to
// [execution.RubricScore.Score] via the *float64.
func MetricsProto(metrics []execution.Metric) []*evalspb.AgentEvalResults_Case_Metric {
	out := make([]*evalspb.AgentEvalResults_Case_Metric, len(metrics))
	for i, m := range metrics {
		wm := &evalspb.AgentEvalResults_Case_Metric{
			Id:      m.ID,
			Status:  m.Status,
			Score:   m.Score,
			Message: m.Message,
		}
		if m.Score != nil {
			wm.Threshold = new(m.Threshold)
		}
		if len(m.Rubric) > 0 {
			wm.Rubric = make([]*evalspb.AgentEvalResults_Case_Metric_RubricScore, len(m.Rubric))
			for j, r := range m.Rubric {
				wr := &evalspb.AgentEvalResults_Case_Metric_RubricScore{
					Id:     r.ID,
					Status: r.Status,
					Score:  r.Score,
				}
				if r.Rationale != "" {
					wr.Rationale = new(r.Rationale)
				}
				wm.Rubric[j] = wr
			}
		}
		out[i] = wm
	}
	return out
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
