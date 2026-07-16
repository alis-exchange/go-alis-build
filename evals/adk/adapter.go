package adk

import (
	"fmt"
	"time"

	"go.alis.build/adk/launchers/evals/evaluation/models"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/internal/result"
	"google.golang.org/protobuf/types/known/durationpb"
)

// JudgeContext carries optional judge provenance for AgentEvalResults.
//
// Model and ModelVersion identify the LLM-as-judge configuration used
// to score the run. CallCount and ErrorCount are counters populated by
// callers who have visibility into the judge invocation cardinality
// (either derived by [SynthesizeJudgeContext] /
// [SynthesizeJudgeContextFromMetrics], or fed in from an out-of-band
// signal). See [alis.evals.v1.AgentEvalResults.JudgeInfo] for the wire
// shape.
//
// CallCount as populated by SynthesizeJudgeContext is a lower bound —
// it counts LLM-as-judge metric result entries in RunEvalResults, not
// per-invocation samples. See [go.alis.build/evals/execution.CaseResult.JudgeCallCount]
// for the same caveat, and the track spec's follow-up note about
// preserving criterion.judgeModelOptions on the launcher's
// EvalMetricResult for a more precise count.
//
// ErrorCount is not derived by this package: the framework does not
// attribute an EvalStatus of NOT_EVALUATED to "judge errored" because
// the two are not equivalent (NOT_EVALUATED can also mean the metric
// was skipped for reasons unrelated to the judge). Callers with an
// out-of-band error signal may set ErrorCount directly.
type JudgeContext struct {
	Model        string
	ModelVersion string
	CallCount    int64
	ErrorCount   int64
}

// isZero reports whether the JudgeContext carries no information that
// would populate any AgentEvalResults.JudgeInfo field.
func (j JudgeContext) isZero() bool {
	return j.Model == "" && j.ModelVersion == "" && j.CallCount == 0 && j.ErrorCount == 0
}

// CaseFromRunEvalResult maps one ADK case result to an internal case result.
//
// JudgeCallCount is populated as the count of LLM-as-judge metric result
// entries in r.OverallEvalMetricResults (per [isJudgeMetric]); see
// [execution.CaseResult.JudgeCallCount] for the lower-bound caveat.
func CaseFromRunEvalResult(r models.RunEvalResult, duration time.Duration) *execution.CaseResult {
	metrics := make([]execution.Metric, len(r.OverallEvalMetricResults))
	var judgeCalls int64
	for i, mr := range r.OverallEvalMetricResults {
		metrics[i] = metricFromADK(mr)
		if isJudgeMetric(mr.MetricName) {
			judgeCalls++
		}
	}
	return &execution.CaseResult{
		Name:           r.EvalID,
		Status:         statusFromADK(r.FinalEvalStatus),
		SessionID:      r.SessionID,
		Metrics:        metrics,
		Duration:       duration,
		JudgeCallCount: judgeCalls,
	}
}

// AgentEvalResultsFromRunEvalResults maps ADK results to a wire AgentEvalResults branch.
//
// The returned message has a non-nil Judge sidecar iff the JudgeContext
// is non-zero on any of Model, ModelVersion, CallCount, or ErrorCount.
// A fully zero-valued JudgeContext yields a nil Judge (no wire sidecar),
// which is the correct signal for a non-judge run.
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
// metric result entries in results. Model and modelVersion are the
// caller-declared provenance (authoritative; not probed from criteria —
// use [SynthesizeJudgeContextFromMetrics] for that). CallCount is the
// sum across all cases of entries whose MetricName is judge-backed per
// [isJudgeMetric]. ErrorCount is always 0 (see [JudgeContext]).
func SynthesizeJudgeContext(results []models.RunEvalResult, model, modelVersion string) JudgeContext {
	return JudgeContext{
		Model:        model,
		ModelVersion: modelVersion,
		CallCount:    countJudgeCalls(results),
	}
}

// SynthesizeJudgeContextFromMetrics is [SynthesizeJudgeContext] with an
// auto-derived Model: it always probes the given metrics via
// [probeJudgeModel] (walking them in slice declaration order and
// returning the first non-empty judgeModelOptions.judgeModel found).
// modelVersion has no fallback source and is used verbatim. Prefer this
// over [SynthesizeJudgeContext] when the caller does not have an
// authoritative model string but does have the metric list that was
// evaluated.
func SynthesizeJudgeContextFromMetrics(results []models.RunEvalResult, metrics []models.EvalMetric, modelVersion string) JudgeContext {
	return JudgeContext{
		Model:        probeJudgeModel(metrics),
		ModelVersion: modelVersion,
		CallCount:    countJudgeCalls(results),
	}
}

// countJudgeCalls returns the total number of LLM-as-judge metric result
// entries across all cases in results. Shared by
// [adk.Provider.Run] and the SynthesizeJudgeContext* helpers so both
// paths use identical counting rules.
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

// caseProtoFromRunEvalResult maps one ADK case into the wire AgentEvalResults
// case shape, reusing [CaseFromRunEvalResult] for status and metric conversion.
func caseProtoFromRunEvalResult(r models.RunEvalResult, duration time.Duration) *evalspb.AgentEvalResults_Case {
	internalCase := CaseFromRunEvalResult(r, duration)
	return &evalspb.AgentEvalResults_Case{
		Id:        internalCase.Name,
		Status:    internalCase.Status,
		SessionId: internalCase.SessionID,
		Duration:  durationProto(duration),
		Metrics:   result.MetricsProto(internalCase.Metrics),
	}
}

// metricFromADK converts one ADK EvalMetricResult into the internal execution
// metric model, including optional rubric breakdown and failure messages.
func metricFromADK(mr models.EvalMetricResult) execution.Metric {
	m := execution.Metric{
		ID:        mr.MetricName,
		Status:    statusFromADK(mr.EvalStatus),
		Threshold: mr.Threshold,
	}
	if mr.Score != nil {
		m.Score = new(*mr.Score)
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

// rubricScoreFromADK maps one ADK rubric score to execution.RubricScore,
// deriving pass/fail from the metric threshold when a score is present.
func rubricScoreFromADK(rs models.RubricScore, threshold float64) execution.RubricScore {
	out := execution.RubricScore{ID: rs.RubricID}
	if rs.Score != nil {
		out.Score = rs.Score
		out.Status = rubricStatus(*rs.Score, threshold)
	} else {
		out.Status = evalspb.Status_NOT_EVALUATED
	}
	if rs.Rationale != nil {
		out.Rationale = *rs.Rationale
	}
	return out
}

// rubricStatus compares a rubric score against the metric threshold.
func rubricStatus(score, threshold float64) evalspb.Status {
	if score >= threshold {
		return evalspb.Status_PASSED
	}
	return evalspb.Status_FAILED
}

// metricMessage synthesises a human-readable failure reason for wire output
// when the ADK result carries no explicit message.
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

// statusFromADK maps ADK EvalStatus values to alis.evals.v1.Status.
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

// durationProto returns nil for zero durations so optional proto fields stay unset.
func durationProto(d time.Duration) *durationpb.Duration {
	if d == 0 {
		return nil
	}
	return durationpb.New(d)
}
