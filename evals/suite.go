package evals

import (
	"context"
	"fmt"

	"go.alis.build/evals/execution"
	"go.alis.build/evals/suite"
)

// ContextDecorator transforms an outgoing context before the runner hands
// it to suite hooks and case bodies. Use it with [WithContext] to stamp
// caller identity, auth tokens, tracing state, or any other request-scoped
// value on outgoing calls made from inside a case.
type ContextDecorator = suite.ContextDecorator

// CaseFunc is the shape of a case body: measure the SUT via Call, then record
// leaves on T.
type CaseFunc func(ctx context.Context, t *T)

// SuiteKind distinguishes integration test suites from agent-eval suites.
type SuiteKind int

const (
	// KindTest is the default: cases record checks, results surface as checks
	// on the wire.
	KindTest SuiteKind = iota
	// KindEval marks a suite as agent-eval: recorded leaves surface as metrics
	// on the wire (including score and threshold from t.Score).
	KindEval
)

// String returns a human-readable name for the suite kind.
func (k SuiteKind) String() string {
	switch k {
	case KindTest:
		return "KindTest"
	case KindEval:
		return "KindEval"
	default:
		return fmt.Sprintf("SuiteKind(%d)", int(k))
	}
}

// Suite is the author-facing suite handle. It wraps the internal grouping
// primitives in [suite] and adapts CaseFunc values to the erased case
// interfaces the runner consumes.
type Suite struct {
	kind    SuiteKind
	test    *suite.TestSuite
	eval    *suite.EvalSuite
	options []configApplier
}

// SuiteOption configures a suite at construction time (excluding cases).
type SuiteOption interface {
	applyTest(*suite.TestSuite) error
	applyEval(*suite.EvalSuite) error
}

type configApplier struct {
	test func(*suite.TestSuite) error
	eval func(*suite.EvalSuite) error
}

func (a configApplier) applyTest(s *suite.TestSuite) error {
	if a.test == nil {
		return nil
	}
	return a.test(s)
}

func (a configApplier) applyEval(s *suite.EvalSuite) error {
	if a.eval == nil {
		return nil
	}
	return a.eval(s)
}

// WithEnv declares one or more shared environments the suite requires.
// Environments must have been registered with env.Register before the suite
// is constructed.
func WithEnv(names ...string) SuiteOption {
	return configApplier{
		test: func(s *suite.TestSuite) error {
			return suite.WithEnvironment(names...)(s)
		},
		eval: func(s *suite.EvalSuite) error {
			return suite.WithEvalEnvironment(names...)(s)
		},
	}
}

// WithSetup registers an optional suite-level setup hook.
func WithSetup(h suite.SuiteHook) SuiteOption {
	return configApplier{
		test: suite.WithSetup(h),
		eval: suite.WithEvalSetup(h),
	}
}

// WithTeardown registers an optional suite-level teardown hook.
func WithTeardown(h suite.SuiteHook) SuiteOption {
	return configApplier{
		test: suite.WithTeardown(h),
		eval: suite.WithEvalTeardown(h),
	}
}

// WithContext installs a [ContextDecorator] applied to the context handed
// to the suite's setup, teardown, and every case body. It is the framework's
// only auth-adjacent surface: callers use it to stamp caller identity,
// auth headers, tracing state, or any other request-scoped values on
// outgoing calls issued from inside cases. The framework never inspects
// what a decorator attaches; it only propagates it.
//
// Case authors can further decorate the context they receive inside a
// case body — the ctx handed to a case is always a descendant of the
// caller's ctx (deadlines, cancellation, and values are preserved).
//
// A nil decorator is a no-op.
func WithContext(fn ContextDecorator) SuiteOption {
	return configApplier{
		test: suite.WithContext(fn),
		eval: suite.WithEvalContext(fn),
	}
}

// StopOnFailure marks the suite so remaining cases are recorded as
// NOT_EVALUATED once any case ends with a failed status. Use for stateful
// flows where later steps have no meaning after an earlier failure.
func StopOnFailure() SuiteOption {
	return configApplier{
		test: suite.WithStopOnFailure(),
		eval: suite.WithEvalStopOnFailure(),
	}
}

