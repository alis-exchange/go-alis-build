package authn

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// audienceToken builds an unsigned access token expiring at exp. A nil aud
// omits the claim; any other value, including a JSON null or an array, is
// written as given.
func audienceToken(t *testing.T, exp time.Time, aud any, omit bool) string {
	t.Helper()
	claims := map[string]any{"exp": exp.Unix(), "email": "john@example.com", "sub": "1934872948"}
	if !omit {
		claims["aud"] = aud
	}
	return testJWT(t, claims)
}

// TestValidateTokenWithAudience pins how each shape of aud claim RFC 7519
// allows is compared against the expected audience, and that a mismatch is
// reported as ErrInvalidAudience with the historical message.
func TestValidateTokenWithAudience(t *testing.T) {
	const audience = "https://app.example.com"
	now := time.Now()
	valid := now.Add(time.Minute)
	client := NewClient("https://identity.example.com")
	client.SkipSignatureValidation = true

	tests := []struct {
		name         string
		token        string
		wantErr      bool
		wantAudience bool
	}{
		{name: "absent aud is accepted", token: audienceToken(t, valid, nil, true)},
		{name: "empty string aud is accepted", token: audienceToken(t, valid, "", false)},
		{name: "null aud is accepted", token: audienceToken(t, valid, nil, false)},
		{name: "empty array aud is accepted", token: audienceToken(t, valid, []string{}, false)},
		{name: "matching string aud is accepted", token: audienceToken(t, valid, audience, false)},
		{name: "array aud containing the audience is accepted", token: audienceToken(t, valid, []string{"other", audience}, false)},
		{name: "different string aud is rejected", token: audienceToken(t, valid, "my-client-id", false), wantErr: true, wantAudience: true},
		{name: "array aud without the audience is rejected", token: audienceToken(t, valid, []string{"a", "b"}, false), wantErr: true, wantAudience: true},
		{name: "non-string aud is a malformed claim, not a mismatch", token: audienceToken(t, valid, 42, false), wantErr: true},
		{name: "array with a non-string element is a malformed claim", token: audienceToken(t, valid, []any{audience, 1}, false), wantErr: true},
		{name: "expiry is reported before the audience", token: audienceToken(t, now.Add(-time.Minute), "my-client-id", false), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.ValidateTokenWithAudience(tt.token, now, audience)
			expect(t, err != nil, tt.wantErr)
			expect(t, errors.Is(err, ErrInvalidAudience), tt.wantAudience)
			if tt.wantAudience {
				// Callers that matched on the text before the sentinel existed
				// keep working.
				expect(t, err.Error(), "invalid audience")
			}
		})
	}
}

// audienceRefreshServer is a local token endpoint that answers every refresh
// with accessToken and counts the calls it receives.
type audienceRefreshServer struct {
	calls atomic.Int32
}

