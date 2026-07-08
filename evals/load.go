package evals

import (
	"context"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/suite"
)

// Target is the shape of a load case body: perform exactly one request. A
// non-nil error marks the request as failed and increments the case's
// per-code error count.
type Target = loadgen.Target

// Profile re-exports [loadgen.Profile] so authors of load suites do not need
// to import the internal loadgen package.
type Profile = loadgen.Profile

// LoadSuite is the author-facing handle for a load-test suite. Cases within
// a load suite always run sequentially and the framework owns pacing,
// concurrency, warmup, and aggregation — case authors only supply a target
// function and its SLOs.
type LoadSuite struct {
	inner *suite.LoadSuite
	// generator is the loadgen used by every case adapter registered under
	// this suite. Exposed only for tests to substitute a fake generator; the
	// public API always uses [loadgen.New].
	generator loadgen.Generator
}

// LoadSuiteOption configures a LoadSuite at construction time.
type LoadSuiteOption interface {
	applyLoad(*suite.LoadSuite) error
}

type loadOption func(*suite.LoadSuite) error

func (o loadOption) applyLoad(s *suite.LoadSuite) error { return o(s) }

// WithLoadEnv declares one or more shared environments the load suite requires.
func WithLoadEnv(names ...string) LoadSuiteOption {
	return loadOption(func(s *suite.LoadSuite) error {
		return suite.WithLoadEnvironment(names...)(s)
	})
}

// WithLoadSetup registers an optional suite-level setup hook.
func WithLoadSetup(h suite.SuiteHook) LoadSuiteOption {
	return loadOption(func(s *suite.LoadSuite) error {
		return suite.WithLoadSetup(h)(s)
	})
}

// WithLoadTeardown registers an optional suite-level teardown hook.
func WithLoadTeardown(h suite.SuiteHook) LoadSuiteOption {
	return loadOption(func(s *suite.LoadSuite) error {
		return suite.WithLoadTeardown(h)(s)
	})
}

// WithLoadProfile overrides the framework default profile for a specific
// mode when this suite is run at that mode. Other modes keep their defaults.
func WithLoadProfile(mode evalspb.RunLoadTestRequest_Mode, p Profile) LoadSuiteOption {
	return loadOption(func(s *suite.LoadSuite) error {
		return suite.WithLoadProfileOverride(mode, p)(s)
	})
}

// NewLoadSuite constructs a load-test suite. Returns a typed error on
// invalid config (empty or dotted name, unknown environment, or a failing
// option). See the [suite] package for the typed error values.
func NewLoadSuite(name string, opts ...LoadSuiteOption) (*LoadSuite, error) {
	s, err := suite.NewLoadSuite(name)
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		if err := opt.applyLoad(s); err != nil {
			return nil, err
		}
	}
	return &LoadSuite{inner: s, generator: loadgen.New()}, nil
}

// MustNewLoadSuite is like [NewLoadSuite] but panics on error.
func MustNewLoadSuite(name string, opts ...LoadSuiteOption) *LoadSuite {
	s, err := NewLoadSuite(name, opts...)
	if err != nil {
		panic(err)
	}
	return s
}

// LoadCase registers a case under the suite. The target is invoked once per
// scheduled request during the load window. Any SLOs are evaluated against
// the aggregate metrics after the window closes. Returns a typed error
// on nil suite ([suite.ErrNilSuite]), nil target ([ErrNilTarget]), an
// invalid case name ([suite.ErrInvalidCaseName]), or a duplicate
// ([suite.ErrDuplicateCase]). Use [LoadSuite.MustLoadCase] for fluent
// chaining.
func (s *LoadSuite) LoadCase(name string, target Target, slos ...SLO) error {
	if s == nil {
		return suite.ErrNilSuite{}
	}
	if target == nil {
		return ErrNilTarget{Case: name}
	}
	adapter := &loadCaseAdapter{
		name:      name,
		target:    target,
		slos:      append([]SLO(nil), slos...),
		generator: s.generator,
	}
	return s.inner.AddCase(adapter)
}

