package registry

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
	name string
}

func (c stubLoadCase) Name() string { return c.name }

func (c stubLoadCase) Run(context.Context, evalspb.RunLoadTestRequest_Mode, loadgen.Profile) *execution.LoadCaseResult {
	return &execution.LoadCaseResult{Name: c.name}
}

func mustLoadSuite(t *testing.T, name string, opts ...suite.LoadSuiteOption) *suite.LoadSuite {
	t.Helper()
	s, err := suite.NewLoadSuite(name, opts...)
	if err != nil {
		t.Fatalf("NewLoadSuite: %v", err)
	}
	return s
}

func TestRegistry_SelectLoadRuns_all(t *testing.T) {
	t.Parallel()

	reg := New()
	s := mustLoadSuite(t, "load-a")
	_ = s.AddCase(stubLoadCase{name: "one"})
	_ = s.AddCase(stubLoadCase{name: "two"})
	reg.RegisterLoadSuite(s)

	runs, err := reg.SelectLoadRuns(nil)
	if err != nil {
		t.Fatalf("SelectLoadRuns: %v", err)
	}
	if len(runs) != 1 || len(runs[0].Cases) != 2 {
		t.Fatalf("runs=%+v, want 1 suite × 2 cases", runs)
	}
	if runs[0].Name != "load-a" {
		t.Fatalf("Name=%q", runs[0].Name)
	}
}

func TestRegistry_SelectLoadRuns_caseFilter(t *testing.T) {
	t.Parallel()

	reg := New()
	s := mustLoadSuite(t, "load-a")
	_ = s.AddCase(stubLoadCase{name: "one"})
	_ = s.AddCase(stubLoadCase{name: "two"})
	reg.RegisterLoadSuite(s)

	runs, err := reg.SelectLoadRuns([]string{"load-a.two"})
	if err != nil {
		t.Fatalf("SelectLoadRuns: %v", err)
	}
	if len(runs) != 1 || len(runs[0].Cases) != 1 || runs[0].Cases[0].Name() != "load-a.two" {
		t.Fatalf("runs=%+v", runs)
	}
}

func TestRegistry_SelectLoadRuns_profileOverridesFlow(t *testing.T) {
	t.Parallel()

	reg := New()
	p := loadgen.Profile{QPS: 250, Concurrency: 40, Duration: 30 * time.Second}
	s := mustLoadSuite(t, "load-a",
		suite.WithLoadProfileOverride(evalspb.RunLoadTestRequest_MODERATE, p),
	)
	_ = s.AddCase(stubLoadCase{name: "one"})
	reg.RegisterLoadSuite(s)

	runs, err := reg.SelectLoadRuns(nil)
	if err != nil {
		t.Fatalf("SelectLoadRuns: %v", err)
	}
	got, ok := runs[0].ProfileOverrides[evalspb.RunLoadTestRequest_MODERATE]
	if !ok || got.QPS != 250 {
		t.Fatalf("override snapshot: got=%+v ok=%v", got, ok)
	}
}

func TestRegistry_ValidateSelection_load(t *testing.T) {
	t.Parallel()

	reg := New()
	s := mustLoadSuite(t, "load-a")
	_ = s.AddCase(stubLoadCase{name: "one"})
	reg.RegisterLoadSuite(s)

	// Unknown case.
	if err := reg.ValidateSelection(evalspb.Run_LOAD_TEST, []string{"load-a.missing"}); err == nil {
		t.Fatal("expected error for unknown case")
	}
	// Known case.
	if err := reg.ValidateSelection(evalspb.Run_LOAD_TEST, []string{"load-a.one"}); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
}

func TestRegistry_ValidateSelection_noLoadSuites(t *testing.T) {
	t.Parallel()

	reg := New()
	err := reg.ValidateSelection(evalspb.Run_LOAD_TEST, nil)
	var want ErrNoLoadSuites
	if !errors.As(err, &want) {
		t.Fatalf("err=%v, want ErrNoLoadSuites", err)
	}
}
