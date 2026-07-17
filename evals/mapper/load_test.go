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
				Tags:   map[string]string{"rpc": "ListFiles"},
				Summary: execution.LoadCaseSummary{
					Mode:             evalspb.RunLoadTestRequest_MODERATE,
					TargetQPS:        100,
					Concurrency:      25,
					Duration:         time.Second,
					RequestCount:     95,
					ErrorCount:       2,
					CheckPassedCount: 90,
					CheckFailedCount: 3,
					DroppedCount:     5,
					ActualQPS:        95,
					QPSStages:        []execution.LoadStage{{Duration: time.Second, Target: 100}},
					Latency:          execution.LoadLatency{P50Ms: 8, P95Ms: 60, P99Ms: 120, MinMs: 2, MeanMs: 12, MaxMs: 250},
					ErrorsByCode:     map[string]int64{"UNAVAILABLE": 2},
					Stream: &execution.LoadStreamSummary{
						StreamCount:       10,
						MessagesSentTotal: 40,
						TTFB:              execution.LoadLatency{P99Ms: 25},
					},
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
	if got, ok := StringEntryValue(c.GetTags(), "rpc"); !ok || got != "ListFiles" {
		t.Fatalf("Tags=%v", c.GetTags())
	}
	summary := c.GetSummary()
	if summary.GetMode() != evalspb.RunLoadTestRequest_MODERATE {
		t.Fatalf("Summary.Mode=%v", summary.GetMode())
	}
	if summary.GetTargetQps() != 100 || summary.GetConcurrency() != 25 {
		t.Fatalf("Summary target/conc=%v/%v", summary.GetTargetQps(), summary.GetConcurrency())
	}
	if summary.GetCheckPassedCount() != 90 || summary.GetCheckFailedCount() != 3 {
		t.Fatalf("check counts=%d/%d", summary.GetCheckPassedCount(), summary.GetCheckFailedCount())
	}
	if summary.GetDroppedCount() != 5 {
		t.Fatalf("DroppedCount=%d", summary.GetDroppedCount())
	}
	if len(summary.GetQpsStages()) != 1 || summary.GetQpsStages()[0].GetTarget() != 100 {
		t.Fatalf("QpsStages=%v", summary.GetQpsStages())
	}
	if summary.GetLatency().GetP99Ms() != 120 {
		t.Fatalf("P99=%v", summary.GetLatency().GetP99Ms())
	}
	if got, ok := Int64EntryValue(summary.GetErrorsByCode(), "UNAVAILABLE"); !ok || got != 2 {
		t.Fatalf("ErrorsByCode[UNAVAILABLE]=%d", got)
	}
	if summary.GetStream().GetStreamCount() != 10 || summary.GetStream().GetMessagesSentTotal() != 40 {
		t.Fatalf("Stream=%+v", summary.GetStream())
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
