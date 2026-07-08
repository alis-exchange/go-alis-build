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

// recordingTransport is a stub http.RoundTripper that records the last
// request it saw and delegates to http.DefaultTransport. Tests use it to
// assert the client wired a custom transport into every outbound call.
type recordingTransport struct {
	lastReq *http.Request
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.lastReq = req
	return http.DefaultTransport.RoundTrip(req)
}

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
		_ = json.NewEncoder(w).Encode(want)
	}))
	t.Cleanup(srv.Close)

	rt := &recordingTransport{}
	client := adk.NewHTTPClient(srv.URL, adk.WithTransport(rt))

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
	if rt.lastReq == nil {
		t.Fatal("transport was not invoked; custom transport not wired")
	}
}

func TestHTTPClient_RunEval_withPathPrefixOverride(t *testing.T) {
	t.Parallel()

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode([]models.RunEvalResult{})
	}))
	t.Cleanup(srv.Close)

	client := adk.NewHTTPClient(srv.URL, adk.WithPathPrefix("/custom"))
	if _, err := client.RunEval(context.Background(), adk.RunEvalParams{
		AppName:   "my_app",
		EvalSetID: "set1",
	}); err != nil {
		t.Fatalf("RunEval() error = %v", err)
	}
	if want := "/custom/dev/apps/my_app/eval_sets/set1/run_eval"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
}

func TestHTTPClient_RunEval_perRequestPathPrefixWins(t *testing.T) {
	t.Parallel()

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode([]models.RunEvalResult{})
	}))
	t.Cleanup(srv.Close)

	client := adk.NewHTTPClient(srv.URL, adk.WithPathPrefix("/from-option"))
	if _, err := client.RunEval(context.Background(), adk.RunEvalParams{
		PathPrefix: "/from-params",
		AppName:    "my_app",
		EvalSetID:  "set1",
	}); err != nil {
		t.Fatalf("RunEval() error = %v", err)
	}
	if want := "/from-params/dev/apps/my_app/eval_sets/set1/run_eval"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
}

func ptrFloat(v float64) *float64 { return &v }
