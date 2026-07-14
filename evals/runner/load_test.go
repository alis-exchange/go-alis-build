package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/suite"
)

type stubLoadCase struct {
	name   string
	result *execution.LoadCaseResult
	// panicWith, if set, is what Run panics with.
	panicWith any
}

func (c stubLoadCase) Name() string { return c.name }

func (c stubLoadCase) Run(context.Context, evalspb.RunLoadTestRequest_Mode, loadgen.Profile) *execution.LoadCaseResult {
	if c.panicWith != nil {
		panic(c.panicWith)
	}
	return c.result
}

func passedLoad(name string) *execution.LoadCaseResult {
	return &execution.LoadCaseResult{Name: name, Status: evalspb.Status_PASSED}
}

func failedLoad(name string) *execution.LoadCaseResult {
	return &execution.LoadCaseResult{Name: name, Status: evalspb.Status_FAILED}
}

func loadSuiteRun(cases ...suite.LoadCase) []suite.LoadSuiteRun {
	return []suite.LoadSuiteRun{{Cases: cases}}
}

func defaultResolver() LoadProfileResolver {
	return func(suite.LoadSuiteRun, evalspb.RunLoadTestRequest_Mode) (loadgen.Profile, bool) {
		return loadgen.Profile{QPS: 1, Concurrency: 1, Duration: time.Millisecond}, true
	}
}

func TestRunLoadSuites_AllPassed(t *testing.T) {
	t.Parallel()

	runner := New()
	runs := loadSuiteRun(
		stubLoadCase{name: "s.a", result: passedLoad("s.a")},
		stubLoadCase{name: "s.b", result: passedLoad("s.b")},
	)
	got, err := runner.RunLoadSuites(context.Background(), runs, evalspb.RunLoadTestRequest_MINIMAL, defaultResolver(), nil)
	if err != nil {
		t.Fatalf("RunLoadSuites: %v", err)
	}
	if len(got) != 1 || len(got[0].Cases) != 2 {
		t.Fatalf("got=%+v, want 1 suite × 2 cases", got)
	}
	if s := RollupLoadSuiteStatus(got[0]); s != evalspb.Status_PASSED {
		t.Fatalf("rollup=%v, want PASSED", s)
	}
}

func TestRunLoadSuites_MixedRollupFailed(t *testing.T) {
	t.Parallel()

	runner := New()
	runs := loadSuiteRun(
		stubLoadCase{name: "s.a", result: passedLoad("s.a")},
		stubLoadCase{name: "s.b", result: failedLoad("s.b")},
	)
	got, _ := runner.RunLoadSuites(context.Background(), runs, evalspb.RunLoadTestRequest_MINIMAL, defaultResolver(), nil)
	if RollupLoadSuiteStatus(got[0]) != evalspb.Status_FAILED {
		t.Fatal("expected FAILED rollup")
	}
}

func TestRunLoadSuites_UnresolvedProfileFailsCase(t *testing.T) {
	t.Parallel()

	runner := New()
	runs := loadSuiteRun(stubLoadCase{name: "s.a", result: passedLoad("s.a")})
	got, err := runner.RunLoadSuites(context.Background(), runs, evalspb.RunLoadTestRequest_MINIMAL,
		func(suite.LoadSuiteRun, evalspb.RunLoadTestRequest_Mode) (loadgen.Profile, bool) {
			return loadgen.Profile{}, false
		}, nil)
	if err != nil {
		t.Fatalf("RunLoadSuites: %v", err)
	}
	if got[0].Cases[0].Status != evalspb.Status_FAILED {
		t.Fatal("unresolved profile: expected FAILED")
	}
	if len(got[0].Cases[0].Checks) == 0 || got[0].Cases[0].Checks[0].ID != "profile" {
		t.Fatalf("expected synthetic 'profile' check, got %+v", got[0].Cases[0].Checks)
	}
}

func TestRunLoadSuites_PanicRecovered(t *testing.T) {
	t.Parallel()

	runner := New()
	runs := loadSuiteRun(
		stubLoadCase{name: "s.a", panicWith: "boom"},
		stubLoadCase{name: "s.b", result: passedLoad("s.b")},
	)
	got, err := runner.RunLoadSuites(context.Background(), runs, evalspb.RunLoadTestRequest_MINIMAL, defaultResolver(), nil)
	if err != nil {
		t.Fatalf("RunLoadSuites: %v", err)
	}
	if got[0].Cases[0].Status != evalspb.Status_FAILED {
		t.Fatal("panicking case should be FAILED")
	}
	if got[0].Cases[1].Status != evalspb.Status_PASSED {
		t.Fatal("subsequent case should still run after panic")
	}
}

func TestRunLoadSuites_SetupFailureRecordsAllCases(t *testing.T) {
	t.Parallel()

	runner := New()
	setupErr := errors.New("setup exploded")
	runs := []suite.LoadSuiteRun{{
		Name:  "s",
		Setup: func(context.Context) error { return setupErr },
		Cases: []suite.LoadCase{
			stubLoadCase{name: "s.a", result: passedLoad("s.a")},
			stubLoadCase{name: "s.b", result: passedLoad("s.b")},
		},
	}}
	got, err := runner.RunLoadSuites(context.Background(), runs, evalspb.RunLoadTestRequest_MINIMAL, defaultResolver(), nil)
	if err != nil {
		t.Fatalf("RunLoadSuites: %v", err)
	}
	for _, c := range got[0].Cases {
		if c.Status != evalspb.Status_FAILED {
			t.Fatalf("setup failure: case %q status=%v, want FAILED", c.Name, c.Status)
		}
	}
}

func TestRunLoadSuites_NilResolver(t *testing.T) {
	t.Parallel()

	runner := New()
	_, err := runner.RunLoadSuites(context.Background(), nil, evalspb.RunLoadTestRequest_MINIMAL, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil resolver")
	}
}

func TestRunLoadSuites_ProgressCallback(t *testing.T) {
	t.Parallel()

	runner := New()
	runs := loadSuiteRun(
		stubLoadCase{name: "s.a", result: passedLoad("s.a")},
		stubLoadCase{name: "s.b", result: passedLoad("s.b")},
	)
	var completed, total int
	progress := func(c, tot int) {
		completed = c
		total = tot
	}
	if _, err := runner.RunLoadSuites(context.Background(), runs, evalspb.RunLoadTestRequest_MINIMAL, defaultResolver(), progress); err != nil {
		t.Fatalf("RunLoadSuites: %v", err)
	}
	if completed != 2 || total != 2 {
		t.Fatalf("progress: completed=%d total=%d, want 2/2", completed, total)
	}
}

type abortCtxLoadCase struct {
	name string
	saw  bool
}

func (c abortCtxLoadCase) Name() string { return c.name }

func (c *abortCtxLoadCase) Run(ctx context.Context, _ evalspb.RunLoadTestRequest_Mode, _ loadgen.Profile) *execution.LoadCaseResult {
	c.saw = loadgen.AbortOnSLOFailure(ctx)
	return passedLoad(c.name)
}

func TestRunLoadSuites_AbortOnSLOFailureContext(t *testing.T) {
	t.Parallel()

	c := &abortCtxLoadCase{name: "s.a"}
	runner := New(WithAbortOnSLOFailure())
	runs := loadSuiteRun(c)
	if _, err := runner.RunLoadSuites(context.Background(), runs, evalspb.RunLoadTestRequest_MINIMAL, defaultResolver(), nil); err != nil {
		t.Fatalf("RunLoadSuites: %v", err)
	}
	if !c.saw {
		t.Fatal("expected abort-on-SLO-failure marker on case ctx")
	}
}
