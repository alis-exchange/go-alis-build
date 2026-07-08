package mapper

import (
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
)

func TestIntegrationRun_allCases(t *testing.T) {
	t.Parallel()

	start := time.Now()
	sr := execution.SuiteResult{
		SuiteName: "files-v2",
		StartTime: start,
		EndTime:   start.Add(time.Second),
		Cases: []execution.CaseResult{
			{
				Name:   "files-v2.pass",
				Status: evalspb.Status_PASSED,
				Checks: []execution.Check{{ID: "ok", Status: evalspb.Status_PASSED}},
			},
			{
				Name:   "files-v2.fail",
				Status: evalspb.Status_FAILED,
				Checks: []execution.Check{
					{ID: "grpc", Status: evalspb.Status_PASSED},
					{ID: "latency", Status: evalspb.Status_FAILED, Message: "slow"},
				},
			},
		},
	}

	run := IntegrationRun(sr, "operations/op-1", "run-abc", "batch-1")
	if run.GetType() != evalspb.Run_INTEGRATION_TEST {
		t.Fatalf("type = %v", run.GetType())
	}
	if run.GetBatchId() != "batch-1" {
		t.Fatalf("batch_id = %q, want batch-1", run.GetBatchId())
	}
	data := run.GetIntegrationTest()
	if len(data.GetCases()) != 2 {
		t.Fatalf("len(cases) = %d, want 2 (all cases)", len(data.GetCases()))
	}
	c := data.GetCases()[1]
	if c.GetId() != "files-v2.fail" {
		t.Fatalf("id = %q, want files-v2.fail", c.GetId())
	}
	if len(c.GetChecks()) != 2 {
		t.Fatalf("len(checks) = %d, want full context", len(c.GetChecks()))
	}
}

func TestAgentEvalRun_mapsMetrics(t *testing.T) {
	t.Parallel()

	score := 0.5
	start := time.Now()
	sr := execution.SuiteResult{
		SuiteName: "core",
		StartTime: start,
		EndTime:   start.Add(time.Second),
		Cases: []execution.CaseResult{
			{
				Name:      "core.eval",
				Status:    evalspb.Status_FAILED,
				SessionID: "sess-1",
				Metrics: []execution.Metric{
					{ID: "latency", Status: evalspb.Status_PASSED},
					{ID: "quality", Status: evalspb.Status_FAILED, Score: &score, Threshold: 0.8},
				},
			},
		},
	}

	run := AgentEvalRun(sr, "operations/op-2", "run-xyz")
	if run.GetType() != evalspb.Run_AGENT_EVAL {
		t.Fatalf("type = %v", run.GetType())
	}
	data := run.GetAgentEval()
	if len(data.GetCases()) != 1 {
		t.Fatalf("len(cases) = %d, want 1", len(data.GetCases()))
	}
	c := data.GetCases()[0]
	if c.GetId() != "core.eval" {
		t.Fatalf("id = %q, want core.eval", c.GetId())
	}
	if c.GetSessionId() != "sess-1" {
		t.Fatalf("session_id = %q, want sess-1", c.GetSessionId())
	}
	if len(c.GetMetrics()) != 2 {
		t.Fatalf("metrics not mapped: %+v", c)
	}
}

func TestIntegrationRun_googleProjectID(t *testing.T) {
	t.Setenv("ALIS_OS_PROJECT", "marvel-sm-dev-123")

	run := IntegrationRun(execution.SuiteResult{}, "operations/op-1", "run-abc", "")
	if run.GetGoogleProjectId() != "marvel-sm-dev-123" {
		t.Fatalf("google_project_id = %q, want marvel-sm-dev-123", run.GetGoogleProjectId())
	}
}

func TestIntegrationRun_googleProjectID_unset(t *testing.T) {
	t.Setenv("ALIS_OS_PROJECT", "")

	run := IntegrationRun(execution.SuiteResult{}, "operations/op-1", "run-abc", "")
	if run.GetGoogleProjectId() != "" {
		t.Fatalf("google_project_id = %q, want empty when ALIS_OS_PROJECT unset", run.GetGoogleProjectId())
	}
}
