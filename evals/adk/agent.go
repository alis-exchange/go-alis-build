package adk

import (
	"encoding/json"

	"go.alis.build/adk/launchers/evals/evaluation/models"
)

// defaultPathPrefix is the path segment where the ADK evals sublauncher mounts
// its HTTP handlers when callers do not override PathPrefix.
const defaultPathPrefix = "/api"

// Agent configures a deployed ADK agent for lazy eval-set discovery and scoring.
type Agent struct {
	BaseURL         string
	PathPrefix      string // default "/api"
	AppName         string
	DefaultMetrics  []models.EvalMetric
	MetricOverrides map[string][]models.EvalMetric // by eval_set_id
	IncludeEvalSet  func(id string) bool           // nil => include all

	// JudgeModel is the caller-declared LLM-as-judge model used by this
	// agent's LLM-as-judge metrics (e.g. "gemini-2.5-pro"). When
	// non-empty it is authoritative for the JudgeInfo.model field on
	// the emitted [alis.evals.v1.AgentEvalResults]. When empty, the
	// framework falls back to parsing the first non-empty
	// judgeModelOptions.judgeModel found on the caller-supplied metric
	// criteria (in slice declaration order across DefaultMetrics or
	// MetricOverrides[setID], via probeJudgeModel).
	//
	// If your metrics mix judge models, declare the primary one here —
	// AgentEvalResults.JudgeInfo carries only a single model string, so
	// the auto-fallback picks the first non-empty match, which may not
	// reflect your intent when the metrics are heterogeneous.
	//
	// adk-python's JudgeModelOptions.judge_model defaults to
	// "gemini-2.5-flash" when not set. The Go helpers in this package
	// ([SafetyV1], [RubricBasedFinalResponseQualityV1], ...) require
	// the caller to pass a model explicitly. If you rely on the ADK
	// backend's implicit default and do not set JudgeModel here,
	// JudgeInfo.model on the wire will be empty even though real judge
	// calls happened. Set it explicitly for observability.
	JudgeModel string

	// JudgeModelVersion is the caller-declared model version (e.g.
	// "2025-06-05"). Optional; no fallback source is available — the
	// ADK criterion carries no version field.
	JudgeModelVersion string
}

// MetricsFor returns metrics for an eval set, applying overrides when configured.
func (a Agent) MetricsFor(setID string) []models.EvalMetric {
	if m, ok := a.MetricOverrides[setID]; ok {
		return m
	}
	return a.DefaultMetrics
}

// pathPrefix returns the effective sublauncher path prefix, substituting
// defaultPathPrefix when PathPrefix is empty.
func (a Agent) pathPrefix() string {
	if a.PathPrefix == "" {
		return defaultPathPrefix
	}
	return a.PathPrefix
}

// ResponseMatchScore returns a response_match_score metric with the given threshold.
func ResponseMatchScore(threshold float64) models.EvalMetric {
	return models.EvalMetric{
		MetricName: models.MetricResponseMatchScore,
		Threshold:  threshold,
	}
}

// SafetyV1 returns a safety_v1 LLM-as-judge metric.
func SafetyV1(threshold float64, judgeModel string) (models.EvalMetric, error) {
	return metricFromJSON(map[string]any{
		"metricName": models.MetricSafetyV1,
		"threshold":  threshold,
		"criterion": map[string]any{
			"threshold": threshold,
			"judgeModelOptions": map[string]any{
				"judgeModel": judgeModel,
			},
		},
	})
}

// RubricBasedFinalResponseQualityV1 returns a rubric-based final response quality metric.
func RubricBasedFinalResponseQualityV1(threshold float64, rubrics []models.Rubric, judgeModel string) (models.EvalMetric, error) {
	return metricFromJSON(map[string]any{
		"metricName": models.MetricRubricBasedFinalResponseQualityV1,
		"threshold":  threshold,
		"criterion": map[string]any{
			"threshold": threshold,
			"rubrics":   rubrics,
			"judgeModelOptions": map[string]any{
				"judgeModel": judgeModel,
			},
		},
	})
}

// metricFromJSON round-trips v through JSON so helper constructors can build
// models.EvalMetric values without duplicating the launcher's wire shape.
func metricFromJSON(v any) (models.EvalMetric, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return models.EvalMetric{}, ErrEncodeMetric{Err: err}
	}
	var m models.EvalMetric
	if err := json.Unmarshal(raw, &m); err != nil {
		return models.EvalMetric{}, ErrDecodeMetric{Err: err}
	}
	return m, nil
}
