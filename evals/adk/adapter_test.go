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
			Score:      &score,
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
			Score:      &score,
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
	r := models.RunEvalResult{
		EvalID:          "case-3",
		FinalEvalStatus: models.EvalStatusFailed,
		OverallEvalMetricResults: []models.EvalMetricResult{{
			MetricName: "rubric_judge",
			Threshold:  0.5,
			EvalStatus: models.EvalStatusFailed,
			Details: &models.EvalMetricResultDetails{
				RubricScores: []models.RubricScore{{
					RubricID: "accuracy",
					Score:    &rubricScore,
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
