package suite

import (
	"context"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/env"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/loadinfra"
)

// ResolveInfraObserveLookback applies lookback precedence: request → per-case → suite.
func ResolveInfraObserveLookback(request, perCase, suiteDefault time.Duration, hasRequest, hasPerCase bool) (time.Duration, error) {
	if hasRequest && request > 0 {
		return request, nil
	}
	if hasPerCase && perCase > 0 {
		return perCase, nil
	}
	if suiteDefault > 0 {
		return suiteDefault, nil
	}
	return 0, ErrInfraObserveLookbackUnset{}
}

// InfraObserveSuite groups infra observation cases that share targets, lookback,
// and lifecycle hooks. Cases run concurrently (read-only Monitoring fetches).
type InfraObserveSuite struct {
	// name is the unqualified suite identifier used in filters and qualified case names.
	name string
	// environments lists shared env.Get names required before cases run.
	environments []string
	// setup runs once before selected cases when non-nil.
	setup SuiteHook
	// teardown runs once after selected cases when non-nil.
	teardown SuiteHook
	// lookback is the default observation window when request and per-case overrides are absent.
	lookback time.Duration
	// cloudRun holds declared Cloud Run targets observed on every case.
	cloudRun []loadinfra.CloudRunTarget
	// spanner holds declared Spanner targets observed on every case.
	spanner []loadinfra.SpannerTarget
	// cases holds qualified infra-observe cases in registration order.
	cases []InfraObserveCase
	// decorate is applied to the context passed to setup, teardown, and cases.
	decorate ContextDecorator
	// stopOnFailure skips remaining cases after the first non-PASSED case.
	stopOnFailure bool
}

// InfraObserveSuiteOption configures an InfraObserveSuite at construction time.
type InfraObserveSuiteOption func(*InfraObserveSuite) error

// WithInfraObserveEnvironment declares shared environments required by the suite.
func WithInfraObserveEnvironment(names ...string) InfraObserveSuiteOption {
	return func(s *InfraObserveSuite) error {
		return addEnvironments(&s.environments, names)
	}
}

// WithInfraObserveSetup registers optional suite-level setup.
func WithInfraObserveSetup(h SuiteHook) InfraObserveSuiteOption {
	return func(s *InfraObserveSuite) error {
		s.setup = h
		return nil
	}
}

// WithInfraObserveTeardown registers optional suite-level teardown.
func WithInfraObserveTeardown(h SuiteHook) InfraObserveSuiteOption {
	return func(s *InfraObserveSuite) error {
		s.teardown = h
		return nil
	}
}

// WithLookback sets the default lookback window for cases in this suite.
func WithLookback(d time.Duration) InfraObserveSuiteOption {
	return func(s *InfraObserveSuite) error {
		if d <= 0 {
			return ErrInvalidLookback{Value: d}
		}
		s.lookback = d
		return nil
	}
}

// WithInfraObserveCloudRunTargets appends Cloud Run targets to an infra observe suite.
func WithInfraObserveCloudRunTargets(targets ...loadinfra.CloudRunTarget) InfraObserveSuiteOption {
	return func(s *InfraObserveSuite) error {
		// Reuse load-suite target validation (ENTRY count, required fields) without duplicating rules.
		opt := WithCloudRunTargets(targets...)
		tmp := &LoadSuite{cloudRun: s.cloudRun}
		if err := opt(tmp); err != nil {
			return err
		}
		s.cloudRun = tmp.cloudRun
		return nil
	}
}

// WithInfraObserveSpannerTargets appends Spanner targets to an infra observe suite.
func WithInfraObserveSpannerTargets(targets ...loadinfra.SpannerTarget) InfraObserveSuiteOption {
	return func(s *InfraObserveSuite) error {
		opt := WithSpannerTargets(targets...)
		tmp := &LoadSuite{spanner: s.spanner}
		if err := opt(tmp); err != nil {
			return err
		}
		s.spanner = tmp.spanner
		return nil
	}
}

// WithInfraObserveContext installs a [ContextDecorator] on the infra observe suite.
func WithInfraObserveContext(fn ContextDecorator) InfraObserveSuiteOption {
	return func(s *InfraObserveSuite) error {
		s.decorate = fn
		return nil
	}
}

