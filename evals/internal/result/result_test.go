package result

import (
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
)

func TestRollupCaseStatus_errorMetric(t *testing.T) {
	t.Parallel()

	status := RollupCaseStatus([]execution.Metric{{Status: evalspb.Status_FAILED}})
	if status != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED", status)
	}
}
