package mapper

import (
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
)

func TestLoadRun_maps(t *testing.T) {
	t.Parallel()

	start := time.Now()
	sr := execution.LoadSuiteResult{
		SuiteName: "files-v2-load",
		StartTime: start,
		EndTime:   start.Add(2 * time.Second),
		Cases: []execution.LoadCaseResult{
			{
				Name:   "files-v2-load.list-files",
				Status: evalspb.Status_PASSED,
				Summary: execution.LoadCaseSummary{
					Mode:         evalspb.RunLoadTestRequest_MODERATE,
					TargetQPS:    100,
					Concurrency:  25,
					Duration:     time.Second,
					RequestCount: 95,
					ErrorCount:   2,
					ActualQPS:    95,
					Latency:      execution.LoadLatency{P50Ms: 8, P95Ms: 60, P99Ms: 120, MinMs: 2, MeanMs: 12, MaxMs: 250},
					ErrorsByCode: map[string]int64{"UNAVAILABLE": 2},
				},
				Checks: []execution.SloCheckResult{
					{ID: "latency.p99_ms", Status: evalspb.Status_PASSED, Observed: 120, Limit: 500, Unit: "ms"},
					{ID: "error_rate", Status: evalspb.Status_FAILED, Observed: 2.1, Limit: 1.0, Unit: "%", Message: "2.1% exceeds limit 1.0%"},
				},
			},
		},
	}

	got := LoadRun(sr, "operations/1", "run-1", "batch-x")
	if got.GetType() != evalspb.Run_LOAD_TEST {
		t.Fatalf("Type=%v", got.GetType())
	}
	if got.GetName() != "runs/run-1" {
		t.Fatalf("Name=%q", got.GetName())
	}
	if got.GetOperation() != "operations/1" {
		t.Fatalf("Operation=%q", got.GetOperation())
	}
	if got.GetBatchId() != "batch-x" {
		t.Fatalf("BatchId=%q", got.GetBatchId())
	}

	lt := got.GetLoadTest()
	if lt == nil || len(lt.GetCases()) != 1 {
		t.Fatalf("unexpected LoadTest data: %+v", lt)
	}
	c := lt.GetCases()[0]
	if c.GetId() != "files-v2-load.list-files" || c.GetStatus() != evalspb.Status_PASSED {
		t.Fatalf("case=%+v", c)
	}
	if c.GetSummary().GetMode() != evalspb.RunLoadTestRequest_MODERATE {
		t.Fatalf("Summary.Mode=%v", c.GetSummary().GetMode())
	}
	if c.GetSummary().GetTargetQps() != 100 || c.GetSummary().GetConcurrency() != 25 {
		t.Fatalf("Summary target/conc=%v/%v", c.GetSummary().GetTargetQps(), c.GetSummary().GetConcurrency())
	}
	if c.GetSummary().GetLatency().GetP99Ms() != 120 {
		t.Fatalf("P99=%v", c.GetSummary().GetLatency().GetP99Ms())
	}
	if got := c.GetSummary().GetErrorsByCode()["UNAVAILABLE"]; got != 2 {
		t.Fatalf("ErrorsByCode[UNAVAILABLE]=%d", got)
	}
	if len(c.GetChecks()) != 2 {
		t.Fatalf("len(checks)=%d", len(c.GetChecks()))
	}
	if c.GetChecks()[1].GetStatus() != evalspb.Status_FAILED || c.GetChecks()[1].GetUnit() != "%" {
		t.Fatalf("error_rate check malformed: %+v", c.GetChecks()[1])
	}
}

func TestLoadRun_noBatchID(t *testing.T) {
	t.Parallel()

	got := LoadRun(execution.LoadSuiteResult{}, "op", "r", "")
	if got.BatchId != nil {
		t.Fatalf("BatchId should be nil when empty, got %v", got.BatchId)
	}
}
