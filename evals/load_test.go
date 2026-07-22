package evals

import (
	"context"
	"errors"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/verdict"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeGenerator returns predetermined metrics/error, records the profile it
// was called with, and counts invocations.
type fakeGenerator struct {
	metrics    *loadgen.Metrics
	err        error
	lastProf   loadgen.Profile
	calls      int
	lastTarget loadgen.ResultTarget
}

func (f *fakeGenerator) Run(ctx context.Context, p loadgen.Profile, target loadgen.ResultTarget) (*loadgen.Metrics, error) {
	f.calls++
	f.lastProf = p
	f.lastTarget = target
	if target != nil && f.metrics != nil && f.metrics.RequestCount > 0 {
		for i := uint64(1); i <= uint64(f.metrics.RequestCount); i++ {
			target(ctx, loadgen.CallData{RequestNumber: i, WorkerID: 0})
		}
	}
	return f.metrics, f.err
}

func TestNewLoadSuite_ErrorsOnBadName(t *testing.T) {
	t.Parallel()

	if _, err := NewLoadSuite(""); err == nil {
		t.Fatal("empty name: expected error")
	}
	if _, err := NewLoadSuite("a.b"); err == nil {
		t.Fatal("dotted name: expected error")
	}
}

func TestLoadSuite_LoadCase_ErrorsOnBadInput(t *testing.T) {
	t.Parallel()

	s := MustNewLoadSuite("load-suite")
	if err := s.LoadCase("case", nil, nil); err == nil {
		t.Fatal("nil target: expected error")
	}
	target := TransportTarget(func(context.Context) error { return nil })
	if err := s.LoadCase("case", target, NoSLOs()); err != nil {
		t.Fatalf("first LoadCase: %v", err)
	}
	if err := s.LoadCase("case", target, NoSLOs()); err == nil {
		t.Fatal("duplicate: expected error")
	}
	if err := s.LoadCase("case", target, nil); err == nil {
		t.Fatal("empty SLO slice: expected error")
	}
	if err := s.LoadCase("a.b", target, NoSLOs()); err == nil {
		t.Fatal("dotted case name: expected error")
	}
	if err := s.LoadCase("", target, NoSLOs()); err == nil {
		t.Fatal("empty case name: expected error")
	}
}

func TestLoadSuite_LoadCase_duplicateSLOID(t *testing.T) {
	t.Parallel()

	s := MustNewLoadSuite("dup-slo-" + t.Name())
	target := TransportTarget(func(context.Context) error { return nil })
	err := s.LoadCase("c", target, []SLO{
		SLOLatencyP99(50 * time.Millisecond),
		SLOLatencyP99(100 * time.Millisecond),
	})
	var dup ErrDuplicateSLOID
	if !errors.As(err, &dup) {
		t.Fatalf("LoadCase() error = %v, want ErrDuplicateSLOID", err)
	}
}

func TestLoadSuite_LoadCase_dualDataSourcesRejected(t *testing.T) {
	t.Parallel()

	s := MustNewLoadSuite("dual-data-" + t.Name())
	target := TransportTarget(func(context.Context) error { return nil })
	err := s.LoadCase("c", target, NoSLOs(),
		WithLoadCaseData("a"),
		WithLoadCaseDataProvider(func(_ CallData) (any, error) { return "b", nil }),
	)
	var dual ErrDualLoadCaseData
	if !errors.As(err, &dual) {
		t.Fatalf("LoadCase() error = %v, want ErrDualLoadCaseData", err)
	}
}

func TestLoadSuite_MustLoadCase_PanicsOnNilTarget(t *testing.T) {
	t.Parallel()

	s := MustNewLoadSuite("must-load-suite")
	assertPanics(t, "nil target", func() { s.MustLoadCase("case", nil, nil) })
}

func TestLoadCaseAdapter_PassingRun(t *testing.T) {
	t.Parallel()

	fake := &fakeGenerator{
		metrics: &loadgen.Metrics{
			Duration:     time.Second,
			RequestCount: 100,
			ErrorCount:   0,
			ActualQPS:    100,
			Latency:      loadgen.LatencySummary{P50Ms: 5, P95Ms: 15, P99Ms: 20},
			ErrorsByCode: map[string]int64{},
		},
	}
	s := MustNewLoadSuite("s")
	s.setGenerator(fake)
	s.MustLoadCase("c",
		TransportTarget(func(context.Context) error { return nil }),
		[]SLO{SLOLatencyP99(50 * time.Millisecond), SLOErrorRate(0.01)},
	)

	inner := s.Inner()
	cases := inner.Cases()
	if len(cases) != 1 {
		t.Fatalf("len(cases)=%d, want 1", len(cases))
	}
	profile := loadgen.Profile{QPS: 100, Concurrency: 10, Duration: time.Second}
	result := cases[0].Run(context.Background(), evalspb.RunLoadTestRequest_MODERATE, profile)
	if result == nil {
		t.Fatal("nil result")
	}
	if result.Name != "s.c" {
		t.Fatalf("Name=%q, want s.c", result.Name)
	}
	if result.Status != evalspb.Status_PASSED {
		t.Fatalf("Status=%v, want PASSED", result.Status)
	}
	if len(result.Checks) != 2 {
		t.Fatalf("len(Checks)=%d, want 2 SLOs", len(result.Checks))
	}
	if result.Summary.Mode != evalspb.RunLoadTestRequest_MODERATE {
		t.Fatalf("Summary.Mode=%v", result.Summary.Mode)
	}
	if result.Summary.TargetQPS != 100 || result.Summary.Concurrency != 10 {
		t.Fatalf("Summary target/conc = %v/%d", result.Summary.TargetQPS, result.Summary.Concurrency)
	}
	if fake.calls != 1 || fake.lastProf.QPS != profile.QPS || fake.lastProf.Concurrency != profile.Concurrency {
		t.Fatalf("generator not called correctly: calls=%d prof=%+v", fake.calls, fake.lastProf)
	}
}

func TestLoadCaseAdapter_GeneratorErrorSurfacesAsFailed(t *testing.T) {
	t.Parallel()

	fake := &fakeGenerator{
		metrics: &loadgen.Metrics{},
		err:     errors.New("simulated"),
	}
	s := MustNewLoadSuite("s")
	s.setGenerator(fake)
	s.MustLoadCase("c", TransportTarget(func(context.Context) error { return nil }), NoSLOs())

	result := s.Inner().Cases()[0].Run(context.Background(),
		evalspb.RunLoadTestRequest_MINIMAL,
		loadgen.Profile{QPS: 1, Concurrency: 1, Duration: time.Millisecond},
	)
	if result.Status != evalspb.Status_FAILED {
		t.Fatalf("Status=%v, want FAILED", result.Status)
	}
	// Synthetic "generator" check must be present.
	found := false
	for _, c := range result.Checks {
		if c.ID == "generator" {
			found = true
			if c.Status != evalspb.Status_FAILED || c.Message == "" {
				t.Fatalf("generator check malformed: %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("missing synthetic generator check")
	}
}

func TestLoadCaseAdapter_FailingSLOFailsRun(t *testing.T) {
	t.Parallel()

	fake := &fakeGenerator{
		metrics: &loadgen.Metrics{
			RequestCount: 100,
			Latency:      loadgen.LatencySummary{P99Ms: 999},
		},
	}
	s := MustNewLoadSuite("s")
	s.setGenerator(fake)
	s.MustLoadCase("c", TransportTarget(func(context.Context) error { return nil }), []SLO{SLOLatencyP99(100 * time.Millisecond)})

	result := s.Inner().Cases()[0].Run(context.Background(),
		evalspb.RunLoadTestRequest_MODERATE,
		loadgen.Profile{QPS: 1, Concurrency: 1, Duration: time.Millisecond},
	)
	if result.Status != evalspb.Status_FAILED {
		t.Fatalf("Status=%v, want FAILED", result.Status)
	}
}

func TestResolveLoadProfile(t *testing.T) {
	t.Parallel()

	// Default when no override present.
	got, ok := ResolveLoadProfile(evalspb.RunLoadTestRequest_MINIMAL, nil)
	if !ok || got.QPS != 5 {
		t.Fatalf("default MINIMAL: got=%+v ok=%v", got, ok)
	}

	// Override wins for the specified mode; other modes keep defaults.
	overrides := map[evalspb.RunLoadTestRequest_Mode]loadgen.Profile{
		evalspb.RunLoadTestRequest_MODERATE: {QPS: 200, Concurrency: 40, Duration: 30 * time.Second},
	}
	got, ok = ResolveLoadProfile(evalspb.RunLoadTestRequest_MODERATE, overrides)
	if !ok || got.QPS != 200 || got.Concurrency != 40 {
		t.Fatalf("override MODERATE: got=%+v ok=%v", got, ok)
	}
	got, ok = ResolveLoadProfile(evalspb.RunLoadTestRequest_HIGH, overrides)
	if !ok || got.QPS != 400 {
		t.Fatalf("HIGH should keep default: got=%+v ok=%v", got, ok)
	}

	// UNSPECIFIED has no default and no override — unresolved.
	if _, ok := ResolveLoadProfile(evalspb.RunLoadTestRequest_MODE_UNSPECIFIED, nil); ok {
		t.Fatal("MODE_UNSPECIFIED should be unresolved")
	}
}

func TestWithLoadProfile_RejectsUnspecifiedMode(t *testing.T) {
	t.Parallel()

	_, err := NewLoadSuite("s", WithLoadProfile(evalspb.RunLoadTestRequest_MODE_UNSPECIFIED, loadgen.Profile{
		QPS: 1, Concurrency: 1, Duration: time.Second,
	}))
	if err == nil {
		t.Fatal("UNSPECIFIED mode override: expected error")
	}
}

func TestRegisterLoad_ErrorsOnNil(t *testing.T) {
	t.Parallel()
	if err := RegisterLoad(nil); err == nil {
		t.Fatal("nil suite: expected error")
	}
}

func TestLoadCase_TagsAndData(t *testing.T) {
	t.Parallel()

	var seenData []any
	var seenNums []uint64
	fake := &fakeGenerator{
		metrics: &loadgen.Metrics{Duration: time.Second, RequestCount: 2, CheckPassedCount: 2},
	}
	s := MustNewLoadSuite("s")
	s.setGenerator(fake)
	s.MustLoadCase("c",
		func(_ context.Context, d CallData) TargetResult {
			seenData = append(seenData, d.Data)
			seenNums = append(seenNums, d.RequestNumber)
			return TargetResult{}
		},
		NoSLOs(),
		WithLoadCaseTags(map[string]string{"model": "gpt-4"}),
		WithLoadCaseData("a", "b"),
	)

	result := s.Inner().Cases()[0].Run(context.Background(), evalspb.RunLoadTestRequest_MINIMAL,
		loadgen.Profile{QPS: 1, Concurrency: 1, Duration: time.Millisecond})
	if result.Tags["model"] != "gpt-4" {
		t.Fatalf("Tags=%v", result.Tags)
	}
	if len(seenData) != 2 || seenData[0] != "a" || seenData[1] != "b" {
		t.Fatalf("data rotation: %v", seenData)
	}
	if result.Summary.CheckPassedCount != 2 {
		t.Fatalf("CheckPassedCount=%d", result.Summary.CheckPassedCount)
	}
}

func TestLoadCase_CheckErrSeparateFromTransport(t *testing.T) {
	t.Parallel()

	fake := &fakeGenerator{
		metrics: &loadgen.Metrics{
			Duration:         time.Second,
			RequestCount:     1,
			CheckFailedCount: 1,
		},
	}
	s := MustNewLoadSuite("s")
	s.setGenerator(fake)
	s.MustLoadCase("c", func(context.Context, CallData) TargetResult {
		return TargetResult{CheckErr: errors.New("bad score")}
	}, NoSLOs())

	result := s.Inner().Cases()[0].Run(context.Background(), evalspb.RunLoadTestRequest_MINIMAL,
		loadgen.Profile{QPS: 1, Concurrency: 1, Duration: time.Millisecond})
	if result.Summary.ErrorCount != 0 {
		t.Fatalf("ErrorCount=%d, want 0", result.Summary.ErrorCount)
	}
	if result.Summary.CheckFailedCount != 1 {
		t.Fatalf("CheckFailedCount=%d, want 1", result.Summary.CheckFailedCount)
	}
	if result.Status != evalspb.Status_FAILED {
		t.Fatalf("Status=%v, want FAILED when CheckErr without SLO", result.Status)
	}
	foundChecks := false
	for _, c := range result.Checks {
		if c.ID == "checks" {
			foundChecks = true
		}
	}
	if !foundChecks {
		t.Fatal("expected synthetic checks SloCheckResult")
	}
}

func TestLoadCase_AbortOnSLOFailure(t *testing.T) {
	t.Parallel()

	s := MustNewLoadSuite("s")
	errTarget := func(context.Context, CallData) TargetResult {
		time.Sleep(5 * time.Millisecond)
		return TargetResult{TransportErr: status.Error(codes.Unavailable, "down")}
	}
	if err := s.LoadCase("c", errTarget, []SLO{SLOErrorRate(0)}); err != nil {
		t.Fatalf("LoadCase: %v", err)
	}

	ctx := loadgen.ContextWithAbortOnSLOFailure(context.Background())
	profile := loadgen.Profile{QPS: 100, Concurrency: 10, Duration: 30 * time.Second}
	start := time.Now()
	result := s.Inner().Cases()[0].Run(ctx, evalspb.RunLoadTestRequest_MINIMAL, profile)
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Fatalf("expected early abort, took %v", elapsed)
	}
	if result.Status != evalspb.Status_FAILED {
		t.Fatalf("status=%v, want FAILED", result.Status)
	}
	foundAborted := false
	for _, c := range result.Checks {
		if c.ID == verdict.IDAborted {
			foundAborted = true
		}
	}
	if !foundAborted {
		t.Fatalf("expected %s check, got %+v", verdict.IDAborted, result.Checks)
	}
}

func TestClientStreamTargetResult(t *testing.T) {
	t.Parallel()

	tr := ClientStreamTargetResult(ClientStreamResult[string]{
		SendDuration:    12 * time.Millisecond,
		ResponseLatency: 3 * time.Millisecond,
		TotalDuration:   20 * time.Millisecond,
		MessagesSent:    4,
	})
	if tr.Stream == nil {
		t.Fatal("Stream=nil")
	}
	if tr.Stream.MessagesSent != 4 || tr.Stream.SendDuration != 12*time.Millisecond {
		t.Fatalf("Stream=%+v", tr.Stream)
	}
}

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("%s: expected panic", name)
		}
	}()
	fn()
}
