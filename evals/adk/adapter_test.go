package adk_test

import (
	"testing"
	"time"

	"go.alis.build/adk/launchers/evals/evaluation/models"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/adk"
)

func TestCaseFromRunEvalResult_mapsFields(t *testing.T) {
	t.Parallel()

	score := 0.9
	r := models.RunEvalResult{
		EvalID:          "case-1",
		SessionID:       "sess-abc",
		FinalEvalStatus: models.EvalStatusPassed,
		OverallEvalMetricResults: []models.EvalMetricResult{{
			MetricName: "response_match_score",
			Threshold:  0.5,
			Score:      new(score),
			EvalStatus: models.EvalStatusPassed,
		}},
	}

	got := adk.CaseFromRunEvalResult(r, 2*time.Second)
	if got.Name != "case-1" {
		t.Fatalf("name = %q", got.Name)
	}
	if got.SessionID != "sess-abc" {
		t.Fatalf("session_id = %q", got.SessionID)
	}
	if got.Status != evalspb.Status_PASSED {
		t.Fatalf("status = %v", got.Status)
	}
	if len(got.Metrics) != 1 || got.Metrics[0].ID != "response_match_score" {
		t.Fatalf("metrics = %+v", got.Metrics)
	}
}

func TestCaseFromRunEvalResult_failedMetricMessage(t *testing.T) {
	t.Parallel()

	score := 0.2
	r := models.RunEvalResult{
		EvalID:          "case-2",
		FinalEvalStatus: models.EvalStatusFailed,
		OverallEvalMetricResults: []models.EvalMetricResult{{
			MetricName: "quality",
			Threshold:  0.8,
			Score:      new(score),
			EvalStatus: models.EvalStatusFailed,
		}},
	}

	got := adk.CaseFromRunEvalResult(r, 0)
	if got.Metrics[0].Message == "" {
		t.Fatal("expected failure message on metric")
	}
}

func TestCaseFromRunEvalResult_rubricScores(t *testing.T) {
	t.Parallel()

	rubricScore := 0.3
	rationale := "answer omitted the reference year"
	r := models.RunEvalResult{
		EvalID:          "case-3",
		FinalEvalStatus: models.EvalStatusFailed,
		OverallEvalMetricResults: []models.EvalMetricResult{{
			MetricName: "rubric_judge",
			Threshold:  0.5,
			EvalStatus: models.EvalStatusFailed,
			Details: &models.EvalMetricResultDetails{
				RubricScores: []models.RubricScore{{
					RubricID:  "accuracy",
					Score:     &rubricScore,
					Rationale: new(rationale),
				}},
			},
		}},
	}

	got := adk.CaseFromRunEvalResult(r, 0)
	if len(got.Metrics[0].Rubric) != 1 {
		t.Fatalf("rubric = %+v", got.Metrics[0].Rubric)
	}
	if got.Metrics[0].Rubric[0].Status != evalspb.Status_FAILED {
		t.Fatalf("rubric status = %v", got.Metrics[0].Rubric[0].Status)
	}
	if got.Metrics[0].Rubric[0].Rationale != rationale {
		t.Fatalf("rubric rationale = %q, want %q", got.Metrics[0].Rubric[0].Rationale, rationale)
	}
}

func TestAgentEvalResultsFromRunEvalResults(t *testing.T) {
	t.Parallel()

	proto := adk.AgentEvalResultsFromRunEvalResults(
		[]models.RunEvalResult{{
			EvalID:          "case-1",
			FinalEvalStatus: models.EvalStatusPassed,
		}},
		[]time.Duration{time.Second},
		adk.JudgeContext{Model: "gemini-2.5-pro"},
	)
	if len(proto.GetCases()) != 1 {
		t.Fatalf("cases = %d", len(proto.GetCases()))
	}
	if proto.GetJudge().GetModel() != "gemini-2.5-pro" {
		t.Fatalf("judge model = %q", proto.GetJudge().GetModel())
	}
}

