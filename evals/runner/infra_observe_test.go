package runner

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/loadinfra"
	"go.alis.build/evals/suite"
)

type slowInfraObserveCase struct {
	name  string
	delay time.Duration
	hits  *int32
}

func (c slowInfraObserveCase) Name() string { return c.name }

func (c slowInfraObserveCase) Lookback() (time.Duration, bool) { return 0, false }

func (c slowInfraObserveCase) Run(ctx context.Context, cfg suite.InfraObserveCaseConfig) *execution.InfraObserveCaseResult {
	atomic.AddInt32(c.hits, 1)
	time.Sleep(c.delay)
	return &execution.InfraObserveCaseResult{Name: c.name, Status: evalspb.Status_PASSED}
}

func TestRunInfraObserveSuites_casesRunConcurrently(t *testing.T) {
	t.Parallel()
	var overlap int32
	delay := 80 * time.Millisecond
	cases := []suite.InfraObserveCase{
		slowInfraObserveCase{name: "peak.a", delay: delay, hits: &overlap},
		slowInfraObserveCase{name: "peak.b", delay: delay, hits: &overlap},
		slowInfraObserveCase{name: "peak.c", delay: delay, hits: &overlap},
	}
	runs := []suite.InfraObserveSuiteRun{{
		Name:     "peak",
		Lookback: time.Minute,
		Cases:    cases,
	}}

	start := time.Now()
	r := New()
	got, err := r.RunInfraObserveSuites(context.Background(), runs, InfraObserveRunParams{}, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("RunInfraObserveSuites: %v", err)
	}
	if len(got[0].Cases) != 3 {
		t.Fatalf("cases=%d", len(got[0].Cases))
	}
	if overlap != 3 {
		t.Fatalf("hits=%d, want 3", overlap)
	}
	// Sequential would take ~240ms; concurrent should finish closer to ~80ms.
	if elapsed > 200*time.Millisecond {
		t.Fatalf("elapsed=%v suggests sequential execution", elapsed)
	}
}

func TestRunInfraObserveSuites_withFakeClient(t *testing.T) {
	t.Parallel()
	client := &loadinfra.FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{}}
	cloud := loadinfra.CloudRunTarget{
		ID: "entry", Role: loadinfra.RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
	}
	filter := `resource.type="cloud_run_revision" AND resource.labels.service_name="svc" AND resource.labels.location="r" AND metric.type="run.googleapis.com/request_count"`
	client.ByFilter[filter] = []*monitoringpb.TimeSeries{{
		Points: []*monitoringpb.Point{{
			Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_Int64Value{Int64Value: 3}},
		}},
	}}

	adapter := &evalsInfraCase{
		name:     "peak.hourly",
		cloudRun: []loadinfra.CloudRunTarget{cloud},
	}
	runs := []suite.InfraObserveSuiteRun{{
		Name:     "peak",
		Lookback: time.Minute,
		CloudRun: []loadinfra.CloudRunTarget{cloud},
		Cases:    []suite.InfraObserveCase{adapter},
	}}

	ctx := loadinfra.WithClient(context.Background(), client)
	got, err := New().RunInfraObserveSuites(ctx, runs, InfraObserveRunParams{}, nil)
	if err != nil {
		t.Fatalf("RunInfraObserveSuites: %v", err)
	}
	if len(got[0].Cases[0].CloudRun) != 1 {
		t.Fatalf("CloudRun snapshots=%d", len(got[0].Cases[0].CloudRun))
	}
}

