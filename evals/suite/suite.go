package suite

import (
	"context"
	"fmt"
	"strings"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/env"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/internal/result"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/loadinfra"
)

// SuiteHook runs once per suite execution (before or after selected cases).
type SuiteHook func(context.Context) error

// ContextDecorator transforms an outgoing context before the runner hands
// it to suite hooks and case bodies. It is the framework's only
// auth-adjacent surface: callers stamp caller identity, auth tokens,
// tracing state, or any other request-scoped data by returning a derived
// context. The framework itself never inspects the values it carries.
type ContextDecorator func(context.Context) context.Context

// TestSuite groups test cases that share optional environment and lifecycle hooks.
// Case names are qualified as "{suite}.{case}" at construction time.
type TestSuite struct {
	// name is the unqualified suite identifier used in filters and qualified case names.
	name string
	// environments lists shared env.Get names required before cases run.
	environments []string
	// setup runs once before selected cases when non-nil.
	setup SuiteHook
	// teardown runs once after selected cases when non-nil.
	teardown SuiteHook
	// cases holds qualified test cases in registration order.
	cases []TestCase
	// decorate overrides the runner-level ContextDecorator for this suite.
	decorate ContextDecorator
	// stopOnFailure skips remaining cases after the first non-PASSED result.
	stopOnFailure bool
}

// EvalSuite groups eval cases that share optional environment and lifecycle hooks.
type EvalSuite struct {
	// name is the unqualified suite identifier used in filters and qualified case names.
	name string
	// environments lists shared env.Get names required before cases run.
	environments []string
	// setup runs once before selected cases when non-nil.
	setup SuiteHook
	// teardown runs once after selected cases when non-nil.
	teardown SuiteHook
	// cases holds qualified eval cases in registration order.
	cases []EvalCase
	// decorate overrides the runner-level ContextDecorator for this suite.
	decorate ContextDecorator
	// stopOnFailure skips remaining cases after the first non-PASSED result.
	stopOnFailure bool
}

// TestSuiteOption configures a TestSuite at construction time (excluding cases).
type TestSuiteOption func(*TestSuite) error

// EvalSuiteOption configures an EvalSuite at construction time (excluding cases).
type EvalSuiteOption func(*EvalSuite) error

// WithSetup registers optional suite-level setup.
func WithSetup(h SuiteHook) TestSuiteOption {
	return func(s *TestSuite) error {
		s.setup = h
		return nil
	}
}

// WithTeardown registers optional suite-level teardown.
func WithTeardown(h SuiteHook) TestSuiteOption {
	return func(s *TestSuite) error {
		s.teardown = h
		return nil
	}
}

// WithEvalSetup registers optional suite-level setup for eval suites.
func WithEvalSetup(h SuiteHook) EvalSuiteOption {
	return func(s *EvalSuite) error {
		s.setup = h
		return nil
	}
}

// WithEvalTeardown registers optional suite-level teardown for eval suites.
func WithEvalTeardown(h SuiteHook) EvalSuiteOption {
	return func(s *EvalSuite) error {
		s.teardown = h
		return nil
	}
}

// WithEnvironment declares shared environments required by the suite.
func WithEnvironment(names ...string) TestSuiteOption {
	return func(s *TestSuite) error {
		return addEnvironments(&s.environments, names)
	}
}

// WithEvalEnvironment declares shared environments required by the eval suite.
func WithEvalEnvironment(names ...string) EvalSuiteOption {
	return func(s *EvalSuite) error {
		return addEnvironments(&s.environments, names)
	}
}

// WithContext installs a [ContextDecorator] applied to the context passed
// to the suite's setup, teardown, and every case body. It is the seam
// through which callers stamp caller identity, auth headers, or any
// other request-scoped values on outgoing calls. A nil decorator is a
// no-op.
func WithContext(fn ContextDecorator) TestSuiteOption {
	return func(s *TestSuite) error {
		s.decorate = fn
		return nil
	}
}

// WithEvalContext is [WithContext] for eval suites.
func WithEvalContext(fn ContextDecorator) EvalSuiteOption {
	return func(s *EvalSuite) error {
		s.decorate = fn
		return nil
	}
}

// WithStopOnFailure marks the suite so the runner records remaining cases as
// NOT_EVALUATED once any case ends with a failed status. Use for stateful
// flows where later cases have no meaning after an earlier step fails.
func WithStopOnFailure() TestSuiteOption {
	return func(s *TestSuite) error {
		s.stopOnFailure = true
		return nil
	}
}

