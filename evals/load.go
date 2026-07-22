package evals

import (
	"context"
	"errors"
	"fmt"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/loadinfra"
	"go.alis.build/evals/suite"
	"go.alis.build/evals/verdict"
)

// ResultTarget executes exactly one load request.
type ResultTarget = loadgen.ResultTarget

// TransportTarget adapts a transport-only function to a [ResultTarget].
func TransportTarget(fn func(context.Context) error) ResultTarget {
	return loadgen.TransportTarget(fn)
}

// CallData is per-request context for load targets.
type CallData = loadgen.CallData

// TargetResult separates transport and semantic check outcomes.
type TargetResult = loadgen.TargetResult

// Profile re-exports [loadgen.Profile] so authors of load suites do not need
// to import the internal loadgen package.
type Profile = loadgen.Profile

// CloudRunTarget declares a Cloud Run scope for infrastructure observation.
type CloudRunTarget = loadinfra.CloudRunTarget

// SpannerTarget declares a Spanner scope for infrastructure observation.
type SpannerTarget = loadinfra.SpannerTarget

// Infra target role constants.
const (
	RoleEntry      = loadinfra.RoleEntry
	RoleDependency = loadinfra.RoleDependency
)

// LoadSuite is the author-facing handle for a load-test suite. Cases within
// a load suite always run sequentially and the framework owns pacing,
// concurrency, warmup, and aggregation — case authors only supply a target
// function and its SLOs.
type LoadSuite struct {
	inner *suite.LoadSuite // underlying suite registered with the runner
	// generator is the loadgen used by every case adapter registered under
	// this suite. Exposed only for tests to substitute a fake generator; the
	// public API always uses [loadgen.New].
	generator loadgen.Generator
}

// LoadSuiteOption configures a LoadSuite at construction time.
type LoadSuiteOption interface {
	applyLoad(*suite.LoadSuite) error
}

// loadOption is a functional [LoadSuiteOption] that mutates suite.LoadSuite.
type loadOption func(*suite.LoadSuite) error

// applyLoad invokes the option against a load suite.
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

// WithLoadContext installs a [ContextDecorator] on the load suite.
func WithLoadContext(fn ContextDecorator) LoadSuiteOption {
	return loadOption(func(s *suite.LoadSuite) error {
		return suite.WithLoadContext(fn)(s)
	})
}

// WithLoadStopOnFailure marks the suite so remaining load cases are recorded
// NOT_EVALUATED after the first non-PASSED case.
func WithLoadStopOnFailure() LoadSuiteOption {
	return loadOption(func(s *suite.LoadSuite) error {
		return suite.WithLoadStopOnFailure()(s)
	})
}

// WithCloudRunTargets declares Cloud Run infrastructure targets. Valid on load
// suites ([LoadSuiteOption]) and infra observation suites ([InfraObserveSuiteOption]).
func WithCloudRunTargets(targets ...CloudRunTarget) interface {
	LoadSuiteOption
	InfraObserveSuiteOption
} {
	return cloudRunTargetsOption{targets: targets}
}

// WithSpannerTargets declares Spanner infrastructure targets. Valid on load
// suites ([LoadSuiteOption]) and infra observation suites ([InfraObserveSuiteOption]).
func WithSpannerTargets(targets ...SpannerTarget) interface {
	LoadSuiteOption
	InfraObserveSuiteOption
} {
	return spannerTargetsOption{targets: targets}
}

// cloudRunTargetsOption carries Cloud Run infra targets for load and observe suites.
type cloudRunTargetsOption struct {
	// targets are copied into suite config at apply time.
	targets []loadinfra.CloudRunTarget
}

// applyLoad registers Cloud Run targets on a load suite.
func (o cloudRunTargetsOption) applyLoad(s *suite.LoadSuite) error {
	return suite.WithCloudRunTargets(o.targets...)(s)
}

// applyInfraObserve registers Cloud Run targets on an infra observation suite.
func (o cloudRunTargetsOption) applyInfraObserve(s *suite.InfraObserveSuite) error {
	return suite.WithInfraObserveCloudRunTargets(o.targets...)(s)
}

// spannerTargetsOption carries Spanner infra targets for load and observe suites.
type spannerTargetsOption struct {
	// targets are copied into suite config at apply time.
	targets []loadinfra.SpannerTarget
}

// applyLoad registers Spanner targets on a load suite.
func (o spannerTargetsOption) applyLoad(s *suite.LoadSuite) error {
	return suite.WithSpannerTargets(o.targets...)(s)
}