func TestRunInfraObserveSuites_qualifiedCaseName(t *testing.T) {
	t.Parallel()
	cloud := loadinfra.CloudRunTarget{
		ID: "entry", Role: loadinfra.RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
	}
	s, err := suite.NewInfraObserveSuite("peak",
		suite.WithLookback(time.Minute),
		suite.WithInfraObserveCloudRunTargets(cloud),
	)
	if err != nil {
		t.Fatalf("NewInfraObserveSuite: %v", err)
	}
	if err := s.AddCase(&evalsInfraCase{name: "hourly", cloudRun: []loadinfra.CloudRunTarget{cloud}}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}

	client := &loadinfra.FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{}}
	filter := `resource.type="cloud_run_revision" AND resource.labels.service_name="svc" AND resource.labels.location="r" AND metric.type="run.googleapis.com/request_count"`
	client.ByFilter[filter] = []*monitoringpb.TimeSeries{{
		Points: []*monitoringpb.Point{{
			Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_Int64Value{Int64Value: 1}},
		}},
	}}

	runs := []suite.InfraObserveSuiteRun{{
		Name:     s.Name(),
		Lookback: time.Minute,
		CloudRun: []loadinfra.CloudRunTarget{cloud},
		Cases:    s.SelectInfraObserveCases(nil),
	}}
	ctx := loadinfra.WithClient(context.Background(), client)
	got, err := New().RunInfraObserveSuites(ctx, runs, InfraObserveRunParams{}, nil)
	if err != nil {
		t.Fatalf("RunInfraObserveSuites: %v", err)
	}
	if got[0].Cases[0].Name != "peak.hourly" {
		t.Fatalf("case name=%q, want peak.hourly", got[0].Cases[0].Name)
	}
}

func TestRunInfraObserveSuites_requestLookback(t *testing.T) {
	t.Parallel()
	suiteLB := 30 * time.Minute
	requestLB := 5 * time.Minute
	cloud := loadinfra.CloudRunTarget{
		ID: "entry", Role: loadinfra.RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
	}
	cases := []suite.InfraObserveCase{lookbackRecordingInfraCase{
		name:     "peak.hourly",
		cloudRun: []loadinfra.CloudRunTarget{cloud},
	}}
	runs := []suite.InfraObserveSuiteRun{{
		Name:     "peak",
		Lookback: suiteLB,
		CloudRun: []loadinfra.CloudRunTarget{cloud},
		Cases:    cases,
	}}
	ctx := loadinfra.WithClient(context.Background(), &loadinfra.FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{}})
	got, err := New().RunInfraObserveSuites(ctx, runs, InfraObserveRunParams{
		RequestLookback: &requestLB,
	}, nil)
	if err != nil {
		t.Fatalf("RunInfraObserveSuites: %v", err)
	}
	if got[0].Cases[0].Lookback != requestLB {
		t.Fatalf("lookback=%v, want request override %v", got[0].Cases[0].Lookback, requestLB)
	}
}

type lookbackRecordingInfraCase struct {
	name     string
	cloudRun []loadinfra.CloudRunTarget
}

func (a lookbackRecordingInfraCase) Name() string { return a.name }

func (a lookbackRecordingInfraCase) Lookback() (time.Duration, bool) { return 0, false }

func (a lookbackRecordingInfraCase) Run(ctx context.Context, cfg suite.InfraObserveCaseConfig) *execution.InfraObserveCaseResult {
	lookback, err := suite.ResolveInfraObserveLookback(cfg.RequestLookback, 0, cfg.SuiteLookback, cfg.HasRequest, false)
	if err != nil {
		return &execution.InfraObserveCaseResult{Name: a.name, Status: evalspb.Status_FAILED}
	}
	return &execution.InfraObserveCaseResult{Name: a.name, Status: evalspb.Status_PASSED, Lookback: lookback}
}

// evalsInfraCase mirrors the public adapter shape for runner-level tests.
type evalsInfraCase struct {
	name     string
	cloudRun []loadinfra.CloudRunTarget
}

func (a *evalsInfraCase) Name() string { return a.name }

func (a *evalsInfraCase) Lookback() (time.Duration, bool) { return 0, false }

func (a *evalsInfraCase) Run(ctx context.Context, cfg suite.InfraObserveCaseConfig) *execution.InfraObserveCaseResult {
	lookback, err := suite.ResolveInfraObserveLookback(cfg.RequestLookback, 0, cfg.SuiteLookback, cfg.HasRequest, false)
	if err != nil {
		return &execution.InfraObserveCaseResult{Name: a.name, Status: evalspb.Status_FAILED}
	}
	client := loadinfra.ClientFromContext(ctx)
	settle := loadinfra.SettleDuration(len(a.cloudRun) > 0, false)
	window := loadinfra.WindowLookback(lookback, time.Now(), settle)
	obs, _ := loadinfra.Observe(ctx, client, a.cloudRun, nil, window, false)
	return &execution.InfraObserveCaseResult{
		Name:        a.name,
		Status:      evalspb.Status_PASSED,
		Lookback:    lookback,
		WindowStart: window.Start,
		WindowEnd:   window.End,
		CloudRun:    obs.CloudRun,
	}
}
