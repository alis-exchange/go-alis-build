package registry

import (
	"context"
	"sort"
	"sync"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/env"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/suite"
)

// AgentEvalProvider executes agent evaluations lazily (for example via ADK discovery).
type AgentEvalProvider interface {
	Run(ctx context.Context, filters []string) ([]execution.SuiteResult, error)
}

// AgentEvalProviderFunc adapts a function to AgentEvalProvider.
type AgentEvalProviderFunc func(context.Context, []string) ([]execution.SuiteResult, error)

// Run implements AgentEvalProvider.
func (f AgentEvalProviderFunc) Run(ctx context.Context, filters []string) ([]execution.SuiteResult, error) {
	if f == nil {
		return nil, nil
	}
	return f(ctx, filters)
}

// Registry holds registered suites keyed by run type.
type Registry struct {
	mu sync.RWMutex
	// byType indexes integration-test suites by run type (today only INTEGRATION_TEST).
	byType map[evalspb.Run_Type][]*suite.TestSuite
	// evals holds eagerly registered agent-eval suites selected by filter paths.
	evals []*suite.EvalSuite
	// loads holds registered load-test suites.
	loads []*suite.LoadSuite
	// infraObserves holds registered infra-observation suites.
	infraObserves []*suite.InfraObserveSuite
	// evalProviders supplies lazy agent-eval suites (for example ADK discovery).
	evalProviders []AgentEvalProvider
	// envRegistry selects which environment registry Freeze validates against.
	// Nil uses [env.DefaultRegistry].
	envRegistry *env.Registry
	// frozen is set after a successful Freeze; further registration is rejected.
	frozen bool
}

// New returns an empty Registry. [evals.DefaultRegistry] uses this constructor.
func New() *Registry {
	return &Registry{
		byType: make(map[evalspb.Run_Type][]*suite.TestSuite),
	}
}

