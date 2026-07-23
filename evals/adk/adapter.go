package adk

import (
	"fmt"
	"time"

	"go.alis.build/adk/launchers/evals/evaluation/models"
	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

// JudgeContext carries optional judge provenance for AgentEvalResults.
//
// Model and ModelVersion identify the LLM-as-judge configuration used to score
// the run. CallCount and ErrorCount are counters populated by callers who have
// visibility into judge invocation cardinality.
//
// CallCount as populated by SynthesizeJudgeContext is a lower bound: it counts
// LLM-as-judge metric result entries in RunEvalResults, not per-invocation
// samples.
type JudgeContext struct {
	Model        string
	ModelVersion string
	CallCount    int64
	ErrorCount   int64
}

func (j JudgeContext) isZero() bool {
	return j.Model == "" && j.ModelVersion == "" && j.CallCount == 0 && j.ErrorCount == 0
}

// CaseFromRunEvalResult maps one ADK case result to the wire case result.
// suiteName qualifies the wire case id as "{suite}.{case}".
func CaseFromRunEvalResult(suiteName string, r models.RunEvalResult, duration time.Duration) *evalspb.AgentEvalResults_Case {
	metrics := make([]*evalspb.AgentEvalResults_Case_Metric, len(r.OverallEvalMetricResults))
	for i, mr := range r.OverallEvalMetricResults {
		metrics[i] = metricFromADK(mr)
	}
	return &evalspb.AgentEvalResults_Case{
		Id:        qualifiedADKCaseID(suiteName, r.EvalID),
		Status:    statusFromADK(r.FinalEvalStatus),
		SessionId: r.SessionID,
		Duration:  durationProto(duration),
		Metrics:   metrics,
	}
}

// AgentEvalResultsFromRunEvalResults maps ADK results to a wire AgentEvalResults branch.
// suiteName qualifies each emitted case ID as "{suite}.{case}".
func AgentEvalResultsFromRunEvalResults(suiteName string, results []models.RunEvalResult, durations []time.Duration, judge JudgeContext) *evalspb.AgentEvalResults {
	cases := make([]*evalspb.AgentEvalResults_Case, len(results))
	for i, r := range results {
		d := time.Duration(0)
		if i < len(durations) {
			d = durations[i]
		}
		cases[i] = CaseFromRunEvalResult(suiteName, r, d)
	}
	out := &evalspb.AgentEvalResults{Cases: cases}
	if !judge.isZero() {
		judgeInfo := &evalspb.AgentEvalResults_JudgeInfo{
			Model:           judge.Model,
			JudgeCallCount:  judge.CallCount,
			JudgeErrorCount: judge.ErrorCount,
		}
		if judge.ModelVersion != "" {
			judgeInfo.ModelVersion = new(judge.ModelVersion)
		}
		out.Judge = judgeInfo
	}
	return out
}

// SynthesizeJudgeContext derives a JudgeContext by counting LLM-as-judge
// metric result entries in results.
func SynthesizeJudgeContext(results []models.RunEvalResult, model, modelVersion string) JudgeContext {
	return JudgeContext{
		Model:        model,
		ModelVersion: modelVersion,
		CallCount:    countJudgeCalls(results),
	}
}

// SynthesizeJudgeContextFromMetrics is [SynthesizeJudgeContext] with an
// auto-derived Model from metric judge options.
func SynthesizeJudgeContextFromMetrics(results []models.RunEvalResult, metrics []models.EvalMetric, modelVersion string) JudgeContext {
	return JudgeContext{
		Model:        probeJudgeModel(metrics),
		ModelVersion: modelVersion,
		CallCount:    countJudgeCalls(results),
	}
}

func countJudgeCalls(results []models.RunEvalResult) int64 {
	var n int64
	for _, r := range results {
		for _, mr := range r.OverallEvalMetricResults {
			if isJudgeMetric(mr.MetricName) {
				n++
			}
		}
	}
	return n
}

func qualifiedADKCaseID(suiteName, caseID string) string {
	return suiteName + "." + caseID
}

func metricFromADK(mr models.EvalMetricResult) *evalspb.AgentEvalResults_Case_Metric {
	m := &evalspb.AgentEvalResults_Case_Metric{
		Id:     mr.MetricName,
		Status: statusFromADK(mr.EvalStatus),
	}
	if mr.Score != nil {
		m.Score = mr.Score
	}
	if mr.Threshold != 0 {
		m.Threshold = &mr.Threshold
	}
	m.Message = metricMessage(mr)
	if mr.Details != nil && len(mr.Details.RubricScores) > 0 {
		m.Rubric = make([]*evalspb.AgentEvalResults_Case_Metric_RubricScore, len(mr.Details.RubricScores))
		for i, rs := range mr.Details.RubricScores {
			m.Rubric[i] = rubricScoreFromADK(rs, mr.Threshold)
		}
	}
	return m
}

func rubricScoreFromADK(rs models.RubricScore, threshold float64) *evalspb.AgentEvalResults_Case_Metric_RubricScore {
	out := &evalspb.AgentEvalResults_Case_Metric_RubricScore{Id: rs.RubricID}
	if rs.Score != nil {
		out.Score = rs.Score
		out.Status = rubricStatus(*rs.Score, threshold)
	} else {
		out.Status = evalspb.Status_NOT_EVALUATED
	}
	if rs.Rationale != nil {
		out.Rationale = rs.Rationale
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
