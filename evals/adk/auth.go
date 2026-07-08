package adk

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	iam "go.alis.build/iam/v3"
	"golang.org/x/oauth2"
)

const (
	headerServerlessAuth = "X-Serverless-Authorization"
	headerAlisIdentity   = "X-Alis-Identity"
	headerForwardedAuth  = "X-Alis-Forwarded-Authorization"
)

// Transport stamps Cloud Run and Alis identity headers on outbound HTTP requests.
type Transport struct {
	Base        http.RoundTripper
	TokenSource oauth2.TokenSource
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	token, err := t.TokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("adk auth: token: %w", err)
	}
	tokenType := token.Type()
	if tokenType == "" {
		tokenType = "Bearer"
	}

	cloned := req.Clone(req.Context())
	cloned.Header.Set(headerServerlessAuth, tokenType+" "+token.AccessToken)

	identity := identityFromContext(req.Context())
	identityCtx := identity.Context(req.Context())
	cloned.Header.Set(headerAlisIdentity, string(identity.Marshal()))
	cloned.Header.Set(headerForwardedAuth, identity.UnsignedJWT(identityCtx))

	return base.RoundTrip(cloned)
}

func identityFromContext(ctx context.Context) *iam.Identity {
	id, err := iam.FromContext(ctx)
	if err != nil || id == nil {
		return iam.SystemIdentity
	}
	return id
}

// AudienceFromBaseURL returns the Cloud Run ID token audience for a target base URL.
func AudienceFromBaseURL(baseURL string) (string, error) {
	host, err := hostFromBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return "https://" + host, nil
}

func hostFromBaseURL(baseURL string) (string, error) {
	s := strings.TrimSpace(baseURL)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if s == "" {
		return "", fmt.Errorf("adk auth: empty base URL")
	}
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, ':'); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "", fmt.Errorf("adk auth: invalid base URL %q", baseURL)
	}
	return s, nil
}
