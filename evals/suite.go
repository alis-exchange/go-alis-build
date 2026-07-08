package evals

import (
	"context"
	"fmt"

	"go.alis.build/evals/execution"
	"go.alis.build/evals/suite"
	iam "go.alis.build/iam/v3"
)

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

// WithIdentity simulates a specific caller for every case in the suite.
func WithIdentity(identity *iam.Identity) SuiteOption {
	return configApplier{
		test: suite.WithIdentity(identity),
		eval: suite.WithEvalIdentity(identity),
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

// NewSuite constructs an integration-test suite. Panics on invalid config
// (empty or dotted name, unknown environment).
func NewSuite(name string, opts ...SuiteOption) *Suite {
	s, err := suite.NewTestSuite(name)
	if err != nil {
		panic(fmt.Errorf("evals.NewSuite: %w", err))
	}
	for _, opt := range opts {
		if err := opt.applyTest(s); err != nil {
			panic(fmt.Errorf("evals.NewSuite %q: %w", name, err))
		}
	}
	return &Suite{kind: KindTest, test: s}
}

// NewEvalSuite constructs an agent-eval suite. Panics on invalid config.
func NewEvalSuite(name string, opts ...SuiteOption) *Suite {
	s, err := suite.NewEvalSuite(name)
	if err != nil {
		panic(fmt.Errorf("evals.NewEvalSuite: %w", err))
	}
	for _, opt := range opts {
		if err := opt.applyEval(s); err != nil {
			panic(fmt.Errorf("evals.NewEvalSuite %q: %w", name, err))
		}
	}
	return &Suite{kind: KindEval, eval: s}
}

// Case registers a case under the suite. Panics on invalid case names or
// duplicates within the suite.
func (s *Suite) Case(name string, fn CaseFunc) *Suite {
	if s == nil {
		panic("evals.Suite.Case: nil suite")
	}
	if fn == nil {
		panic(fmt.Errorf("evals.Suite.Case %q: nil func", name))
	}
	switch s.kind {
	case KindTest:
		if err := s.test.AddCase(&testCaseAdapter{name: name, fn: fn}); err != nil {
			panic(fmt.Errorf("evals.Suite.Case %q: %w", name, err))
		}
	case KindEval:
		if err := s.eval.AddCase(&evalCaseAdapter{name: name, fn: fn}); err != nil {
			panic(fmt.Errorf("evals.Suite.Case %q: %w", name, err))
		}
	default:
		panic(fmt.Errorf("evals.Suite.Case: unknown kind %d", s.kind))
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