// WithEvalStopOnFailure is [WithStopOnFailure] for eval suites.
func WithEvalStopOnFailure() EvalSuiteOption {
	return func(s *EvalSuite) error {
		s.stopOnFailure = true
		return nil
	}
}

// NewTestSuite creates a test suite. Pass optional configuration via WithEnvironment,
// WithSetup, and WithTeardown; register cases with AddCase or AddCases.
func NewTestSuite(name string, opts ...TestSuiteOption) (*TestSuite, error) {
	if err := validateSuiteName(name); err != nil {
		return nil, err
	}
	s := &TestSuite{name: name}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// NewEvalSuite creates an eval suite. Register cases with AddCase or AddCases.
func NewEvalSuite(name string, opts ...EvalSuiteOption) (*EvalSuite, error) {
	if err := validateSuiteName(name); err != nil {
		return nil, err
	}
	s := &EvalSuite{name: name}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// AddCase registers one test case under the suite.
func (s *TestSuite) AddCase(c TestCase) error {
	if s == nil {
		return ErrNilSuite{}
	}
	return s.addTestCase(c)
}

// AddCases registers multiple test cases under the suite.
func (s *TestSuite) AddCases(cases ...TestCase) error {
	if s == nil {
		return ErrNilSuite{}
	}
	for _, c := range cases {
		if err := s.addTestCase(c); err != nil {
			return err
		}
	}
	return nil
}

// addEnvironments appends registered environment names to dst, rejecting unknown names.
func addEnvironments(dst *[]string, names []string) error {
	for _, name := range names {
		if env.Get(name) == nil {
			return ErrUnknownEnvironment{Name: name}
		}
		if !containsString(*dst, name) {
			*dst = append(*dst, name)
		}
	}
	return nil
}

// AddCase registers one eval case under the suite.
func (s *EvalSuite) AddCase(c EvalCase) error {
	if s == nil {
		return ErrNilSuite{}
	}
	return s.addEvalCase(c)
}

// AddCases registers multiple eval cases under the suite.
func (s *EvalSuite) AddCases(cases ...EvalCase) error {
	if s == nil {
		return ErrNilSuite{}
	}
	for _, c := range cases {
		if err := s.addEvalCase(c); err != nil {
			return err
		}
	}
	return nil
}

// addTestCase qualifies and appends one test case, enforcing unique short names.
func (s *TestSuite) addTestCase(c TestCase) error {
	if s == nil {
		return ErrNilSuite{}
	}

	qualified, err := qualifyTestCase(s.name, c, s.cases)
	if err != nil {
		return err
	}
	s.cases = append(s.cases, qualified)
	return nil
}

// addEvalCase qualifies and appends one eval case, enforcing unique short names.
func (s *EvalSuite) addEvalCase(c EvalCase) error {
	if s == nil {
		return ErrNilSuite{}
	}

	qualified, err := qualifyEvalCase(s.name, c, s.cases)
	if err != nil {
		return err
	}
	s.cases = append(s.cases, qualified)
	return nil
}

// qualifyTestCase wraps c with a suite-qualified name and rejects duplicates.
func qualifyTestCase(suiteName string, c TestCase, existing []TestCase) (TestCase, error) {
	short := c.Name()
	if strings.Contains(short, ".") {
		return nil, ErrInvalidCaseName{Name: short}
	}
	qualified := QualifiedName(suiteName, short)
	for _, e := range existing {
		if e.Name() == qualified {
			return nil, ErrDuplicateCase{Suite: suiteName, Case: short}
		}
	}
	return &qualifiedTestCase{name: qualified, inner: c}, nil
}

// qualifyEvalCase is [qualifyTestCase] for eval cases.
func qualifyEvalCase(suiteName string, c EvalCase, existing []EvalCase) (EvalCase, error) {
	short := c.Name()
	if strings.Contains(short, ".") {
		return nil, ErrInvalidCaseName{Name: short}
	}
	qualified := QualifiedName(suiteName, short)
	for _, e := range existing {
		if e.Name() == qualified {
			return nil, ErrDuplicateCase{Suite: suiteName, Case: short}
		}
	}
	return &qualifiedEvalCase{name: qualified, inner: c}, nil
}

// containsString reports whether want appears in ss.
func containsString(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// TestSuiteRun is a filtered suite ready for runner execution.
type TestSuiteRun struct {
	Name          string
	Environments  []string
	Setup         SuiteHook
	Teardown      SuiteHook
	Cases         []TestCase
	Decorate      ContextDecorator
	StopOnFailure bool
}

// EvalSuiteRun is a filtered eval suite ready for runner execution.
type EvalSuiteRun struct {
	Name          string
	Environments  []string
	Setup         SuiteHook
	Teardown      SuiteHook
	Cases         []EvalCase
	Decorate      ContextDecorator
	StopOnFailure bool
}

// Name returns the suite name (unqualified).
func (s *TestSuite) Name() string {
	if s != nil {
		return s.name
	}
	return ""
}

// Cases returns the registered test cases in registration order. The
// returned slice aliases internal state; treat it as read-only.
func (s *TestSuite) Cases() []TestCase {
	if s != nil {
		return s.cases
	}
	return nil
}

// SetupHook returns the pre-cases hook, or nil when none was configured.
func (s *TestSuite) SetupHook() SuiteHook {
	if s != nil {
		return s.setup
	}
	return nil
}

// TeardownHook returns the post-cases hook, or nil when none was configured.
func (s *TestSuite) TeardownHook() SuiteHook {
	if s != nil {
		return s.teardown
	}
	return nil
}

// Environments returns a copy of the shared environment names declared
// by the suite. Safe to mutate.
func (s *TestSuite) Environments() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.environments...)
}

// Environments returns a copy of the shared environment names declared
// by the eval suite. Safe to mutate.
func (s *EvalSuite) Environments() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.environments...)
}

