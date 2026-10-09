package adk_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"go.alis.build/adk/launchers/evals/evaluation/models"
	"go.alis.build/evals/adk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// runOnlyClient implements Client but not CaseLister.
type runOnlyClient struct{}

// RunEval returns no results.
func (runOnlyClient) RunEval(context.Context, adk.RunEvalParams) ([]models.RunEvalResult, error) {
	return nil, nil
}

// ListEvalSets returns no eval sets.
func (runOnlyClient) ListEvalSets(context.Context, string) ([]string, error) { return nil, nil }

// TestProvider_ListCases_returnsLauncherIDs lists through the HTTP client.
func TestProvider_ListCases_returnsLauncherIDs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/dev/apps/test.agent.v1/eval_sets/smoke/evals" {
			t.Errorf("path = %s", r.URL.EscapedPath())
			http.Error(w, "unexpected", http.StatusTeapot)
			return
		}
		_, _ = w.Write([]byte(`["case_a","case_b.v2"]`))
	}))
	t.Cleanup(srv.Close)

	p := adk.NewProvider(adk.Agent{BaseURL: srv.URL, AppName: "test.agent.v1"})
	got, err := p.ListCases(context.Background(), "smoke")
	if err != nil {
		t.Fatalf("ListCases() error = %v", err)
	}
	if want := []string{"case_a", "case_b.v2"}; !slices.Equal(got, want) {
		t.Fatalf("ListCases() = %q, want %q", got, want)
	}
}

// TestProvider_ListCases_unsupportedClient returns ErrCaseListingUnsupported.
func TestProvider_ListCases_unsupportedClient(t *testing.T) {
	t.Parallel()

	p := adk.NewProvider(
		adk.Agent{BaseURL: "http://example.invalid", AppName: "app"},
		adk.WithClientFactory(func(context.Context, string, string) (adk.Client, error) {
			return runOnlyClient{}, nil
		}),
	)
	_, err := p.ListCases(context.Background(), "smoke")
	if !errors.Is(err, adk.ErrCaseListingUnsupported{}) {
		t.Fatalf("ListCases() error = %v, want ErrCaseListingUnsupported", err)
	}
	if got := status.Code(err); got != codes.Unimplemented {
		t.Fatalf("status.Code = %v, want Unimplemented", got)
	}
}

// TestProvider_ListCases_validation checks configuration errors.
func TestProvider_ListCases_validation(t *testing.T) {
	t.Parallel()

	var nilProvider *adk.Provider
	if _, err := nilProvider.ListCases(context.Background(), "smoke"); !errors.Is(err, adk.ErrNilProvider{}) {
		t.Fatalf("nil provider error = %v", err)
	}
	if _, err := adk.NewProvider(adk.Agent{}).ListCases(context.Background(), "smoke"); !errors.Is(err, adk.ErrMissingProviderConfig{}) {
		t.Fatalf("empty config error = %v", err)
	}
	p := adk.NewProvider(adk.Agent{BaseURL: "http://example.invalid", AppName: "app"})
	if _, err := p.ListCases(context.Background(), ""); !errors.Is(err, adk.ErrMissingAppNameEvalSetID{}) {
		t.Fatalf("empty set error = %v", err)
	}
}

// TestProvider_RunCase_runsOneCase checks the filter, metrics, session
// state, and per-case judge context.
func TestProvider_RunCase_runsOneCase(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/api/dev/apps/test.agent.v1/eval_sets/smoke/run_eval" {
			t.Errorf("request = %s %s", r.Method, r.URL.EscapedPath())
			http.Error(w, "unexpected", http.StatusTeapot)
			return
		}
		var body struct {
			EvalCaseIDs  []string       `json:"eval_case_ids"`
			SessionState map[string]any `json:"session_state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if !slices.Equal(body.EvalCaseIDs, []string{"case_b.v2"}) || body.SessionState["tenant"] != "acme" {
			t.Errorf("body = %+v", body)
		}
		_ = json.NewEncoder(w).Encode([]models.RunEvalResult{{
			EvalID:          "case_b.v2",
			SessionID:       "sess-b",
			FinalEvalStatus: models.EvalStatusPassed,
			OverallEvalMetricResults: []models.EvalMetricResult{
				{MetricName: models.MetricResponseMatchScore, Threshold: 0.3, Score: new(1.0), EvalStatus: models.EvalStatusPassed},
				{MetricName: models.MetricFinalResponseMatchV2, Threshold: 0.5, Score: new(0.9), EvalStatus: models.EvalStatusPassed},
			},
		}})
	}))
	t.Cleanup(srv.Close)

	p := adk.NewProvider(adk.Agent{
		BaseURL:           srv.URL,
		AppName:           "test.agent.v1",
		DefaultMetrics:    []models.EvalMetric{adk.ResponseMatchScore(0.3)},
		JudgeModel:        "gemini-2.5-pro",
		JudgeModelVersion: "2025-06-05",
	})
	got, err := p.RunCase(context.Background(), "smoke", "case_b.v2", adk.WithSessionState(map[string]any{"tenant": "acme"}))
	if err != nil {
		t.Fatalf("RunCase() error = %v", err)
	}
	if got.SetID != "smoke" || got.Result.EvalID != "case_b.v2" || got.Result.SessionID != "sess-b" {
		t.Fatalf("RunCase() = %+v", got)
	}
	want := adk.JudgeContext{Model: "gemini-2.5-pro", ModelVersion: "2025-06-05", CallCount: 1}
	if got.Judge != want {
		t.Fatalf("Judge = %+v, want %+v", got.Judge, want)
	}
}

// TestProvider_RunCase_missingCase returns ErrRunEval when run_eval drops the id.
func TestProvider_RunCase_missingCase(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)

	p := adk.NewProvider(adk.Agent{BaseURL: srv.URL, AppName: "test.agent.v1"})
	_, err := p.RunCase(context.Background(), "smoke", "nope")
	if !errors.Is(err, adk.ErrRunEval{}) {
		t.Fatalf("RunCase() error = %v, want ErrRunEval", err)
	}
	if want := `run_eval smoke: adk: case "nope" missing from run_eval response`; err.Error() != want {
		t.Fatalf("RunCase() error = %q, want %q", err, want)
	}
}

// TestProvider_RunCase_validation rejects empty ids before any request.
func TestProvider_RunCase_validation(t *testing.T) {
	t.Parallel()

	p := adk.NewProvider(adk.Agent{BaseURL: "http://example.invalid", AppName: "app"})
	if _, err := p.RunCase(context.Background(), "", "c1"); !errors.Is(err, adk.ErrMissingAppNameEvalSetID{}) {
		t.Fatalf("empty set error = %v", err)
	}
	_, err := p.RunCase(context.Background(), "smoke", "")
	if !errors.Is(err, adk.ErrRunEval{}) || !strings.Contains(err.Error(), "case id is required") {
		t.Fatalf("empty case error = %v", err)
	}
	var nilProvider *adk.Provider
	if _, err := nilProvider.RunCase(context.Background(), "smoke", "c1"); !errors.Is(err, adk.ErrNilProvider{}) {
		t.Fatalf("nil provider error = %v", err)
	}
}
