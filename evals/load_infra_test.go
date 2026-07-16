package evals

import (
	"context"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/loadinfra"
)

func TestNewLoadSuite_infraDuplicateID(t *testing.T) {
	t.Parallel()
	_, err := NewLoadSuite("load",
		WithCloudRunTargets(CloudRunTarget{
			ID: "x", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "s",
		}),
		WithSpannerTargets(SpannerTarget{
			ID: "x", ProjectID: "p", InstanceID: "i", Location: "r", Database: "d",
		}),
	)
	if err == nil {
		t.Fatal("expected duplicate ID error")
	}
}

func TestLoadCaseAdapter_attachesInfraSnapshots(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute)
	fake := &fakeGenerator{
		metrics: &loadgen.Metrics{
			Duration:         time.Minute,
			RequestCount:     5,
			MeasurementStart: start,
			MeasurementEnd:   end,
		},
	}
	cloud := CloudRunTarget{
		ID: "entry", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
	}
	client := &loadinfra.FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{}}
	// Pre-populate request_count filter so fetch succeeds partially
	filter := `resource.type="cloud_run_revision" AND resource.labels.service_name="svc" AND resource.labels.location="r" AND metric.type="run.googleapis.com/request_count"`
	client.ByFilter[filter] = []*monitoringpb.TimeSeries{{
		Points: []*monitoringpb.Point{{
			Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_Int64Value{Int64Value: 9}},
		}},
	}}

	adapter := &loadCaseAdapter{
		name:      "case",
		target:    TransportTarget(func(context.Context) error { return nil }),
		generator: fake,
		cloudRun:  []loadinfra.CloudRunTarget{cloud},
	}
	ctx := loadinfra.WithClient(context.Background(), client)
	result := adapter.Run(ctx, evalspb.RunLoadTestRequest_MINIMAL, loadgen.Profile{QPS: 1, Concurrency: 1, Duration: time.Second})
	if len(result.CloudRun) != 1 {
		t.Fatalf("CloudRun snapshots=%d, want 1", len(result.CloudRun))
	}
	if result.CloudRun[0].Id != "entry" {
		t.Fatalf("snapshot id=%q", result.CloudRun[0].Id)
	}
	if result.CloudRun[0].Metrics.RequestCount != 9 {
		t.Fatalf("RequestCount=%d, want 9", result.CloudRun[0].Metrics.RequestCount)
	}
	if client.Calls == 0 {
		t.Fatal("FakeMetricClient not called")
	}
}

func TestLoadCaseAdapter_passesWhenInfraFetchUnavailable(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute)
	fake := &fakeGenerator{
		metrics: &loadgen.Metrics{
			Duration:         time.Minute,
			RequestCount:     5,
			MeasurementStart: start,
			MeasurementEnd:   end,
		},
	}
	cloud := CloudRunTarget{
		ID: "entry", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
	}
	client := &loadinfra.FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{}}

	adapter := &loadCaseAdapter{
		name:      "case",
		target:    TransportTarget(func(context.Context) error { return nil }),
		generator: fake,
		cloudRun:  []loadinfra.CloudRunTarget{cloud},
		slos:      []SLO{SLOErrorRate(0.1)},
	}
	ctx := loadinfra.WithClient(context.Background(), client)
	result := adapter.Run(ctx, evalspb.RunLoadTestRequest_MINIMAL, loadgen.Profile{QPS: 1, Concurrency: 1, Duration: time.Second})
	if result.Status != evalspb.Status_PASSED {
		t.Fatalf("status=%v, want PASSED when SLOs pass despite infra fetch failure", result.Status)
	}
	if len(result.CloudRun) != 1 {
		t.Fatalf("CloudRun snapshots=%d, want 1", len(result.CloudRun))
	}
	if result.CloudRun[0].FetchStatus != evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE {
		t.Fatalf("FetchStatus=%v, want UNAVAILABLE", result.CloudRun[0].FetchStatus)
	}
}

func TestLoadSuite_storesInfraTargets(t *testing.T) {
	t.Parallel()
	s, err := NewLoadSuite("load",
		WithCloudRunTargets(CloudRunTarget{
			ID: "entry", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
		}),
		WithSpannerTargets(SpannerTarget{
			ID: "orders", ProjectID: "p", InstanceID: "i", Location: "r", Database: "orders",
		}),
	)
	if err != nil {
		t.Fatalf("NewLoadSuite: %v", err)
	}
	if len(s.inner.CloudRunTargets()) != 1 || len(s.inner.SpannerTargets()) != 1 {
		t.Fatalf("cloud=%d spanner=%d", len(s.inner.CloudRunTargets()), len(s.inner.SpannerTargets()))
	}
}
