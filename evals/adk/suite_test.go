package adk_test

import (
	"context"
	"errors"
	"testing"

	"go.alis.build/evals"
	"go.alis.build/evals/adk"
)

// TestSuiteCaseName replaces every "." with "_".
func TestSuiteCaseName(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"case_a":    "case_a",
		"case_b.v2": "case_b_v2",
		"a.b.c":     "a_b_c",
	}
	for in, want := range tests {
		if got := adk.SuiteCaseName(in); got != want {
			t.Fatalf("SuiteCaseName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSuiteCaseName_collisionIsDuplicateCase documents that ids differing
// only by "." and "_" collide and the suite rejects them before running.
func TestSuiteCaseName_collisionIsDuplicateCase(t *testing.T) {
	t.Parallel()

	suite := evals.NewAgentEvalSuite("smoke")
	for _, id := range []string{"a.b", "a_b"} {
		suite.AddCase(adk.SuiteCaseName(id), func(context.Context, *evals.AgentEvalResult) {
			t.Error("case ran despite a configuration error")
		})
	}
	if _, err := suite.Run(context.Background()); !errors.Is(err, evals.ErrDuplicateCase{}) {
		t.Fatalf("Run() error = %v, want ErrDuplicateCase", err)
	}
}
