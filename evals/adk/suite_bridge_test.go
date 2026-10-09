package adk_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.alis.build/adk/launchers/evals/evaluation/models"
	evalspb "go.alis.build/common/alis/evals"
	"go.alis.build/evals"
	"go.alis.build/evals/adk"
)

// *evals.AgentEvalResult must satisfy CaseRecorder without adk importing evals.
var _ adk.CaseRecorder = (*evals.AgentEvalResult)(nil)

// bridgeServer serves the verified launcher shapes: a sorted id list and a
// run_eval answer for the single requested case.
func bridgeServer(t *testing.T) *httptest.Server {
	t.Helper()
	results := map[string]models.RunEvalResult{
		"case_a": {
			EvalID: "case_a", SessionID: "sess-a", FinalEvalStatus: models.EvalStatusPassed,
			OverallEvalMetricResults: []models.EvalMetricResult{
				{MetricName: models.MetricFinalResponseMatchV2, Threshold: 0.5, Score: new(0.9), EvalStatus: models.EvalStatusPassed},
			},
		},
		"case_b.v2": {
			EvalID: "case_b.v2", SessionID: "sess-b", FinalEvalStatus: models.EvalStatusFailed,
			OverallEvalMetricResults: []models.EvalMetricResult{
				{MetricName: models.MetricFinalResponseMatchV2, Threshold: 0.5, Score: new(0.2), EvalStatus: models.EvalStatusFailed},
			},
		},
		"case_c": {
			EvalID: "case_c", SessionID: "sess-c", FinalEvalStatus: models.EvalStatusNotEvaluated,
			OverallEvalMetricResults: []models.EvalMetricResult{
				{MetricName: models.MetricFinalResponseMatchV2, Threshold: 0.5, EvalStatus: models.EvalStatusNotEvaluated},
			},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/dev/apps/test.agent.v1/eval_sets/smoke/evals":
			_, _ = w.Write([]byte(`["case_a","case_b.v2","case_c"]`))
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/api/dev/apps/test.agent.v1/eval_sets/smoke/run_eval":
			var body struct {
				EvalCaseIDs []string `json:"eval_case_ids"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.EvalCaseIDs) != 1 {
				t.Errorf("run_eval body = %+v, err = %v", body, err)
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode([]models.RunEvalResult{results[body.EvalCaseIDs[0]]})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
			http.Error(w, "unexpected", http.StatusTeapot)
		}
	}))
}

// TestSuiteBridge_runsEachADKCaseAsASuiteCase builds an AgentEvalSuite from
// ListCases, RunCase, and RecordTo and checks the materialized run.
func TestSuiteBridge_runsEachADKCaseAsASuiteCase(t *testing.T) {
	t.Parallel()

	srv := bridgeServer(t)
	t.Cleanup(srv.Close)

	ctx := context.Background()
	provider := adk.NewProvider(adk.Agent{BaseURL: srv.URL, AppName: "test.agent.v1", JudgeModel: "gemini-2.5-pro"})
	ids, err := provider.ListCases(ctx, "smoke")
	if err != nil {
		t.Fatalf("ListCases() error = %v", err)
	}
	suite := evals.NewAgentEvalSuite("smoke")
	for _, id := range ids {
		suite.AddCase(adk.SuiteCaseName(id), func(ctx context.Context, r *evals.AgentEvalResult) {
			res, err := provider.RunCase(ctx, "smoke", id)
			if err != nil {
				r.Fail(err)
				return
			}
			res.RecordTo(r)
		})
	}

	run, err := suite.Run(ctx, evals.WithMaxConcurrency(2))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("run status = %v, want FAILED", run.GetStatus())
	}
	cases := run.GetAgentEval().GetCases()
	type got struct {
		id      string
		status  evalspb.Status
		session string
	}
	want := []got{
		{"smoke.case_a", evalspb.Status_PASSED, "sess-a"},
		{"smoke.case_b_v2", evalspb.Status_FAILED, "sess-b"},
		{"smoke.case_c", evalspb.Status_NOT_EVALUATED, "sess-c"},
	}
	if len(cases) != len(want) {
		t.Fatalf("cases = %d, want %d", len(cases), len(want))
	}
	for i, c := range cases {
		if g := (got{c.GetId(), c.GetStatus(), c.GetSessionId()}); g != want[i] {
			t.Fatalf("case %d = %+v, want %+v", i, g, want[i])
		}
	}
	hasValidation := func(c *evalspb.AgentEvalResults_Case, status evalspb.Status, msg string) bool {
		for _, v := range c.GetValidations() {
			if v.GetId() == "_evals.case" && v.GetStatus() == status && v.GetMessage() == msg {
				return true
			}
		}
		return false
	}
	if !hasValidation(cases[1], evalspb.Status_FAILED, "adk: final eval status FAILED") {
		t.Fatalf("case_b_v2 validations = %v, want _evals.case final-status failure", cases[1].GetValidations())
	}
	if !hasValidation(cases[2], evalspb.Status_NOT_EVALUATED, "adk: final eval status NOT_EVALUATED") {
		t.Fatalf("case_c validations = %v, want _evals.case NOT_EVALUATED final status", cases[2].GetValidations())
	}
	if got := len(cases[2].GetMetrics()); got != 1 {
		t.Fatalf("case_c metrics = %d, want 1 (data kept)", got)
	}
	judge := run.GetAgentEval().GetJudge()
	if judge.GetModel() != "gemini-2.5-pro" || judge.GetJudgeCallCount() != 3 {
		t.Fatalf("judge = %v, want gemini-2.5-pro with 3 calls", judge)
	}
}
