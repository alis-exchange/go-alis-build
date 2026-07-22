package evals

import (
	"context"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/loadinfra"
	"go.alis.build/evals/suite"
	"go.alis.build/evals/verdict"
)

// InfraObserveSuite runs standalone infrastructure observation over a lookback
// window without load generation.
type InfraObserveSuite struct {
	inner *suite.InfraObserveSuite // underlying suite registered with the runner
}

// InfraObserveSuiteOption configures an InfraObserveSuite at construction time.
type InfraObserveSuiteOption interface {
	applyInfraObserve(*suite.InfraObserveSuite) error
}

// infraObserveOption is a functional [InfraObserveSuiteOption].
type infraObserveOption func(*suite.InfraObserveSuite) error

// applyInfraObserve invokes the option against an infra observation suite.
func (o infraObserveOption) applyInfraObserve(s *suite.InfraObserveSuite) error {
	return o(s)
}

// WithInfraObserveEnv declares shared environments the suite requires.
func WithInfraObserveEnv(names ...string) InfraObserveSuiteOption {
	return infraObserveOption(func(s *suite.InfraObserveSuite) error {
		return suite.WithInfraObserveEnvironment(names...)(s)
	})
}

// WithInfraObserveSetup registers an optional suite-level setup hook.
func WithInfraObserveSetup(h suite.SuiteHook) InfraObserveSuiteOption {
	return infraObserveOption(func(s *suite.InfraObserveSuite) error {
		return suite.WithInfraObserveSetup(h)(s)
	})
}

// WithInfraObserveTeardown registers an optional suite-level teardown hook.
func WithInfraObserveTeardown(h suite.SuiteHook) InfraObserveSuiteOption {
	return infraObserveOption(func(s *suite.InfraObserveSuite) error {
		return suite.WithInfraObserveTeardown(h)(s)
	})
}

// WithLookback sets the default lookback for cases in this suite.
func WithLookback(d time.Duration) InfraObserveSuiteOption {
	return infraObserveOption(func(s *suite.InfraObserveSuite) error {
		return suite.WithLookback(d)(s)
	})
}

// WithInfraObserveContext installs a [ContextDecorator] on the infra observe suite.
func WithInfraObserveContext(fn ContextDecorator) InfraObserveSuiteOption {
	return infraObserveOption(func(s *suite.InfraObserveSuite) error {
		return suite.WithInfraObserveContext(fn)(s)
	})
}

// WithInfraObserveStopOnFailure marks the suite so remaining cases are recorded
// NOT_EVALUATED after the first non-PASSED case.
func WithInfraObserveStopOnFailure() InfraObserveSuiteOption {
	return infraObserveOption(func(s *suite.InfraObserveSuite) error {
		return suite.WithInfraObserveStopOnFailure()(s)
	})
}

// NewInfraObserveSuite constructs an infra observation suite.
func NewInfraObserveSuite(name string, opts ...InfraObserveSuiteOption) (*InfraObserveSuite, error) {
	for _, opt := range opts {
		if opt == nil {
			return nil, suite.ErrNilOption{}
		}
	}
	suiteOpts := make([]suite.InfraObserveSuiteOption, len(opts))
	for i, opt := range opts {
		suiteOpts[i] = func(s *suite.InfraObserveSuite) error {
			return opt.applyInfraObserve(s)
		}
	}
	s, err := suite.NewInfraObserveSuite(name, suiteOpts...)
	if err != nil {
		return nil, err
	}
	return &InfraObserveSuite{inner: s}, nil
}

// MustNewInfraObserveSuite is like [NewInfraObserveSuite] but panics on error.
func MustNewInfraObserveSuite(name string, opts ...InfraObserveSuiteOption) *InfraObserveSuite {
	s, err := NewInfraObserveSuite(name, opts...)
	if err != nil {
		panic(err)
	}
	return s
}

// InfraObserveCaseOption configures an individual infra observation case.
type InfraObserveCaseOption interface {
	applyInfraObserveCase(*infraObserveCaseAdapter)
}

// infraObserveCaseOption is a functional [InfraObserveCaseOption].
type infraObserveCaseOption func(*infraObserveCaseAdapter)

// applyInfraObserveCase mutates the adapter before case registration.
func (o infraObserveCaseOption) applyInfraObserveCase(a *infraObserveCaseAdapter) { o(a) }

// WithObserveCaseLookback overrides the suite lookback for one case.
func WithObserveCaseLookback(d time.Duration) InfraObserveCaseOption {
	return infraObserveCaseOption(func(a *infraObserveCaseAdapter) {
		a.lookback = d
		a.hasLookback = true
	})
}

// WithInfraObserveCaseTags attaches labels to the case wire result.
func WithInfraObserveCaseTags(tags map[string]string) InfraObserveCaseOption {
	return infraObserveCaseOption(func(a *infraObserveCaseAdapter) {
		a.tags = cloneStringMap(tags)
	})
}

// InfraObserveCase registers an infra observation case under the suite.
func (s *InfraObserveSuite) InfraObserveCase(name string, opts ...InfraObserveCaseOption) error {
	if s == nil {
		return suite.ErrNilSuite{}
	}
	adapter := &infraObserveCaseAdapter{
		name:     name,
		cloudRun: s.inner.CloudRunTargets(),
		spanner:  s.inner.SpannerTargets(),
	}
	for _, opt := range opts {
		opt.applyInfraObserveCase(adapter)
	}
	if adapter.hasLookback && adapter.lookback <= 0 {
		return suite.ErrInvalidLookback{Value: adapter.lookback}
	}
	return s.inner.AddCase(adapter)
}

