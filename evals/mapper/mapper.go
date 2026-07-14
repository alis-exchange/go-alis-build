package mapper

import (
	"os"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/internal/result"
	"go.alis.build/evals/runner"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// IntegrationRun maps a completed suite to a wire Run for integration tests.
func IntegrationRun(sr execution.SuiteResult, operation, runID, batchID string) *evalspb.Run {
	now := timestamppb.Now()
	run := &evalspb.Run{
		Name:       "runs/" + runID,
		Type:       evalspb.Run_INTEGRATION_TEST,
		Status:     runner.RollupSuiteStatus(sr),
		StartTime:  timestamppb.New(sr.StartTime),
		EndTime:    timestamppb.New(sr.EndTime),
		Operation:  operation,
		CreateTime: now,
		Data: &evalspb.Run_IntegrationTest{
			IntegrationTest: integrationData(sr),
		},
		GoogleProjectId: os.Getenv("ALIS_OS_PROJECT"),
	}
	if batchID != "" {
		run.BatchId = &batchID
	}

	return run
}

// LoadRun maps a completed load suite result to a wire Run.
func LoadRun(sr execution.LoadSuiteResult, operation, runID, batchID string) *evalspb.Run {
	now := timestamppb.Now()
	run := &evalspb.Run{
		Name:       "runs/" + runID,
		Type:       evalspb.Run_LOAD_TEST,
		Status:     runner.RollupLoadSuiteStatus(sr),
		StartTime:  timestamppb.New(sr.StartTime),
		EndTime:    timestamppb.New(sr.EndTime),
		Operation:  operation,
		CreateTime: now,
		Data: &evalspb.Run_LoadTest{
			LoadTest: loadTestData(sr),
		},
		GoogleProjectId: os.Getenv("ALIS_OS_PROJECT"),
	}
	if batchID != "" {
		run.BatchId = &batchID
	}
	return run
}

func loadTestData(sr execution.LoadSuiteResult) *evalspb.LoadTestResults {
	cases := make([]*evalspb.LoadTestResults_Case, 0, len(sr.Cases))
	for _, c := range sr.Cases {
		cases = append(cases, &evalspb.LoadTestResults_Case{
			Id:      c.Name,
			Status:  c.Status,
			Tags:    c.Tags,
			Summary: mapLoadSummary(c.Summary),
			Checks:  mapSloChecks(c.Checks),
		})
	}
	return &evalspb.LoadTestResults{Cases: cases}
}

func mapLoadSummary(s execution.LoadCaseSummary) *evalspb.LoadTestResults_Summary {
	out := &evalspb.LoadTestResults_Summary{
		Mode:             s.Mode,
		TargetQps:        s.TargetQPS,
		Concurrency:      s.Concurrency,
		Duration:         durationpb.New(s.Duration),
		RequestCount:     s.RequestCount,
		ErrorCount:       s.ErrorCount,
		CheckPassedCount: s.CheckPassedCount,
		CheckFailedCount: s.CheckFailedCount,
		DroppedCount:     s.DroppedCount,
		ActualQps:        s.ActualQPS,
		Latency: &evalspb.LoadTestResults_LatencyPercentiles{
			P50Ms:  s.Latency.P50Ms,
			P95Ms:  s.Latency.P95Ms,
			P99Ms:  s.Latency.P99Ms,
			MinMs:  s.Latency.MinMs,
			MeanMs: s.Latency.MeanMs,
			MaxMs:  s.Latency.MaxMs,
		},
		ErrorsByCode:        s.ErrorsByCode,
		QpsStages:           mapLoadStages(s.QPSStages),
		ConcurrencyStages:   mapLoadStages(s.ConcurrencyStages),
		Stream:              mapLoadStreamSummary(s.Stream),
	}
	return out
}

func mapLoadStages(stages []execution.LoadStage) []*evalspb.LoadTestResults_LoadStage {
	if len(stages) == 0 {
		return nil
	}
	out := make([]*evalspb.LoadTestResults_LoadStage, len(stages))
	for i, s := range stages {
		out[i] = &evalspb.LoadTestResults_LoadStage{
			Duration: durationpb.New(s.Duration),
			Target:   s.Target,
		}
	}
	return out
}

func mapLoadStreamSummary(s *execution.LoadStreamSummary) *evalspb.LoadTestResults_StreamSummary {
	if s == nil {
		return nil
	}
	return &evalspb.LoadTestResults_StreamSummary{
		StreamCount:       s.StreamCount,
		MessagesSentTotal: s.MessagesSentTotal,
		Ttfb:              mapLoadLatency(s.TTFB),
		ResponseLatency:   mapLoadLatency(s.ResponseLatency),
		TotalDuration:     mapLoadLatency(s.TotalDuration),
	}
}

func mapLoadLatency(l execution.LoadLatency) *evalspb.LoadTestResults_LatencyPercentiles {
	if l == (execution.LoadLatency{}) {
		return nil
	}
	return &evalspb.LoadTestResults_LatencyPercentiles{
		P50Ms:  l.P50Ms,
		P95Ms:  l.P95Ms,
		P99Ms:  l.P99Ms,
		MinMs:  l.MinMs,
		MeanMs: l.MeanMs,
		MaxMs:  l.MaxMs,
	}
}

func mapSloChecks(checks []execution.SloCheckResult) []*evalspb.LoadTestResults_SloCheck {
	out := make([]*evalspb.LoadTestResults_SloCheck, len(checks))
	for i, c := range checks {
		out[i] = &evalspb.LoadTestResults_SloCheck{
			Id:       c.ID,
			Status:   c.Status,
			Message:  c.Message,
			Observed: c.Observed,
			Limit:    c.Limit,
			Unit:     c.Unit,
		}
	}
	return out
}

// AgentEvalRun maps a completed suite to a wire Run for agent evaluations.
func AgentEvalRun(sr execution.SuiteResult, operation, runID string) *evalspb.Run {
	now := timestamppb.Now()
	run := &evalspb.Run{
		Name:       "runs/" + runID,
		Type:       evalspb.Run_AGENT_EVAL,
		Status:     runner.RollupSuiteStatus(sr),
		StartTime:  timestamppb.New(sr.StartTime),
		EndTime:    timestamppb.New(sr.EndTime),
		Operation:  operation,
		CreateTime: now,
		Data: &evalspb.Run_AgentEval{
			AgentEval: agentEvalData(sr),
		},
		GoogleProjectId: os.Getenv("ALIS_OS_PROJECT"),
	}

	return run
}

func integrationData(sr execution.SuiteResult) *evalspb.IntegrationTestResults {
	cases := make([]*evalspb.IntegrationTestResults_Case, 0, len(sr.Cases))
	for _, c := range sr.Cases {
		cases = append(cases, &evalspb.IntegrationTestResults_Case{
			Id:       c.Name,
			Status:   c.Status,
			Checks:   mapChecks(c.Checks),
			Duration: durationpb.New(c.Duration),
		})
	}
	return &evalspb.IntegrationTestResults{
		Cases: cases,
	}
}

func agentEvalData(sr execution.SuiteResult) *evalspb.AgentEvalResults {
	cases := make([]*evalspb.AgentEvalResults_Case, 0, len(sr.Cases))
	for _, c := range sr.Cases {
		cases = append(cases, &evalspb.AgentEvalResults_Case{
			Id:        c.Name,
			Status:    c.Status,
			SessionId: c.SessionID,
			Metrics:   result.MetricsProto(c.Metrics),
			Duration:  durationpb.New(c.Duration),
		})
	}
	out := &evalspb.AgentEvalResults{Cases: cases}
	// Emit the JudgeInfo sidecar only when we have provenance to report
	// or a non-zero count. A fully zero suite (no judge model declared,
	// no judge calls counted) is a non-judge run and gets no sidecar.
	// This replaces the always-empty `Judge{}` emission that shipped in
	// evals v0.1.4, which was indistinguishable from an unpopulated
	// judge run on the wire.
	if !sr.Judge.IsZero() || sr.JudgeCallCount != 0 {
		out.Judge = &evalspb.AgentEvalResults_JudgeInfo{
			Model:          sr.Judge.Model,
			ModelVersion:   sr.Judge.ModelVersion,
			JudgeCallCount: sr.JudgeCallCount,
			// JudgeErrorCount is not derived here; see adk.JudgeContext
			// godoc for why NOT_EVALUATED is not classified as an
			// error. Callers with an out-of-band signal can populate
			// the field by post-processing the returned proto.
		}
	}
	return out
}

func mapChecks(checks []execution.Check) []*evalspb.IntegrationTestResults_Case_Check {
	out := make([]*evalspb.IntegrationTestResults_Case_Check, len(checks))
	for i, c := range checks {
		out[i] = &evalspb.IntegrationTestResults_Case_Check{
			Id:      c.ID,
			Status:  c.Status,
			Message: c.Message,
		}
	}
	return out
}

