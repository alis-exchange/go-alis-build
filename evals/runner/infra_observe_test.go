package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/env"
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

type peakTrackingInfraCase struct {
	name  string
	delay time.Duration
	cur   *int32
	peak  *int32
}

type gatedInfraObserveCase struct {
	name    string
	started chan<- struct{}
	release <-chan struct{}
	hits    *atomic.Int32
}

func (c gatedInfraObserveCase) Name() string { return c.name }

func (c gatedInfraObserveCase) Lookback() (time.Duration, bool) { return 0, false }

func (c gatedInfraObserveCase) Run(context.Context, suite.InfraObserveCaseConfig) *execution.InfraObserveCaseResult {
	c.hits.Add(1)
	if c.started != nil {
		c.started <- struct{}{}
	}
	if c.release != nil {
		<-c.release
	}
	return &execution.InfraObserveCaseResult{Name: c.name, Status: evalspb.Status_PASSED}
}

func (c peakTrackingInfraCase) Name() string { return c.name }

func (c peakTrackingInfraCase) Lookback() (time.Duration, bool) { return 0, false }

func (c peakTrackingInfraCase) Run(context.Context, suite.InfraObserveCaseConfig) *execution.InfraObserveCaseResult {
	n := atomic.AddInt32(c.cur, 1)
	defer atomic.AddInt32(c.cur, -1)
	for {
		p := atomic.LoadInt32(c.peak)
		if n <= p {
			break
		}
		if atomic.CompareAndSwapInt32(c.peak, p, n) {
			break
		}
	}
	time.Sleep(c.delay)
	return &execution.InfraObserveCaseResult{Name: c.name, Status: evalspb.Status_PASSED}
}

func TestRunInfraObserveSuites_respectsCaseConcurrencyBound(t *testing.T) {
	t.Parallel()

	const (
		caseCount = 20
		bound     = 4
	)
	var peak, cur int32
	cases := make([]suite.InfraObserveCase, caseCount)
	for i := range cases {
		cases[i] = peakTrackingInfraCase{
			name:  fmt.Sprintf("peak.%d", i),
			delay: 30 * time.Millisecond,
			cur:   &cur,
			peak:  &peak,
		}
	}
	runs := []suite.InfraObserveSuiteRun{{
		Name:     "peak",
		Lookback: time.Minute,
		Cases:    cases,
	}}

	if _, err := New(WithInfraObserveConcurrency(bound)).RunInfraObserveSuites(
		context.Background(), runs, InfraObserveRunParams{}, nil, nil,
	); err != nil {
		t.Fatalf("RunInfraObserveSuites: %v", err)
	}
	if peak > bound {
		t.Fatalf("peak concurrent cases=%d, want <= %d", peak, bound)
	}
}

func TestRunInfraObserveSuites_progressPerCase(t *testing.T) {
	t.Parallel()

	const caseCount = 5
	cases := make([]suite.InfraObserveCase, caseCount)
	for i := range cases {
		cases[i] = slowInfraObserveCase{
			name:  fmt.Sprintf("c.%d", i),
			delay: 10 * time.Millisecond,
			hits:  new(int32),
		}
	}
	runs := []suite.InfraObserveSuiteRun{{
		Name:  "suite-a",
		Cases: cases,
	}}

	var progress [][2]int
	var progressMu sync.Mutex
	_, err := New().RunInfraObserveSuites(context.Background(), runs, InfraObserveRunParams{}, func(completed, total int) {
		progressMu.Lock()
		progress = append(progress, [2]int{completed, total})
		progressMu.Unlock()
	}, nil)
	if err != nil {
		t.Fatalf("RunInfraObserveSuites: %v", err)
	}
	if len(progress) != caseCount {
		t.Fatalf("progress calls=%d, want %d incremental updates", len(progress), caseCount)
	}
	for i, call := range progress {
		wantCompleted := i + 1
		if call[0] != wantCompleted || call[1] != caseCount {
			t.Fatalf("progress[%d]=%v, want [%d,%d]", i, call, wantCompleted, caseCount)
		}
	}
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
	got, err := r.RunInfraObserveSuites(context.Background(), runs, InfraObserveRunParams{}, nil, nil)
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

func TestRunInfraObserveSuites_cancelMidSuiteDoesNotStartQueuedCases(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var hits atomic.Int32
	runs := []suite.InfraObserveSuiteRun{{
		Name: "cancel",
		Cases: []suite.InfraObserveCase{
			gatedInfraObserveCase{name: "first", started: started, release: release, hits: &hits},
			gatedInfraObserveCase{name: "second", hits: &hits},
			gatedInfraObserveCase{name: "third", hits: &hits},
		},
	}}

	done := make(chan struct{})
	var got []execution.InfraObserveSuiteResult
	var err error
	go func() {
		got, err = New(WithInfraObserveConcurrency(1)).RunInfraObserveSuites(ctx, runs, InfraObserveRunParams{}, nil, nil)
		close(done)
	}()
	<-started
	cancel()
	close(release)
	<-done

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunInfraObserveSuites() error = %v, want context.Canceled", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("case bodies started = %d, want 1", hits.Load())
	}
	if len(got) != 1 || len(got[0].Cases) != 3 {
		t.Fatalf("results = %+v, want one suite with three case slots", got)
	}
	if got[0].Cases[1].Status != evalspb.Status_NOT_EVALUATED || got[0].Cases[2].Status != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("queued statuses = %v, %v; want NOT_EVALUATED", got[0].Cases[1].Status, got[0].Cases[2].Status)
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
	got, err := New().RunInfraObserveSuites(ctx, runs, InfraObserveRunParams{}, nil, nil)
	if err != nil {
		t.Fatalf("RunInfraObserveSuites: %v", err)
	}
	if client.Calls == 0 {
		t.Fatal("FakeMetricClient was not used; attachInfraClient ignored context client")
	}
	if len(got[0].Cases[0].CloudRun) != 1 {
		t.Fatalf("CloudRun snapshots=%d", len(got[0].Cases[0].CloudRun))
	}
	snap := got[0].Cases[0].CloudRun[0]
	if snap.GetFetchStatus() != evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK {
		t.Fatalf("FetchStatus=%v, want OK", snap.GetFetchStatus())
	}
	if snap.Metrics == nil || snap.Metrics.RequestCount != 3 {
		t.Fatalf("RequestCount=%v, want 3 from fake series", snap.Metrics)
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
	got, err := New().RunInfraObserveSuites(ctx, runs, InfraObserveRunParams{}, nil, nil)
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
	}, nil, nil)
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
	obs, _ := loadinfra.Observe(ctx, client, a.cloudRun, nil, window, false, 0)
	return &execution.InfraObserveCaseResult{
		Name:        a.name,
		Status:      evalspb.Status_PASSED,
		Lookback:    lookback,
		WindowStart: window.Start,
		WindowEnd:   window.End,
		CloudRun:    obs.CloudRun,
	}
}