// newAudienceTestClient returns a client that skips signature validation and
// refreshes against a local token endpoint answering with accessToken.
func newAudienceTestClient(t *testing.T, accessToken string) (*Client, *audienceRefreshServer) {
	t.Helper()
	srv := &audienceRefreshServer{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(&Tokens{AccessToken: accessToken, RefreshToken: "rotated-refresh-token"}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	client := NewClient("https://identity.example.com")
	client.TokenURL = server.URL
	client.SkipSignatureValidation = true
	return client, srv
}

// TestAuthenticateWithAudienceDefault pins the behaviour of a client that has
// not set StrictAudience, which must not change: every validation failure,
// including a wrong audience, is refreshed when a refresh token is present, and
// the refreshed token is returned unchecked.
//
// alis.os.iam.v2 mints every access token with aud set to the app's client id,
// so services calling AuthenticateWithAudience with their own origin rely on
// exactly this path today.
func TestAuthenticateWithAudienceDefault(t *testing.T) {
	const audience = "https://app.example.com"
	now := time.Now()
	valid := now.Add(time.Minute)
	clientIDToken := audienceToken(t, valid, "my-client-id", false)

	t.Run("client id aud is refreshed and accepted", func(t *testing.T) {
		client, srv := newAudienceTestClient(t, clientIDToken)
		tokens := &Tokens{AccessToken: clientIDToken, RefreshToken: "refresh-token"}
		refreshed, err := client.AuthenticateWithAudience(tokens, now, audience)
		expect(t, err, nil)
		expect(t, refreshed, true)
		expect(t, srv.calls.Load(), int32(1))
		expect(t, tokens.AccessToken, clientIDToken)
		expect(t, tokens.RefreshToken, "rotated-refresh-token")
	})

	t.Run("client id aud without a refresh token is rejected", func(t *testing.T) {
		client, srv := newAudienceTestClient(t, "")
		refreshed, err := client.AuthenticateWithAudience(&Tokens{AccessToken: clientIDToken}, now, audience)
		expect(t, errors.Is(err, ErrInvalidAudience), true)
		expect(t, refreshed, false)
		expect(t, srv.calls.Load(), int32(0))
	})

	t.Run("matching aud is not refreshed", func(t *testing.T) {
		client, srv := newAudienceTestClient(t, "")
		tokens := &Tokens{AccessToken: audienceToken(t, valid, audience, false), RefreshToken: "refresh-token"}
		refreshed, err := client.AuthenticateWithAudience(tokens, now, audience)
		expect(t, err, nil)
		expect(t, refreshed, false)
		expect(t, srv.calls.Load(), int32(0))
	})
}

// TestAuthenticateWithAudienceStrict covers the opted-in behaviour: a wrong
// audience is never refreshed away, and a token obtained by refreshing for any
// other reason has its audience checked before it is accepted.
func TestAuthenticateWithAudienceStrict(t *testing.T) {
	const audience = "https://app.example.com"
	now := time.Now()
	valid := now.Add(time.Minute)
	expired := now.Add(-time.Minute)

	tests := []struct {
		name          string
		accessToken   string
		refreshed     string
		wantErr       bool
		wantAudience  bool
		wantRefreshed bool
		wantCalls     int32
	}{
		{
			name:        "wrong aud is rejected without refreshing",
			accessToken: audienceToken(t, valid, "my-client-id", false),
			refreshed:   audienceToken(t, valid, audience, false),
			wantErr:     true, wantAudience: true,
		},
		{
			name:        "matching aud is accepted without refreshing",
			accessToken: audienceToken(t, valid, audience, false),
		},
		{
			name:        "absent aud is accepted without refreshing",
			accessToken: audienceToken(t, valid, nil, true),
		},
		{
			name:          "expired token is refreshed into a matching one",
			accessToken:   audienceToken(t, expired, audience, false),
			refreshed:     audienceToken(t, valid, []string{audience}, false),
			wantRefreshed: true, wantCalls: 1,
		},
		{
			name:          "expired token is refreshed into one without aud",
			accessToken:   audienceToken(t, expired, audience, false),
			refreshed:     audienceToken(t, valid, nil, true),
			wantRefreshed: true, wantCalls: 1,
		},
		{
			name:        "expired token refreshed into a wrong aud is rejected",
			accessToken: audienceToken(t, expired, audience, false),
			refreshed:   audienceToken(t, valid, "my-client-id", false),
			wantErr:     true, wantAudience: true, wantCalls: 1,
		},
		{
			name:        "expired token with a wrong aud is refreshed and then rejected",
			accessToken: audienceToken(t, expired, "my-client-id", false),
			refreshed:   audienceToken(t, valid, "my-client-id", false),
			wantErr:     true, wantAudience: true, wantCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, srv := newAudienceTestClient(t, tt.refreshed)
			client.StrictAudience = true
			tokens := &Tokens{AccessToken: tt.accessToken, RefreshToken: "refresh-token"}
			refreshed, err := client.AuthenticateWithAudience(tokens, now, audience)
			expect(t, err != nil, tt.wantErr)
			expect(t, errors.Is(err, ErrInvalidAudience), tt.wantAudience)
			expect(t, refreshed, tt.wantRefreshed)
			expect(t, srv.calls.Load(), tt.wantCalls)
		})
	}
}
