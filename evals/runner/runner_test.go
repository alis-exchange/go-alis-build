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
	"go.alis.build/evals/suite"
	iam "go.alis.build/iam/v3"
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

type identityCapturingCase struct {
	name string
	seen **iam.Identity
}

func (c identityCapturingCase) Name() string { return c.name }

func (c identityCapturingCase) Run(ctx context.Context) *execution.CaseResult {
	id, err := iam.FromContext(ctx)
	if err != nil {
		return result.SetupErrorResult(c.name, err)
	}
	*c.seen = id
	return passedCase(c.name)
}

func TestRunner_appliesIdentityToCases(t *testing.T) {
	t.Parallel()

	custom := &iam.Identity{Type: iam.User, ID: "runner-user", Email: "runner@example.com"}
	var seen *iam.Identity

	runner := New(WithIdentity(custom))
	_, err := runner.RunTestSuites(context.Background(), []suite.TestSuiteRun{{
		Cases: []suite.TestCase{identityCapturingCase{name: "capture", seen: &seen}},
	}}, nil)
	if err != nil {
		t.Fatalf("RunTestSuites() error = %v", err)
	}
	if seen == nil {
		t.Fatal("case did not observe identity")
	}
	if seen.ID != custom.ID || seen.Email != custom.Email {
		t.Fatalf("seen identity = %+v, want %+v", seen, custom)
	}
}

func TestRunner_suiteIdentityOverridesRunner(t *testing.T) {
	t.Parallel()

	runnerIdentity := &iam.Identity{Type: iam.User, ID: "runner", Email: "runner@example.com"}
	suiteIdentity := &iam.Identity{Type: iam.User, ID: "suite", Email: "suite@example.com"}
	var seen *iam.Identity

	runner := New(WithIdentity(runnerIdentity))
	_, err := runner.RunTestSuites(context.Background(), []suite.TestSuiteRun{{
		Identity: suiteIdentity,
		Cases:    []suite.TestCase{identityCapturingCase{name: "capture", seen: &seen}},
	}}, nil)
	if err != nil {
		t.Fatalf("RunTestSuites() error = %v", err)
	}
	if seen == nil || seen.ID != suiteIdentity.ID {
		t.Fatalf("seen identity = %+v, want suite identity %+v", seen, suiteIdentity)
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
