package adk_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.alis.build/evals/adk"
	iam "go.alis.build/iam/v3"
	"golang.org/x/oauth2"
)

type stubTokenSource struct {
	token *oauth2.Token
	err   error
}

func (s stubTokenSource) Token() (*oauth2.Token, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.token != nil {
		return s.token, nil
	}
	return &oauth2.Token{AccessToken: "cloud-run-token", TokenType: "Bearer"}, nil
}

func TestTransport_setsAuthHeaders(t *testing.T) {
	t.Parallel()

	var gotServerless, gotIdentity, gotForwarded string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotServerless = r.Header.Get("X-Serverless-Authorization")
		gotIdentity = r.Header.Get("X-Alis-Identity")
		gotForwarded = r.Header.Get("X-Alis-Forwarded-Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := &http.Client{
		Transport: &adk.Transport{
			Base:        http.DefaultTransport,
			TokenSource: stubTokenSource{},
		},
	}

	identity := &iam.Identity{Type: iam.User, ID: "eval-user", Email: "eval@example.com"}
	ctx := identity.Context(context.Background())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if gotServerless != "Bearer cloud-run-token" {
		t.Fatalf("X-Serverless-Authorization = %q", gotServerless)
	}
	if gotIdentity == "" {
		t.Fatal("expected X-Alis-Identity")
	}
	if gotForwarded == "" {
		t.Fatal("expected X-Alis-Forwarded-Authorization")
	}
}

func TestAudienceFromBaseURL(t *testing.T) {
	t.Parallel()

	got, err := adk.AudienceFromBaseURL("https://my-agent-abc.a.run.app")
	if err != nil {
		t.Fatalf("AudienceFromBaseURL() error = %v", err)
	}
	if got != "https://my-agent-abc.a.run.app" {
		t.Fatalf("audience = %q", got)
	}
}