// Decorator returns the [ContextDecorator] applied to every case in the
// suite, or nil to fall back to the runner default.
func (s *TestSuite) Decorator() ContextDecorator {
	if s != nil {
		return s.decorate
	}
	return nil
}

// StopOnFailure reports whether the suite should skip remaining cases after a
// case ends with a failed status.
func (s *TestSuite) StopOnFailure() bool {
	return s != nil && s.stopOnFailure
}

// Name returns the eval suite name (unqualified).
func (s *EvalSuite) Name() string {
	if s != nil {
		return s.name
	}
	return ""
}

// Cases returns the registered eval cases in registration order. The
// returned slice aliases internal state; treat it as read-only.
func (s *EvalSuite) Cases() []EvalCase {
	if s != nil {
		return s.cases
	}
	return nil
}

// SetupHook returns the pre-cases hook, or nil when none was configured.
func (s *EvalSuite) SetupHook() SuiteHook {
	if s != nil {
		return s.setup
	}
	return nil
}

// TeardownHook returns the post-cases hook, or nil when none was configured.
func (s *EvalSuite) TeardownHook() SuiteHook {
	if s != nil {
		return s.teardown
	}
	return nil
}

// Decorator returns the [ContextDecorator] applied to every case in the
// eval suite, or nil to fall back to the runner default.
func (s *EvalSuite) Decorator() ContextDecorator {
	if s != nil {
		return s.decorate
	}
	return nil
}

// StopOnFailure reports whether the eval suite should skip remaining cases
// after a case ends with a failed status.
func (s *EvalSuite) StopOnFailure() bool {
	return s != nil && s.stopOnFailure
}

// QualifiedName returns the canonical filter/result name for a case in a suite.
func QualifiedName(suite, short string) string {
	return suite + "." + short
}

// validateSuiteName rejects empty names and names containing '.'.
func validateSuiteName(name string) error {
	if name == "" {
		return ErrInvalidSuiteName{Reason: "suite name is required"}
	}
	if strings.Contains(name, ".") {
		return ErrInvalidSuiteName{Name: name, Reason: "must not contain '.'"}
	}
	return nil
}

// FilterPath is a parsed case filter ("suite" or "suite.case").
type FilterPath struct {
	Suite    string
	CaseName string // empty means whole suite
}

// ParseFilterPaths parses filter strings for registry selection.
func ParseFilterPaths(paths []string) ([]FilterPath, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	out := make([]FilterPath, len(paths))
	for i, p := range paths {
		fp, err := parseFilterPath(p)
		if err != nil {
			return nil, err
		}
		out[i] = fp
	}
	return out, nil
}

