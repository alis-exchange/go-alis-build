package log

import (
	"context"
	"strings"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestReporter_ReportRun_nilSafe(t *testing.T) {
	t.Parallel()
	if err := (Reporter{}).ReportRun(context.Background(), nil); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestReporter_ReportRun_alwaysNilError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status evalspb.Status
	}{
		{"passed", evalspb.Status_PASSED},
		{"failed", evalspb.Status_FAILED},
		{"unspecified", evalspb.Status_STATUS_UNSPECIFIED},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			run := &evalspb.Run{Name: "runs/" + tt.name, Status: tt.status}
			if err := (Reporter{}).ReportRun(context.Background(), run); err != nil {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestFormatRun_integrationIncludesCaseCounts(t *testing.T) {
	t.Parallel()
	start := time.Now()
	run := &evalspb.Run{
		Name:      "runs/abc",
		Type:      evalspb.Run_INTEGRATION_TEST,
		Status:    evalspb.Status_FAILED,
		Operation: "operations/xyz",
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(start.Add(2 * time.Second)),
		Data: &evalspb.Run_IntegrationTest{
			IntegrationTest: &evalspb.IntegrationTestResults{
				Cases: []*evalspb.IntegrationTestResults_Case{
					{Id: "files-v2.a", Status: evalspb.Status_PASSED},
					{Id: "files-v2.b", Status: evalspb.Status_FAILED},
					{Id: "files-v2.c", Status: evalspb.Status_NOT_EVALUATED},
				},
			},
		},
	}
	got := formatRun(run)
	for _, want := range []string{
		"runs/abc",
		"type=INTEGRATION_TEST",
		"status=FAILED",
		"cases=3",
		"passed=1",
		"failed=1",
		"skipped=1",
		"duration=2s",
		"op=operations/xyz",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary %q missing %q", got, want)
		}
	}
}

func TestFormatRun_agentEvalIncludesCaseCounts(t *testing.T) {
	t.Parallel()
	run := &evalspb.Run{
		Name:   "runs/agent",
		Type:   evalspb.Run_AGENT_EVAL,
		Status: evalspb.Status_PASSED,
		Data: &evalspb.Run_AgentEval{
			AgentEval: &evalspb.AgentEvalResults{
				Cases: []*evalspb.AgentEvalResults_Case{
					{Id: "eval_set_1.a", Status: evalspb.Status_PASSED},
					{Id: "eval_set_1.b", Status: evalspb.Status_PASSED},
				},
			},
		},
	}
	got := formatRun(run)
	for _, want := range []string{
		"type=AGENT_EVAL",
		"status=PASSED",
		"cases=2",
		"passed=2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary %q missing %q", got, want)
		}
	}
	for _, dontWant := range []string{"failed=", "skipped="} {
		if strings.Contains(got, dontWant) {
			t.Fatalf("summary %q unexpectedly contains %q", got, dontWant)
		}
	}
}

func TestJudgeDriftWarning_triggersWhenJudgeMetricsButZeroCount(t *testing.T) {
	t.Parallel()

	run := &evalspb.Run{
		Name:   "runs/agent-drift",
		Type:   evalspb.Run_AGENT_EVAL,
		Status: evalspb.Status_PASSED,
		Data: &evalspb.Run_AgentEval{
			AgentEval: &evalspb.AgentEvalResults{
				Cases: []*evalspb.AgentEvalResults_Case{{
					Id:     "eval_set_1.a",
					Status: evalspb.Status_PASSED,
					Metrics: []*evalspb.AgentEvalResults_Case_Metric{
						{Id: "rubric_based_final_response_quality_v1", Status: evalspb.Status_PASSED},
					},
				}},
				Judge: &evalspb.AgentEvalResults_JudgeInfo{Model: "gemini-2.5-pro"}, // count is 0
			},
		},
	}
	if !judgeDriftDetected(run) {
		t.Errorf("judgeDriftDetected() = false, want true (judge metric present, count = 0)")
	}
}

func TestJudgeDriftWarning_noTriggerWhenCountPositive(t *testing.T) {
	t.Parallel()

	run := &evalspb.Run{
		Type: evalspb.Run_AGENT_EVAL,
		Data: &evalspb.Run_AgentEval{
			AgentEval: &evalspb.AgentEvalResults{
				Cases: []*evalspb.AgentEvalResults_Case{{
					Id:     "a",
					Status: evalspb.Status_PASSED,
					Metrics: []*evalspb.AgentEvalResults_Case_Metric{
						{Id: "rubric_based_final_response_quality_v1", Status: evalspb.Status_PASSED},
					},
				}},
				Judge: &evalspb.AgentEvalResults_JudgeInfo{Model: "gemini-2.5-pro", JudgeCallCount: 1},
			},
		},
	}
	if judgeDriftDetected(run) {
		t.Errorf("judgeDriftDetected() = true, want false (count > 0)")
	}
}

func TestJudgeDriftWarning_noTriggerWhenNoJudgeMetrics(t *testing.T) {
	t.Parallel()

	run := &evalspb.Run{
		Type: evalspb.Run_AGENT_EVAL,
		Data: &evalspb.Run_AgentEval{
			AgentEval: &evalspb.AgentEvalResults{
				Cases: []*evalspb.AgentEvalResults_Case{{
					Id:     "a",
					Status: evalspb.Status_PASSED,
					Metrics: []*evalspb.AgentEvalResults_Case_Metric{
						{Id: "response_match_score", Status: evalspb.Status_PASSED},
					},
				}},
				// No judge sidecar at all.
			},
		},
	}
	if judgeDriftDetected(run) {
		t.Errorf("judgeDriftDetected() = true, want false (no judge metrics)")
	}
}

func TestJudgeDriftWarning_notForNonAgentEval(t *testing.T) {
	t.Parallel()

	run := &evalspb.Run{
		Type: evalspb.Run_INTEGRATION_TEST,
	}
	if judgeDriftDetected(run) {
		t.Errorf("judgeDriftDetected() = true, want false for non-AGENT_EVAL runs")
	}
}

func TestFormatRun_batchIdIncludedWhenSet(t *testing.T) {
	t.Parallel()
	bid := "batch-42"
	run := &evalspb.Run{
		Name:    "runs/b",
		Type:    evalspb.Run_INTEGRATION_TEST,
		Status:  evalspb.Status_PASSED,
		BatchId: new(bid),
	}
	got := formatRun(run)
	if !strings.Contains(got, "batch=batch-42") {
		t.Fatalf("summary %q missing batch id", got)
	}
}
