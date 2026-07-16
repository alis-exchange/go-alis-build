package log

import (
	"context"
	"strconv"
	"strings"

	"go.alis.build/alog"
	evalspb "go.alis.build/common/alis/evals/v1"
)

// Reporter writes a one-line summary of each completed Run to alog. It is
// the default reporter for local development and any deployment that has no
// external sink (Pub/Sub, BigQuery, Spanner, etc.) wired up. Passing runs are
// logged at Info; failing runs at Warn so they stand out in Cloud Logging.
type Reporter struct{}

// ReportRun implements report.Reporter.
func (Reporter) ReportRun(ctx context.Context, run *evalspb.Run) error {
	if run == nil {
		return nil
	}
	summary := formatRun(run)
	if run.GetStatus() == evalspb.Status_PASSED {
		alog.Infof(ctx, "evals run: %s", summary)
	} else {
		alog.Warnf(ctx, "evals run: %s", summary)
	}
	if judgeDriftDetected(run) {
		alog.Warnf(ctx, "evals: judge metrics configured but judge_call_count=0 for run %q — check that Agent.JudgeModel or metric criterion carries judge provenance, or that the ADK launcher is actually invoking judges",
			run.GetName())
	}
	return nil
}

// judgeMetricNames mirrors go.alis.build/evals/adk.judgeMetricNames.
// Kept in sync manually to avoid importing the adk package into the
// report/log package (which would pull ADK http/client machinery into
// every deployment that only uses the log reporter). If you change this
// map, update the one in evals/adk/judge.go too.
var judgeMetricNames = map[string]struct{}{
	"final_response_match_v2":                       {},
	"rubric_based_final_response_quality_v1":        {},
	"rubric_based_tool_use_quality_v1":              {},
	"rubric_based_multi_turn_trajectory_quality_v1": {},
	"hallucinations_v1":                             {},
	"per_turn_user_simulator_quality_v1":            {},
}

// judgeDriftDetected reports whether the run is an agent eval run that
// has at least one judge-classified metric result but its JudgeInfo
// sidecar reports zero judge calls. This signals a wiring bug: either
// the caller forgot to declare provenance via Agent.JudgeModel /
// SynthesizeJudgeContextFromMetrics, or the ADK launcher isn't actually
// invoking judges even though judge metrics are configured.
func judgeDriftDetected(run *evalspb.Run) bool {
	if run.GetType() != evalspb.Run_AGENT_EVAL {
		return false
	}
	ae := run.GetAgentEval()
	if ae == nil {
		return false
	}
	if ae.GetJudge().GetJudgeCallCount() > 0 {
		return false
	}
	for _, c := range ae.GetCases() {
		for _, m := range c.GetMetrics() {
			if _, ok := judgeMetricNames[m.GetId()]; ok {
				return true
			}
		}
	}
	return false
}

// formatRun renders a compact, grep-friendly one-line summary for alog output.
func formatRun(run *evalspb.Run) string {
	var b strings.Builder
	b.WriteString(run.GetName())
	b.WriteString(" type=")
	b.WriteString(strings.TrimPrefix(run.GetType().String(), "TYPE_"))
	b.WriteString(" status=")
	b.WriteString(run.GetStatus().String())

	total, passed, failed, skipped := runCounts(run)
	if total > 0 {
		b.WriteString(" cases=")
		b.WriteString(strconv.Itoa(total))
		b.WriteString(" passed=")
		b.WriteString(strconv.Itoa(passed))
		if failed > 0 {
			b.WriteString(" failed=")
			b.WriteString(strconv.Itoa(failed))
		}
		if skipped > 0 {
			b.WriteString(" skipped=")
			b.WriteString(strconv.Itoa(skipped))
		}
	}

	if start, end := run.GetStartTime(), run.GetEndTime(); start != nil && end != nil {
		if d := end.AsTime().Sub(start.AsTime()); d > 0 {
			b.WriteString(" duration=")
			b.WriteString(d.String())
		}
	}
	if op := run.GetOperation(); op != "" {
		b.WriteString(" op=")
		b.WriteString(op)
	}
	if bid := run.GetBatchId(); bid != "" {
		b.WriteString(" batch=")
		b.WriteString(bid)
	}
	return b.String()
}

// runCounts tallies case outcomes from whichever oneof arm populates the run.
// Returns zeros when the run has no case-bearing payload.
func runCounts(run *evalspb.Run) (total, passed, failed, skipped int) {
	switch d := run.GetData().(type) {
	case *evalspb.Run_IntegrationTest:
		for _, c := range d.IntegrationTest.GetCases() {
			total++
			bumpCounts(c.GetStatus(), &passed, &failed, &skipped)
		}
	case *evalspb.Run_AgentEval:
		for _, c := range d.AgentEval.GetCases() {
			total++
			bumpCounts(c.GetStatus(), &passed, &failed, &skipped)
		}
	case *evalspb.Run_LoadTest:
		for _, c := range d.LoadTest.GetCases() {
			total++
			bumpCounts(c.GetStatus(), &passed, &failed, &skipped)
		}
	case *evalspb.Run_InfraObservation:
		for _, c := range d.InfraObservation.GetCases() {
			total++
			bumpCounts(c.GetStatus(), &passed, &failed, &skipped)
		}
	}
	return
}

// bumpCounts increments passed, failed, or skipped from a case status.
// Any non-PASSED, non-NOT_EVALUATED status counts as failed.
func bumpCounts(status evalspb.Status, passed, failed, skipped *int) {
	switch status {
	case evalspb.Status_PASSED:
		*passed++
	case evalspb.Status_NOT_EVALUATED:
		*skipped++
	default:
		*failed++
	}
}
