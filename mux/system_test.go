package mux

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/api/idtoken"
)

// setupSystemTest installs a fresh mux with one system route and replaces
// Google ID token validation with a stub that returns payload, or err when it
// is set. It returns a flag the route sets when it runs. Package state is
// restored when the test ends.
func setupSystemTest(t *testing.T, payload *idtoken.Payload, err error, audiences []string) *atomic.Bool {
	t.Helper()
	mux = http.NewServeMux()
	gateway = nil
	oldValidate, oldAudiences := validateIDToken, SystemAudiences
	t.Cleanup(func() { validateIDToken, SystemAudiences = oldValidate, oldAudiences })

	validateIDToken = func(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
		if audience != "" {
			t.Errorf("idtoken audience = %q, want empty: SystemAudiences is compared after validation", audience)
		}
		return payload, err
	}
	SystemAudiences = audiences

	var ran atomic.Bool
	SystemGet("/system", func(w http.ResponseWriter, r *http.Request) error {
		ran.Store(true)
		return nil
	})
	return &ran
}

// systemRequest calls the route registered by setupSystemTest with a bearer
// token that iam.MustFromJWT can parse.
func systemRequest(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	token := testJWT(t, map[string]any{"email": EnvironmentServiceAccountEmail, "sub": "1234"})
	req := httptest.NewRequest(http.MethodGet, "https://svc.example.com/system", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// systemPayload returns a validated ID token payload for audience carrying
// email, or no email claim when email is empty.
func systemPayload(audience, email string) *idtoken.Payload {
	claims := map[string]any{}
	if email != "" {
		claims["email"] = email
	}
	return &idtoken.Payload{Audience: audience, Claims: claims}
}

// TestSystemMiddleware pins how system routes treat a Google ID token's
// signature, email and audience. With SystemAudiences unset, which is the
// default, any audience is accepted as before; a token without an email claim
// is rejected rather than panicking.
func TestSystemMiddleware(t *testing.T) {
	const runApp = "https://svc-abc123.a.run.app"
	tests := []struct {
		name      string
		payload   *idtoken.Payload
		err       error
		audiences []string
		wantCode  int
		wantBody  string
	}{
		{
			name:     "invalid signature is rejected",
			err:      errors.New("bad signature"),
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "token without an email claim is rejected, not a panic",
			payload:  systemPayload(runApp, ""),
			wantCode: http.StatusUnauthorized,
			wantBody: "not environment service account",
		},
		{
			name:     "other service account is rejected",
			payload:  systemPayload(runApp, "someone@else.iam.gserviceaccount.com"),
			wantCode: http.StatusUnauthorized,
			wantBody: "not environment service account",
		},
		{
			name:     "any audience is accepted while SystemAudiences is unset",
			payload:  systemPayload("https://some-other-service.example.com", EnvironmentServiceAccountEmail),
			wantCode: http.StatusOK,
		},
		{
			name:      "listed audience is accepted",
			payload:   systemPayload(runApp, EnvironmentServiceAccountEmail),
			audiences: []string{"https://svc.example.com", runApp},
			wantCode:  http.StatusOK,
		},
		{
			name:      "unlisted audience is rejected and named",
			payload:   systemPayload("https://some-other-service.example.com", EnvironmentServiceAccountEmail),
			audiences: []string{runApp},
			wantCode:  http.StatusUnauthorized,
			wantBody:  `"https://some-other-service.example.com"`,
		},
		{
			name:      "audience is only checked for the environment service account",
			payload:   systemPayload("https://some-other-service.example.com", "someone@else.iam.gserviceaccount.com"),
			audiences: []string{runApp},
			wantCode:  http.StatusUnauthorized,
			wantBody:  "not environment service account",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ran := setupSystemTest(t, tt.payload, tt.err, tt.audiences)
			rec := systemRequest(t)
			expect(t, rec.Code, tt.wantCode)
			expect(t, ran.Load(), tt.wantCode == http.StatusOK)
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body %q does not contain %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}
