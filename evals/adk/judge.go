package adk

import (
	"go.alis.build/adk/launchers/evals/evaluation/models"
)

// judgeMetricNames is the exact set of metric names whose evaluator uses
// cfg.JudgeClient (LLM-as-judge), per
// go.alis.build/adk/launchers/evals/evaluation/metrics/registry.go —
// specifically the entries wired to newFinalResponseMatchV2Evaluator,
// newRubricBasedEvaluator, newHallucinationsEvaluator, and
// newPerTurnSimulatorEvaluator.
//
// Vertex Rapid Eval-backed metrics (safety_v1, response_evaluation_score,
// multi_turn_*) go through cfg.VertexClient and are intentionally excluded
// even though some of them still carry judgeModelOptions in their criterion
// (that model config is used by the Vertex evaluator, not an LLM judge).
//
// Kept in sync with the mirror map in
// go.alis.build/evals/report/log; if you change either, change both.
var judgeMetricNames = map[string]struct{}{
	models.MetricFinalResponseMatchV2:                    {},
	models.MetricRubricBasedFinalResponseQualityV1:       {},
	models.MetricRubricBasedToolUseQualityV1:             {},
	models.MetricRubricBasedMultiTurnTrajectoryQualityV1: {},
	models.MetricHallucinationsV1:                        {},
	models.MetricPerTurnUserSimulatorQualityV1:           {},
}

// isJudgeMetric reports whether the given metric name is an
// LLM-as-judge-backed metric per judgeMetricNames. Deterministic and
// Vertex Rapid Eval metrics return false.
func isJudgeMetric(name string) bool {
	_, ok := judgeMetricNames[name]
	return ok
}

// probeJudgeModel returns the first non-empty
// judgeModelOptions.judgeModel discovered by walking metrics in caller
// declaration order. It tries the LLM-as-judge, rubric-based, and
// hallucinations criterion variants (in that order) for each metric and
// skips entries whose criterion is any other variant or is missing.
// Returns "" when no judge model is found.
//
// Note: probeJudgeModel walks judge-model-carrying criteria regardless
// of whether the metric name is classified as judge-backed by
// [isJudgeMetric]. That means a Vertex Rapid Eval metric configured
// with a JudgeModelOptions (e.g. safety_v1) still contributes its
// judgeModel to the probe result. This is intentional: any metric a
// caller has annotated with a judgeModel is a valid provenance source
// for [alis.evals.v1.AgentEvalResults.JudgeInfo.model]; the judge-vs-
// non-judge distinction is [isJudgeMetric]'s job and applies to
// counting, not to model provenance.
func probeJudgeModel(metrics []models.EvalMetric) string {
	for _, m := range metrics {
		if v, ok := m.Criterion.AsLlmJudge(); ok {
			if v.JudgeModelOptions.JudgeModel != "" {
				return v.JudgeModelOptions.JudgeModel
			}
			continue
		}
		if v, ok := m.Criterion.AsRubrics(); ok {
			if v.JudgeModelOptions.JudgeModel != "" {
				return v.JudgeModelOptions.JudgeModel
			}
			continue
		}
		if v, ok := m.Criterion.AsHallucinations(); ok {
			if v.JudgeModelOptions.JudgeModel != "" {
				return v.JudgeModelOptions.JudgeModel
			}
			continue
		}
	}
	return ""
}
