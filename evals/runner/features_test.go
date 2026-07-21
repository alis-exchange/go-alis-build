package runner

import (
	"context"
	"strings"
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/internal/result"
	"go.alis.build/evals/suite"
)

type panickingTestCase struct{ name string }

func (c panickingTestCase) Name() string { return c.name }
func (c panickingTestCase) Run(context.Context) *execution.CaseResult {
	panic("intentional test panic")
}

type panickingEvalCase struct{ name string }

func (c panickingEvalCase) Name() string { return c.name }
func (c panickingEvalCase) Run(context.Context) *execution.CaseResult {
	panic("intentional eval panic")
}

func failedCase(name string) *execution.CaseResult {
	return &execution.CaseResult{Name: name, Status: evalspb.Status_FAILED}
}

func TestRunner_TestSuite_recoversPanic(t *testing.T) {
	t.Parallel()

	runs := []suite.TestSuiteRun{{
		Cases: []suite.TestCase{
			panickingTestCase{name: "boom"},
			stubTestCase{name: "next", result: passedCase("next")},
		},
	}}
	out, err := New().RunTestSuites(context.Background(), runs, nil, nil)
	if err != nil {
		t.Fatalf("RunTestSuites: %v", err)
	}
	if len(out) != 1 || len(out[0].Cases) != 2 {
		t.Fatalf("cases = %v", out[0].Cases)
	}
	first := out[0].Cases[0]
	if first.Status != evalspb.Status_FAILED {
		t.Fatalf("panicked case status = %v, want FAILED", first.Status)
	}
	if len(first.Checks) == 0 || first.Checks[0].ID != result.CaseErrorCheckName {
		t.Fatalf("panicked case checks = %+v", first.Checks)
	}
	if !strings.Contains(first.Checks[0].Message, "panic") {
		t.Fatalf("panic message not preserved: %q", first.Checks[0].Message)
	}
	if out[0].Cases[1].Status != evalspb.Status_PASSED {
		t.Fatalf("subsequent case status = %v, want PASSED (batch continues)", out[0].Cases[1].Status)
	}
}

func TestRunner_EvalSuite_recoversPanic(t *testing.T) {
	t.Parallel()

	runs := []suite.EvalSuiteRun{{
		Cases: []suite.EvalCase{
			panickingEvalCase{name: "boom"},
			stubEvalCase{name: "next", result: passedCase("next")},
		},
	}}
	out, err := New().RunEvalSuites(context.Background(), runs, nil, nil)
	if err != nil {
		t.Fatalf("RunEvalSuites: %v", err)
	}
	first := out[0].Cases[0]
	if first.Status != evalspb.Status_FAILED {
		t.Fatalf("panicked eval status = %v, want FAILED", first.Status)
	}
	if len(first.Metrics) == 0 || first.Metrics[0].ID != result.CaseErrorCheckName {
		t.Fatalf("panicked eval metrics = %+v", first.Metrics)
	}
	if out[0].Cases[1].Status != evalspb.Status_PASSED {
		t.Fatal("subsequent eval case did not run after panic")
	}
}

func TestRunner_TestSuite_stopOnFailureMarksRemainingSkipped(t *testing.T) {
	t.Parallel()

	runs := []suite.TestSuiteRun{{
		StopOnFailure: true,
		Cases: []suite.TestCase{
			stubTestCase{name: "first", result: passedCase("first")},
			stubTestCase{name: "second", result: failedCase("second")},
			stubTestCase{name: "third", result: passedCase("third")},
			stubTestCase{name: "fourth", result: passedCase("fourth")},
		},
	}}

	out, err := New().RunTestSuites(context.Background(), runs, nil, nil)
	if err != nil {
		t.Fatalf("RunTestSuites: %v", err)
	}
	cases := out[0].Cases
	if len(cases) != 4 {
		t.Fatalf("cases = %d, want 4", len(cases))
	}
	if cases[0].Status != evalspb.Status_PASSED {
		t.Fatalf("case[0] = %v, want PASSED", cases[0].Status)
	}
	if cases[1].Status != evalspb.Status_FAILED {
		t.Fatalf("case[1] = %v, want FAILED", cases[1].Status)
	}
	for i := 2; i < 4; i++ {
		if cases[i].Status != evalspb.Status_NOT_EVALUATED {
			t.Fatalf("case[%d] = %v, want NOT_EVALUATED", i, cases[i].Status)
		}
		if len(cases[i].Checks) == 0 || cases[i].Checks[0].ID != result.SkippedCheckName {
			t.Fatalf("case[%d] skip marker = %+v", i, cases[i].Checks)
		}
		if !strings.Contains(cases[i].Checks[0].Message, "second") {
			t.Fatalf("case[%d] skip message = %q, want mention of 'second'", i, cases[i].Checks[0].Message)
		}
	}
}

func TestRunner_TestSuite_stopOnFailureCountsSkippedInProgress(t *testing.T) {
	t.Parallel()

	var calls [][2]int
	runs := []suite.TestSuiteRun{{
		StopOnFailure: true,
		Cases: []suite.TestCase{
			stubTestCase{name: "one", result: failedCase("one")},
			stubTestCase{name: "two", result: passedCase("two")},
			stubTestCase{name: "three", result: passedCase("three")},
		},
	}}
	_, err := New().RunTestSuites(context.Background(), runs, func(completed, total int) {
		calls = append(calls, [2]int{completed, total})
	}, nil)
	if err != nil {
		t.Fatalf("RunTestSuites: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("progress calls = %d, want 3 (skipped cases still count)", len(calls))
	}
	if calls[2] != [2]int{3, 3} {
		t.Fatalf("final progress = %v, want [3 3]", calls[2])
	}
}

func TestRunner_EvalSuite_stopOnFailureMarksRemainingSkipped(t *testing.T) {
	t.Parallel()

	runs := []suite.EvalSuiteRun{{
		StopOnFailure: true,
		Cases: []suite.EvalCase{
			stubEvalCase{name: "first", result: failedCase("first")},
			stubEvalCase{name: "second", result: passedCase("second")},
		},
	}}
	out, err := New().RunEvalSuites(context.Background(), runs, nil, nil)
	if err != nil {
		t.Fatalf("RunEvalSuites: %v", err)
	}
	skipped := out[0].Cases[1]
	if skipped.Status != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("skipped eval status = %v", skipped.Status)
	}
	if len(skipped.Metrics) == 0 || skipped.Metrics[0].ID != result.SkippedCheckName {
		t.Fatalf("skipped eval marker = %+v", skipped.Metrics)
	}
}
