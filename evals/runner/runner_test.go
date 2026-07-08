package runner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/env"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/internal/result"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/suite"
)

type stubTestCase struct {
	name   string
	result *execution.CaseResult
}

func (c stubTestCase) Name() string { return c.name }

func (c stubTestCase) Run(context.Context) *execution.CaseResult {
	return c.result
}

type stubEvalCase struct {
	name   string
	result *execution.CaseResult
}

func (c stubEvalCase) Name() string { return c.name }

func (c stubEvalCase) Run(context.Context) *execution.CaseResult {
	return c.result
}

func passedCase(name string) *execution.CaseResult {
	return &execution.CaseResult{Name: name, Status: evalspb.Status_PASSED}
}

func testSuiteRun(cases ...suite.TestCase) []suite.TestSuiteRun {
	return []suite.TestSuiteRun{{Cases: cases}}
}

func evalSuiteRun(cases ...suite.EvalCase) []suite.EvalSuiteRun {
	return []suite.EvalSuiteRun{{Cases: cases}}
}

func TestRollupRunStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []evalspb.Status
		want evalspb.Status
	}{
		{"empty", nil, evalspb.Status_PASSED},
		{"all passed", []evalspb.Status{evalspb.Status_PASSED, evalspb.Status_PASSED}, evalspb.Status_PASSED},
		{"one failed", []evalspb.Status{evalspb.Status_PASSED, evalspb.Status_FAILED}, evalspb.Status_FAILED},
		{"failed wins", []evalspb.Status{evalspb.Status_FAILED, evalspb.Status_FAILED}, evalspb.Status_FAILED},
		{"unspecified counts as failed", []evalspb.Status{evalspb.Status_PASSED, evalspb.Status_STATUS_UNSPECIFIED}, evalspb.Status_FAILED},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := result.RollupRunStatus(tt.in); got != tt.want {
				t.Fatalf("RollupRunStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunner_RunTestSuites_allPassed(t *testing.T) {
	t.Parallel()

	runner := New()
	runs := testSuiteRun(
		stubTestCase{name: "a", result: passedCase("a")},
		stubTestCase{name: "b", result: passedCase("b")},
	)

	suites, err := runner.RunTestSuites(context.Background(), runs, nil)
	if err != nil {
		t.Fatalf("RunTestSuites() error = %v", err)
	}
	if len(suites) != 1 {
		t.Fatalf("len(suites) = %d, want 1", len(suites))
	}
	if RollupSuiteStatus(suites[0]) != evalspb.Status_PASSED {
		t.Fatalf("status = %v, want PASSED", RollupSuiteStatus(suites[0]))
	}
	if len(suites[0].Cases) != 2 {
		t.Fatalf("len(cases) = %d, want 2", len(suites[0].Cases))
	}
}

func TestRunner_RunTestSuites_mixFailed(t *testing.T) {
	t.Parallel()

	runner := New()
	runs := testSuiteRun(
		stubTestCase{result: passedCase("")},
		stubTestCase{result: &execution.CaseResult{Status: evalspb.Status_FAILED}},
	)

	suites, err := runner.RunTestSuites(context.Background(), runs, nil)
	if err != nil {
		t.Fatalf("RunTestSuites() error = %v", err)
	}
	if RollupSuiteStatus(suites[0]) != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED", RollupSuiteStatus(suites[0]))
	}
}

func TestRunner_RunTestSuites_empty(t *testing.T) {
	t.Parallel()

	suites, err := New().RunTestSuites(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("RunTestSuites() error = %v", err)
	}
	if len(suites) != 0 {
		t.Fatalf("len(suites) = %d, want 0", len(suites))
	}
}

func TestRunner_RunTestSuites_withProgress(t *testing.T) {
	t.Parallel()

	var calls [][2]int
	progress := func(completed, total int) {
		calls = append(calls, [2]int{completed, total})
	}
	runs := testSuiteRun(
		stubTestCase{result: passedCase("")},
		stubTestCase{result: passedCase("")},
		stubTestCase{result: passedCase("")},
	)

	if _, err := New().RunTestSuites(context.Background(), runs, progress); err != nil {
		t.Fatalf("RunTestSuites() error = %v", err)
	}
	want := [][2]int{{1, 3}, {2, 3}, {3, 3}}
	if len(calls) != len(want) {
		t.Fatalf("progress calls = %d, want %d", len(calls), len(want))
	}
	for i, w := range want {
		if calls[i] != w {
			t.Fatalf("call[%d] = %v, want %v", i, calls[i], w)
		}
	}
}

func TestRunner_RunTestSuites_cancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := New().RunTestSuites(ctx, testSuiteRun(
		stubTestCase{result: passedCase("")},
	), nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestRunner_RunTestSuites_suiteLifecycle(t *testing.T) {
	t.Parallel()

	var setupCalls, teardownCalls atomic.Int32
	s, err := suite.NewTestSuite("files-v2",
		suite.WithSetup(func(context.Context) error {
			setupCalls.Add(1)
			return nil
		}),
		suite.WithTeardown(func(context.Context) error {
			teardownCalls.Add(1)
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s.AddCases(
		stubTestCase{name: "upload", result: passedCase("upload")},
		stubTestCase{name: "delete", result: passedCase("delete")},
	); err != nil {
		t.Fatalf("AddCases: %v", err)
	}

	runs := []suite.TestSuiteRun{{
		Name:     s.Name(),
		Setup:    s.SetupHook(),
		Teardown: s.TeardownHook(),
		Cases:    s.Cases(),
	}}

	suites, err := New().RunTestSuites(context.Background(), runs, nil)
	if err != nil {
		t.Fatalf("RunTestSuites: %v", err)
	}
	if setupCalls.Load() != 1 || teardownCalls.Load() != 1 {
		t.Fatalf("setup=%d teardown=%d, want 1 each", setupCalls.Load(), teardownCalls.Load())
	}
	if len(suites[0].Cases) != 2 {
		t.Fatalf("len = %d, want 2", len(suites[0].Cases))
	}
	if suites[0].Cases[0].Name != "files-v2.upload" {
		t.Fatalf("name = %q", suites[0].Cases[0].Name)
	}
	if suites[0].SuiteName != "files-v2" {
		t.Fatalf("suiteName = %q, want files-v2", suites[0].SuiteName)
	}
}

func TestRunner_RunTestSuites_setupFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("populate failed")
	var teardownCalls atomic.Int32
	runs := []suite.TestSuiteRun{{
		Setup: func(context.Context) error { return wantErr },
		Teardown: func(context.Context) error {
			teardownCalls.Add(1)
			return nil
		},
		Cases: []suite.TestCase{
			stubTestCase{name: "files-v2.upload"},
			stubTestCase{name: "files-v2.delete"},
		},
	}}

	suites, err := New().RunTestSuites(context.Background(), runs, nil)
	if err != nil {
		t.Fatalf("RunTestSuites: %v", err)
	}
	if teardownCalls.Load() != 0 {
		t.Fatal("teardown ran after failed setup")
	}
	if RollupSuiteStatus(suites[0]) != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED", RollupSuiteStatus(suites[0]))
	}
	for _, r := range suites[0].Cases {
		if len(r.Checks) == 0 || r.Checks[0].ID != result.SetupErrorCheckName {
			t.Fatalf("check = %+v, want setup error", r.Checks)
		}
	}
}

func TestRunner_RunTestSuites_recordsDuration(t *testing.T) {
	t.Parallel()

	runs := testSuiteRun(stubTestCase{
		result: passedCase("a"),
	})
	suites, err := New().RunTestSuites(context.Background(), runs, nil)
	if err != nil {
		t.Fatalf("RunTestSuites: %v", err)
	}
	if suites[0].Cases[0].Duration < 0 {
		t.Fatalf("duration = %v, want >= 0", suites[0].Cases[0].Duration)
	}
	if !suites[0].EndTime.After(suites[0].StartTime) && suites[0].EndTime != suites[0].StartTime {
		if suites[0].EndTime.Before(suites[0].StartTime) {
			t.Fatalf("end before start: %v %v", suites[0].StartTime, suites[0].EndTime)
		}
	}
	_ = time.Millisecond
}

func TestRunner_RunEvalSuites_allPassed(t *testing.T) {
	t.Parallel()

	runner := New()
	runs := evalSuiteRun(
		stubEvalCase{result: passedCase("")},
	)

	suites, err := runner.RunEvalSuites(context.Background(), runs, nil)
	if err != nil {
		t.Fatalf("RunEvalSuites() error = %v", err)
	}
	if RollupSuiteStatus(suites[0]) != evalspb.Status_PASSED {
		t.Fatalf("status = %v, want PASSED", RollupSuiteStatus(suites[0]))
	}
}

func TestRunner_RunEvalSuites_oneError(t *testing.T) {
	t.Parallel()

	runner := New()
	runs := evalSuiteRun(
		stubEvalCase{result: &execution.CaseResult{Status: evalspb.Status_FAILED}},
	)

	suites, err := runner.RunEvalSuites(context.Background(), runs, nil)
	if err != nil {
		t.Fatalf("RunEvalSuites() error = %v", err)
	}
	if RollupSuiteStatus(suites[0]) != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED", RollupSuiteStatus(suites[0]))
	}
}

type ctxKey struct{}

type ctxValueCapturingCase struct {
	name string
	seen *string
}

func (c ctxValueCapturingCase) Name() string { return c.name }

func (c ctxValueCapturingCase) Run(ctx context.Context) *execution.CaseResult {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		*c.seen = v
	} else {
		*c.seen = ""
	}
	return passedCase(c.name)
}

func stampDecorator(value string) suite.ContextDecorator {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, ctxKey{}, value)
	}
}

