package adk_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.alis.build/adk/launchers/evals/evaluation/models"
	"go.alis.build/evals/adk"
)

func TestHTTPClient_RunEval_legacyArrayResponse(t *testing.T) {
	t.Parallel()

	want := []models.RunEvalResult{{
		EvalID:          "case1",
		SessionID:       "sess-1",
		FinalEvalStatus: models.EvalStatusPassed,
		OverallEvalMetricResults: []models.EvalMetricResult{{
			MetricName: "response_match_score",
			Threshold:  0.5,
			Score:      ptrFloat(1.0),
			EvalStatus: models.EvalStatusPassed,
		}},
	}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/dev/apps/my_app/eval_sets/set1/run_eval" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Serverless-Authorization"); got == "" {
			t.Fatal("missing X-Serverless-Authorization")
		}
		_ = json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	client := adk.NewHTTPClientWithTokenSource(srv.URL, "/api", stubTokenSource{})

	got, err := client.RunEval(context.Background(), adk.RunEvalParams{
		AppName:   "my_app",
		EvalSetID: "set1",
		Metrics: []models.EvalMetric{{
			MetricName: models.MetricResponseMatchScore,
			Threshold:  0.5,
		}},
	})
	if err != nil {
		t.Fatalf("RunEval() error = %v", err)
	}
	if len(got) != 1 || got[0].EvalID != "case1" {
		t.Fatalf("results = %+v", got)
	}
}

func ptrFloat(v float64) *float64 { return &v }
