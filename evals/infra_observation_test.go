package evals

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/proto"
)

func TestInfraObservationResult_buildsCaseWithWindowSnapshotsSLOsAndValidations(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC)
	end := start.Add(15 * time.Minute)
	cloud := &evalspb.CloudRunTargetSnapshot{Id: "checkout-api"}
	spanner := &evalspb.SpannerTargetSnapshot{Id: "orders-db"}
	check := &evalspb.InfraSloCheck{
		Kind:     evalspb.InfraKind_INFRA_KIND_CLOUD_RUN,
		TargetId: "checkout-api",
		CheckId:  "latency.p95",
		Status:   evalspb.Status_PASSED,
	}

	run, err := NewInfraObservationSuite("infra-builder").
		AddCase("peak", func(_ context.Context, r *InfraObservationResult) {
			if r.Validator() == nil {
				t.Fatal("Validator() returned nil")
			}
			r.SetWindow(10*time.Minute, start, end)
			r.AddCloudRunSnapshot(cloud)
			r.AddSpannerSnapshot(spanner)
			r.AddSLOCheck(check)
			r.Validator().Custom("monitoring query completed", true)
			r.Validator().Custom("all targets present", false)

			cloud.Id = "mutated"
			spanner.Id = "mutated"
			check.CheckId = "mutated"
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cases := run.GetInfraObservation().GetCases()
	if len(cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(cases))
	}
	c := cases[0]
	if c.GetId() != "infra-builder.peak" {
		t.Fatalf("case id = %q, want qualified id", c.GetId())
	}
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED from broken validation", c.GetStatus())
	}
	if got := c.GetLookback().AsDuration(); got != 10*time.Minute {
		t.Fatalf("lookback = %v, want 10m", got)
	}
	if got := c.GetWindowStart().AsTime(); !got.Equal(start) {
		t.Fatalf("window_start = %v, want %v", got, start)
	}
	if got := c.GetWindowEnd().AsTime(); !got.Equal(end) {
		t.Fatalf("window_end = %v, want %v", got, end)
	}
	if c.GetCloudRun()[0].GetId() != "checkout-api" {
		t.Fatalf("cloud_run id = %q, want cloned value", c.GetCloudRun()[0].GetId())
	}
	if c.GetSpanner()[0].GetId() != "orders-db" {
		t.Fatalf("spanner id = %q, want cloned value", c.GetSpanner()[0].GetId())
	}
	if c.GetInfraChecks()[0].GetCheckId() != "latency.p95" {
		t.Fatalf("infra check id = %q, want cloned value", c.GetInfraChecks()[0].GetCheckId())
	}
	gotValidations := validationTriples(c.GetValidations())
	wantValidations := []validationTriple{
		{id: "monitoring query completed", status: evalspb.Status_PASSED},
		{id: "all targets present", status: evalspb.Status_FAILED, message: "all targets present"},
	}
	if !reflect.DeepEqual(gotValidations, wantValidations) {
		t.Fatalf("validations = %+v, want %+v", gotValidations, wantValidations)
	}
	if run.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("run status = %v, want FAILED", run.GetStatus())
	}
}

func TestInfraObservationResult_failPreservesPartialData(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC)
	run, err := NewInfraObservationSuite("infra-fail").
		AddCase("partial", func(_ context.Context, r *InfraObservationResult) {
			r.SetWindow(time.Minute, start, start.Add(time.Minute))
			r.AddCloudRunSnapshot(&evalspb.CloudRunTargetSnapshot{Id: "checkout-api"})
			r.Fail(errors.New("monitoring unavailable"))
			r.Fail(nil)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetInfraObservation().GetCases()[0]
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED", c.GetStatus())
	}
	if c.GetLookback().AsDuration() != time.Minute || len(c.GetCloudRun()) != 1 {
		t.Fatalf("partial data not preserved: %+v", c)
	}
	want := []validationTriple{{id: "_evals.case", status: evalspb.Status_FAILED, message: "monitoring unavailable"}}
	if got := validationTriples(c.GetValidations()); !reflect.DeepEqual(got, want) {
		t.Fatalf("validations = %+v, want %+v", got, want)
	}
}

func TestInfraObservationResult_emptyCaseIsNotEvaluated(t *testing.T) {
	t.Parallel()

	run, err := NewInfraObservationSuite("infra-empty").
		AddCase("empty", noopObservationCase).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetInfraObservation().GetCases()[0]
	if c.GetStatus() != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("case status = %v, want NOT_EVALUATED", c.GetStatus())
	}
	if run.GetStatus() != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("run status = %v, want NOT_EVALUATED", run.GetStatus())
	}
}

func TestInfraObservationResult_sloFailureFailsCase(t *testing.T) {
	t.Parallel()

	run, err := NewInfraObservationSuite("infra-slo").
		AddCase("cpu", func(_ context.Context, r *InfraObservationResult) {
			r.AddSLOCheck(&evalspb.InfraSloCheck{CheckId: "cpu", Status: evalspb.Status_FAILED})
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetInfraObservation().GetCases()[0]
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED", c.GetStatus())
	}
}

func TestInfraObservationResult_nilAndDuplicateBuilderInputsBecomeValidations(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC)
	run, err := NewInfraObservationSuite("infra-builder-errors").
		AddCase("bad-inputs", func(_ context.Context, r *InfraObservationResult) {
			r.SetWindow(time.Minute, start, start.Add(time.Minute))
			r.SetWindow(2*time.Minute, start, start.Add(2*time.Minute))
			r.AddSLOCheck(nil)
			r.AddCloudRunSnapshot(nil)
			r.AddSpannerSnapshot(nil)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetInfraObservation().GetCases()[0]
	if got := c.GetLookback().AsDuration(); got != time.Minute {
		t.Fatalf("lookback = %v, want first value retained", got)
	}
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED", c.GetStatus())
	}
	gotMessages := []string{}
	for _, v := range c.GetValidations() {
		gotMessages = append(gotMessages, v.GetMessage())
	}
	wantMessages := []string{
		"evals: infra observation window already set",
		"evals: nil infra observation SLO check",
		"evals: nil infra observation Cloud Run snapshot",
		"evals: nil infra observation Spanner snapshot",
	}
	if !reflect.DeepEqual(gotMessages, wantMessages) {
		t.Fatalf("validation messages = %v, want %v", gotMessages, wantMessages)
	}
}

func TestInfraObservationResult_usesProtobufNativeValues(t *testing.T) {
	t.Parallel()

	cloud := &evalspb.CloudRunTargetSnapshot{Id: "checkout-api"}
	spanner := &evalspb.SpannerTargetSnapshot{Id: "orders-db"}
	check := &evalspb.InfraSloCheck{CheckId: "latency.p99", Status: evalspb.Status_PASSED}
	run, err := NewInfraObservationSuite("infra-native").
		AddCase("case", func(_ context.Context, r *InfraObservationResult) {
			r.AddCloudRunSnapshot(cloud)
			r.AddSpannerSnapshot(spanner)
			r.AddSLOCheck(check)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetInfraObservation().GetCases()[0]
	if !proto.Equal(c.GetCloudRun()[0], cloud) {
		t.Fatalf("cloud_run = %v, want protobuf-native value %v", c.GetCloudRun()[0], cloud)
	}
	if !proto.Equal(c.GetSpanner()[0], spanner) {
		t.Fatalf("spanner = %v, want protobuf-native value %v", c.GetSpanner()[0], spanner)
	}
	if !proto.Equal(c.GetInfraChecks()[0], check) {
		t.Fatalf("infra_check = %v, want protobuf-native value %v", c.GetInfraChecks()[0], check)
	}
}