func TestRunner_appliesDecoratorToCases(t *testing.T) {
	t.Parallel()

	var seen string
	runner := New(WithContext(stampDecorator("runner-value")))
	_, err := runner.RunTestSuites(context.Background(), []suite.TestSuiteRun{{
		Cases: []suite.TestCase{ctxValueCapturingCase{name: "capture", seen: &seen}},
	}}, nil)
	if err != nil {
		t.Fatalf("RunTestSuites() error = %v", err)
	}
	if seen != "runner-value" {
		t.Fatalf("seen = %q, want %q", seen, "runner-value")
	}
}

func TestRunner_suiteDecoratorOverridesRunner(t *testing.T) {
	t.Parallel()

	var seen string
	runner := New(WithContext(stampDecorator("runner-value")))
	_, err := runner.RunTestSuites(context.Background(), []suite.TestSuiteRun{{
		Decorate: stampDecorator("suite-value"),
		Cases:    []suite.TestCase{ctxValueCapturingCase{name: "capture", seen: &seen}},
	}}, nil)
	if err != nil {
		t.Fatalf("RunTestSuites() error = %v", err)
	}
	if seen != "suite-value" {
		t.Fatalf("seen = %q, want %q", seen, "suite-value")
	}
}

// callerCtxKey and callerValueCase together assert the propagation
// contract: whatever the caller attached to the ctx handed to the runner
// must still be visible inside the case body after decoration.
type callerCtxKey struct{}

