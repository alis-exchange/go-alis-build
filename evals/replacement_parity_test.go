package evals

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/internal/paritytest"
	"go.alis.build/validation"
	"google.golang.org/protobuf/proto"
)

func TestTypedSuiteRun_matchesP0BranchParity(t *testing.T) {
	tests := []struct {
		name  string
		build func(*testing.T) *evalspb.Run
		want  *evalspb.Run
	}{
		{
			name:  "integration",
			build: buildIntegrationParityRun,
			want:  paritytest.IntegrationBaselineRun(),
		},
		{
			name:  "agent",
			build: buildAgentParityRun,
			want:  paritytest.AgentBaselineRun(),
		},
		{
			name:  "load",
			build: buildLoadParityRun,
			want:  paritytest.LoadBaselineRun(),
		},
		{
			name:  "infra_observation",
			build: buildInfraObservationParityRun,
			want:  paritytest.InfraObservationBaselineRun(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := paritytest.NormalizeRun(tc.build(t), paritytest.DefaultFixedRunMeta)
			want := tc.want
			if tc.name == "integration" {
				got = normalizeIntegrationValidatorMessages(got)
				want = normalizeIntegrationValidatorMessages(want)
			}
			assertDeterministicProtoEqual(t, got, want)
		})
	}
}

