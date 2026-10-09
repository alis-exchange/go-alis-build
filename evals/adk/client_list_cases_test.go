package adk_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"go.alis.build/evals/adk"
)

// TestHTTPClient_ListEvalCases_returnsServerOrder checks the request path and
// that ids come back in the order the launcher wrote them.
func TestHTTPClient_ListEvalCases_returnsServerOrder(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.EscapedPath() != "/api/dev/apps/test.agent.v1/eval_sets/smoke/evals" {
			t.Errorf("request = %s %s", r.Method, r.URL.EscapedPath())
			http.Error(w, "unexpected", http.StatusTeapot)
			return
		}
		_, _ = w.Write([]byte("[\"case_a\",\"case_b.v2\"]\n"))
	}))
	t.Cleanup(srv.Close)

	got, err := adk.NewHTTPClient(srv.URL).ListEvalCases(context.Background(), "test.agent.v1", "smoke")
	if err != nil {
		t.Fatalf("ListEvalCases() error = %v", err)
	}
	if want := []string{"case_a", "case_b.v2"}; !slices.Equal(got, want) {
		t.Fatalf("ListEvalCases() = %q, want %q", got, want)
	}
}

// TestHTTPClient_ListEvalCases_escapesPathSegments checks app and set ids are
// percent-escaped so a reserved character cannot split the path.
func TestHTTPClient_ListEvalCases_escapesPathSegments(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/dev/apps/a%3Fb/eval_sets/smoke/evals"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)

	if _, err := adk.NewHTTPClient(srv.URL).ListEvalCases(context.Background(), "a?b", "smoke"); err != nil {
		t.Fatalf("ListEvalCases() error = %v", err)
	}
}

// TestHTTPClient_ListEvalCases_errors checks each failure maps to the
// package's existing typed errors.
func TestHTTPClient_ListEvalCases_errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		body   string
		app    string
		set    string
		want   error
	}{
		{
			name:   "missing set",
			status: http.StatusNotFound,
			body:   `{"error":"eval set \"nope\" not found"}`,
			app:    "app",
			set:    "nope",
			want:   adk.ErrRunEvalFailed{},
		},
		{name: "not an array", status: http.StatusOK, body: `{"x":1}`, app: "app", set: "smoke", want: adk.ErrDecodeResponse{}},
		{name: "empty app", status: http.StatusOK, body: `[]`, app: "", set: "smoke", want: adk.ErrMissingAppNameEvalSetID{}},
		{name: "empty set", status: http.StatusOK, body: `[]`, app: "app", set: "", want: adk.ErrMissingAppNameEvalSetID{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(srv.Close)

			_, err := adk.NewHTTPClient(srv.URL).ListEvalCases(context.Background(), tt.app, tt.set)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ListEvalCases() error = %v, want %T", err, tt.want)
			}
			var failed adk.ErrRunEvalFailed
			if errors.As(err, &failed) && failed.StatusCode != tt.status {
				t.Fatalf("StatusCode = %d, want %d", failed.StatusCode, tt.status)
			}
		})
	}
}

// TestHTTPClient_ListEvalCases_emptyBody returns no ids and no error.
func TestHTTPClient_ListEvalCases_emptyBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)

	got, err := adk.NewHTTPClient(srv.URL).ListEvalCases(context.Background(), "app", "smoke")
	if err != nil || got != nil {
		t.Fatalf("ListEvalCases() = %q, %v; want nil, nil", got, err)
	}
}

// TestHTTPClient_ListEvalCases_nilReceiver returns ErrNilClient.
func TestHTTPClient_ListEvalCases_nilReceiver(t *testing.T) {
	t.Parallel()

	var c *adk.HTTPClient
	if _, err := c.ListEvalCases(context.Background(), "app", "smoke"); !errors.Is(err, adk.ErrNilClient{}) {
		t.Fatalf("ListEvalCases() error = %v, want ErrNilClient", err)
	}
}

// TestHTTPClient_implementsCaseLister pins the optional interface.
func TestHTTPClient_implementsCaseLister(t *testing.T) {
	t.Parallel()

	var _ adk.CaseLister = adk.NewHTTPClient("http://example.invalid")
}
