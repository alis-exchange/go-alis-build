package adk

import (
	"fmt"
	"time"

	"go.alis.build/adk/launchers/evals/evaluation/models"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"google.golang.org/protobuf/types/known/durationpb"
)

// JudgeContext carries optional judge provenance for AgentEvalResults.
type JudgeContext struct {
	Model        string
	ModelVersion string
}

// CaseFromRunEvalResult maps one ADK case result to an internal case result.
func CaseFromRunEvalResult(r models.RunEvalResult, duration time.Duration) *execution.CaseResult {
	metrics := make([]execution.Metric, len(r.OverallEvalMetricResults))
	for i, mr := range r.OverallEvalMetricResults {
		metrics[i] = metricFromADK(mr)
	}
	return &execution.CaseResult{
		Name:      r.EvalID,
		Status:    statusFromADK(r.FinalEvalStatus),
		SessionID: r.SessionID,
		Metrics:   metrics,
		Duration:  duration,
	}
}

// AgentEvalResultsFromRunEvalResults maps ADK results to a wire AgentEvalResults branch.
func AgentEvalResultsFromRunEvalResults(results []models.RunEvalResult, durations []time.Duration, judge JudgeContext) *evalspb.AgentEvalResults {
	cases := make([]*evalspb.AgentEvalResults_Case, len(results))
	for i, r := range results {
		d := time.Duration(0)
		if i < len(durations) {
			d = durations[i]
		}
		cases[i] = caseProtoFromRunEvalResult(r, d)
	}
	out := &evalspb.AgentEvalResults{Cases: cases}
	if judge.Model != "" || judge.ModelVersion != "" {
		out.Judge = &evalspb.AgentEvalResults_JudgeInfo{
			Model:        judge.Model,
			ModelVersion: judge.ModelVersion,
		}
	}
	return out
}

func caseProtoFromRunEvalResult(r models.RunEvalResult, duration time.Duration) *evalspb.AgentEvalResults_Case {
	internal := CaseFromRunEvalResult(r, duration)
	return &evalspb.AgentEvalResults_Case{
		Id:        internal.Name,
		Status:    internal.Status,
		SessionId: internal.SessionID,
		Duration:  durationProto(duration),
		Metrics:   metricsProto(internal.Metrics),
	}
}

func metricsProto(metrics []execution.Metric) []*evalspb.AgentEvalResults_Case_Metric {
	out := make([]*evalspb.AgentEvalResults_Case_Metric, len(metrics))
	for i, m := range metrics {
		wm := &evalspb.AgentEvalResults_Case_Metric{
			Id:        m.ID,
			Status:    m.Status,
			Threshold: m.Threshold,
			Message:   m.Message,
		}
		if m.Score != nil {
			wm.Score = m.Score
		}
		if len(m.Rubric) > 0 {
			wm.Rubric = make([]*evalspb.AgentEvalResults_Case_Metric_RubricScore, len(m.Rubric))
			for j, r := range m.Rubric {
				wr := &evalspb.AgentEvalResults_Case_Metric_RubricScore{
					Id:     r.ID,
					Status: r.Status,
				}
				if r.Score != nil {
					wr.Score = r.Score
				}
				wm.Rubric[j] = wr
			}
		}
		out[i] = wm
	}
	return out
}

func metricFromADK(mr models.EvalMetricResult) execution.Metric {
	m := execution.Metric{
		ID:        mr.MetricName,
		Status:    statusFromADK(mr.EvalStatus),
		Threshold: mr.Threshold,
	}
	if mr.Score != nil {
		score := *mr.Score
		m.Score = &score
	}
	m.Message = metricMessage(mr)
	if mr.Details != nil && len(mr.Details.RubricScores) > 0 {
		m.Rubric = make([]execution.RubricScore, len(mr.Details.RubricScores))
		for i, rs := range mr.Details.RubricScores {
			m.Rubric[i] = rubricScoreFromADK(rs, mr.Threshold)
		}
	}
	return m
}

func rubricScoreFromADK(rs models.RubricScore, threshold float64) execution.RubricScore {
	out := execution.RubricScore{ID: rs.RubricID}
	if rs.Score != nil {
		out.Score = rs.Score
		out.Status = rubricStatus(*rs.Score, threshold)
	} else {
		out.Status = evalspb.Status_NOT_EVALUATED
	}
	return out
}

func rubricStatus(score, threshold float64) evalspb.Status {
	if score >= threshold {
		return evalspb.Status_PASSED
	}
	return evalspb.Status_FAILED
}

func metricMessage(mr models.EvalMetricResult) string {
	switch mr.EvalStatus {
	case models.EvalStatusNotEvaluated:
		return "metric not evaluated"
	case models.EvalStatusFailed:
		if mr.Score != nil {
			return fmt.Sprintf("score %.4f below threshold %.4f", *mr.Score, mr.Threshold)
		}
		return "metric failed"
	default:
		return ""
	}
}

func statusFromADK(s models.EvalStatus) evalspb.Status {
	switch s {
	case models.EvalStatusPassed:
		return evalspb.Status_PASSED
	case models.EvalStatusFailed:
		return evalspb.Status_FAILED
	case models.EvalStatusNotEvaluated:
		return evalspb.Status_NOT_EVALUATED
	default:
		return evalspb.Status_STATUS_UNSPECIFIED
	}
}

func durationProto(d time.Duration) *durationpb.Duration {
	if d == 0 {
		return nil
	}
	return durationpb.New(d)
}
