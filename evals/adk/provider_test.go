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

	client := adk.NewHTTPClientWithTokenSource(srv.URL, "/api", stubTokenSource{})

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
					Score:      ptrFloat(1.0),
					EvalStatus: models.EvalStatusPassed,
				}},
			}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	factory := func(_ context.Context, baseURL, pathPrefix string) (adk.Client, error) {
		return adk.NewHTTPClientWithTokenSource(baseURL, pathPrefix, stubTokenSource{}), nil
	}

	p := adk.NewProvider(adk.Agent{
		BaseURL: srv.URL,
		AppName: "test.agent.v1",
		DefaultMetrics: []models.EvalMetric{
			adk.ResponseMatchScore(0.3),
		},
	}, adk.WithClientFactory(factory))

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
	if len(results[0].Cases) != 1 || results[0].Cases[0].Name != "eval_set_1.hi" {
		t.Fatalf("cases = %#v", results[0].Cases)
	}
	if results[0].Cases[0].Status != evalspb.Status_PASSED {
		t.Fatalf("status = %v", results[0].Cases[0].Status)
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

	factory := func(_ context.Context, baseURL, pathPrefix string) (adk.Client, error) {
		return adk.NewHTTPClientWithTokenSource(baseURL, pathPrefix, stubTokenSource{}), nil
	}

	p := adk.NewProvider(adk.Agent{
		BaseURL: srv.URL,
		AppName: "test.agent.v1",
		DefaultMetrics: []models.EvalMetric{
			adk.ResponseMatchScore(0.3),
		},
		IncludeEvalSet: func(id string) bool { return id == "eval_set_1" },
	}, adk.WithClientFactory(factory))

	results, err := p.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(results) != 1 || results[0].SuiteName != "eval_set_1" {
		t.Fatalf("results = %#v", results)
	}
}
