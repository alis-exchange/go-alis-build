package evals

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestLoadResult_buildsCaseWithSummarySLOsSnapshotsTagsAndValidations(t *testing.T) {
	t.Parallel()

	summary := &evalspb.LoadTestResults_Summary{
		Mode:         evalspb.RunLoadTestRequest_MODERATE,
		TargetQps:    50,
		Concurrency:  5,
		Duration:     durationpb.New(30 * time.Second),
		RequestCount: 1500,
		Latency:      &evalspb.LatencyPercentiles{P95Ms: 120},
	}
	slo := &evalspb.LoadTestResults_SloCheck{
		Id:       "latency.p95",
		Status:   evalspb.Status_PASSED,
		Observed: 120,
		Limit:    200,
		Unit:     "ms",
	}
	cloud := &evalspb.CloudRunTargetSnapshot{}
	spanner := &evalspb.SpannerTargetSnapshot{}
	infra := &evalspb.InfraSloCheck{CheckId: "cpu", Status: evalspb.Status_PASSED}

	run, err := NewLoadSuite("load-builder").
		AddCase("steady", func(_ context.Context, r *LoadResult) {
			if r.Validator() == nil {
				t.Fatal("Validator() returned nil")
			}
			r.SetSummary(summary)
			r.AddSLOCheck(slo)
			r.AddTag(&evalspb.LoadTestResults_StringEntry{Key: "rpc", Value: "Checkout"})
			r.AddCloudRunSnapshot(cloud)
			r.AddSpannerSnapshot(spanner)
			r.AddInfraSLOCheck(infra)
			r.Validator().Custom("load reached steady state", true)
			r.Validator().Custom("error budget preserved", false)

			summary.TargetQps = 999
			slo.Id = "mutated"
			infra.CheckId = "mutated"
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cases := run.GetLoadTest().GetCases()
	if len(cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(cases))
	}
	c := cases[0]
	if c.GetId() != "load-builder.steady" {
		t.Fatalf("case id = %q, want qualified id", c.GetId())
	}
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED from broken validation", c.GetStatus())
	}
	if c.GetSummary().GetTargetQps() != 50 {
		t.Fatalf("summary target_qps = %v, want cloned value 50", c.GetSummary().GetTargetQps())
	}
	if c.GetChecks()[0].GetId() != "latency.p95" {
		t.Fatalf("slo id = %q, want cloned source value", c.GetChecks()[0].GetId())
	}
	if c.GetInfraChecks()[0].GetCheckId() != "cpu" {
		t.Fatalf("infra slo id = %q, want cloned source value", c.GetInfraChecks()[0].GetCheckId())
	}
	if len(c.GetTags()) != 1 || c.GetTags()[0].GetKey() != "rpc" {
		t.Fatalf("tags = %+v, want insertion order tag", c.GetTags())
	}
	if len(c.GetCloudRun()) != 1 || len(c.GetSpanner()) != 1 {
		t.Fatalf("snapshots missing: cloud=%d spanner=%d", len(c.GetCloudRun()), len(c.GetSpanner()))
	}
	gotValidations := validationTriples(c.GetValidations())
	wantValidations := []validationTriple{
		{id: "load reached steady state", status: evalspb.Status_PASSED},
		{id: "error budget preserved", status: evalspb.Status_FAILED, message: "error budget preserved"},
	}
	if !reflect.DeepEqual(gotValidations, wantValidations) {
		t.Fatalf("validations = %+v, want %+v", gotValidations, wantValidations)
	}
	if run.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("run status = %v, want FAILED", run.GetStatus())
	}
}

