package adk_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
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
