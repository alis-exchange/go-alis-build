package runner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/env"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/registry"
	"go.alis.build/evals/suite"
	"go.alis.build/evals/verdict"
)

func TestRunner_RunTestSuites_teardownFailureSurfacesOnCases(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("teardown failed")
	runs := []suite.TestSuiteRun{{
		Name:  "suite-a",
		Cases: []suite.TestCase{stubTestCase{name: "a", result: passedCase("a")}},
		Teardown: func(context.Context) error {
			return wantErr
		},
	}}

	suites, err := New().RunTestSuites(context.Background(), runs, nil, nil)
	if err != nil {
		t.Fatalf("RunTestSuites() error = %v", err)
	}
	if RollupSuiteStatus(suites[0]) != evalspb.Status_FAILED {
		t.Fatalf("rollup = %v, want FAILED after teardown error", RollupSuiteStatus(suites[0]))
	}
	if len(suites[0].Cases) != 1 {
		t.Fatalf("len(cases) = %d, want 1", len(suites[0].Cases))
	}
	found := false
	for _, chk := range suites[0].Cases[0].Checks {
		if chk.ID == verdict.IDTeardown && chk.Status == evalspb.Status_FAILED {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("checks = %+v, want %q FAILED marker", suites[0].Cases[0].Checks, verdict.IDTeardown)
	}
}

func TestRunner_suiteTeardownRunsAfterCancellationWithDetachedContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	var teardownLive atomic.Bool
	runs := []suite.TestSuiteRun{{
		Name: "suite-a",
		Cases: []suite.TestCase{
			cancelOnRunTestCase{name: "cancel", cancel: cancel},
			stubTestCase{name: "queued", result: passedCase("queued")},
		},
		Teardown: func(ctx context.Context) error {
			if ctx.Err() != nil {
				return errors.New("teardown received cancelled context")
			}
			teardownLive.Store(true)
			return nil
		},
	}}

	_, err := New().RunTestSuites(ctx, runs, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunTestSuites() error = %v, want context.Canceled", err)
	}
	if !teardownLive.Load() {
		t.Fatal("suite teardown did not run with a live detached context")
	}
}

func TestRunner_customEnvironmentRegistryUsedForExecution(t *testing.T) {
	t.Parallel()

	envRegistry := env.New()
	var setupCalled atomic.Bool
	if err := envRegistry.Register("isolated", env.WithSetup(func(context.Context) error {
		setupCalled.Store(true)
		return nil
	})); err != nil {
		t.Fatalf("Register environment: %v", err)
	}
	s, err := suite.NewTestSuite("suite-a", suite.WithEnvironment("isolated"))
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s.AddCase(stubTestCase{name: "case", result: passedCase("case")}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}
	reg := registry.New()
	if err := reg.SetEnvRegistry(envRegistry); err != nil {
		t.Fatalf("SetEnvRegistry: %v", err)
	}
	if err := reg.RegisterIntegrationSuite(s); err != nil {
		t.Fatalf("RegisterIntegrationSuite: %v", err)
	}
	if err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	runs, err := reg.SelectTestRuns(evalspb.Run_INTEGRATION_TEST, nil)
	if err != nil {
		t.Fatalf("SelectTestRuns: %v", err)
	}
	if _, err := New().RunTestSuites(context.Background(), runs, nil, nil); err != nil {
		t.Fatalf("RunTestSuites: %v", err)
	}
	if !setupCalled.Load() {
		t.Fatal("custom environment registry setup was not used")
	}
}

func TestRunner_infraTeardownFailureCarriesDiagnosticSnapshot(t *testing.T) {
	t.Parallel()

	runs := []suite.InfraObserveSuiteRun{{
		Name: "infra",
		Cases: []suite.InfraObserveCase{
			slowInfraObserveCase{name: "case", hits: new(int32)},
		},
		Teardown: func(context.Context) error { return errors.New("cleanup failed") },
	}}
	results, err := New().RunInfraObserveSuites(context.Background(), runs, InfraObserveRunParams{}, nil, nil)
	if err != nil {
		t.Fatalf("RunInfraObserveSuites: %v", err)
	}
	got := results[0].Cases[0]
	if got.Status != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED", got.Status)
	}
	if len(got.CloudRun) != 1 || got.CloudRun[0].GetId() != verdict.IDTeardown {
		t.Fatalf("diagnostic snapshots = %+v, want %q", got.CloudRun, verdict.IDTeardown)
	}
	if got.CloudRun[0].GetFetchMessage() != "cleanup failed" {
		t.Fatalf("diagnostic message = %q", got.CloudRun[0].GetFetchMessage())
	}
}

func TestRunner_envTeardownAfterCancelUsesDetachedContext(t *testing.T) {
	t.Parallel()

	envName := "runner-env-teardown-cancel-" + t.Name()
	var teardownLive atomic.Bool
	if err := env.Register(envName,
		env.WithSetup(func(context.Context) error { return nil }),
		env.WithTeardown(func(ctx context.Context) error {
			if ctx.Err() != nil {
				return errors.New("teardown ctx cancelled")
			}
			teardownLive.Store(true)
			return nil
		}),
	); err != nil {
		t.Fatalf("env.Register: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runs := []suite.TestSuiteRun{
		{
			Name:         "suite-a",
			Environments: []string{envName},
			Cases:        []suite.TestCase{stubTestCase{name: "a", result: passedCase("a")}},
		},
		{
			Name:  "suite-b",
			Cases: []suite.TestCase{stubTestCase{name: "b", result: passedCase("b")}},
		},
	}
	hook := func(_ context.Context, sr execution.SuiteResult) error {
		if sr.SuiteName == "suite-a" {
			cancel()
		}
		return nil
	}

	_, err := New().RunTestSuites(ctx, runs, nil, hook)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunTestSuites() error = %v, want context.Canceled", err)
	}
	if !teardownLive.Load() {
		t.Fatal("environment teardown did not run with a live detached context")
	}
}