// MustLoadCase is like [LoadSuite.LoadCase] but panics on error and
// returns the suite for fluent chaining.
func (s *LoadSuite) MustLoadCase(name string, target Target, slos ...SLO) *LoadSuite {
	if err := s.LoadCase(name, target, slos...); err != nil {
		panic(err)
	}
	return s
}

// Name returns the suite name.
func (s *LoadSuite) Name() string {
	if s == nil {
		return ""
	}
	return s.inner.Name()
}

// Inner exposes the underlying suite.LoadSuite for the registry to consume.
// Not intended for case authors.
func (s *LoadSuite) Inner() *suite.LoadSuite {
	if s == nil {
		return nil
	}
	return s.inner
}

// loadCaseAdapter bridges (target, slos) to the erased suite.LoadCase
// interface: run the generator with the caller-supplied profile, then
// evaluate every SLO against the returned metrics and assemble a
// LoadCaseResult.
type loadCaseAdapter struct {
	name      string
	target    Target
	slos      []SLO
	generator loadgen.Generator
}

func (a *loadCaseAdapter) Name() string { return a.name }

func (a *loadCaseAdapter) Run(ctx context.Context, mode evalspb.RunLoadTestRequest_Mode, profile loadgen.Profile) *execution.LoadCaseResult {
	m, err := a.generator.Run(ctx, profile, a.target)
	if m == nil {
		m = &loadgen.Metrics{}
	}
	result := &execution.LoadCaseResult{
		Name:    a.name,
		Summary: summaryFromMetrics(mode, profile, m),
	}
	if err != nil {
		// Generator failure (invalid profile, ctx cancelled) — surface as a
		// synthetic failed check so the case rolls up FAILED even if no SLOs
		// were declared. SLO checks still evaluate against whatever partial
		// metrics we got.
		result.Checks = append(result.Checks, execution.SloCheckResult{
			ID:      "generator",
			Status:  evalspb.Status_FAILED,
			Message: err.Error(),
			Unit:    "",
		})
	}
	result.Checks = append(result.Checks, evaluateSLOs(a.slos, m)...)
	result.Status = rollupLoadCaseStatus(result.Checks)
	return result
}

func rollupLoadCaseStatus(checks []execution.SloCheckResult) evalspb.Status {
	for _, c := range checks {
		if c.Status != evalspb.Status_PASSED {
			return evalspb.Status_FAILED
		}
	}
	return evalspb.Status_PASSED
}

func summaryFromMetrics(mode evalspb.RunLoadTestRequest_Mode, profile loadgen.Profile, m *loadgen.Metrics) execution.LoadCaseSummary {
	// duration reflects the measurement window as configured; the generator
	// preserves this on m.Duration (it does not shrink under saturation).
	dur := m.Duration
	if dur == 0 {
		dur = profile.Duration
	}
	return execution.LoadCaseSummary{
		Mode:         mode,
		TargetQPS:    profile.QPS,
		Concurrency:  int32(profile.Concurrency),
		Duration:     dur,
		RequestCount: m.RequestCount,
		ErrorCount:   m.ErrorCount,
		ActualQPS:    m.ActualQPS,
		Latency: execution.LoadLatency{
			P50Ms:  m.Latency.P50Ms,
			P95Ms:  m.Latency.P95Ms,
			P99Ms:  m.Latency.P99Ms,
			MinMs:  m.Latency.MinMs,
			MeanMs: m.Latency.MeanMs,
			MaxMs:  m.Latency.MaxMs,
		},
		ErrorsByCode: cloneErrorsByCode(m.ErrorsByCode),
	}
}

func cloneErrorsByCode(in map[string]int64) map[string]int64 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// setGenerator replaces the generator used by any subsequently-registered
// case. Package-internal seam for tests; not part of the public API.
func (s *LoadSuite) setGenerator(g loadgen.Generator) {
	if s != nil && g != nil {
		s.generator = g
	}
}
