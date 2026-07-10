package adk

import (
	"testing"

	"go.alis.build/adk/launchers/evals/evaluation/models"
)

func TestIsJudgeMetric(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name string
		in   string
		want bool
	}{
		// Judge-backed metrics (evaluator uses cfg.JudgeClient).
		{"final_response_match_v2", models.MetricFinalResponseMatchV2, true},
		{"rubric_based_final_response_quality_v1", models.MetricRubricBasedFinalResponseQualityV1, true},
		{"rubric_based_tool_use_quality_v1", models.MetricRubricBasedToolUseQualityV1, true},
		{"rubric_based_multi_turn_trajectory_quality_v1", models.MetricRubricBasedMultiTurnTrajectoryQualityV1, true},
		{"hallucinations_v1", models.MetricHallucinationsV1, true},
		{"per_turn_user_simulator_quality_v1", models.MetricPerTurnUserSimulatorQualityV1, true},

		// Vertex Rapid Eval metrics (evaluator uses cfg.VertexClient, NOT judge).
		{"safety_v1", models.MetricSafetyV1, false},
		{"response_evaluation_score", models.MetricResponseEvaluationScore, false},
		{"multi_turn_task_success_v1", models.MetricMultiTurnTaskSuccessV1, false},
		{"multi_turn_trajectory_quality_v1", models.MetricMultiTurnTrajectoryQualityV1, false},
		{"multi_turn_tool_use_quality_v1", models.MetricMultiTurnToolUseQualityV1, false},

		// Deterministic metrics (no external client).
		{"tool_trajectory_avg_score", models.MetricToolTrajectoryAvgScore, false},
		{"response_match_score", models.MetricResponseMatchScore, false},

		// Sentinels.
		{"empty", "", false},
		{"unknown", "made_up_metric", false},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isJudgeMetric(tc.in); got != tc.want {
				t.Errorf("isJudgeMetric(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestProbeJudgeModel(t *testing.T) {
	t.Parallel()

	safetyPro, err := SafetyV1(0.8, "gemini-2.5-pro")
	if err != nil {
		t.Fatalf("SafetyV1: %v", err)
	}
	rubricFlash, err := RubricBasedFinalResponseQualityV1(0.7, nil, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("RubricBasedFinalResponseQualityV1: %v", err)
	}
	rubricPro, err := RubricBasedFinalResponseQualityV1(0.7, nil, "gemini-2.5-pro")
	if err != nil {
		t.Fatalf("RubricBasedFinalResponseQualityV1: %v", err)
	}
	rubricNoModel, err := RubricBasedFinalResponseQualityV1(0.7, nil, "")
	if err != nil {
		t.Fatalf("RubricBasedFinalResponseQualityV1: %v", err)
	}

	tt := []struct {
		name string
		in   []models.EvalMetric
		want string
	}{
		{
			name: "empty input",
			in:   nil,
			want: "",
		},
		{
			name: "single rubric-based judge metric",
			in:   []models.EvalMetric{rubricFlash},
			want: "gemini-2.5-flash",
		},
		{
			name: "first non-empty wins in slice order",
			in:   []models.EvalMetric{rubricFlash, rubricPro},
			want: "gemini-2.5-flash",
		},
		{
			name: "leading empty judge model is skipped",
			in:   []models.EvalMetric{rubricNoModel, rubricPro},
			want: "gemini-2.5-pro",
		},
		{
			name: "SafetyV1 (Vertex, not judge-classified) still contributes its judgeModel",
			in:   []models.EvalMetric{safetyPro},
			want: "gemini-2.5-pro",
		},
		{
			name: "non-judge-only slice",
			in:   []models.EvalMetric{ResponseMatchScore(0.7)},
			want: "",
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := probeJudgeModel(tc.in); got != tc.want {
				t.Errorf("probeJudgeModel(...) = %q, want %q", got, tc.want)
			}
		})
	}
}
