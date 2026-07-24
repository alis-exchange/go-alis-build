package adk_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.alis.build/adk/launchers/evals/evaluation/models"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/adk"
)

func TestHTTPClient_ListEvalSets(t *testing.T) {
	t.Parallel()

	want := []string{"eval_set_1", "smoke"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/dev/apps/test.agent.v1/eval_sets" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	client := adk.NewHTTPClient(srv.URL)

	got, err := client.ListEvalSets(context.Background(), "test.agent.v1")
	if err != nil {
		t.Fatalf("ListEvalSets() error = %v", err)
	}
	if len(got) != 2 || got[0] != "eval_set_1" {
		t.Fatalf("got = %#v", got)
	}
}

func TestProvider_Run_discoversSets(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/dev/apps/test.agent.v1/eval_sets":
			_ = json.NewEncoder(w).Encode([]string{"eval_set_1"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/dev/apps/test.agent.v1/eval_sets/eval_set_1/run_eval":
			_ = json.NewEncoder(w).Encode([]models.RunEvalResult{{
				EvalID:          "hi",
				SessionID:       "sess-1",
				FinalEvalStatus: models.EvalStatusPassed,
				OverallEvalMetricResults: []models.EvalMetricResult{{
					MetricName: models.MetricResponseMatchScore,
					Threshold:  0.3,
					Score:      new(1.0),
					EvalStatus: models.EvalStatusPassed,
				}},
			}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	p := adk.NewProvider(adk.Agent{
		BaseURL: srv.URL,
		AppName: "test.agent.v1",
		DefaultMetrics: []models.EvalMetric{
			adk.ResponseMatchScore(0.3),
		},
	})

	results, err := p.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %#v", results)
	}
	if results[0].SuiteName != "eval_set_1" {
		t.Fatalf("suite = %q", results[0].SuiteName)
	}
	cases := results[0].Results.GetCases()
	if len(cases) != 1 || cases[0].GetId() != "eval_set_1.hi" {
		t.Fatalf("cases = %#v", cases)
	}
	if cases[0].GetStatus() != evalspb.Status_PASSED {
		t.Fatalf("status = %v", cases[0].GetStatus())
	}
}

func TestProvider_Run_populatesJudgeFromAgent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/dev/apps/test.agent.v1/eval_sets":
			_ = json.NewEncoder(w).Encode([]string{"eval_set_1"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/dev/apps/test.agent.v1/eval_sets/eval_set_1/run_eval":
			_ = json.NewEncoder(w).Encode([]models.RunEvalResult{
				{
					EvalID:          "c1",
					FinalEvalStatus: models.EvalStatusPassed,
					OverallEvalMetricResults: []models.EvalMetricResult{
						{MetricName: models.MetricRubricBasedFinalResponseQualityV1, EvalStatus: models.EvalStatusPassed},
						{MetricName: models.MetricResponseMatchScore, EvalStatus: models.EvalStatusPassed},
					},
				},
				{
					EvalID:          "c2",
					FinalEvalStatus: models.EvalStatusPassed,
					OverallEvalMetricResults: []models.EvalMetricResult{
						{MetricName: models.MetricHallucinationsV1, EvalStatus: models.EvalStatusPassed},
					},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	rubric, err := adk.RubricBasedFinalResponseQualityV1(0.7, nil, "ignored-because-agent-declared")
	if err != nil {
		t.Fatalf("RubricBasedFinalResponseQualityV1: %v", err)
	}
	p := adk.NewProvider(adk.Agent{
		BaseURL:           srv.URL,
		AppName:           "test.agent.v1",
		JudgeModel:        "gemini-2.5-pro",
		JudgeModelVersion: "2025-06-05",
		DefaultMetrics:    []models.EvalMetric{rubric},
	})

	results, err := p.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %#v", results)
	}
	sr := results[0]
	if sr.Results.GetJudge().GetModel() != "gemini-2.5-pro" {
		t.Errorf("Judge.Model = %q, want gemini-2.5-pro", sr.Results.GetJudge().GetModel())
	}
	if sr.Results.GetJudge().GetModelVersion() != "2025-06-05" {
		t.Errorf("Judge.ModelVersion = %q, want 2025-06-05", sr.Results.GetJudge().GetModelVersion())
	}
	// 2 judge metrics total: rubric_based_final_response_quality_v1 (c1) and hallucinations_v1 (c2).
	if sr.Results.GetJudge().GetJudgeCallCount() != 2 {
		t.Errorf("Judge.JudgeCallCount = %d, want 2", sr.Results.GetJudge().GetJudgeCallCount())
	}
}

func TestProvider_Run_probesJudgeModelWhenAgentEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/dev/apps/test.agent.v1/eval_sets":
			_ = json.NewEncoder(w).Encode([]string{"eval_set_1"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/dev/apps/test.agent.v1/eval_sets/eval_set_1/run_eval":
			_ = json.NewEncoder(w).Encode([]models.RunEvalResult{{
				EvalID:          "c1",
				FinalEvalStatus: models.EvalStatusPassed,
				OverallEvalMetricResults: []models.EvalMetricResult{
					{MetricName: models.MetricRubricBasedFinalResponseQualityV1, EvalStatus: models.EvalStatusPassed},
				},
			}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	rubric, err := adk.RubricBasedFinalResponseQualityV1(0.7, nil, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("RubricBasedFinalResponseQualityV1: %v", err)
	}
	p := adk.NewProvider(adk.Agent{
		BaseURL:        srv.URL,
		AppName:        "test.agent.v1",
		DefaultMetrics: []models.EvalMetric{rubric},
	})

	results, err := p.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := results[0].Results.GetJudge().GetModel(), "gemini-2.5-flash"; got != want {
		t.Errorf("Judge.Model = %q, want %q (probed fallback)", got, want)
	}
	if results[0].Results.GetJudge().GetModelVersion() != "" {
		t.Errorf("Judge.ModelVersion = %q, want empty (no fallback source)", results[0].Results.GetJudge().GetModelVersion())
	}
	if results[0].Results.GetJudge().GetJudgeCallCount() != 1 {
		t.Errorf("Judge.JudgeCallCount = %d, want 1", results[0].Results.GetJudge().GetJudgeCallCount())
	}
}

func TestProvider_Run_probesJudgeModelFromMetricOverride(t *testing.T) {
	t.Parallel()

	// Exercises the MetricOverrides[setID] path in probeJudgeModel
	// fallback: DefaultMetrics carries one judgeModel, but the set's
	// override carries a different one — the override must win.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/dev/apps/test.agent.v1/eval_sets":
			_ = json.NewEncoder(w).Encode([]string{"override_set"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/dev/apps/test.agent.v1/eval_sets/override_set/run_eval":
			_ = json.NewEncoder(w).Encode([]models.RunEvalResult{{
				EvalID:          "c1",
				FinalEvalStatus: models.EvalStatusPassed,
				OverallEvalMetricResults: []models.EvalMetricResult{
					{MetricName: models.MetricRubricBasedFinalResponseQualityV1, EvalStatus: models.EvalStatusPassed},
				},
			}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	defaultRubric, err := adk.RubricBasedFinalResponseQualityV1(0.7, nil, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("default RubricBasedFinalResponseQualityV1: %v", err)
	}
	overrideRubric, err := adk.RubricBasedFinalResponseQualityV1(0.7, nil, "gemini-2.5-pro")
	if err != nil {
		t.Fatalf("override RubricBasedFinalResponseQualityV1: %v", err)
	}

	p := adk.NewProvider(adk.Agent{
		BaseURL:        srv.URL,
		AppName:        "test.agent.v1",
		DefaultMetrics: []models.EvalMetric{defaultRubric},
		MetricOverrides: map[string][]models.EvalMetric{
			"override_set": {overrideRubric},
		},
	})

	results, err := p.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := results[0].Results.GetJudge().GetModel(), "gemini-2.5-pro"; got != want {
		t.Errorf("Judge.Model = %q, want %q (override set)", got, want)
	}
}

func TestProvider_Run_noJudgeWhenNothingConfigured(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/dev/apps/test.agent.v1/eval_sets":
			_ = json.NewEncoder(w).Encode([]string{"eval_set_1"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/dev/apps/test.agent.v1/eval_sets/eval_set_1/run_eval":
			_ = json.NewEncoder(w).Encode([]models.RunEvalResult{{
				EvalID:          "c1",
				FinalEvalStatus: models.EvalStatusPassed,
				OverallEvalMetricResults: []models.EvalMetricResult{
					{MetricName: models.MetricResponseMatchScore, EvalStatus: models.EvalStatusPassed},
				},
			}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	p := adk.NewProvider(adk.Agent{
		BaseURL:        srv.URL,
		AppName:        "test.agent.v1",
		DefaultMetrics: []models.EvalMetric{adk.ResponseMatchScore(0.3)},
	})

	results, err := p.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := results[0].Results.GetJudge().GetModel(); got != "" {
		t.Errorf("Judge.Model = %q, want empty", got)
	}
	if got := results[0].Results.GetJudge().GetJudgeCallCount(); got != 0 {
		t.Errorf("Judge.JudgeCallCount = %d, want 0", got)
	}
}

func TestProvider_Run_respectsIncludeEvalSet(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/dev/apps/test.agent.v1/eval_sets":
			_ = json.NewEncoder(w).Encode([]string{"draft_set", "eval_set_1"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/dev/apps/test.agent.v1/eval_sets/eval_set_1/run_eval":
			_ = json.NewEncoder(w).Encode([]models.RunEvalResult{})
		case r.Method == http.MethodPost:
			t.Fatalf("unexpected run_eval for %s", r.URL.Path)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	p := adk.NewProvider(adk.Agent{
		BaseURL: srv.URL,
		AppName: "test.agent.v1",
		DefaultMetrics: []models.EvalMetric{
			adk.ResponseMatchScore(0.3),
		},
		IncludeEvalSet: func(id string) bool { return id == "eval_set_1" },
	})

	results, err := p.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(results) != 1 || results[0].SuiteName != "eval_set_1" {
		t.Fatalf("results = %#v", results)
	}
}