// parseFilterPath splits one filter string into suite and optional case segments.
func parseFilterPath(s string) (FilterPath, error) {
	if s == "" {
		return FilterPath{}, ErrInvalidFilterPath{Path: s, err: fmt.Errorf("empty filter path")}
	}
	if strings.Count(s, ".") > 1 {
		return FilterPath{}, ErrInvalidFilterPath{Path: s, err: fmt.Errorf("at most one '.' allowed")}
	}
	suite, caseName, hasCase := strings.Cut(s, ".")
	if suite == "" {
		return FilterPath{}, ErrInvalidFilterPath{Path: s}
	}
	if hasCase && caseName == "" {
		return FilterPath{}, ErrInvalidFilterPath{Path: s}
	}
	return FilterPath{Suite: suite, CaseName: caseName}, nil
}

// SelectTestCases returns cases from s matching parsed filters.
func (s *TestSuite) SelectTestCases(filters []FilterPath) []TestCase {
	return selectTestCasesInSuite(s, filters)
}

// SelectEvalCases returns cases from s matching parsed filters.
func (s *EvalSuite) SelectEvalCases(filters []FilterPath) []EvalCase {
	return selectEvalCasesInSuite(s, filters)
}

// selectTestCasesInSuite returns cases from suite matching parsed filters.
func selectTestCasesInSuite(suite *TestSuite, filters []FilterPath) []TestCase {
	if suite == nil {
		return nil
	}
	if len(filters) == 0 {
		return append([]TestCase(nil), suite.cases...)
	}
	wantAll := false
	selected := make(map[string]bool)
	mentioned := false
	for _, f := range filters {
		if f.Suite != suite.name {
			continue
		}
		mentioned = true
		if f.CaseName == "" {
			wantAll = true
			break
		}
		selected[QualifiedName(suite.name, f.CaseName)] = true
	}
	if !mentioned {
		return nil
	}
	if wantAll {
		return append([]TestCase(nil), suite.cases...)
	}
	out := make([]TestCase, 0, len(selected))
	for _, c := range suite.cases {
		if selected[c.Name()] {
			out = append(out, c)
		}
	}
	return out
}

// selectEvalCasesInSuite is [selectTestCasesInSuite] for eval suites.
func selectEvalCasesInSuite(suite *EvalSuite, filters []FilterPath) []EvalCase {
	if suite == nil {
		return nil
	}
	if len(filters) == 0 {
		return append([]EvalCase(nil), suite.cases...)
	}
	wantAll := false
	selected := make(map[string]bool)
	mentioned := false
	for _, f := range filters {
		if f.Suite != suite.name {
			continue
		}
		mentioned = true
		if f.CaseName == "" {
			wantAll = true
			break
		}
		selected[QualifiedName(suite.name, f.CaseName)] = true
	}
	if !mentioned {
		return nil
	}
	if wantAll {
		return append([]EvalCase(nil), suite.cases...)
	}
	out := make([]EvalCase, 0, len(selected))
	for _, c := range suite.cases {
		if selected[c.Name()] {
			out = append(out, c)
		}
	}
	return out
}

// TotalTestCases counts cases across suite runs (for LRO progress metadata).
func TotalTestCases(runs []TestSuiteRun) int {
	n := 0
	for _, run := range runs {
		n += len(run.Cases)
	}
	return n
}

// TotalEvalCases counts cases across eval suite runs.
func TotalEvalCases(runs []EvalSuiteRun) int {
	n := 0
	for _, run := range runs {
		n += len(run.Cases)
	}
	return n
}

// TotalLoadCases counts cases across load suite runs.
func TotalLoadCases(runs []LoadSuiteRun) int {
	n := 0
	for _, run := range runs {
		n += len(run.Cases)
	}
	return n
}

// LoadSuite groups load cases that share environment, lifecycle hooks, and
// per-mode profile overrides. Case names are qualified as "{suite}.{case}"
// at construction time. Load cases within a suite always run sequentially:
// running two load windows concurrently against different targets would
// contaminate each other's measurements.
type LoadSuite struct {
	// name is the unqualified suite identifier used in filters and qualified case names.
	name string
	// environments lists shared env.Get names required before cases run.
	environments []string
	// setup runs once before selected cases when non-nil.
	setup SuiteHook
	// teardown runs once after selected cases when non-nil.
	teardown SuiteHook
	// cases holds qualified load cases in registration order.
	cases []LoadCase
	// profileOverrides replaces framework default profiles per RunLoadTestRequest_Mode.
	profileOverrides map[evalspb.RunLoadTestRequest_Mode]loadgen.Profile
	// cloudRun holds declared Cloud Run infra targets for post-load observation.
	cloudRun []loadinfra.CloudRunTarget
	// spanner holds declared Spanner infra targets for post-load observation.
	spanner []loadinfra.SpannerTarget
}

