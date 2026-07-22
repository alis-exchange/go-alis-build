package suite

import (
	"context"
	"errors"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/loadgen"
)

type stubLoadCase struct {
	name   string
	result *execution.LoadCaseResult
}

func (c stubLoadCase) Name() string { return c.name }

func (c stubLoadCase) Run(context.Context, evalspb.RunLoadTestRequest_Mode, loadgen.Profile) *execution.LoadCaseResult {
	return c.result
}

func mustLoadSuite(t *testing.T, name string, opts ...LoadSuiteOption) *LoadSuite {
	t.Helper()
	s, err := NewLoadSuite(name, opts...)
	if err != nil {
		t.Fatalf("NewLoadSuite: %v", err)
	}
	return s
}

func TestNewLoadSuite_qualifiedNames(t *testing.T) {
	t.Parallel()

	s := mustLoadSuite(t, "files-load")
	if err := s.AddCase(stubLoadCase{name: "list", result: &execution.LoadCaseResult{}}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}
	if err := s.AddCase(stubLoadCase{name: "get", result: &execution.LoadCaseResult{}}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}
	got := s.Cases()
	want := []string{"files-load.list", "files-load.get"}
	for i, c := range got {
		if c.Name() != want[i] {
			t.Fatalf("Cases[%d].Name()=%q, want %q", i, c.Name(), want[i])
		}
	}
}

func TestNewLoadSuite_rejectsBadNames(t *testing.T) {
	t.Parallel()

	if _, err := NewLoadSuite(""); err == nil {
		t.Fatal("empty name: expected error")
	}
	if _, err := NewLoadSuite("has.dot"); err == nil {
		t.Fatal("dotted name: expected error")
	}
}

func TestLoadSuite_AddCase_rejectsDuplicatesAndDottedNames(t *testing.T) {
	t.Parallel()

	s := mustLoadSuite(t, "s")
	c1 := stubLoadCase{name: "dup", result: &execution.LoadCaseResult{}}
	if err := s.AddCase(c1); err != nil {
		t.Fatalf("AddCase: %v", err)
	}
	if err := s.AddCase(stubLoadCase{name: "dup", result: &execution.LoadCaseResult{}}); err == nil {
		t.Fatal("duplicate: expected error")
	}
	var dup ErrDuplicateCase
	if err := s.AddCase(stubLoadCase{name: "dup"}); !errors.As(err, &dup) {
		t.Fatalf("expected ErrDuplicateCase, got %v", err)
	}
	if err := s.AddCase(stubLoadCase{name: "a.b"}); err == nil {
		t.Fatal("dotted case name: expected error")
	}
}

func TestLoadSuite_ProfileOverride(t *testing.T) {
	t.Parallel()

	p := loadgen.Profile{QPS: 250, Concurrency: 40, Duration: 15 * time.Second}
	s := mustLoadSuite(t, "s", WithLoadProfileOverride(evalspb.RunLoadTestRequest_MODERATE, p))
	got, ok := s.ProfileOverride(evalspb.RunLoadTestRequest_MODERATE)
	if !ok || got.QPS != 250 {
		t.Fatalf("MODERATE override = %+v ok=%v", got, ok)
	}
	if _, ok := s.ProfileOverride(evalspb.RunLoadTestRequest_HIGH); ok {
		t.Fatal("HIGH should have no override")
	}
}

func TestLoadSuite_ProfileOverride_rejectsUnspecified(t *testing.T) {
	t.Parallel()

	if _, err := NewLoadSuite("s", WithLoadProfileOverride(evalspb.RunLoadTestRequest_MODE_UNSPECIFIED, loadgen.Profile{})); err == nil {
		t.Fatal("expected error for UNSPECIFIED mode")
	}
}

func TestNewLoadSuite_nilOption(t *testing.T) {
	t.Parallel()

	var nilOpt LoadSuiteOption
	_, err := NewLoadSuite("s", nilOpt)
	if !errors.Is(err, ErrNilOption{}) {
		t.Fatalf("NewLoadSuite() error = %v, want ErrNilOption", err)
	}
}

func TestLoadSuite_SelectLoadCases(t *testing.T) {
	t.Parallel()

	s := mustLoadSuite(t, "s")
	_ = s.AddCase(stubLoadCase{name: "a", result: &execution.LoadCaseResult{}})
	_ = s.AddCase(stubLoadCase{name: "b", result: &execution.LoadCaseResult{}})

	// Empty filter selects all.
	if got := s.SelectLoadCases(nil); len(got) != 2 {
		t.Fatalf("nil filter: len=%d, want 2", len(got))
	}
	// Suite filter selects all in that suite.
	got := s.SelectLoadCases([]FilterPath{{Suite: "s"}})
	if len(got) != 2 {
		t.Fatalf("suite filter: len=%d, want 2", len(got))
	}
	// Case-scoped filter picks just that case.
	got = s.SelectLoadCases([]FilterPath{{Suite: "s", CaseName: "a"}})
	if len(got) != 1 || got[0].Name() != "s.a" {
		t.Fatalf("case filter: got=%v", got)
	}
	// Filter for another suite yields none.
	got = s.SelectLoadCases([]FilterPath{{Suite: "other"}})
	if got != nil {
		t.Fatalf("other-suite filter: expected nil, got %v", got)
	}
}