// applyInfraObserve registers Spanner targets on an infra observation suite.
func (o spannerTargetsOption) applyInfraObserve(s *suite.InfraObserveSuite) error {
	return suite.WithInfraObserveSpannerTargets(o.targets...)(s)
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
		if opt == nil {
			return nil, suite.ErrNilOption{}
		}
		if err := opt.applyLoad(s); err != nil {
			return nil, err
		}
	}
	if err := suite.ValidateInfraTargets(s); err != nil {
		return nil, err
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

// DataProvider supplies per-request data for a load case.
type DataProvider func(CallData) (any, error)

// LoadCaseOption configures an individual load case.
type LoadCaseOption interface {
	applyLoadCase(*loadCaseAdapter) error
}

// loadCaseOption is a functional [LoadCaseOption] applied at case registration.
type loadCaseOption func(*loadCaseAdapter) error

// applyLoadCase mutates the adapter before it is registered on the suite.
func (o loadCaseOption) applyLoadCase(a *loadCaseAdapter) error { return o(a) }

// WithLoadCaseTags attaches labels to the case wire result.
func WithLoadCaseTags(tags map[string]string) LoadCaseOption {
	return loadCaseOption(func(a *loadCaseAdapter) error {
		a.tags = cloneStringMap(tags)
		return nil
	})
}

// WithLoadCaseData sets round-robin payloads rotated by request number.
func WithLoadCaseData(data ...any) LoadCaseOption {
	return loadCaseOption(func(a *loadCaseAdapter) error {
		a.data = append([]any(nil), data...)
		return nil
	})
}

// WithLoadCaseDataProvider sets a programmatic data provider.
func WithLoadCaseDataProvider(p DataProvider) LoadCaseOption {
	return loadCaseOption(func(a *loadCaseAdapter) error {
		a.provider = p
		return nil
	})
}

// NoSLOs marks a load case as deliberately measure-only. The case still fails
// when transport errors are not covered by an error-rate SLO.
func NoSLOs() []SLO {
	return append([]SLO(nil), noSLOsSentinel...)
}

var noSLOsSentinel = []SLO{{id: "__evals_no_slos_sentinel__"}}

func isNoSLOs(slos []SLO) bool {
	return len(slos) == 1 && slos[0].id == noSLOsSentinel[0].id
}

func hasErrorRateSLO(slos []SLO) bool {
	for _, s := range slos {
		if s.id == "error_rate" {
			return true
		}
	}
	return false
}

// LoadCase registers a load case under the suite.
func (s *LoadSuite) LoadCase(name string, target ResultTarget, slos []SLO, opts ...LoadCaseOption) error {
	return s.loadCase(name, target, opts, slos...)
}

// loadCase registers a load case after validating target and applying options.
func (s *LoadSuite) loadCase(name string, target ResultTarget, opts []LoadCaseOption, slos ...SLO) error {
	if s == nil {
		return suite.ErrNilSuite{}
	}
	if target == nil {
		return ErrNilTarget{Case: name}
	}
	if len(slos) == 0 {
		return ErrEmptySLOs{Case: name}
	}
	if isNoSLOs(slos) {
		slos = nil
	}
	if err := validateUniqueSLOIDs(name, slos); err != nil {
		return err
	}
	if err := validateSLOValues(name, slos); err != nil {
		return err
	}
	adapter := &loadCaseAdapter{
		name:      name,
		target:    target,
		slos:      append([]SLO(nil), slos...),
		generator: s.generator,
		cloudRun:  s.inner.CloudRunTargets(),
		spanner:   s.inner.SpannerTargets(),
	}
	for _, opt := range opts {
		if opt == nil {
			return suite.ErrNilOption{}
		}
		if err := opt.applyLoadCase(adapter); err != nil {
			return err
		}
	}
	if len(adapter.data) > 0 && adapter.provider != nil {
		return ErrDualLoadCaseData{Case: name}
	}
	return s.inner.AddCase(adapter)
}

// MustLoadCase is like [LoadSuite.LoadCase] but panics on error and
// returns the suite for fluent chaining.
func (s *LoadSuite) MustLoadCase(name string, target ResultTarget, slos []SLO, opts ...LoadCaseOption) *LoadSuite {
	if err := s.LoadCase(name, target, slos, opts...); err != nil {
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
	// name is the case name registered via LoadCase.
	name string
	// target is the per-request SUT invocation.
	target ResultTarget
	// slos are thresholds evaluated after the generator run.
	slos []SLO
	// generator is copied from LoadSuite at registration.
	generator loadgen.Generator
	// tags are wire labels; set by WithLoadCaseTags.
	tags map[string]string
	// data holds round-robin payloads; set by WithLoadCaseData.
	data []any
	// provider is the programmatic payload resolver; set by WithLoadCaseDataProvider.
	provider DataProvider
	// cloudRun is infra scope; copied from suite at registration.
	cloudRun []loadinfra.CloudRunTarget
	// spanner is infra scope; copied from suite at registration.
	spanner []loadinfra.SpannerTarget
}

// Name returns the registered load case name.
func (a *loadCaseAdapter) Name() string { return a.name }

// Run drives load generation, evaluates SLOs, and optionally fetches infra snapshots.
func (a *loadCaseAdapter) Run(ctx context.Context, mode evalspb.RunLoadTestRequest_Mode, profile loadgen.Profile) *execution.LoadCaseResult {
	if loadgen.AbortOnSLOFailure(ctx) {
		profile.AbortCheck = abortCheckForSLOs(a.slos)
	}
	m, err := a.generator.Run(ctx, profile, a.wrapTarget())
	if m == nil {
		m = &loadgen.Metrics{}
	}
	result := &execution.LoadCaseResult{
		Name:    a.name,
		Tags:    cloneStringMap(a.tags),
		Summary: summaryFromMetrics(mode, profile, m),
	}
	if err != nil {
		if errors.Is(err, context.Canceled) && loadgen.AbortOnSLOFailure(ctx) {
			if chk, ok := abortedSLOCheck(a.slos, m); ok {
				result.Checks = append(result.Checks, chk)
			} else {
				result.Checks = append(result.Checks, execution.SloCheckResult{
					ID:      "generator",
					Status:  evalspb.Status_FAILED,
					Message: err.Error(),
					Unit:    "",
				})
			}
		} else {
			result.Checks = append(result.Checks, execution.SloCheckResult{
				ID:      "generator",
				Status:  evalspb.Status_FAILED,
				Message: err.Error(),
				Unit:    "",
			})
		}
	}
	if m.CheckFailedCount > 0 {
		result.Checks = append(result.Checks, execution.SloCheckResult{
			ID:       "checks",
			Status:   evalspb.Status_FAILED,
			Message:  fmt.Sprintf("%d semantic check(s) failed", m.CheckFailedCount),
			Observed: float64(m.CheckFailedCount),
			Limit:    0,
			Unit:     "count",
		})
	}
	result.Checks = append(result.Checks, evaluateSLOs(a.slos, m)...)
	if m.ErrorCount > 0 && !hasErrorRateSLO(a.slos) {
		result.Checks = append(result.Checks, execution.SloCheckResult{
			ID:       verdict.IDTransportErrors,
			Status:   evalspb.Status_FAILED,
			Message:  fmt.Sprintf("%d transport error(s) with no error-rate SLO declared", m.ErrorCount),
			Observed: float64(m.ErrorCount),
			Limit:    0,
			Unit:     "count",
		})
	}
	result.Status = rollupLoadCaseStatus(result.Checks)
	if client := loadinfra.ClientFromContext(ctx); client != nil && (len(a.cloudRun) > 0 || len(a.spanner) > 0) {
		// Infra fetch failures are recorded per-target on the snapshot
		// (FetchStatus/FetchMessage); they do not fail the load case in v1.
		obs, _ := loadinfra.Observe(ctx, client, a.cloudRun, a.spanner, loadinfra.WindowFromMetrics(m), true, 0)
		result.CloudRun = obs.CloudRun
		result.Spanner = obs.Spanner
	}
	return result
}

// rollupLoadCaseStatus fails the case when any SloCheckResult is non-PASSED.
func rollupLoadCaseStatus(checks []execution.SloCheckResult) evalspb.Status {
	for _, c := range checks {
		if c.Status != evalspb.Status_PASSED {
			return evalspb.Status_FAILED
		}
	}
	return evalspb.Status_PASSED
}

// wrapTarget resolves per-request data then delegates to the author target.
func (a *loadCaseAdapter) wrapTarget() loadgen.ResultTarget {
	return func(ctx context.Context, data CallData) TargetResult {
		resolved, err := a.resolveData(data)
		if err != nil {
			return TargetResult{TransportErr: err}
		}
		data.Data = resolved
		return a.target(ctx, data)
	}
}

// resolveData picks the payload for request data via provider or round-robin data.
func (a *loadCaseAdapter) resolveData(data CallData) (any, error) {
	if a.provider != nil {
		return a.provider(data)
	}
	if len(a.data) == 0 {
		return nil, nil
	}
	idx := int((data.RequestNumber - 1) % uint64(len(a.data)))
	return a.data[idx], nil
}

// cloneStringMap returns a defensive copy of in, or nil when empty.
func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// summaryFromMetrics maps generator metrics and profile into a wire LoadCaseSummary.
func summaryFromMetrics(mode evalspb.RunLoadTestRequest_Mode, profile loadgen.Profile, m *loadgen.Metrics) execution.LoadCaseSummary {
	// duration reflects the measurement window as configured; the generator
	// preserves this on m.Duration (it does not shrink under saturation).
	dur := m.Duration
	if dur == 0 {
		dur = profile.Duration
	}
	return execution.LoadCaseSummary{
		Mode:             mode,
		TargetQPS:        profile.EffectiveQPS(),
		Concurrency:      int32(profile.MaxConcurrency()),
		Duration:         dur,
		RequestCount:     m.RequestCount,
		ErrorCount:       m.ErrorCount,
		CheckPassedCount: m.CheckPassedCount,
		CheckFailedCount: m.CheckFailedCount,
		DroppedCount:     m.DroppedCount,
		ActualQPS:        m.ActualQPS,
		QPSStages:        cloneStages(profile.QPSStages),
		ConcurrencyStages: cloneStages(profile.ConcurrencyStages),
		Latency: execution.LoadLatency{
			P50Ms:  m.Latency.P50Ms,
			P95Ms:  m.Latency.P95Ms,
			P99Ms:  m.Latency.P99Ms,
			MinMs:  m.Latency.MinMs,
			MeanMs: m.Latency.MeanMs,
			MaxMs:  m.Latency.MaxMs,
		},
		Stream:       streamSummaryFromMetrics(m.Stream),
		ErrorsByCode: cloneErrorsByCode(m.ErrorsByCode),
	}
}

// streamSummaryFromMetrics copies stream aggregates when any stream samples exist.
func streamSummaryFromMetrics(s *loadgen.StreamSummary) *execution.LoadStreamSummary {
	if s == nil {
		return nil
	}
	return &execution.LoadStreamSummary{
		StreamCount:       s.StreamCount,
		MessagesSentTotal: s.MessagesSentTotal,
		TTFB:              latencyFromLoadgen(s.TTFB),
		ResponseLatency:   latencyFromLoadgen(s.ResponseLatency),
		TotalDuration:     latencyFromLoadgen(s.TotalDuration),
	}
}

// latencyFromLoadgen copies percentile fields into the execution wire shape.
func latencyFromLoadgen(l loadgen.LatencySummary) execution.LoadLatency {
	return execution.LoadLatency{
		P50Ms:  l.P50Ms,
		P95Ms:  l.P95Ms,
		P99Ms:  l.P99Ms,
		MinMs:  l.MinMs,
		MeanMs: l.MeanMs,
		MaxMs:  l.MaxMs,
	}
}

// cloneStages copies profile stages for the wire summary.
func cloneStages(stages []loadgen.Stage) []execution.LoadStage {
	if len(stages) == 0 {
		return nil
	}
	out := make([]execution.LoadStage, len(stages))
	for i, s := range stages {
		out[i] = execution.LoadStage{Duration: s.Duration, Target: s.Target}
	}
	return out
}

// cloneErrorsByCode returns a defensive copy of transport error counts.
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

func abortedSLOCheck(slos []SLO, m *loadgen.Metrics) (execution.SloCheckResult, bool) {
	if m == nil || m.RequestCount == 0 || len(slos) == 0 {
		return execution.SloCheckResult{}, false
	}
	for _, s := range slos {
		observed := s.extract(m)
		if !s.pass(observed, s.limit) {
			msg := s.failMessage(observed, s.limit)
			return execution.SloCheckResult{
				ID:       verdict.IDAborted,
				Status:   evalspb.Status_FAILED,
				Message:  fmt.Sprintf("abort-on-SLO: %s (%s)", s.id, msg),
				Observed: observed,
				Limit:    s.limit,
				Unit:     s.unit,
			}, true
		}
	}
	return execution.SloCheckResult{}, false
}
