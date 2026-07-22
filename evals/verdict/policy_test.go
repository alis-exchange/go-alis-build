package verdict_test

import (
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/verdict"
)

func TestDefaultRunPolicy_neutralNotEvaluated(t *testing.T) {
	t.Parallel()
	p := verdict.DefaultRunPolicy()
	if p.NotEvaluatedRollsUpAs != evalspb.Status_PASSED {
		t.Fatalf("NotEvaluatedRollsUpAs = %v, want PASSED", p.NotEvaluatedRollsUpAs)
	}
}

func TestLoadCasePolicy_transportErrors(t *testing.T) {
	t.Parallel()

	noSLO := verdict.LoadCasePolicy(3, false)
	status, _ := verdict.Case(verdict.Evidence{Errors: 3}, noSLO)
	if status != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED", status)
	}

	withSLO := verdict.LoadCasePolicy(3, true)
	status, _ = verdict.Case(verdict.Evidence{Errors: 3}, withSLO)
	if status != evalspb.Status_PASSED {
		t.Fatalf("status = %v, want PASSED when error-rate SLO covers transport", status)
	}
}

func TestStandaloneInfraObservePolicy_emptyLeavesFail(t *testing.T) {
	t.Parallel()
	status, _ := verdict.Case(verdict.Evidence{}, verdict.StandaloneInfraObservePolicy())
	if status != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED for empty observation", status)
	}
}

func TestRollup_unspecifiedStatusFailsByDefault(t *testing.T) {
	t.Parallel()
	got := verdict.Run([]evalspb.Status{evalspb.Status_PASSED, evalspb.Status_STATUS_UNSPECIFIED}, verdict.Policy{})
	if got != evalspb.Status_FAILED {
		t.Fatalf("Run() = %v, want FAILED for unspecified status", got)
	}
}

func TestSuite_notEvaluatedNeutral(t *testing.T) {
	t.Parallel()
	p := verdict.DefaultRunPolicy()
	got := verdict.Suite([]evalspb.Status{evalspb.Status_PASSED, evalspb.Status_NOT_EVALUATED}, p)
	if got != evalspb.Status_PASSED {
		t.Fatalf("Suite() = %v, want PASSED", got)
	}
}
