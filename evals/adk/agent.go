package adk

import (
	"encoding/json"
	"fmt"

	"go.alis.build/adk/launchers/evals/evaluation/models"
)

const defaultPathPrefix = "/api"

// Agent configures a deployed ADK agent for lazy eval-set discovery and scoring.
type Agent struct {
	BaseURL         string
	PathPrefix      string // default "/api"
	AppName         string
	DefaultMetrics  []models.EvalMetric
	MetricOverrides map[string][]models.EvalMetric // by eval_set_id
	IncludeEvalSet  func(id string) bool           // nil => include all
}

// MetricsFor returns metrics for an eval set, applying overrides when configured.
func (a Agent) MetricsFor(setID string) []models.EvalMetric {
	if m, ok := a.MetricOverrides[setID]; ok {
		return m
	}
	return a.DefaultMetrics
}

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

func metricFromJSON(v any) (models.EvalMetric, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return models.EvalMetric{}, fmt.Errorf("adk: encode metric: %w", err)
	}
	var m models.EvalMetric
	if err := json.Unmarshal(raw, &m); err != nil {
		return models.EvalMetric{}, fmt.Errorf("adk: decode metric: %w", err)
	}
	return m, nil
}