func TestAgentEvalResultsFromRunEvalResults_rubricRationaleOnWire(t *testing.T) {
	t.Parallel()

	rubricScore := 0.42
	rationale := "response paraphrased the reference correctly but omitted the source citation"
	proto := adk.AgentEvalResultsFromRunEvalResults(
		[]models.RunEvalResult{{
			EvalID:          "case-1",
			FinalEvalStatus: models.EvalStatusFailed,
			OverallEvalMetricResults: []models.EvalMetricResult{{
				MetricName: "rubric_based_final_response_quality_v1",
				Threshold:  0.7,
				EvalStatus: models.EvalStatusFailed,
				Score:      &rubricScore,
				Details: &models.EvalMetricResultDetails{
					RubricScores: []models.RubricScore{{
						RubricID:  "accuracy",
						Score:     &rubricScore,
						Rationale: new(rationale),
					}},
				},
			}},
		}},
		[]time.Duration{time.Second},
		adk.JudgeContext{Model: "gemini-2.5-flash", CallCount: 1},
	)

	cases := proto.GetCases()
	if len(cases) != 1 || len(cases[0].GetMetrics()) != 1 {
		t.Fatalf("unexpected cases/metrics shape: %+v", cases)
	}
	rubrics := cases[0].GetMetrics()[0].GetRubric()
	if len(rubrics) != 1 {
		t.Fatalf("rubrics on wire = %d, want 1", len(rubrics))
	}
	if got := rubrics[0].GetRationale(); got != rationale {
		t.Errorf("wire rationale = %q, want %q", got, rationale)
	}
}

func TestAgentEvalResultsFromRunEvalResults_rubricRationaleOmittedWhenEmpty(t *testing.T) {
	t.Parallel()

	rubricScore := 0.9
	proto := adk.AgentEvalResultsFromRunEvalResults(
		[]models.RunEvalResult{{
			EvalID:          "case-1",
			FinalEvalStatus: models.EvalStatusPassed,
			OverallEvalMetricResults: []models.EvalMetricResult{{
				MetricName: "rubric_based_final_response_quality_v1",
				Threshold:  0.7,
				EvalStatus: models.EvalStatusPassed,
				Score:      &rubricScore,
				Details: &models.EvalMetricResultDetails{
					RubricScores: []models.RubricScore{{
						RubricID: "accuracy",
						Score:    &rubricScore,
					}},
				},
			}},
		}},
		[]time.Duration{time.Second},
		adk.JudgeContext{Model: "gemini-2.5-flash", CallCount: 1},
	)

	// Rationale is a proto3 optional (oneof); when unset the getter returns "",
	// and the underlying pointer must be nil to keep the wire byte
	// distinguishable from an explicit empty string.
	rubrics := proto.GetCases()[0].GetMetrics()[0].GetRubric()
	if rubrics[0].Rationale != nil {
		t.Errorf("wire Rationale pointer = %v, want nil (source Rationale was nil)", rubrics[0].Rationale)
	}
}

func TestJudgeContext_CallCountRoundtrip(t *testing.T) {
	t.Parallel()

	proto := adk.AgentEvalResultsFromRunEvalResults(
		[]models.RunEvalResult{{
			EvalID:          "case-1",
			FinalEvalStatus: models.EvalStatusPassed,
		}},
		[]time.Duration{time.Second},
		adk.JudgeContext{Model: "m", ModelVersion: "v", CallCount: 3, ErrorCount: 0},
	)
	judge := proto.GetJudge()
	if judge == nil {
		t.Fatal("Judge is nil, want populated")
	}
	if got, want := judge.GetModel(), "m"; got != want {
		t.Errorf("Model = %q, want %q", got, want)
	}
	if got, want := judge.GetModelVersion(), "v"; got != want {
		t.Errorf("ModelVersion = %q, want %q", got, want)
	}
	if got, want := judge.GetJudgeCallCount(), int64(3); got != want {
		t.Errorf("JudgeCallCount = %d, want %d", got, want)
	}
	if got, want := judge.GetJudgeErrorCount(), int64(0); got != want {
		t.Errorf("JudgeErrorCount = %d, want %d", got, want)
	}
}

func TestAgentEvalResults_JudgeNilWhenZeroValued(t *testing.T) {
	t.Parallel()

	proto := adk.AgentEvalResultsFromRunEvalResults(
		[]models.RunEvalResult{{
			EvalID:          "case-1",
			FinalEvalStatus: models.EvalStatusPassed,
		}},
		[]time.Duration{time.Second},
		adk.JudgeContext{},
	)
	if proto.GetJudge() != nil {
		t.Errorf("Judge = %+v, want nil", proto.GetJudge())
	}
}