// LoadSuiteOption configures a LoadSuite at construction time.
type LoadSuiteOption func(*LoadSuite) error

// WithLoadEnvironment declares shared environments required by the load suite.
func WithLoadEnvironment(names ...string) LoadSuiteOption {
	return func(s *LoadSuite) error {
		return addEnvironments(&s.environments, names)
	}
}

// WithLoadSetup registers optional suite-level setup for load suites.
func WithLoadSetup(h SuiteHook) LoadSuiteOption {
	return func(s *LoadSuite) error {
		s.setup = h
		return nil
	}
}

// WithLoadTeardown registers optional suite-level teardown for load suites.
func WithLoadTeardown(h SuiteHook) LoadSuiteOption {
	return func(s *LoadSuite) error {
		s.teardown = h
		return nil
	}
}

// WithLoadProfileOverride records a per-mode profile override. When the suite
// runs at the given mode the override replaces the framework defaults.
func WithLoadProfileOverride(mode evalspb.RunLoadTestRequest_Mode, p loadgen.Profile) LoadSuiteOption {
	return func(s *LoadSuite) error {
		if mode == evalspb.RunLoadTestRequest_MODE_UNSPECIFIED {
			return ErrLoadProfileUnspecifiedMode{}
		}
		if s.profileOverrides == nil {
			s.profileOverrides = make(map[evalspb.RunLoadTestRequest_Mode]loadgen.Profile)
		}
		s.profileOverrides[mode] = p
		return nil
	}
}

// NewLoadSuite creates a load suite. Register cases with AddCase.
func NewLoadSuite(name string, opts ...LoadSuiteOption) (*LoadSuite, error) {
	if err := validateSuiteName(name); err != nil {
		return nil, err
	}
	s := &LoadSuite{name: name}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	if err := ValidateInfraTargets(s); err != nil {
		return nil, err
	}
	return s, nil
}

// AddCase registers one load case under the suite.
func (s *LoadSuite) AddCase(c LoadCase) error {
	if s == nil {
		return ErrNilSuite{}
	}
	return s.addLoadCase(c)
}

// addLoadCase qualifies and appends one load case, enforcing unique short names.
func (s *LoadSuite) addLoadCase(c LoadCase) error {
	if s == nil {
		return ErrNilSuite{}
	}
	qualified, err := qualifyLoadCase(s.name, c, s.cases)
	if err != nil {
		return err
	}
	s.cases = append(s.cases, qualified)
	return nil
}

// qualifyLoadCase is [qualifyTestCase] for load cases.
func qualifyLoadCase(suiteName string, c LoadCase, existing []LoadCase) (LoadCase, error) {
	short := c.Name()
	if strings.Contains(short, ".") {
		return nil, ErrInvalidCaseName{Name: short}
	}
	qualified := QualifiedName(suiteName, short)
	for _, e := range existing {
		if e.Name() == qualified {
			return nil, ErrDuplicateCase{Suite: suiteName, Case: short}
		}
	}
	return &qualifiedLoadCase{name: qualified, inner: c}, nil
}

// Name returns the suite name.
func (s *LoadSuite) Name() string {
	if s != nil {
		return s.name
	}
	return ""
}

// Cases returns the qualified load cases in registration order.
func (s *LoadSuite) Cases() []LoadCase {
	if s != nil {
		return s.cases
	}
	return nil
}

// Environments returns a copy of the required environment names.
func (s *LoadSuite) Environments() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.environments...)
}

// SetupHook returns the optional setup hook.
func (s *LoadSuite) SetupHook() SuiteHook {
	if s != nil {
		return s.setup
	}
	return nil
}

// TeardownHook returns the optional teardown hook.
func (s *LoadSuite) TeardownHook() SuiteHook {
	if s != nil {
		return s.teardown
	}
	return nil
}

// ProfileOverride returns the per-mode profile override registered for mode,
// or the zero Profile and false if none exists.
func (s *LoadSuite) ProfileOverride(mode evalspb.RunLoadTestRequest_Mode) (loadgen.Profile, bool) {
	if s == nil {
		return loadgen.Profile{}, false
	}
	p, ok := s.profileOverrides[mode]
	return p, ok
}