// WithInfraObserveStopOnFailure marks the suite so remaining cases are skipped
// after the first non-PASSED case result.
func WithInfraObserveStopOnFailure() InfraObserveSuiteOption {
	return func(s *InfraObserveSuite) error {
		s.stopOnFailure = true
		return nil
	}
}

// NewInfraObserveSuite creates an infra observation suite.
func NewInfraObserveSuite(name string, opts ...InfraObserveSuiteOption) (*InfraObserveSuite, error) {
	if err := validateSuiteName(name); err != nil {
		return nil, err
	}
	s := &InfraObserveSuite{name: name}
	for _, opt := range opts {
		if opt == nil {
			return nil, ErrNilOption{}
		}
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// Validate checks infra-observe suite invariants before registration or execution.
func (s *InfraObserveSuite) Validate() error {
	if s == nil {
		return ErrNilSuite{}
	}
	if err := validateInfraTargetIDs(s.cloudRun, s.spanner); err != nil {
		return err
	}
	if len(s.cloudRun) == 0 && len(s.spanner) == 0 {
		return ErrInfraObserveNoTargets{}
	}
	return nil
}

// AddCase registers one infra observation case.
func (s *InfraObserveSuite) AddCase(c InfraObserveCase) error {
	if s == nil {
		return ErrNilSuite{}
	}
	qualified, err := qualifyInfraObserveCase(s.name, c, s.cases)
	if err != nil {
		return err
	}
	s.cases = append(s.cases, qualified)
	return nil
}

// qualifyInfraObserveCase is [qualifyTestCase] for infra-observe cases.
func qualifyInfraObserveCase(suiteName string, c InfraObserveCase, existing []InfraObserveCase) (InfraObserveCase, error) {
	if c == nil {
		return nil, ErrNilInfraObserveCase{}
	}
	short := c.Name()
	if err := validateCaseName(short); err != nil {
		return nil, err
	}
	qualified := QualifiedName(suiteName, short)
	for _, e := range existing {
		if e.Name() == qualified {
			return nil, ErrDuplicateCase{Suite: suiteName, Case: short}
		}
	}
	return &qualifiedInfraObserveCase{name: qualified, inner: c}, nil
}

// Name returns the suite name.
func (s *InfraObserveSuite) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Lookback returns the suite default lookback, or zero when unset.
func (s *InfraObserveSuite) Lookback() time.Duration {
	if s == nil {
		return 0
	}
	return s.lookback
}

// Environments returns declared environment names.
func (s *InfraObserveSuite) Environments() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.environments...)
}

// SetupHook returns the suite setup hook.
func (s *InfraObserveSuite) SetupHook() SuiteHook {
	if s == nil {
		return nil
	}
	return s.setup
}

// TeardownHook returns the suite teardown hook.
func (s *InfraObserveSuite) TeardownHook() SuiteHook {
	if s == nil {
		return nil
	}
	return s.teardown
}

// CloudRunTargets returns declared Cloud Run targets.
func (s *InfraObserveSuite) CloudRunTargets() []loadinfra.CloudRunTarget {
	if s == nil || len(s.cloudRun) == 0 {
		return nil
	}
	return append([]loadinfra.CloudRunTarget(nil), s.cloudRun...)
}

// SpannerTargets returns declared Spanner targets.
func (s *InfraObserveSuite) SpannerTargets() []loadinfra.SpannerTarget {
	if s == nil || len(s.spanner) == 0 {
		return nil
	}
	return append([]loadinfra.SpannerTarget(nil), s.spanner...)
}

// Decorator returns the suite [ContextDecorator], or nil when unset.
func (s *InfraObserveSuite) Decorator() ContextDecorator {
	if s != nil {
		return s.decorate
	}
	return nil
}

// StopOnFailure reports whether remaining cases should be skipped after the
// first non-PASSED case.
func (s *InfraObserveSuite) StopOnFailure() bool {
	return s != nil && s.stopOnFailure
}

