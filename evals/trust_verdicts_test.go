package evals

import (
	"context"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/loadinfra"
	"go.alis.build/evals/suite"
	"go.alis.build/evals/verdict"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLoadCase_transportErrorsFailWithoutErrorRateSLO(t *testing.T) {
	t.Parallel()

	fake := &fakeGenerator{
		metrics: &loadgen.Metrics{
			Duration:     time.Second,
			RequestCount: 10,
			ErrorCount:   10,
			Latency:      loadgen.LatencySummary{P99Ms: 5},
		},
	}
	s := MustNewLoadSuite("s")
	s.setGenerator(fake)
	s.MustLoadCase("c", TransportTarget(func(context.Context) error { return nil }), []SLO{SLOLatencyP99(50 * time.Millisecond)})

	result := s.Inner().Cases()[0].Run(context.Background(), evalspb.RunLoadTestRequest_MINIMAL,
		loadgen.Profile{QPS: 1, Concurrency: 1, Duration: time.Millisecond})
	if result.Status != evalspb.Status_FAILED {
		t.Fatalf("Status=%v, want FAILED", result.Status)
	}
	found := false
	for _, c := range result.Checks {
		if c.ID == verdict.IDTransportErrors {
			found = true
		}
	}
	if !found {
		t.Fatalf("checks=%+v, want %s", result.Checks, verdict.IDTransportErrors)
	}
}

func TestLoadCase_NoSLOsStillFailsOnTransportErrors(t *testing.T) {
	t.Parallel()

	fake := &fakeGenerator{
		metrics: &loadgen.Metrics{
			RequestCount: 5,
			ErrorCount:   5,
		},
	}
	s := MustNewLoadSuite("s")
	s.setGenerator(fake)
	s.MustLoadCase("c", TransportTarget(func(context.Context) error { return nil }), NoSLOs())

	result := s.Inner().Cases()[0].Run(context.Background(), evalspb.RunLoadTestRequest_MINIMAL,
		loadgen.Profile{QPS: 1, Concurrency: 1, Duration: time.Millisecond})
	if result.Status != evalspb.Status_FAILED {
		t.Fatalf("Status=%v, want FAILED", result.Status)
	}
}

func TestInfraObserveCaseAdapter_allFetchFailuresFailStandalone(t *testing.T) {
	t.Parallel()

	adapter := &infraObserveCaseAdapter{
		name: "hourly",
		cloudRun: []loadinfra.CloudRunTarget{{
			ID: "entry", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
		}},
	}
	cfg := suite.InfraObserveCaseConfig{
		CloudRun: []loadinfra.CloudRunTarget{{
			ID: "entry", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
		}},
		SuiteLookback: time.Minute,
	}
	ctx := loadinfra.WithClient(context.Background(), &loadinfra.FakeMetricClient{
		Err: status.Error(codes.PermissionDenied, "denied"),
	})
	result := adapter.Run(ctx, cfg)
	if result.Status != evalspb.Status_FAILED {
		t.Fatalf("Status=%v, want FAILED", result.Status)
	}
}

func TestLoadCaseAdapter_loadIntegratedInfraFetchDoesNotFailCase(t *testing.T) {
	t.Parallel()

	fake := &fakeGenerator{
		metrics: &loadgen.Metrics{Duration: time.Second, RequestCount: 1},
	}
	s := MustNewLoadSuite("s", WithCloudRunTargets(CloudRunTarget{
		ID: "entry", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
	}))
	s.setGenerator(fake)
	s.MustLoadCase("c", TransportTarget(func(context.Context) error { return nil }), NoSLOs())

	ctx := loadinfra.WithClient(context.Background(), &loadinfra.FakeMetricClient{
		Err: status.Error(codes.PermissionDenied, "denied"),
	})
	result := s.Inner().Cases()[0].Run(ctx, evalspb.RunLoadTestRequest_MINIMAL,
		loadgen.Profile{QPS: 1, Concurrency: 1, Duration: time.Millisecond})
	if result.Status != evalspb.Status_PASSED {
		t.Fatalf("load-integrated Status=%v, want PASSED", result.Status)
	}
	if len(result.CloudRun) == 0 {
		t.Fatal("expected infra snapshots on load case")
	}
}