type callerValueCase struct {
	name       string
	seenCaller *string
	seenStamp  *string
}

func (c callerValueCase) Name() string { return c.name }

func (c callerValueCase) Run(ctx context.Context) *execution.CaseResult {
	if v, ok := ctx.Value(callerCtxKey{}).(string); ok {
		*c.seenCaller = v
	}
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		*c.seenStamp = v
	}
	return passedCase(c.name)
}

func TestRunner_decoratorPreservesCallerCtxValues(t *testing.T) {
	t.Parallel()

	var seenCaller, seenStamp string
	runner := New(WithContext(stampDecorator("runner-value")))
	ctx := context.WithValue(context.Background(), callerCtxKey{}, "caller-value")
	_, err := runner.RunTestSuites(ctx, []suite.TestSuiteRun{{
		Cases: []suite.TestCase{callerValueCase{name: "capture", seenCaller: &seenCaller, seenStamp: &seenStamp}},
	}}, nil)
	if err != nil {
		t.Fatalf("RunTestSuites() error = %v", err)
	}
	if seenCaller != "caller-value" {
		t.Fatalf("caller value dropped by runner: seenCaller = %q, want %q", seenCaller, "caller-value")
	}
	if seenStamp != "runner-value" {
		t.Fatalf("runner decorator not applied: seenStamp = %q, want %q", seenStamp, "runner-value")
	}
}