// InfraObserveSuiteRun is a filtered infra observe suite ready for runner execution.
type InfraObserveSuiteRun struct {
	Name         string
	Environments []string
	// EnvRegistry is the registry that validated the environment names. Nil
	// makes the runner use env.DefaultRegistry for manually assembled runs.
	EnvRegistry   *env.Registry
	Setup         SuiteHook
	Teardown      SuiteHook
	Decorate      ContextDecorator
	StopOnFailure bool
	// Lookback is the suite default lookback applied when a case has no override.
	Lookback time.Duration
	// CloudRun is the suite's declared Cloud Run targets copied at selection time.
	CloudRun []loadinfra.CloudRunTarget
	// Spanner is the suite's declared Spanner targets copied at selection time.
	Spanner []loadinfra.SpannerTarget
	Cases   []InfraObserveCase
}

// SelectInfraObserveCases returns cases matching parsed filters.
func (s *InfraObserveSuite) SelectInfraObserveCases(filters []FilterPath) []InfraObserveCase {
	return selectInfraObserveCasesInSuite(s, filters)
}

// selectInfraObserveCasesInSuite is [selectTestCasesInSuite] for infra-observe suites.
func selectInfraObserveCasesInSuite(s *InfraObserveSuite, filters []FilterPath) []InfraObserveCase {
	if s == nil {
		return nil
	}
	if len(filters) == 0 {
		return append([]InfraObserveCase(nil), s.cases...)
	}
	wantAll := false
	selected := make(map[string]bool)
	mentioned := false
	for _, f := range filters {
		if f.Suite != s.name {
			continue
		}
		mentioned = true
		if f.CaseName == "" {
			wantAll = true
			break
		}
		selected[QualifiedName(s.name, f.CaseName)] = true
	}
	if !mentioned {
		return nil
	}
	if wantAll {
		return append([]InfraObserveCase(nil), s.cases...)
	}
	out := make([]InfraObserveCase, 0, len(selected))
	for _, c := range s.cases {
		if selected[c.Name()] {
			out = append(out, c)
		}
	}
	return out
}

// TotalInfraObserveCases counts cases across infra observe suite runs.
func TotalInfraObserveCases(runs []InfraObserveSuiteRun) int {
	n := 0
	for _, run := range runs {
		n += len(run.Cases)
	}
	return n
}

// qualifiedInfraObserveCase wraps a user InfraObserveCase with a suite-qualified Name().
type qualifiedInfraObserveCase struct {
	// name is the canonical "suite.case" identifier stamped on results.
	name string
	// inner is the user-registered case implementation.
	inner InfraObserveCase
}

// Name returns the qualified case name.
func (q *qualifiedInfraObserveCase) Name() string { return q.name }

// Lookback forwards to inner when present.
func (q *qualifiedInfraObserveCase) Lookback() (time.Duration, bool) {
	if q.inner == nil {
		return 0, false
	}
	return q.inner.Lookback()
}

// Run delegates to inner and overwrites the result Name with the qualified value.
func (q *qualifiedInfraObserveCase) Run(ctx context.Context, cfg InfraObserveCaseConfig) *execution.InfraObserveCaseResult {
	if q.inner == nil {
		return &execution.InfraObserveCaseResult{Name: q.name, Status: evalspb.Status_FAILED}
	}
	r := q.inner.Run(ctx, cfg)
	if r == nil {
		return &execution.InfraObserveCaseResult{Name: q.name, Status: evalspb.Status_FAILED}
	}
	r.Name = q.name
	return r
}

// InfraObserveCaseConfig carries per-run lookback resolution inputs.
type InfraObserveCaseConfig struct {
	// RequestLookback is the lookback from RunInfraObservationRequest when
	// HasRequest is true.
	RequestLookback time.Duration
	// HasRequest reports whether RequestLookback was set on the RPC request.
	HasRequest bool
	// SuiteLookback is the default lookback from WithLookback on the suite.
	SuiteLookback time.Duration
	// CloudRun is the suite's declared Cloud Run targets for this run.
	CloudRun []loadinfra.CloudRunTarget
	// Spanner is the suite's declared Spanner targets for this run.
	Spanner []loadinfra.SpannerTarget
}