func TestLoadResult_failPreservesPartialData(t *testing.T) {
	t.Parallel()

	run, err := NewLoadSuite("load-fail").
		AddCase("partial", func(_ context.Context, r *LoadResult) {
			r.SetSummary(&evalspb.LoadTestResults_Summary{RequestCount: 10})
			r.AddSLOCheck(&evalspb.LoadTestResults_SloCheck{Id: "latency.p99", Status: evalspb.Status_PASSED})
			r.Fail(errors.New("generator stopped"))
			r.Fail(nil)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetLoadTest().GetCases()[0]
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED", c.GetStatus())
	}
	if c.GetSummary().GetRequestCount() != 10 || len(c.GetChecks()) != 1 {
		t.Fatalf("partial data not preserved: %+v", c)
	}
	want := []validationTriple{{id: "_evals.case", status: evalspb.Status_FAILED, message: "generator stopped"}}
	if got := validationTriples(c.GetValidations()); !reflect.DeepEqual(got, want) {
		t.Fatalf("validations = %+v, want %+v", got, want)
	}
}

func TestLoadResult_emptyCaseIsNotEvaluated(t *testing.T) {
	t.Parallel()

	run, err := NewLoadSuite("load-empty").
		AddCase("empty", noopLoadCase).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetLoadTest().GetCases()[0]
	if c.GetStatus() != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("case status = %v, want NOT_EVALUATED", c.GetStatus())
	}
	if run.GetStatus() != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("run status = %v, want NOT_EVALUATED", run.GetStatus())
	}
}

func TestLoadResult_sloAndInfraFailureFailCase(t *testing.T) {
	t.Parallel()

	run, err := NewLoadSuite("load-failing-checks").
		AddCase("load-slo", func(_ context.Context, r *LoadResult) {
			r.SetSummary(&evalspb.LoadTestResults_Summary{RequestCount: 1})
			r.AddSLOCheck(&evalspb.LoadTestResults_SloCheck{Id: "error_rate", Status: evalspb.Status_FAILED})
			r.AddInfraSLOCheck(&evalspb.InfraSloCheck{CheckId: "cpu", Status: evalspb.Status_FAILED})
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetLoadTest().GetCases()[0]
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED", c.GetStatus())
	}
}

func TestLoadResult_nilAndDuplicateBuilderInputsBecomeValidations(t *testing.T) {
	t.Parallel()

	run, err := NewLoadSuite("load-builder-errors").
		AddCase("bad-inputs", func(_ context.Context, r *LoadResult) {
			r.SetSummary(&evalspb.LoadTestResults_Summary{RequestCount: 1})
			r.SetSummary(&evalspb.LoadTestResults_Summary{RequestCount: 2})
			r.SetSummary(nil)
			r.AddSLOCheck(nil)
			r.AddTag(nil)
			r.AddCloudRunSnapshot(nil)
			r.AddSpannerSnapshot(nil)
			r.AddInfraSLOCheck(nil)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetLoadTest().GetCases()[0]
	if c.GetSummary().GetRequestCount() != 1 {
		t.Fatalf("summary request_count = %d, want first value retained", c.GetSummary().GetRequestCount())
	}
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED", c.GetStatus())
	}
	gotMessages := []string{}
	for _, v := range c.GetValidations() {
		gotMessages = append(gotMessages, v.GetMessage())
	}
	wantMessages := []string{
		"evals: load summary already set",
		"evals: nil load summary",
		"evals: nil load SLO check",
		"evals: nil load tag",
		"evals: nil load Cloud Run snapshot",
		"evals: nil load Spanner snapshot",
		"evals: nil load infra SLO check",
	}
	if !reflect.DeepEqual(gotMessages, wantMessages) {
		t.Fatalf("validation messages = %v, want %v", gotMessages, wantMessages)
	}
}

func TestLoadResult_usesProtobufNativeValues(t *testing.T) {
	t.Parallel()

	summary := &evalspb.LoadTestResults_Summary{
		Mode:         evalspb.RunLoadTestRequest_HIGH,
		TargetQps:    100,
		Concurrency:  10,
		Duration:     durationpb.New(time.Minute),
		RequestCount: 6000,
		Stream:       &evalspb.LoadTestResults_StreamSummary{StreamCount: 12},
	}
	check := &evalspb.LoadTestResults_SloCheck{Id: "latency.mean", Status: evalspb.Status_PASSED}
	run, err := NewLoadSuite("load-native").
		AddCase("case", func(_ context.Context, r *LoadResult) {
			r.SetSummary(summary)
			r.AddSLOCheck(check)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetLoadTest().GetCases()[0]
	if !proto.Equal(c.GetSummary(), summary) {
		t.Fatalf("summary = %v, want protobuf-native value %v", c.GetSummary(), summary)
	}
	if !proto.Equal(c.GetChecks()[0], check) {
		t.Fatalf("check = %v, want protobuf-native value %v", c.GetChecks()[0], check)
	}
}