type ctxValueCapturingEvalCase struct {
	name string
	seen *string
}

func (c ctxValueCapturingEvalCase) Name() string { return c.name }

func (c ctxValueCapturingEvalCase) Run(ctx context.Context) *execution.CaseResult {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		*c.seen = v
	} else {
		*c.seen = ""
	}
	return passedCase(c.name)
}

func TestRunner_appliesDecoratorToEvalCases(t *testing.T) {
	t.Parallel()

	var seen string
	runner := New(WithContext(stampDecorator("runner-value")))
	_, err := runner.RunEvalSuites(context.Background(), []suite.EvalSuiteRun{{
		Cases: []suite.EvalCase{ctxValueCapturingEvalCase{name: "capture", seen: &seen}},
	}}, nil)
	if err != nil {
		t.Fatalf("RunEvalSuites() error = %v", err)
	}
	if seen != "runner-value" {
		t.Fatalf("seen = %q, want %q", seen, "runner-value")
	}
}

func TestRunner_evalSuiteDecoratorOverridesRunner(t *testing.T) {
	t.Parallel()

	var seen string
	runner := New(WithContext(stampDecorator("runner-value")))
	_, err := runner.RunEvalSuites(context.Background(), []suite.EvalSuiteRun{{
		Decorate: stampDecorator("suite-value"),
		Cases:    []suite.EvalCase{ctxValueCapturingEvalCase{name: "capture", seen: &seen}},
	}}, nil)
	if err != nil {
		t.Fatalf("RunEvalSuites() error = %v", err)
	}
	if seen != "suite-value" {
		t.Fatalf("seen = %q, want %q", seen, "suite-value")
	}
}

type ctxValueCapturingLoadCase struct {
	name string
	seen *string
}

func (c ctxValueCapturingLoadCase) Name() string { return c.name }

func (c ctxValueCapturingLoadCase) Run(ctx context.Context, _ evalspb.RunLoadTestRequest_Mode, _ loadgen.Profile) *execution.LoadCaseResult {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		*c.seen = v
	} else {
		*c.seen = ""
	}
	return passedLoad(c.name)
}

func TestRunner_appliesDecoratorToLoadCases(t *testing.T) {
	t.Parallel()

	var seen string
	runner := New(WithContext(stampDecorator("runner-value")))
	runs := []suite.LoadSuiteRun{{
		Cases: []suite.LoadCase{ctxValueCapturingLoadCase{name: "capture", seen: &seen}},
	}}
	if _, err := runner.RunLoadSuites(context.Background(), runs, evalspb.RunLoadTestRequest_MINIMAL, defaultResolver(), nil); err != nil {
		t.Fatalf("RunLoadSuites() error = %v", err)
	}
	if seen != "runner-value" {
		t.Fatalf("seen = %q, want %q", seen, "runner-value")
	}
}

