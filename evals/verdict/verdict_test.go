package verdict_test

import (
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/verdict"
)

func TestCase_emptyLeavesRequireEvidence(t *testing.T) {
	t.Parallel()

	status, leaves := verdict.Case(verdict.Evidence{}, verdict.Policy{RequireEvidence: true})

	if status != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED", status)
	}
	if len(leaves) != 0 {
		t.Fatalf("leaves = %v, want none from Case (caller adds synthetic checks)", leaves)
	}
}

func TestCase_passingLeaves(t *testing.T) {
	t.Parallel()

	status, _ := verdict.Case(verdict.Evidence{
		Leaves: []verdict.Leaf{{
			ID:     "latency",
			Status: evalspb.Status_PASSED,
		}},
	}, verdict.Policy{})

	if status != evalspb.Status_PASSED {
		t.Fatalf("status = %v, want PASSED", status)
	}
}

func TestCase_failOnUnrepresentedErrors(t *testing.T) {
	t.Parallel()

	status, _ := verdict.Case(verdict.Evidence{
		Leaves: []verdict.Leaf{{
			ID:     "latency",
			Status: evalspb.Status_PASSED,
		}},
		Errors: 3,
	}, verdict.Policy{
		FailOnUnrepresentedErrors: true,
	})

	if status != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED", status)
	}
}

func TestCase_requireSuccessfulObservation(t *testing.T) {
	t.Parallel()

	status, _ := verdict.Case(verdict.Evidence{
		Leaves: []verdict.Leaf{{
			ID:     "target-a",
			Status: evalspb.Status_FAILED,
		}},
	}, verdict.Policy{
		RequireSuccessfulObservation: true,
	})

	if status != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED", status)
	}
}

func TestSuite_allPassed(t *testing.T) {
	t.Parallel()

	got := verdict.Suite(
		[]evalspb.Status{evalspb.Status_PASSED, evalspb.Status_PASSED},
		verdict.Policy{},
	)
	if got != evalspb.Status_PASSED {
		t.Fatalf("Suite() = %v, want PASSED", got)
	}
}

func TestSuite_anyFailed(t *testing.T) {
	t.Parallel()

	got := verdict.Suite(
		[]evalspb.Status{evalspb.Status_PASSED, evalspb.Status_FAILED},
		verdict.Policy{},
	)
	if got != evalspb.Status_FAILED {
		t.Fatalf("Suite() = %v, want FAILED", got)
	}
}

func TestRun_stopOnFailureStillFails(t *testing.T) {
	t.Parallel()

	p := verdict.DefaultRunPolicy()
	got := verdict.Run([]evalspb.Status{
		evalspb.Status_PASSED,
		evalspb.Status_FAILED,
		evalspb.Status_NOT_EVALUATED,
	}, p)
	if got != evalspb.Status_FAILED {
		t.Fatalf("Run() = %v, want FAILED", got)
	}
}

func TestRun_notEvaluatedNeutral(t *testing.T) {
	t.Parallel()

	p := verdict.Policy{NotEvaluatedRollsUpAs: evalspb.Status_PASSED}

	tests := []struct {
		name string
		in   []evalspb.Status
		want evalspb.Status
	}{
		{
			name: "all passed",
			in:   []evalspb.Status{evalspb.Status_PASSED, evalspb.Status_PASSED},
			want: evalspb.Status_PASSED,
		},
		{
			name: "passed and not evaluated",
			in:   []evalspb.Status{evalspb.Status_PASSED, evalspb.Status_NOT_EVALUATED},
			want: evalspb.Status_PASSED,
		},
		{
			name: "all not evaluated",
			in:   []evalspb.Status{evalspb.Status_NOT_EVALUATED, evalspb.Status_NOT_EVALUATED},
			want: evalspb.Status_NOT_EVALUATED,
		},
		{
			name: "failed wins",
			in:   []evalspb.Status{evalspb.Status_FAILED, evalspb.Status_NOT_EVALUATED},
			want: evalspb.Status_FAILED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := verdict.Run(tt.in, p); got != tt.want {
				t.Fatalf("Run() = %v, want %v", got, tt.want)
			}
		})
	}
}
