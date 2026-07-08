package evals

import (
	"context"
	"errors"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/loadgen"
)

// fakeGenerator returns predetermined metrics/error, records the profile it
// was called with, and counts invocations.
type fakeGenerator struct {
	metrics    *loadgen.Metrics
	err        error
	lastProf   loadgen.Profile
	calls      int
	lastTarget loadgen.Target
}

func (f *fakeGenerator) Run(_ context.Context, p loadgen.Profile, target loadgen.Target) (*loadgen.Metrics, error) {
	f.calls++
	f.lastProf = p
	f.lastTarget = target
	return f.metrics, f.err
}

func TestNewLoadSuite_PanicsOnBadName(t *testing.T) {
	t.Parallel()

	assertPanics(t, "empty name", func() { NewLoadSuite("") })
	assertPanics(t, "dotted name", func() { NewLoadSuite("a.b") })
}

func TestLoadSuite_LoadCase_PanicsOnBadInput(t *testing.T) {
	t.Parallel()

	s := NewLoadSuite("load-suite")
	assertPanics(t, "nil target", func() { s.LoadCase("case", nil) })
	// Register once for duplicate test.
	s.LoadCase("case", func(context.Context) error { return nil })
	assertPanics(t, "duplicate", func() {
		s.LoadCase("case", func(context.Context) error { return nil })
	})
	assertPanics(t, "dotted case name", func() {
		s.LoadCase("a.b", func(context.Context) error { return nil })
	})
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
	s := NewLoadSuite("s")
	s.setGenerator(fake)
	s.LoadCase("c",
		func(context.Context) error { return nil },
		SLOLatencyP99(50*time.Millisecond),
		SLOErrorRate(0.01),
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
	if fake.calls != 1 || fake.lastProf != profile {
		t.Fatalf("generator not called correctly: calls=%d prof=%+v", fake.calls, fake.lastProf)
	}
}

func TestLoadCaseAdapter_GeneratorErrorSurfacesAsFailed(t *testing.T) {
	t.Parallel()

	fake := &fakeGenerator{
		metrics: &loadgen.Metrics{},
		err:     errors.New("simulated"),
	}
	s := NewLoadSuite("s")
	s.setGenerator(fake)
	s.LoadCase("c", func(context.Context) error { return nil })

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
	s := NewLoadSuite("s")
	s.setGenerator(fake)
	s.LoadCase("c", func(context.Context) error { return nil }, SLOLatencyP99(100*time.Millisecond))

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

	assertPanics(t, "UNSPECIFIED mode override", func() {
		NewLoadSuite("s", WithLoadProfile(evalspb.RunLoadTestRequest_MODE_UNSPECIFIED, loadgen.Profile{
			QPS: 1, Concurrency: 1, Duration: time.Second,
		}))
	})
}

func TestRegisterLoad_PanicsOnNil(t *testing.T) {
	t.Parallel()
	assertPanics(t, "nil suite", func() { RegisterLoad(nil) })
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