func buildIntegrationParityRun(t *testing.T) *evalspb.Run {
	t.Helper()

	meta := paritytest.DefaultFixedRunMeta
	wantCases := paritytest.IntegrationBaselineRun().GetIntegrationTest().GetCases()
	ctx, cancel := context.WithCancel(context.Background())
	var run *evalspb.Run
	var err error
	withNowSequence(t, []time.Time{
		meta.StartTime,
		meta.StartTime,
		meta.StartTime.Add(wantCases[0].GetDuration().AsDuration()),
		meta.StartTime.Add(wantCases[0].GetDuration().AsDuration()),
		meta.StartTime.Add(wantCases[0].GetDuration().AsDuration()).Add(wantCases[1].GetDuration().AsDuration()),
		meta.EndTime,
	}, func() {
		run, err = NewIntegrationSuite("checkout-regression").
			AddCase("creates-order", func(_ context.Context, v *validation.Validator) {
				addChecksAsValidatorRules(v, wantCases[0].GetChecks())
			}).
			AddCase("latency-budget", func(_ context.Context, v *validation.Validator) {
				addChecksAsValidatorRules(v, wantCases[1].GetChecks())
				cancel()
			}).
			AddCase("cancelled-case", func(context.Context, *validation.Validator) {
				t.Fatal("cancelled case unexpectedly started")
			}).
			Run(ctx, parityRunOptions()...)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	return run
}

func buildAgentParityRun(t *testing.T) *evalspb.Run {
	t.Helper()

	meta := paritytest.DefaultFixedRunMeta
	want := paritytest.AgentBaselineRun().GetAgentEval()
	wantCases := want.GetCases()
	ctx, cancel := context.WithCancel(context.Background())
	modelVersion := want.GetJudge().GetModelVersion()
	var run *evalspb.Run
	var err error
	withNowSequence(t, []time.Time{
		meta.StartTime,
		meta.StartTime,
		meta.StartTime.Add(wantCases[0].GetDuration().AsDuration()),
		meta.StartTime.Add(wantCases[0].GetDuration().AsDuration()),
		meta.StartTime.Add(wantCases[0].GetDuration().AsDuration()).Add(wantCases[1].GetDuration().AsDuration()),
		meta.EndTime,
	}, func() {
		run, err = NewAgentEvalSuite("assistant-quality").
			AddCase("answers-correctly", func(_ context.Context, r *AgentEvalResult) {
				r.SetSessionID(wantCases[0].GetSessionId())
				for _, metric := range wantCases[0].GetMetrics() {
					r.AddMetric(metric)
				}
				r.SetJudgeInfo(&evalspb.AgentEvalResults_JudgeInfo{
					Model:           want.GetJudge().GetModel(),
					ModelVersion:    &modelVersion,
					JudgeCallCount:  2,
					JudgeErrorCount: 0,
				})
			}).
			AddCase("cites-source", func(_ context.Context, r *AgentEvalResult) {
				r.SetSessionID(wantCases[1].GetSessionId())
				for _, metric := range wantCases[1].GetMetrics() {
					r.AddMetric(metric)
				}
				r.SetJudgeInfo(&evalspb.AgentEvalResults_JudgeInfo{
					Model:           want.GetJudge().GetModel(),
					ModelVersion:    &modelVersion,
					JudgeCallCount:  1,
					JudgeErrorCount: 0,
				})
				cancel()
			}).
			AddCase("cancelled-case", func(context.Context, *AgentEvalResult) {
				t.Fatal("cancelled case unexpectedly started")
			}).
			Run(ctx, parityRunOptions()...)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	return run
}

func buildLoadParityRun(t *testing.T) *evalspb.Run {
	t.Helper()

	meta := paritytest.DefaultFixedRunMeta
	wantCase := paritytest.LoadBaselineRun().GetLoadTest().GetCases()[0]
	var run *evalspb.Run
	var err error
	withNowSequence(t, []time.Time{meta.StartTime, meta.StartTime, meta.EndTime, meta.EndTime}, func() {
		run, err = NewLoadSuite("checkout-capacity").
			AddCase("steady-traffic", func(_ context.Context, r *LoadResult) {
				r.SetSummary(wantCase.GetSummary())
				for _, check := range wantCase.GetChecks() {
					r.AddSLOCheck(check)
				}
				for _, tag := range wantCase.GetTags() {
					r.AddTag(tag)
				}
				for _, snapshot := range wantCase.GetCloudRun() {
					r.AddCloudRunSnapshot(snapshot)
				}
				for _, snapshot := range wantCase.GetSpanner() {
					r.AddSpannerSnapshot(snapshot)
				}
				for _, check := range wantCase.GetInfraChecks() {
					r.AddInfraSLOCheck(check)
				}
			}).
			Run(context.Background(), parityRunOptions()...)
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return run
}

func buildInfraObservationParityRun(t *testing.T) *evalspb.Run {
	t.Helper()

	meta := paritytest.DefaultFixedRunMeta
	wantCases := paritytest.InfraObservationBaselineRun().GetInfraObservation().GetCases()
	var run *evalspb.Run
	var err error
	withNowSequence(t, []time.Time{meta.StartTime, meta.StartTime, meta.EndTime, meta.EndTime, meta.EndTime, meta.EndTime}, func() {
		run, err = NewInfraObservationSuite("checkout-runtime").
			AddCase("peak-window", func(_ context.Context, r *InfraObservationResult) {
				addObservationCase(r, wantCases[0])
			}).
			AddCase("partial-fetch", func(_ context.Context, r *InfraObservationResult) {
				addObservationCase(r, wantCases[1])
			}).
			Run(context.Background(), parityRunOptions()...)
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return run
}

func parityRunOptions() []RunOption {
	meta := paritytest.DefaultFixedRunMeta
	return []RunOption{
		WithMaxConcurrency(1),
		WithOperation(meta.Operation),
		WithBatchID(meta.BatchID),
		WithGoogleProjectID(meta.GoogleProjectID),
	}
}

func withNowSequence(t *testing.T, times []time.Time, fn func()) {
	t.Helper()
	oldNow := now
	t.Cleanup(func() { now = oldNow })
	idx := 0
	now = func() time.Time {
		if idx >= len(times) {
			return times[len(times)-1]
		}
		out := times[idx]
		idx++
		return out
	}
	fn()
}

func addChecksAsValidatorRules(v *validation.Validator, checks []*evalspb.IntegrationTestResults_Case_Check) {
	for _, check := range checks {
		v.Custom(check.GetId(), check.GetStatus() == evalspb.Status_PASSED)
	}
}

func addObservationCase(r *InfraObservationResult, c *evalspb.InfraObservationResults_Case) {
	if c.GetLookback() != nil || c.GetWindowStart() != nil || c.GetWindowEnd() != nil {
		r.SetWindow(c.GetLookback().AsDuration(), c.GetWindowStart().AsTime(), c.GetWindowEnd().AsTime())
	}
	for _, snapshot := range c.GetCloudRun() {
		r.AddCloudRunSnapshot(snapshot)
	}
	for _, snapshot := range c.GetSpanner() {
		r.AddSpannerSnapshot(snapshot)
	}
	for _, check := range c.GetInfraChecks() {
		r.AddSLOCheck(check)
	}
}

func normalizeIntegrationValidatorMessages(run *evalspb.Run) *evalspb.Run {
	out := proto.Clone(run).(*evalspb.Run)
	for _, c := range out.GetIntegrationTest().GetCases() {
		for _, check := range c.GetChecks() {
			if check.GetStatus() == evalspb.Status_FAILED {
				check.Message = check.GetId()
			}
		}
	}
	return out
}

func assertDeterministicProtoEqual(t *testing.T, got, want *evalspb.Run) {
	t.Helper()
	gotBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(got)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	wantBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(want)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	if !bytes.Equal(gotBytes, wantBytes) {
		t.Fatalf("typed suite parity mismatch\n got: %v\nwant: %v", got, want)
	}
}