// LoadSuiteRun is a filtered load suite ready for runner execution.
type LoadSuiteRun struct {
	Name             string
	Environments     []string
	Setup            SuiteHook
	Teardown         SuiteHook
	Cases            []LoadCase
	ProfileOverrides map[evalspb.RunLoadTestRequest_Mode]loadgen.Profile
	// CloudRun is the suite's declared Cloud Run targets copied at selection time.
	CloudRun []loadinfra.CloudRunTarget
	// Spanner is the suite's declared Spanner targets copied at selection time.
	Spanner []loadinfra.SpannerTarget
}

// SelectLoadCases returns cases from s matching parsed filters.
func (s *LoadSuite) SelectLoadCases(filters []FilterPath) []LoadCase {
	return selectLoadCasesInSuite(s, filters)
}

// selectLoadCasesInSuite is [selectTestCasesInSuite] for load suites.
func selectLoadCasesInSuite(s *LoadSuite, filters []FilterPath) []LoadCase {
	if s == nil {
		return nil
	}
	if len(filters) == 0 {
		return append([]LoadCase(nil), s.cases...)
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
		return append([]LoadCase(nil), s.cases...)
	}
	out := make([]LoadCase, 0, len(selected))
	for _, c := range s.cases {
		if selected[c.Name()] {
			out = append(out, c)
		}
	}
	return out
}

// qualifiedLoadCase wraps a user LoadCase with a suite-qualified Name().
type qualifiedLoadCase struct {
	// name is the canonical "suite.case" identifier stamped on results.
	name string
	// inner is the user-registered case implementation.
	inner LoadCase
}

// Name returns the qualified case name.
func (q *qualifiedLoadCase) Name() string {
	if q == nil {
		return ""
	}
	return q.name
}

// Run delegates to inner and overwrites the result Name with the qualified value.
func (q *qualifiedLoadCase) Run(ctx context.Context, mode evalspb.RunLoadTestRequest_Mode, profile loadgen.Profile) *execution.LoadCaseResult {
	if q == nil || q.inner == nil {
		return &execution.LoadCaseResult{Name: q.Name(), Status: evalspb.Status_FAILED}
	}
	r := q.inner.Run(ctx, mode, profile)
	if r == nil {
		return &execution.LoadCaseResult{Name: q.name, Status: evalspb.Status_FAILED}
	}
	r.Name = q.name
	return r
}

// qualifiedTestCase wraps a user TestCase with a suite-qualified Name().
type qualifiedTestCase struct {
	// name is the canonical "suite.case" identifier stamped on results.
	name string
	// inner is the user-registered case implementation.
	inner TestCase
}

// Name returns the qualified case name.
func (q *qualifiedTestCase) Name() string {
	if q == nil {
		return ""
	}
	return q.name
}

// Run delegates to inner and overwrites the result Name with the qualified value.
func (q *qualifiedTestCase) Run(ctx context.Context) *execution.CaseResult {
	if q == nil {
		return result.SetupErrorResult("", ErrNilCaseResult{})
	}
	if q.inner == nil {
		return result.SetupErrorResult(q.name, ErrNilCaseResult{})
	}
	r := q.inner.Run(ctx)
	if r == nil {
		return result.SetupErrorResult(q.name, ErrNilCaseResult{})
	}
	r.Name = q.name
	return r
}

// qualifiedEvalCase wraps a user EvalCase with a suite-qualified Name().
type qualifiedEvalCase struct {
	// name is the canonical "suite.case" identifier stamped on results.
	name string
	// inner is the user-registered case implementation.
	inner EvalCase
}

// Name returns the qualified case name.
func (q *qualifiedEvalCase) Name() string {
	if q == nil {
		return ""
	}
	return q.name
}

// Run delegates to inner and overwrites the result Name with the qualified value.
func (q *qualifiedEvalCase) Run(ctx context.Context) *execution.CaseResult {
	if q == nil {
		return result.EvalSetupErrorResult("", ErrNilCaseResult{})
	}
	if q.inner == nil {
		return result.EvalSetupErrorResult(q.name, ErrNilCaseResult{})
	}
	r := q.inner.Run(ctx)
	if r == nil {
		return result.EvalSetupErrorResult(q.name, ErrNilCaseResult{})
	}
	r.Name = q.name
	return r
}