// RegisterIntegrationSuite adds a suite for integration test runs. It rejects
// nil, duplicate, and post-Freeze registration with typed errors.
func (r *Registry) RegisterIntegrationSuite(s *suite.TestSuite) error {
	if r == nil {
		return ErrNotConfigured{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen{}
	}
	if s == nil {
		return suite.ErrNilSuite{}
	}
	name := s.Name()
	for _, existing := range r.byType[evalspb.Run_INTEGRATION_TEST] {
		if existing.Name() == name {
			return ErrDuplicateSuite{Kind: "integration", Name: name}
		}
	}
	r.byType[evalspb.Run_INTEGRATION_TEST] = append(r.byType[evalspb.Run_INTEGRATION_TEST], s)
	return nil
}

// RegisterAgentEvalSuite adds an agent evaluation suite. It rejects nil,
// duplicate, and post-Freeze registration with typed errors.
func (r *Registry) RegisterAgentEvalSuite(s *suite.EvalSuite) error {
	if r == nil {
		return ErrNotConfigured{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen{}
	}
	if s == nil {
		return suite.ErrNilSuite{}
	}
	name := s.Name()
	for _, existing := range r.evals {
		if existing.Name() == name {
			return ErrDuplicateSuite{Kind: "eval", Name: name}
		}
	}
	r.evals = append(r.evals, s)
	return nil
}

// RegisterLoadSuite adds a load-test suite. It rejects nil, duplicate, and
// post-Freeze registration with typed errors.
func (r *Registry) RegisterLoadSuite(s *suite.LoadSuite) error {
	if r == nil {
		return ErrNotConfigured{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen{}
	}
	if s == nil {
		return suite.ErrNilSuite{}
	}
	name := s.Name()
	for _, existing := range r.loads {
		if existing.Name() == name {
			return ErrDuplicateSuite{Kind: "load", Name: name}
		}
	}
	r.loads = append(r.loads, s)
	return nil
}

// RegisterInfraObserveSuite adds an infra observation suite. It rejects nil,
// duplicate, and post-Freeze registration with typed errors.
func (r *Registry) RegisterInfraObserveSuite(s *suite.InfraObserveSuite) error {
	if r == nil {
		return ErrNotConfigured{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen{}
	}
	if s == nil {
		return suite.ErrNilSuite{}
	}
	name := s.Name()
	for _, existing := range r.infraObserves {
		if existing.Name() == name {
			return ErrDuplicateSuite{Kind: "infra_observe", Name: name}
		}
	}
	r.infraObserves = append(r.infraObserves, s)
	return nil
}

// RegisterAgentEvalProvider adds a lazy agent eval provider (for example ADK
// discovery). A nil provider is ignored; registration after Freeze returns
// [ErrRegistryFrozen].
func (r *Registry) RegisterAgentEvalProvider(p AgentEvalProvider) error {
	if r == nil {
		return ErrNotConfigured{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen{}
	}
	if p == nil {
		return nil
	}
	r.evalProviders = append(r.evalProviders, p)
	return nil
}

// SetEnvRegistry configures the environment registry used by Freeze and by
// runs selected from this registry. Nil uses [env.DefaultRegistry]. It returns
// [ErrNotConfigured] for a nil receiver and [ErrRegistryFrozen] after Freeze.
func (r *Registry) SetEnvRegistry(er *env.Registry) error {
	if r == nil {
		return ErrNotConfigured{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen{}
	}
	r.envRegistry = er
	return nil
}

// EnvRegistry returns the environment registry used for validation and
// execution. Nil configuration resolves to [env.DefaultRegistry].
func (r *Registry) EnvRegistry() *env.Registry {
	if r == nil {
		return env.DefaultRegistry()
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.envRegistry != nil {
		return r.envRegistry
	}
	return env.DefaultRegistry()
}

func (r *Registry) envRegistryLocked() *env.Registry {
	if r.envRegistry != nil {
		return r.envRegistry
	}
	return env.DefaultRegistry()
}

// Freeze validates registered suites and seals the registry against further
// registration. Validation errors identify duplicate suites, unknown
// environments, or invalid load profiles. A second successful call is a no-op;
// a nil receiver returns [ErrNotConfigured].
func (r *Registry) Freeze() error {
	if r == nil {
		return ErrNotConfigured{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return nil
	}
	if err := r.validateForFreeze(); err != nil {
		return err
	}
	r.frozen = true
	return nil
}

// Frozen reports whether the registry has been sealed by Freeze.
func (r *Registry) Frozen() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.frozen
}

func (r *Registry) validateForFreeze() error {
	if err := validateDuplicateSuiteNames(r.byType[evalspb.Run_INTEGRATION_TEST], "integration"); err != nil {
		return err
	}
	if err := validateDuplicateEvalSuiteNames(r.evals); err != nil {
		return err
	}
	if err := validateDuplicateLoadSuiteNames(r.loads); err != nil {
		return err
	}
	if err := validateDuplicateInfraObserveSuiteNames(r.infraObserves); err != nil {
		return err
	}
	if err := r.validateEnvironments(); err != nil {
		return err
	}
	return validateLoadSuiteProfiles(r.loads)
}

func (r *Registry) validateEnvironments() error {
	envReg := r.envRegistry
	if envReg == nil {
		envReg = env.DefaultRegistry()
	}
	seen := make(map[string]struct{})
	var missing []string
	collect := func(names []string) {
		for _, name := range names {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			if envReg.Get(name) == nil {
				missing = append(missing, name)
			}
		}
	}
	for _, s := range r.byType[evalspb.Run_INTEGRATION_TEST] {
		collect(s.Environments())
	}
	for _, s := range r.evals {
		collect(s.Environments())
	}
	for _, s := range r.loads {
		collect(s.Environments())
	}
	for _, s := range r.infraObserves {
		collect(s.Environments())
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return ErrUnknownEnvironments{Names: missing}
}

func validateLoadSuiteProfiles(suites []*suite.LoadSuite) error {
	for _, s := range suites {
		if s == nil {
			continue
		}
		for _, mode := range allModes {
			if p, ok := s.ProfileOverride(mode); ok {
				if err := p.Validate(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateDuplicateSuiteNames(suites []*suite.TestSuite, kind string) error {
	seen := make(map[string]struct{}, len(suites))
	for _, s := range suites {
		name := s.Name()
		if _, ok := seen[name]; ok {
			return ErrDuplicateSuite{Kind: kind, Name: name}
		}
		seen[name] = struct{}{}
	}
	return nil
}

func validateDuplicateEvalSuiteNames(suites []*suite.EvalSuite) error {
	seen := make(map[string]struct{}, len(suites))
	for _, s := range suites {
		name := s.Name()
		if _, ok := seen[name]; ok {
			return ErrDuplicateSuite{Kind: "eval", Name: name}
		}
		seen[name] = struct{}{}
	}
	return nil
}

func validateDuplicateLoadSuiteNames(suites []*suite.LoadSuite) error {
	seen := make(map[string]struct{}, len(suites))
	for _, s := range suites {
		name := s.Name()
		if _, ok := seen[name]; ok {
			return ErrDuplicateSuite{Kind: "load", Name: name}
		}
		seen[name] = struct{}{}
	}
	return nil
}

func validateDuplicateInfraObserveSuiteNames(suites []*suite.InfraObserveSuite) error {
	seen := make(map[string]struct{}, len(suites))
	for _, s := range suites {
		name := s.Name()
		if _, ok := seen[name]; ok {
			return ErrDuplicateSuite{Kind: "infra_observe", Name: name}
		}
		seen[name] = struct{}{}
	}
	return nil
}

// AgentEvalProviders returns registered lazy agent eval providers.
func (r *Registry) AgentEvalProviders() []AgentEvalProvider {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]AgentEvalProvider(nil), r.evalProviders...)
}

// SelectTestRuns returns filtered suite runs for the given type.
func (r *Registry) SelectTestRuns(runType evalspb.Run_Type, filters []string) ([]suite.TestSuiteRun, error) {
	if r == nil {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	suites := r.byType[runType]
	if len(suites) == 0 {
		return nil, nil
	}
	parsed, err := suite.ParseFilterPaths(filters)
	if err != nil {
		return nil, err
	}
	var runs []suite.TestSuiteRun
	for _, s := range suites {
		cases := s.SelectTestCases(parsed)
		if len(cases) == 0 {
			continue
		}
		runs = append(runs, suite.TestSuiteRun{
			Name:          s.Name(),
			Environments:  s.Environments(),
			EnvRegistry:   r.envRegistryLocked(),
			Setup:         s.SetupHook(),
			Teardown:      s.TeardownHook(),
			Cases:         cases,
			Decorate:      s.Decorator(),
			StopOnFailure: s.StopOnFailure(),
		})
	}
	return runs, nil
}

// SelectLoadRuns returns filtered load suite runs.
func (r *Registry) SelectLoadRuns(filters []string) ([]suite.LoadSuiteRun, error) {
	if r == nil {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.loads) == 0 {
		return nil, nil
	}
	parsed, err := suite.ParseFilterPaths(filters)
	if err != nil {
		return nil, err
	}
	var runs []suite.LoadSuiteRun
	for _, s := range r.loads {
		cases := s.SelectLoadCases(parsed)
		if len(cases) == 0 {
			continue
		}
		runs = append(runs, suite.LoadSuiteRun{
			Name:             s.Name(),
			Environments:     s.Environments(),
			EnvRegistry:      r.envRegistryLocked(),
			Setup:            s.SetupHook(),
			Teardown:         s.TeardownHook(),
			Cases:            cases,
			Decorate:         s.Decorator(),
			StopOnFailure:    s.StopOnFailure(),
			ProfileOverrides: copyProfileOverrides(s),
			CloudRun:         s.CloudRunTargets(),
			Spanner:          s.SpannerTargets(),
		})
	}
	return runs, nil
}

// SelectInfraObserveRuns returns filtered infra observation suite runs.
func (r *Registry) SelectInfraObserveRuns(filters []string) ([]suite.InfraObserveSuiteRun, error) {
	if r == nil {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.infraObserves) == 0 {
		return nil, nil
	}
	parsed, err := suite.ParseFilterPaths(filters)
	if err != nil {
		return nil, err
	}
	var runs []suite.InfraObserveSuiteRun
	for _, s := range r.infraObserves {
		cases := s.SelectInfraObserveCases(parsed)
		if len(cases) == 0 {
			continue
		}
		runs = append(runs, suite.InfraObserveSuiteRun{
			Name:          s.Name(),
			Environments:  s.Environments(),
			EnvRegistry:   r.envRegistryLocked(),
			Setup:         s.SetupHook(),
			Teardown:      s.TeardownHook(),
			Decorate:      s.Decorator(),
			StopOnFailure: s.StopOnFailure(),
			Lookback:      s.Lookback(),
			CloudRun:      s.CloudRunTargets(),
			Spanner:       s.SpannerTargets(),
			Cases:         cases,
		})
	}
	return runs, nil
}

// allModes lists every mode a suite might override, in ascending intensity.
var allModes = []evalspb.RunLoadTestRequest_Mode{
	evalspb.RunLoadTestRequest_MINIMAL,
	evalspb.RunLoadTestRequest_CONSERVATIVE,
	evalspb.RunLoadTestRequest_MODERATE,
	evalspb.RunLoadTestRequest_HIGH,
	evalspb.RunLoadTestRequest_LUDICROUS,
}

// copyProfileOverrides returns a snapshot of the suite's overrides so
// registration-time mutations do not affect an in-progress run.
func copyProfileOverrides(s *suite.LoadSuite) map[evalspb.RunLoadTestRequest_Mode]loadgen.Profile {
	out := make(map[evalspb.RunLoadTestRequest_Mode]loadgen.Profile)
	for _, m := range allModes {
		if p, ok := s.ProfileOverride(m); ok {
			out[m] = p
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SelectEvalRuns returns filtered eval suite runs.
func (r *Registry) SelectEvalRuns(filters []string) ([]suite.EvalSuiteRun, error) {
	if r == nil {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.evals) == 0 {
		return nil, nil
	}
	parsed, err := suite.ParseFilterPaths(filters)
	if err != nil {
		return nil, err
	}
	var runs []suite.EvalSuiteRun
	for _, s := range r.evals {
		cases := s.SelectEvalCases(parsed)
		if len(cases) == 0 {
			continue
		}
		runs = append(runs, suite.EvalSuiteRun{
			Name:          s.Name(),
			Environments:  s.Environments(),
			EnvRegistry:   r.envRegistryLocked(),
			Setup:         s.SetupHook(),
			Teardown:      s.TeardownHook(),
			Cases:         cases,
			Decorate:      s.Decorator(),
			StopOnFailure: s.StopOnFailure(),
		})
	}
	return runs, nil
}

// ValidateSelection checks case_ids before starting an LRO.
func (r *Registry) ValidateSelection(runType evalspb.Run_Type, filters []string) error {
	if r == nil {
		return ErrNotConfigured{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	switch runType {
	case evalspb.Run_INTEGRATION_TEST:
		suites := r.byType[runType]
		if len(suites) == 0 {
			return ErrNoSuites{Type: runType}
		}
		return validateTestSelection(suites, filters)
	case evalspb.Run_AGENT_EVAL:
		if len(r.evals) == 0 && len(r.evalProviders) == 0 {
			return ErrNoEvalSuites{}
		}
		if len(filters) == 0 {
			return nil
		}
		if len(r.evals) == 0 {
			return nil
		}
		return validateEvalSelection(r.evals, filters)
	case evalspb.Run_LOAD_TEST:
		if len(r.loads) == 0 {
			return ErrNoLoadSuites{}
		}
		return validateLoadSelection(r.loads, filters)
	case evalspb.Run_INFRA_OBSERVATION:
		if len(r.infraObserves) == 0 {
			return ErrNoInfraObserveSuites{}
		}
		return validateInfraObserveSelection(r.infraObserves, filters)
	default:
		return ErrUnsupportedRunType{Type: runType}
	}
}

// validateTestSelection rejects unknown case_ids before an LRO starts. Empty
// filters mean "run everything" and skip per-path existence checks.
func validateTestSelection(suites []*suite.TestSuite, filters []string) error {
	parsed, err := suite.ParseFilterPaths(filters)
	if err != nil {
		return err
	}
	if len(parsed) == 0 {
		return nil
	}
	for _, fp := range parsed {
		if !filterMatchesTestSuite(suites, fp) {
			return ErrUnknownCase{Name: filterPathString(fp)}
		}
	}
	return nil
}

// validateEvalSelection is [validateTestSelection] for agent-eval suites.
func validateEvalSelection(suites []*suite.EvalSuite, filters []string) error {
	parsed, err := suite.ParseFilterPaths(filters)
	if err != nil {
		return err
	}
	if len(parsed) == 0 {
		return nil
	}
	for _, fp := range parsed {
		if !filterMatchesEvalSuite(suites, fp) {
			return ErrUnknownCase{Name: filterPathString(fp)}
		}
	}
	return nil
}

// filterMatchesTestSuite reports whether fp resolves to at least one case
// across the registered integration-test suites.
func filterMatchesTestSuite(suites []*suite.TestSuite, fp suite.FilterPath) bool {
	for _, s := range suites {
		if len(s.SelectTestCases([]suite.FilterPath{fp})) > 0 {
			return true
		}
	}
	return false
}

// filterMatchesEvalSuite is [filterMatchesTestSuite] for agent-eval suites.
func filterMatchesEvalSuite(suites []*suite.EvalSuite, fp suite.FilterPath) bool {
	for _, s := range suites {
		if len(s.SelectEvalCases([]suite.FilterPath{fp})) > 0 {
			return true
		}
	}
	return false
}

// validateLoadSelection is [validateTestSelection] for load-test suites.
func validateLoadSelection(suites []*suite.LoadSuite, filters []string) error {
	parsed, err := suite.ParseFilterPaths(filters)
	if err != nil {
		return err
	}
	if len(parsed) == 0 {
		return nil
	}
	for _, fp := range parsed {
		if !filterMatchesLoadSuite(suites, fp) {
			return ErrUnknownCase{Name: filterPathString(fp)}
		}
	}
	return nil
}

// filterMatchesLoadSuite is [filterMatchesTestSuite] for load-test suites.
func filterMatchesLoadSuite(suites []*suite.LoadSuite, fp suite.FilterPath) bool {
	for _, s := range suites {
		if len(s.SelectLoadCases([]suite.FilterPath{fp})) > 0 {
			return true
		}
	}
	return false
}

// validateInfraObserveSelection is [validateTestSelection] for infra-observation suites.
func validateInfraObserveSelection(suites []*suite.InfraObserveSuite, filters []string) error {
	parsed, err := suite.ParseFilterPaths(filters)
	if err != nil {
		return err
	}
	if len(parsed) == 0 {
		return nil
	}
	for _, fp := range parsed {
		if !filterMatchesInfraObserveSuite(suites, fp) {
			return ErrUnknownCase{Name: filterPathString(fp)}
		}
	}
	return nil
}

// filterMatchesInfraObserveSuite is [filterMatchesTestSuite] for infra-observation suites.
func filterMatchesInfraObserveSuite(suites []*suite.InfraObserveSuite, fp suite.FilterPath) bool {
	for _, s := range suites {
		if len(s.SelectInfraObserveCases([]suite.FilterPath{fp})) > 0 {
			return true
		}
	}
	return false
}

// filterPathString formats a parsed filter for ErrUnknownCase messages.
func filterPathString(fp suite.FilterPath) string {
	if fp.CaseName == "" {
		return fp.Suite
	}
	return suite.QualifiedName(fp.Suite, fp.CaseName)
}
