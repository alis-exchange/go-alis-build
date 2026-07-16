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

func TestMetricsProto_thresholdOnlyWithScore(t *testing.T) {
	t.Parallel()

	score := 0.85
	got := MetricsProto([]execution.Metric{
		{ID: "response_match_score", Status: evalspb.Status_PASSED, Threshold: 0.3},
		{ID: "quality", Status: evalspb.Status_PASSED, Threshold: 0.7, Score: &score},
	})

	if len(got) != 2 {
		t.Fatalf("len(metrics)=%d, want 2", len(got))
	}
	if got[0].Threshold != nil {
		t.Fatalf("score-less metric threshold=%v, want nil", got[0].Threshold)
	}
	if got[1].Threshold == nil || *got[1].Threshold != 0.7 {
		t.Fatalf("scored metric threshold=%v, want 0.7", got[1].Threshold)
	}
}