// MustInfraObserveCase is like [InfraObserveSuite.InfraObserveCase] but panics on error.
func (s *InfraObserveSuite) MustInfraObserveCase(name string, opts ...InfraObserveCaseOption) *InfraObserveSuite {
	if err := s.InfraObserveCase(name, opts...); err != nil {
		panic(err)
	}
	return s
}

// Name returns the suite name.
func (s *InfraObserveSuite) Name() string {
	if s == nil {
		return ""
	}
	return s.inner.Name()
}

// Inner exposes the underlying suite for the registry.
func (s *InfraObserveSuite) Inner() *suite.InfraObserveSuite {
	if s == nil {
		return nil
	}
	return s.inner
}

// infraObserveCaseAdapter bridges suite.InfraObserveCase to loadinfra.Observe.
type infraObserveCaseAdapter struct {
	// name is the case name registered via InfraObserveCase.
	name string
	// lookback is the per-case override; set by WithObserveCaseLookback.
	lookback time.Duration
	// hasLookback is true when lookback was explicitly configured.
	hasLookback bool
	// tags are wire labels; set by WithInfraObserveCaseTags.
	tags map[string]string
	// cloudRun holds fallback targets; copied from suite at registration.
	cloudRun []loadinfra.CloudRunTarget
	// spanner holds fallback targets; copied from suite at registration.
	spanner []loadinfra.SpannerTarget
}

// Name returns the registered infra observation case name.
func (a *infraObserveCaseAdapter) Name() string { return a.name }

// Lookback reports a per-case lookback override when hasLookback is true.
func (a *infraObserveCaseAdapter) Lookback() (time.Duration, bool) {
	return a.lookback, a.hasLookback
}

// Run resolves the observation window, fetches infra snapshots, and assembles the case result.
func (a *infraObserveCaseAdapter) Run(ctx context.Context, cfg suite.InfraObserveCaseConfig) *execution.InfraObserveCaseResult {
	perCase, hasPerCase := a.lookback, a.hasLookback
	lookback, err := suite.ResolveInfraObserveLookback(
		cfg.RequestLookback, perCase, cfg.SuiteLookback,
		cfg.HasRequest, hasPerCase,
	)
	if err != nil {
		// Configuration errors fail the case; v1 has no infra_checks leaf for diagnostics.
		return &execution.InfraObserveCaseResult{
			Name:     a.name,
			Status:   evalspb.Status_FAILED,
			Tags:     cloneStringMap(a.tags),
			CloudRun: []*evalspb.CloudRunTargetSnapshot{loadinfra.ConfigFailureSnapshot(err.Error())},
		}
	}

	client := loadinfra.ClientFromContext(ctx)
	if client == nil {
		return &execution.InfraObserveCaseResult{
			Name:     a.name,
			Status:   evalspb.Status_FAILED,
			Tags:     cloneStringMap(a.tags),
			CloudRun: []*evalspb.CloudRunTargetSnapshot{loadinfra.ConfigFailureSnapshot("loadinfra: nil MetricClient")},
		}
	}

	cloud := cfg.CloudRun
	if len(cloud) == 0 {
		cloud = a.cloudRun
	}
	spanner := cfg.Spanner
	if len(spanner) == 0 {
		spanner = a.spanner
	}

	settle := loadinfra.SettleDuration(len(cloud) > 0, len(spanner) > 0)
	window := loadinfra.WindowLookback(lookback, time.Now(), settle)
	// Infra fetch failures are recorded per target and fail the case rollup.
	obs, _ := loadinfra.Observe(ctx, client, cloud, spanner, window, false, 0)

	return &execution.InfraObserveCaseResult{
		Name:        a.name,
		Status:      rollupStandaloneInfraObserve(obs.CloudRun, obs.Spanner),
		Tags:        cloneStringMap(a.tags),
		Lookback:    lookback,
		WindowStart: window.Start,
		WindowEnd:   window.End,
		CloudRun:    obs.CloudRun,
		Spanner:     obs.Spanner,
	}
}

func rollupStandaloneInfraObserve(cloud []*evalspb.CloudRunTargetSnapshot, spanner []*evalspb.SpannerTargetSnapshot) evalspb.Status {
	leaves := infraObserveLeaves(cloud, spanner)
	status, _ := verdict.Case(verdict.Evidence{Leaves: leaves}, verdict.StandaloneInfraObservePolicy())
	return status
}

func infraObserveLeaves(cloud []*evalspb.CloudRunTargetSnapshot, spanner []*evalspb.SpannerTargetSnapshot) []verdict.Leaf {
	out := make([]verdict.Leaf, 0, len(cloud)+len(spanner))
	for _, snap := range cloud {
		if snap == nil {
			continue
		}
		out = append(out, verdict.Leaf{
			ID:     snap.GetId(),
			Status: infraFetchToStatus(snap.GetFetchStatus()),
		})
	}
	for _, snap := range spanner {
		if snap == nil {
			continue
		}
		out = append(out, verdict.Leaf{
			ID:     snap.GetId(),
			Status: infraFetchToStatus(snap.GetFetchStatus()),
		})
	}
	return out
}

func infraFetchToStatus(st evalspb.InfraFetchStatus) evalspb.Status {
	if st == evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK {
		return evalspb.Status_PASSED
	}
	return evalspb.Status_FAILED
}
