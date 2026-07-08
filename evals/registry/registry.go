package registry

import (
	"context"

	evalspb "go.alis.build/common/alis/evals/v1"
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
	byType        map[evalspb.Run_Type][]*suite.TestSuite
	evals         []*suite.EvalSuite
	loads         []*suite.LoadSuite
	evalProviders []AgentEvalProvider
}

func New() *Registry {
	return &Registry{
		byType: make(map[evalspb.Run_Type][]*suite.TestSuite),
	}
}

// RegisterIntegrationSuite adds a suite for integration test runs.
func (r *Registry) RegisterIntegrationSuite(s *suite.TestSuite) {
	if r == nil {
		return
	}
	r.byType[evalspb.Run_INTEGRATION_TEST] = append(r.byType[evalspb.Run_INTEGRATION_TEST], s)
}

// RegisterAgentEvalSuite adds an agent evaluation suite.
func (r *Registry) RegisterAgentEvalSuite(s *suite.EvalSuite) {
	if r == nil {
		return
	}
	r.evals = append(r.evals, s)
}

// RegisterLoadSuite adds a load-test suite.
func (r *Registry) RegisterLoadSuite(s *suite.LoadSuite) {
	if r == nil {
		return
	}
	r.loads = append(r.loads, s)
}

// RegisterAgentEvalProvider adds a lazy agent eval provider (for example ADK discovery).
func (r *Registry) RegisterAgentEvalProvider(p AgentEvalProvider) {
	if r == nil || p == nil {
		return
	}
	r.evalProviders = append(r.evalProviders, p)
}

// AgentEvalProviders returns registered lazy agent eval providers.
func (r *Registry) AgentEvalProviders() []AgentEvalProvider {
	if r == nil {
		return nil
	}
	return append([]AgentEvalProvider(nil), r.evalProviders...)
}

// SelectTestRuns returns filtered suite runs for the given type.
func (r *Registry) SelectTestRuns(runType evalspb.Run_Type, filters []string) ([]suite.TestSuiteRun, error) {
	if r == nil {
		return nil, nil
	}
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
			Setup:         s.SetupHook(),
			Teardown:      s.TeardownHook(),
			Cases:         cases,
			Identity:      s.SuiteIdentity(),
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
			Setup:            s.SetupHook(),
			Teardown:         s.TeardownHook(),
			Cases:            cases,
			ProfileOverrides: copyProfileOverrides(s),
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
			Setup:         s.SetupHook(),
			Teardown:      s.TeardownHook(),
			Cases:         cases,
			Identity:      s.SuiteIdentity(),
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
		if len(r.evalProviders) > 0 {
			return nil
		}
		return validateEvalSelection(r.evals, filters)
	case evalspb.Run_LOAD_TEST:
		if len(r.loads) == 0 {
			return ErrNoLoadSuites{}
		}
		return validateLoadSelection(r.loads, filters)
	default:
		return ErrUnsupportedRunType{Type: runType}
	}
}

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

func filterMatchesTestSuite(suites []*suite.TestSuite, fp suite.FilterPath) bool {
	for _, s := range suites {
		if len(s.SelectTestCases([]suite.FilterPath{fp})) > 0 {
			return true
		}
	}
	return false
}

func filterMatchesEvalSuite(suites []*suite.EvalSuite, fp suite.FilterPath) bool {
	for _, s := range suites {
		if len(s.SelectEvalCases([]suite.FilterPath{fp})) > 0 {
			return true
		}
	}
	return false
}

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

func filterMatchesLoadSuite(suites []*suite.LoadSuite, fp suite.FilterPath) bool {
	for _, s := range suites {
		if len(s.SelectLoadCases([]suite.FilterPath{fp})) > 0 {
			return true
		}
	}
	return false
}

func filterPathString(fp suite.FilterPath) string {
	if fp.CaseName == "" {
		return fp.Suite
	}
	return suite.QualifiedName(fp.Suite, fp.CaseName)
}