func TestAgentEvalResults_JudgeEmittedWhenOnlyCallCountSet(t *testing.T) {
	t.Parallel()

	proto := adk.AgentEvalResultsFromRunEvalResults(
		nil,
		nil,
		adk.JudgeContext{CallCount: 5},
	)
	judge := proto.GetJudge()
	if judge == nil {
		t.Fatal("Judge is nil, want populated when CallCount > 0")
	}
	if got := judge.GetModel(); got != "" {
		t.Errorf("Model = %q, want empty", got)
	}
	if got, want := judge.GetJudgeCallCount(), int64(5); got != want {
		t.Errorf("JudgeCallCount = %d, want %d", got, want)
	}
}

func TestAgentEvalResults_JudgeEmittedWhenOnlyErrorCountSet(t *testing.T) {
	t.Parallel()

	proto := adk.AgentEvalResultsFromRunEvalResults(
		nil,
		nil,
		adk.JudgeContext{ErrorCount: 2},
	)
	if proto.GetJudge() == nil {
		t.Fatal("Judge is nil, want populated when ErrorCount > 0")
	}
	if got, want := proto.GetJudge().GetJudgeErrorCount(), int64(2); got != want {
		t.Errorf("JudgeErrorCount = %d, want %d", got, want)
	}
}

func TestSynthesizeJudgeContext_CountsJudgeMetricsOnly(t *testing.T) {
	t.Parallel()

	// Build 3 cases, each with 2 judge metrics + 1 non-judge metric.
	caseWithMetrics := func(id string) models.RunEvalResult {
		return models.RunEvalResult{
			EvalID:          id,
			FinalEvalStatus: models.EvalStatusPassed,
			OverallEvalMetricResults: []models.EvalMetricResult{
				{MetricName: models.MetricRubricBasedFinalResponseQualityV1, EvalStatus: models.EvalStatusPassed},
				{MetricName: models.MetricHallucinationsV1, EvalStatus: models.EvalStatusPassed},
				{MetricName: models.MetricResponseMatchScore, EvalStatus: models.EvalStatusPassed},
			},
		}
	}
	results := []models.RunEvalResult{caseWithMetrics("c1"), caseWithMetrics("c2"), caseWithMetrics("c3")}

	got := adk.SynthesizeJudgeContext(results, "gemini-2.5-flash", "")
	if got.Model != "gemini-2.5-flash" {
		t.Errorf("Model = %q, want gemini-2.5-flash", got.Model)
	}
	if got.CallCount != 6 {
		t.Errorf("CallCount = %d, want 6 (2 judge metrics * 3 cases)", got.CallCount)
	}
	if got.ErrorCount != 0 {
		t.Errorf("ErrorCount = %d, want 0", got.ErrorCount)
	}
}

func TestSynthesizeJudgeContextFromMetrics_FallsBackToProbedModel(t *testing.T) {
	t.Parallel()

	rubric, err := adk.RubricBasedFinalResponseQualityV1(0.7, nil, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("RubricBasedFinalResponseQualityV1: %v", err)
	}
	results := []models.RunEvalResult{{
		EvalID:          "c1",
		FinalEvalStatus: models.EvalStatusPassed,
		OverallEvalMetricResults: []models.EvalMetricResult{
			{MetricName: models.MetricRubricBasedFinalResponseQualityV1, EvalStatus: models.EvalStatusPassed},
		},
	}}

	got := adk.SynthesizeJudgeContextFromMetrics(results, []models.EvalMetric{rubric}, "")
	if got.Model != "gemini-2.5-flash" {
		t.Errorf("Model = %q, want gemini-2.5-flash (probed)", got.Model)
	}
	if got.CallCount != 1 {
		t.Errorf("CallCount = %d, want 1", got.CallCount)
	}
}

func TestSynthesizeJudgeContextFromMetrics_ModelVersionVerbatim(t *testing.T) {
	t.Parallel()

	rubric, err := adk.RubricBasedFinalResponseQualityV1(0.7, nil, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("RubricBasedFinalResponseQualityV1: %v", err)
	}

	// Empty results → CallCount is 0; Model still comes from the probe;
	// ModelVersion is used verbatim (there is no fallback source for it).
	got := adk.SynthesizeJudgeContextFromMetrics(nil, []models.EvalMetric{rubric}, "explicit-version")
	if got.Model != "gemini-2.5-flash" {
		t.Errorf("Model = %q, want gemini-2.5-flash (probed)", got.Model)
	}
	if got.ModelVersion != "explicit-version" {
		t.Errorf("ModelVersion = %q, want explicit-version", got.ModelVersion)
	}
	if got.CallCount != 0 {
		t.Errorf("CallCount = %d, want 0 (empty results)", got.CallCount)
	}
}
