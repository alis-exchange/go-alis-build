package mapper

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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

func TestAgentEvalRun_rubricRationaleRoundtrip(t *testing.T) {
	t.Parallel()

	rubricScore := 0.42
	sr := execution.SuiteResult{
		SuiteName: "core",
		Cases: []execution.CaseResult{{
			Name:      "core.eval",
			Status:    evalspb.Status_FAILED,
			SessionID: "sess-1",
			Metrics: []execution.Metric{{
				ID:        "rubric_based_final_response_quality_v1",
				Status:    evalspb.Status_FAILED,
				Threshold: 0.7,
				Score:     &rubricScore,
				Rubric: []execution.RubricScore{
					{
						ID:        "accuracy",
						Status:    evalspb.Status_FAILED,
						Score:     &rubricScore,
						Rationale: "response paraphrased the reference correctly but omitted the source citation",
					},
					{
						ID:     "no_rationale",
						Status: evalspb.Status_PASSED,
						Score:  ptrFloat64(0.95),
					},
				},
			}},
		}},
	}

	rubrics := AgentEvalRun(sr, "operations/op-r", "run-r").GetAgentEval().GetCases()[0].GetMetrics()[0].GetRubric()
	if len(rubrics) != 2 {
		t.Fatalf("rubrics = %d, want 2", len(rubrics))
	}
	if got, want := rubrics[0].GetRationale(), "response paraphrased the reference correctly but omitted the source citation"; got != want {
		t.Errorf("wire rationale[0] = %q, want %q", got, want)
	}
	// Empty rationale must yield a nil pointer so proto readers can
	// distinguish "not set" from an explicit empty string.
	if rubrics[1].Rationale != nil {
		t.Errorf("wire Rationale[1] = %v, want nil (source Rationale was empty)", rubrics[1].Rationale)
	}
}

func TestAgentEvalRun_noJudgeWhenSuiteJudgeEmpty(t *testing.T) {
	t.Parallel()

	sr := execution.SuiteResult{
		SuiteName: "core",
		Cases: []execution.CaseResult{{
			Name: "core.eval", Status: evalspb.Status_PASSED,
		}},
	}
	if !sr.Judge.IsZero() {
		t.Fatalf("test setup: sr.Judge = %+v, want IsZero", sr.Judge)
	}

	got := agentEvalData(sr)
	if got.GetJudge() != nil {
		t.Errorf("Judge = %+v, want nil for zero-valued suite Judge + JudgeCallCount==0", got.GetJudge())
	}
}

func TestAgentEvalRun_judgeEmittedWithProvenance(t *testing.T) {
	t.Parallel()

	sr := execution.SuiteResult{
		SuiteName: "core",
		Cases: []execution.CaseResult{{
			Name: "core.eval", Status: evalspb.Status_PASSED, JudgeCallCount: 2,
		}},
		Judge:          execution.JudgeInfo{Model: "gemini-2.5-pro", ModelVersion: "2025-06-05"},
		JudgeCallCount: 2,
	}

	got := agentEvalData(sr).GetJudge()
	if got == nil {
		t.Fatal("Judge is nil, want populated")
	}
	if got.GetModel() != "gemini-2.5-pro" {
		t.Errorf("Model = %q, want gemini-2.5-pro", got.GetModel())
	}
	if got.GetModelVersion() != "2025-06-05" {
		t.Errorf("ModelVersion = %q, want 2025-06-05", got.GetModelVersion())
	}
	if got.GetJudgeCallCount() != 2 {
		t.Errorf("JudgeCallCount = %d, want 2", got.GetJudgeCallCount())
	}
	if got.GetJudgeErrorCount() != 0 {
		t.Errorf("JudgeErrorCount = %d, want 0", got.GetJudgeErrorCount())
	}
}

func TestAgentEvalRun_judgeEmittedWhenOnlyCountSet(t *testing.T) {
	t.Parallel()

	// Suite.Judge is zero-value but JudgeCallCount > 0 — this is the
	// out-of-band-signal-only case; mapper must still emit Judge.
	sr := execution.SuiteResult{
		SuiteName:      "core",
		Cases:          []execution.CaseResult{{Name: "c1", JudgeCallCount: 5}},
		JudgeCallCount: 5,
	}

	got := agentEvalData(sr).GetJudge()
	if got == nil {
		t.Fatal("Judge is nil, want emitted when JudgeCallCount > 0")
	}
	if got.GetModel() != "" {
		t.Errorf("Model = %q, want empty (no provenance declared)", got.GetModel())
	}
	if got.GetJudgeCallCount() != 5 {
		t.Errorf("JudgeCallCount = %d, want 5", got.GetJudgeCallCount())
	}
}

func TestAgentEvalRun_goldenSnapshots(t *testing.T) {
	// Sequential — subtests share fixture-loading helper.

	tt := []struct {
		name    string
		sr      execution.SuiteResult
		fixture string
	}{
		{
			name: "no_judge",
			sr: execution.SuiteResult{
				SuiteName: "core",
				Cases: []execution.CaseResult{{
					Name:      "core.eval",
					Status:    evalspb.Status_PASSED,
					SessionID: "sess-1",
					Metrics: []execution.Metric{
						{ID: "response_match_score", Status: evalspb.Status_PASSED, Threshold: 0.3},
					},
					Duration: time.Second,
				}},
			},
			fixture: "agent_eval_no_judge.golden.json",
		},
		{
			name: "with_judge",
			sr: execution.SuiteResult{
				SuiteName: "core",
				Cases: []execution.CaseResult{{
					Name:      "core.eval",
					Status:    evalspb.Status_PASSED,
					SessionID: "sess-1",
					Metrics: []execution.Metric{
						{ID: "rubric_based_final_response_quality_v1", Status: evalspb.Status_PASSED, Threshold: 0.7, Score: ptrFloat64(0.85)},
						{ID: "hallucinations_v1", Status: evalspb.Status_PASSED, Threshold: 0.8, Score: ptrFloat64(0.9)},
					},
					Duration:       time.Second,
					JudgeCallCount: 2,
				}},
				Judge:          execution.JudgeInfo{Model: "gemini-2.5-pro", ModelVersion: "2025-06-05"},
				JudgeCallCount: 2,
			},
			fixture: "agent_eval_with_judge.golden.json",
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalJSON(t, agentEvalData(tc.sr))
			want := readGolden(t, tc.fixture)
			if !bytes.Equal(got, want) {
				t.Errorf("wire snapshot mismatch for %s\n--- got\n%s\n--- want\n%s", tc.fixture, string(got), string(want))
			}
		})
	}
}

func canonicalJSON(t *testing.T, m proto.Message) []byte {
	t.Helper()
	raw, err := protojson.MarshalOptions{UseProtoNames: false}.Marshal(m)
	if err != nil {
		t.Fatalf("protojson.Marshal: %v", err)
	}
	var into map[string]any
	if err := json.Unmarshal(raw, &into); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	out, err := json.MarshalIndent(into, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent: %v", err)
	}
	return append(out, '\n')
}

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	return b
}

func ptrFloat64(v float64) *float64 { return &v }

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