func TestRunner_environmentHooksReceiveDecorator(t *testing.T) {
	t.Parallel()

	envName := "runner-decorate-env-" + t.Name()
	var setupSeen, teardownSeen atomic.Value
	setupSeen.Store("")
	teardownSeen.Store("")

	if err := env.Register(envName,
		env.WithSetup(func(ctx context.Context) error {
			if v, ok := ctx.Value(ctxKey{}).(string); ok {
				setupSeen.Store(v)
			}
			return nil
		}),
		env.WithTeardown(func(ctx context.Context) error {
			if v, ok := ctx.Value(ctxKey{}).(string); ok {
				teardownSeen.Store(v)
			}
			return nil
		}),
	); err != nil {
		t.Fatalf("env.Register: %v", err)
	}

	s, err := suite.NewTestSuite("env-decorate", suite.WithEnvironment(envName))
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s.AddCase(stubTestCase{name: "one", result: passedCase("env-decorate.one")}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}

	runner := New(WithContext(stampDecorator("runner-value")))
	runs := []suite.TestSuiteRun{{
		Name:         s.Name(),
		Environments: s.Environments(),
		Cases:        s.Cases(),
	}}
	if _, err := runner.RunTestSuites(context.Background(), runs, nil); err != nil {
		t.Fatalf("RunTestSuites: %v", err)
	}
	if got := setupSeen.Load().(string); got != "runner-value" {
		t.Fatalf("env setup ctx value = %q, want %q", got, "runner-value")
	}
	if got := teardownSeen.Load().(string); got != "runner-value" {
		t.Fatalf("env teardown ctx value = %q, want %q", got, "runner-value")
	}
}

func TestRunner_envRunsOnceForTwoSuites(t *testing.T) {
	t.Parallel()

	envName := "runner-shared-" + t.Name()
	var setupCalls atomic.Int32
	if err := env.Register(envName,
		env.WithSetup(func(context.Context) error {
			setupCalls.Add(1)
			return nil
		}),
	); err != nil {
		t.Fatalf("env.Register: %v", err)
	}

	s1, err := suite.NewTestSuite("a", suite.WithEnvironment(envName))
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s1.AddCase(stubTestCase{name: "one", result: passedCase("a.one")}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}

	s2, err := suite.NewTestSuite("b", suite.WithEnvironment(envName))
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s2.AddCase(stubTestCase{name: "two", result: passedCase("b.two")}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}

	runs := []suite.TestSuiteRun{
		{Name: s1.Name(), Environments: s1.Environments(), Cases: s1.Cases()},
		{Name: s2.Name(), Environments: s2.Environments(), Cases: s2.Cases()},
	}
	if _, err := New().RunTestSuites(context.Background(), runs, nil); err != nil {
		t.Fatalf("RunTestSuites: %v", err)
	}
	if setupCalls.Load() != 1 {
		t.Fatalf("env setup calls = %d, want 1", setupCalls.Load())
	}
}

func TestRunner_envSetupFailureMarksAllCases(t *testing.T) {
	t.Parallel()

	envName := "runner-fail-" + t.Name()
	wantErr := errors.New("env init failed")
	if err := env.Register(envName, env.WithSetup(func(context.Context) error { return wantErr })); err != nil {
		t.Fatalf("env.Register: %v", err)
	}

	s, err := suite.NewTestSuite("files-v2", suite.WithEnvironment(envName))
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s.AddCases(
		stubTestCase{name: "upload"},
		stubTestCase{name: "delete"},
	); err != nil {
		t.Fatalf("AddCases: %v", err)
	}

	suites, err := New().RunTestSuites(context.Background(), []suite.TestSuiteRun{{
		Name:         s.Name(),
		Environments: s.Environments(),
		Cases:        s.Cases(),
	}}, nil)
	if err != nil {
		t.Fatalf("RunTestSuites: %v", err)
	}
	if RollupSuiteStatus(suites[0]) != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED", RollupSuiteStatus(suites[0]))
	}
	for _, sc := range suites[0].Cases {
		if len(sc.Checks) == 0 || sc.Checks[0].ID != result.SetupErrorCheckName {
			t.Fatalf("case = %+v, want setup error", sc)
		}
	}
}