type recordingInfraObserveSuiteHook struct {
	names []string
	errOn func(name string) error
}

func (h *recordingInfraObserveSuiteHook) hook() InfraObserveSuiteCompleteHook {
	return func(_ context.Context, sr execution.InfraObserveSuiteResult) error {
		h.names = append(h.names, sr.SuiteName)
		if h.errOn != nil {
			return h.errOn(sr.SuiteName)
		}
		return nil
	}
}

func TestInfraObserveSuiteCompleteHook_calledPerSuiteInOrder(t *testing.T) {
	t.Parallel()

	rec := &recordingInfraObserveSuiteHook{}
	runs := []suite.InfraObserveSuiteRun{
		{Name: "suite-a", Cases: []suite.InfraObserveCase{slowInfraObserveCase{name: "a", delay: time.Millisecond, hits: new(int32)}}},
		{Name: "suite-b", Cases: []suite.InfraObserveCase{slowInfraObserveCase{name: "b", delay: time.Millisecond, hits: new(int32)}}},
	}
	if _, err := New().RunInfraObserveSuites(context.Background(), runs, InfraObserveRunParams{}, nil, rec.hook()); err != nil {
		t.Fatalf("RunInfraObserveSuites: %v", err)
	}
	want := []string{"suite-a", "suite-b"}
	if len(rec.names) != len(want) {
		t.Fatalf("hook calls = %v, want %v", rec.names, want)
	}
}

func TestInfraObserveSuiteCompleteHook_envSetupFailureTimestamps(t *testing.T) {
	t.Parallel()

	envName := "infra-hook-env-fail-" + t.Name()
	setupErr := errors.New("env init failed")
	if err := env.Register(envName, env.WithSetup(func(context.Context) error { return setupErr })); err != nil {
		t.Fatalf("env.Register: %v", err)
	}

	cloud := loadinfra.CloudRunTarget{
		ID: "entry", Role: loadinfra.RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
	}
	s, err := suite.NewInfraObserveSuite("suite-a",
		suite.WithLookback(time.Minute),
		suite.WithInfraObserveEnvironment(envName),
		suite.WithInfraObserveCloudRunTargets(cloud),
	)
	if err != nil {
		t.Fatalf("NewInfraObserveSuite: %v", err)
	}
	if err := s.AddCase(slowInfraObserveCase{name: "a", delay: time.Millisecond, hits: new(int32)}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}

	rec := &recordingInfraObserveSuiteHook{}
	runs := []suite.InfraObserveSuiteRun{{
		Name:         s.Name(),
		Environments: s.Environments(),
		Lookback:     time.Minute,
		CloudRun:     []loadinfra.CloudRunTarget{cloud},
		Cases:        s.SelectInfraObserveCases(nil),
	}}
	got, err := New().RunInfraObserveSuites(context.Background(), runs, InfraObserveRunParams{}, nil, rec.hook())
	if err != nil {
		t.Fatalf("RunInfraObserveSuites: %v", err)
	}
	if len(rec.names) != 1 || rec.names[0] != "suite-a" {
		t.Fatalf("hook calls = %v, want [suite-a]", rec.names)
	}
	if got[0].StartTime.IsZero() || got[0].EndTime.IsZero() {
		t.Fatalf("env failure result missing timestamps: %+v", got[0])
	}
}