// NewIntegrationSuite constructs an integration-test suite. Returns a
// typed error on invalid config (empty or dotted name, unknown
// environment, or a failing option). See the [suite] package for the
// typed error values.
func NewIntegrationSuite(name string, opts ...SuiteOption) (*Suite, error) {
	s, err := suite.NewTestSuite(name)
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		if err := opt.applyTest(s); err != nil {
			return nil, err
		}
	}
	return &Suite{kind: KindTest, test: s}, nil
}

// MustNewIntegrationSuite is like [NewIntegrationSuite] but panics on
// error. Use only in package-init style code where a config error should
// halt the process.
func MustNewIntegrationSuite(name string, opts ...SuiteOption) *Suite {
	s, err := NewIntegrationSuite(name, opts...)
	if err != nil {
		panic(err)
	}
	return s
}

// NewAgentEvalSuite constructs an agent-eval suite. Returns a typed
// error on invalid config. See the [suite] package for the typed error
// values.
func NewAgentEvalSuite(name string, opts ...SuiteOption) (*Suite, error) {
	s, err := suite.NewEvalSuite(name)
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		if err := opt.applyEval(s); err != nil {
			return nil, err
		}
	}
	return &Suite{kind: KindEval, eval: s}, nil
}

// MustNewAgentEvalSuite is like [NewAgentEvalSuite] but panics on error.
func MustNewAgentEvalSuite(name string, opts ...SuiteOption) *Suite {
	s, err := NewAgentEvalSuite(name, opts...)
	if err != nil {
		panic(err)
	}
	return s
}

// Case registers a case under the suite. Returns a typed error for a
// nil suite ([suite.ErrNilSuite]), a nil func ([ErrNilCaseFunc]), an
// invalid case name ([suite.ErrInvalidCaseName]), or a duplicate
// ([suite.ErrDuplicateCase]). Use [Suite.MustCase] for fluent chaining
// when a registration error should halt the process.
func (s *Suite) Case(name string, fn CaseFunc) error {
	if s == nil {
		return suite.ErrNilSuite{}
	}
	if fn == nil {
		return ErrNilCaseFunc{Case: name}
	}
	switch s.kind {
	case KindTest:
		return s.test.AddCase(&testCaseAdapter{name: name, fn: fn})
	case KindEval:
		return s.eval.AddCase(&evalCaseAdapter{name: name, fn: fn})
	default:
		return ErrUnknownSuiteKind{Kind: s.kind}
	}
}

// MustCase is like [Suite.Case] but panics on error and returns the suite
// for fluent chaining. Intended for package-init style registration where
// a bad case declaration should halt the process.
func (s *Suite) MustCase(name string, fn CaseFunc) *Suite {
	if err := s.Case(name, fn); err != nil {
		panic(err)
	}
	return s
}

// Name returns the suite name.
func (s *Suite) Name() string {
	if s == nil {
		return ""
	}
	switch s.kind {
	case KindTest:
		return s.test.Name()
	case KindEval:
		return s.eval.Name()
	}
	return ""
}

// Kind reports whether the suite is KindTest or KindEval.
func (s *Suite) Kind() SuiteKind {
	if s == nil {
		return KindTest
	}
	return s.kind
}

type testCaseAdapter struct {
	name string
	fn   CaseFunc
}

func (a *testCaseAdapter) Name() string { return a.name }

func (a *testCaseAdapter) Run(ctx context.Context) *execution.CaseResult {
	rec := newT()
	a.fn(ctx, rec)
	checks, status := rec.checksAndStatus()
	return &execution.CaseResult{
		Name:   a.name,
		Status: status,
		Checks: checks,
	}
}

type evalCaseAdapter struct {
	name string
	fn   CaseFunc
}

func (a *evalCaseAdapter) Name() string { return a.name }

func (a *evalCaseAdapter) Run(ctx context.Context) *execution.CaseResult {
	rec := newT()
	a.fn(ctx, rec)
	metrics, status := rec.metricsAndStatus()
	return &execution.CaseResult{
		Name:    a.name,
		Status:  status,
		Metrics: metrics,
	}
}
