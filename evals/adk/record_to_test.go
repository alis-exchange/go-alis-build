package adk_test

import (
	"slices"
	"testing"
	"time"

	"go.alis.build/adk/launchers/evals/evaluation/models"
	evalspb "go.alis.build/common/alis/evals"
	"go.alis.build/evals/adk"
	"google.golang.org/protobuf/proto"
)

// recorder is a CaseRecorder that records the order and values of calls.
type recorder struct {
	calls     []string
	sessionID string
	metrics   []*evalspb.AgentEvalResults_Case_Metric
	judge     *evalspb.AgentEvalResults_JudgeInfo
	fails     []error
	reasons   []string
}

// SetSessionID records the session id.
func (r *recorder) SetSessionID(id string) {
	r.calls = append(r.calls, "session")
	r.sessionID = id
}

// AddMetric records one metric.
func (r *recorder) AddMetric(m *evalspb.AgentEvalResults_Case_Metric) {
	r.calls = append(r.calls, "metric")
	r.metrics = append(r.metrics, m)
}

// SetJudgeInfo records the judge info.
func (r *recorder) SetJudgeInfo(j *evalspb.AgentEvalResults_JudgeInfo) {
	r.calls = append(r.calls, "judge")
	r.judge = j
}

// Fail records one failure.
func (r *recorder) Fail(err error) {
	r.calls = append(r.calls, "fail")
	r.fails = append(r.fails, err)
}

// SetNotEvaluated records one NOT_EVALUATED reason.
func (r *recorder) SetNotEvaluated(reason string) {
	r.calls = append(r.calls, "not_evaluated")
	r.reasons = append(r.reasons, reason)
}

// failedCase is a FAILED ADK case with one judge metric.
func failedCase() adk.ProviderCase {
	return adk.ProviderCase{
		SetID: "smoke",
		Result: models.RunEvalResult{
			EvalID:          "case_b.v2",
			SessionID:       "sess-b",
			FinalEvalStatus: models.EvalStatusFailed,
			OverallEvalMetricResults: []models.EvalMetricResult{
				{MetricName: models.MetricFinalResponseMatchV2, Threshold: 0.5, Score: new(0.2), EvalStatus: models.EvalStatusFailed},
			},
		},
		Judge: adk.JudgeContext{Model: "gemini-2.5-pro", ModelVersion: "2025-06-05", CallCount: 1},
	}
}

// TestProviderCase_RecordTo_failedCase records data in order, then fails.
func TestProviderCase_RecordTo_failedCase(t *testing.T) {
	t.Parallel()

	c := failedCase()
	var r recorder
	c.RecordTo(&r)

	if want := []string{"session", "metric", "judge", "fail"}; !slices.Equal(r.calls, want) {
		t.Fatalf("calls = %q, want %q", r.calls, want)
	}
	if r.sessionID != "sess-b" {
		t.Fatalf("session = %q, want sess-b", r.sessionID)
	}
	wantMetrics := adk.AgentEvalResultsFromRunEvalResults("smoke", []models.RunEvalResult{c.Result}, []time.Duration{0}, adk.JudgeContext{}).GetCases()[0].GetMetrics()
	if len(r.metrics) != 1 || !proto.Equal(r.metrics[0], wantMetrics[0]) {
		t.Fatalf("metrics = %v, want %v", r.metrics, wantMetrics)
	}
	version := "2025-06-05"
	wantJudge := &evalspb.AgentEvalResults_JudgeInfo{Model: "gemini-2.5-pro", ModelVersion: &version, JudgeCallCount: 1}
	if !proto.Equal(r.judge, wantJudge) {
		t.Fatalf("judge = %v, want %v", r.judge, wantJudge)
	}
	if len(r.fails) != 1 || r.fails[0].Error() != "adk: final eval status FAILED" {
		t.Fatalf("fails = %v, want one final-status failure", r.fails)
	}
}

// TestProviderCase_RecordTo_passedCaseWithoutJudge skips empty session,
// zero judge, and Fail.
func TestProviderCase_RecordTo_passedCaseWithoutJudge(t *testing.T) {
	t.Parallel()

	c := adk.ProviderCase{SetID: "smoke", Result: models.RunEvalResult{
		EvalID:          "case_a",
		FinalEvalStatus: models.EvalStatusPassed,
		OverallEvalMetricResults: []models.EvalMetricResult{
			{MetricName: models.MetricResponseMatchScore, Threshold: 0.3, Score: new(1.0), EvalStatus: models.EvalStatusPassed},
		},
	}}
	var r recorder
	c.RecordTo(&r)

	if want := []string{"metric"}; !slices.Equal(r.calls, want) {
		t.Fatalf("calls = %q, want %q", r.calls, want)
	}
}

// TestProviderCase_RecordTo_notEvaluatedCase marks the case NOT_EVALUATED,
// keeping its data and without failing it, so an ADK NOT_EVALUATED result
// never reports PASSED in the suite.
func TestProviderCase_RecordTo_notEvaluatedCase(t *testing.T) {
	t.Parallel()

	c := adk.ProviderCase{SetID: "smoke", Result: models.RunEvalResult{
		EvalID:          "case_c",
		SessionID:       "sess-c",
		FinalEvalStatus: models.EvalStatusNotEvaluated,
		OverallEvalMetricResults: []models.EvalMetricResult{
			{MetricName: models.MetricResponseMatchScore, Threshold: 0.3, EvalStatus: models.EvalStatusNotEvaluated},
		},
	}}
	var r recorder
	c.RecordTo(&r)

	if want := []string{"session", "metric", "not_evaluated"}; !slices.Equal(r.calls, want) {
		t.Fatalf("calls = %q, want %q", r.calls, want)
	}
	if want := []string{"adk: final eval status NOT_EVALUATED"}; !slices.Equal(r.reasons, want) {
		t.Fatalf("reasons = %q, want %q", r.reasons, want)
	}
	if len(r.fails) != 0 {
		t.Fatalf("fails = %v, want none", r.fails)
	}
}

// TestProviderCase_RecordTo_nilRecorder does not panic.
func TestProviderCase_RecordTo_nilRecorder(t *testing.T) {
	t.Parallel()

	failedCase().RecordTo(nil)
}
