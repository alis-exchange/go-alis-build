package mux

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.alis.build/iam/v3"
	"go.alis.build/iam/v3/authn"
)

func expect[T comparable](t *testing.T, got, expected T) {
	if got != expected {
		t.Fatalf("got %v, expected %v", got, expected)
	}
}

func TestAuthMiddlewareStartsLoginTransaction(t *testing.T) {
	mux = http.NewServeMux()
	gateway = nil
	oldAuthClient := AuthClient
	AuthClient = authn.NewClient("https://identity.example.com")
	AuthClient.TokenURL = ":"
	defer func() {
		AuthClient = oldAuthClient
	}()

	AuthenticatedGet("/secure", func(w http.ResponseWriter, r *http.Request) error {
		t.Fatal("handler should not run for unauthenticated request")
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "http://app.example.com/secure?tab=one", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("unexpected status code: %d", rec.Code)
	}
	location := rec.Header().Get("Location")
	authURL, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, authURL.Scheme, "https")
	expect(t, authURL.Host, "identity.example.com")
	expect(t, authURL.Path, "/authorize")
	expect(t, authURL.Query().Get("redirect_uri"), "http://app.example.com/auth/callback")
	if authURL.Query().Get("state") == "" {
		t.Fatal("missing state")
	}
	if authURL.Query().Get("state") == "/secure?tab=one" {
		t.Fatal("state should be opaque")
	}
	if authURL.Query().Get("nonce") == "" {
		t.Fatal("missing nonce")
	}

	var foundTransactionCookie bool
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "alis_authn_login" {
			foundTransactionCookie = true
			if !cookie.HttpOnly {
				t.Fatal("expected HttpOnly login transaction cookie")
			}
		}
	}
	if !foundTransactionCookie {
		t.Fatal("missing login transaction cookie")
	}
}

// LoginOptions lets a service add OpenID Connect interaction parameters to
// the login redirect; nil (the default) leaves the redirect exactly as it was.
func TestAuthMiddlewareLoginOptions(t *testing.T) {
	mux = http.NewServeMux()
	gateway = nil
	oldAuthClient, oldOptions := AuthClient, LoginOptions
	AuthClient = authn.NewClient("https://identity.example.com")
	AuthClient.TokenURL = ":"
	LoginOptions = func(r *http.Request) []authn.AuthorizeOption {
		return []authn.AuthorizeOption{authn.WithPrompt("login"), authn.WithLoginHint(r.URL.Query().Get("hint"))}
	}
	defer func() { AuthClient, LoginOptions = oldAuthClient, oldOptions }()

	AuthenticatedGet("/secure", func(w http.ResponseWriter, r *http.Request) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "http://app.example.com/secure?hint=ada%40example.com", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	authURL, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	expect(t, authURL.Query().Get("prompt"), "login")
	expect(t, authURL.Query().Get("login_hint"), "ada@example.com")
	expect(t, authURL.Query().Get("redirect_uri"), "http://app.example.com/auth/callback")
}

// /auth/switch clears this service's auth cookies and starts a login that
// asks the identity provider for its account chooser, returning afterwards
// to a relative path only (never an absolute URL from the query).
func TestSwitchAccountHandle(t *testing.T) {
	oldAuthClient := AuthClient
	AuthClient = authn.NewClient("https://identity.example.com")
	defer func() { AuthClient = oldAuthClient }()

	for _, tc := range []struct {
		returnTo, want string
	}{
		{"/dashboard?tab=one", "/dashboard?tab=one"},
		{"https://evil.example/phish", "/"},
		{"//evil.example", "/"},
		{"", "/"},
	} {
		req := httptest.NewRequest(http.MethodGet, "http://app.example.com"+SwitchAccountPath+"?return_to="+url.QueryEscape(tc.returnTo), nil)
		rec := httptest.NewRecorder()
		if err := switchAccountHandle(rec, req); err != nil {
			t.Fatal(err)
		}
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want 303", rec.Code)
		}
		authURL, err := url.Parse(rec.Header().Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		expect(t, authURL.Host, "identity.example.com")
		expect(t, authURL.Query().Get("prompt"), "select_account")

		cleared := map[string]bool{}
		var transaction *http.Cookie
		for _, c := range rec.Result().Cookies() {
			switch c.Name {
			case AccessTokenCookie, RefreshTokenCookie:
				if c.MaxAge < 0 {
					cleared[c.Name] = true
				}
			case "alis_authn_login":
				transaction = c
			}
		}
		if !cleared[AccessTokenCookie] || !cleared[RefreshTokenCookie] {
			t.Errorf("return_to=%q: auth cookies not cleared: %v", tc.returnTo, cleared)
		}
		if transaction == nil {
			t.Fatalf("return_to=%q: no login transaction cookie", tc.returnTo)
		}
		raw, err := base64.RawURLEncoding.DecodeString(transaction.Value)
		if err != nil {
			t.Fatal(err)
		}
		var tx struct {
			ReturnTo string `json:"return_to"`
		}
		if err := json.Unmarshal(raw, &tx); err != nil {
			t.Fatal(err)
		}
		expect(t, tx.ReturnTo, tc.want)
	}
}

func TestCallbackHandleCompletesLoginTransaction(t *testing.T) {
	oldAuthClient := AuthClient
	var tokenRequest struct {
		GrantType   string `json:"grant_type"`
		Code        string `json:"code"`
		RedirectURI string `json:"redirect_uri"`
	}
	var login *authn.LoginTransaction
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&tokenRequest); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(&authn.Tokens{
			AccessToken:  testJWT(t, map[string]any{"exp": time.Now().Add(time.Minute).Unix(), "type": string(iam.User), "email": "jan@example.com"}),
			RefreshToken: "refresh-token",
			IDToken:      testJWT(t, map[string]any{"exp": time.Now().Add(time.Minute).Unix(), "nonce": login.Nonce}),
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer tokenServer.Close()

	AuthClient = authn.NewClient("https://identity.example.com")
	AuthClient.TokenURL = tokenServer.URL
	AuthClient.SkipSignatureValidation = true
	defer func() {
		AuthClient = oldAuthClient
	}()

	startReq := httptest.NewRequest(http.MethodGet, "http://app.example.com/secure", nil)
	startResp := httptest.NewRecorder()
	var err error
	login, err = AuthClient.StartLogin(startResp, startReq, "http://app.example.com/auth/callback", "/secure")
	if err != nil {
		t.Fatal(err)
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "http://app.example.com/auth/callback?code=auth-code&state="+url.QueryEscape(login.State), nil)
	callbackReq.AddCookie(startResp.Result().Cookies()[0])
	callbackResp := httptest.NewRecorder()
	if err := callbackHandle(callbackResp, callbackReq); err != nil {
		t.Fatal(err)
	}

	if callbackResp.Code != http.StatusTemporaryRedirect {
		t.Fatalf("unexpected status code: %d", callbackResp.Code)
	}
	expect(t, callbackResp.Header().Get("Location"), "/secure")
	expect(t, tokenRequest.GrantType, "authorization_code")
	expect(t, tokenRequest.Code, "auth-code")
	expect(t, tokenRequest.RedirectURI, "http://app.example.com/auth/callback")

	cookies := callbackResp.Result().Cookies()
	if CookieByName(cookies, AccessTokenCookie) == nil {
		t.Fatal("missing access token cookie")
	}
	if CookieByName(cookies, RefreshTokenCookie) == nil {
		t.Fatal("missing refresh token cookie")
	}
	if CookieByName(cookies, AccessTokenCookie).Secure {
		t.Fatal("expected non-secure access token cookie for HTTP callback")
	}
	if CookieByName(cookies, RefreshTokenCookie).Secure {
		t.Fatal("expected non-secure refresh token cookie for HTTP callback")
	}
	loginCookie := CookieByName(cookies, "alis_authn_login")
	if loginCookie == nil {
		t.Fatal("missing cleared login transaction cookie")
	}
	expect(t, loginCookie.MaxAge, -1)
}

func TestShouldSecureAuthCookies(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want bool
	}{
		{
			name: "nil request defaults secure",
			req:  nil,
			want: true,
		},
		{
			name: "local HTTP",
			req:  httptest.NewRequest(http.MethodGet, "http://localhost:8080/auth/callback", nil),
			want: false,
		},
		{
			name: "plain HTTP",
			req:  httptest.NewRequest(http.MethodGet, "http://app.example.com/auth/callback", nil),
			want: false,
		},
		{
			name: "TLS",
			req:  httptest.NewRequest(http.MethodGet, "https://app.example.com/auth/callback", nil),
			want: true,
		},
		{
			name: "ngrok",
			req:  httptest.NewRequest(http.MethodGet, "http://example.ngrok-free.app/auth/callback", nil),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expect(t, shouldSecureAuthCookies(tt.req), tt.want)
		})
	}
}

func TestAuthenticatedHandleHTTP(t *testing.T) {
	mux = http.NewServeMux()
	gateway = nil
	oldAuthClient := AuthClient
	AuthClient = &authn.Client{TokenURL: ":"}
	defer func() {
		AuthClient = oldAuthClient
	}()

	AuthenticatedHandleHTTP("GET /raw-handler", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run for unauthenticated request")
	}))

	req := httptest.NewRequest(http.MethodGet, "/raw-handler", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: %d", rec.Code)
	}
}

func TestAuthenticatedHandleGRPCWeb(t *testing.T) {
	for _, contentType := range []string{
		"application/grpc-web+proto",
		"application/grpc-web-text",
		"application/grpc-web-text+proto",
	} {
		t.Run(contentType, func(t *testing.T) {
			mux = http.NewServeMux()
			gateway = nil
			oldAuthClient := AuthClient
			AuthClient = &authn.Client{TokenURL: ":"}
			defer func() {
				AuthClient = oldAuthClient
			}()

			AuthenticatedHandleGRPCWeb(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			}))

			postReq := httptest.NewRequest(http.MethodPost, "/package.Service/Method", nil)
			postReq.Header.Set("Content-Type", contentType)
			postRec := httptest.NewRecorder()
			mux.ServeHTTP(postRec, postReq)
			if postRec.Code != http.StatusUnauthorized {
				t.Fatalf("unexpected grpc-web post status code: %d", postRec.Code)
			}
		})
	}

	mux = http.NewServeMux()
	gateway = nil
	AuthenticatedHandleGRPCWeb(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	preflightReq := httptest.NewRequest(http.MethodOptions, "/package.Service/Method", nil)
	preflightReq.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflightReq.Header.Set("Access-Control-Request-Headers", "content-type,x-grpc-web")
	preflightRec := httptest.NewRecorder()
	mux.ServeHTTP(preflightRec, preflightReq)
	if preflightRec.Code != http.StatusAccepted {
		t.Fatalf("unexpected grpc-web preflight status code: %d", preflightRec.Code)
	}
}

func TestAuthFlow(t *testing.T) {
	done := atomic.Bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()

	AuthenticatedGet("/whoami", func(w http.ResponseWriter, r *http.Request) error {
		identity := iam.MustFromContext(r.Context())
		msg := fmt.Sprintf("Hello %s\n", identity.Email)
		msg += `
		Clear your access token cookie, and make sure that if you rerun the test, you don't have to sign in again.
		Then clear both your access token and refresh token cookies, and make sure that if you rerun the test, you have to sign in again.`

		w.Write([]byte(msg))
		done.Store(true)
		return nil
	})

	url := server.URL + "/whoami"
	if err := openBrowser(url); err != nil {
		t.Fatal(err)
	}

	startT := time.Now()
	for !done.Load() {
		if time.Since(startT) > time.Second*30 {
			t.Fatal("timeout")
		}
		time.Sleep(1 * time.Second)
	}
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin": // macOS
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported platform")
	}

	return exec.Command(cmd, args...).Start()
}

func CookieByName(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func testJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{"alg": "none", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + "."
}

// audienceTestEnv records what a request reached during an audience test.
type audienceTestEnv struct {
	tokenCalls atomic.Int32
	handlerRan atomic.Bool
}

// setupAudienceTest installs a fresh mux with an authenticated route, an
// AuthClient that skips signature validation and refreshes against a local
// token server returning refreshedAccessToken, and the given accepted
// audiences. Package state is restored when the test ends.
func setupAudienceTest(t *testing.T, accepted []string, refreshedAccessToken string) *audienceTestEnv {
	t.Helper()
	env := &audienceTestEnv{}

	mux = http.NewServeMux()
	gateway = nil
	oldAuthClient, oldAccepted, oldMissing := AuthClient, AcceptedAudiences, MissingAudienceHandler

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.tokenCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(&authn.Tokens{
			AccessToken:  refreshedAccessToken,
			RefreshToken: "rotated-refresh-token",
		}); err != nil {
			t.Error(err)
		}
	}))
	AuthClient = authn.NewClient("https://identity.example.com")
	AuthClient.TokenURL = tokenServer.URL
	AuthClient.SkipSignatureValidation = true
	AcceptedAudiences = accepted

	t.Cleanup(func() {
		tokenServer.Close()
		AuthClient, AcceptedAudiences, MissingAudienceHandler = oldAuthClient, oldAccepted, oldMissing
	})

	AuthenticatedGet("/secure", func(w http.ResponseWriter, r *http.Request) error {
		env.handlerRan.Store(true)
		return nil
	})
	return env
}

// secureRequest calls the route registered by setupAudienceTest. The host makes
// RequestHost report "http://app.example.com".
func secureRequest(accessToken, refreshToken string, navigation bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "http://app.example.com/secure", nil)
	if navigation {
		req.Header.Set("Accept", "text/html")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	if accessToken != "" {
		req.AddCookie(&http.Cookie{Name: AccessTokenCookie, Value: accessToken})
	}
	if refreshToken != "" {
		req.AddCookie(&http.Cookie{Name: RefreshTokenCookie, Value: refreshToken})
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// audienceToken builds an unsigned access token. A nil aud omits the claim.
func audienceToken(t *testing.T, exp time.Time, aud any) string {
	t.Helper()
	claims := map[string]any{"exp": exp.Unix(), "email": "john@example.com", "sub": "1934872948"}
	if aud != nil {
		claims["aud"] = aud
	}
	return testJWT(t, claims)
}

// TestAuthMiddlewareAudienceUnset pins the behaviour of the default
// configuration, which must not change: a populated aud is compared against the
// request's own origin, and a token without one is accepted.
func TestAuthMiddlewareAudienceUnset(t *testing.T) {
	valid := time.Now().Add(time.Minute)

	t.Run("aud equal to the request host is accepted", func(t *testing.T) {
		env := setupAudienceTest(t, nil, "")
		rec := secureRequest(audienceToken(t, valid, "http://app.example.com"), "", false)
		expect(t, rec.Code, http.StatusOK)
		expect(t, env.handlerRan.Load(), true)
		expect(t, env.tokenCalls.Load(), int32(0))
	})

	t.Run("aud that differs is rejected", func(t *testing.T) {
		env := setupAudienceTest(t, nil, "")
		rec := secureRequest(audienceToken(t, valid, "https://other.example.com"), "", false)
		expect(t, rec.Code, http.StatusUnauthorized)
		expect(t, env.handlerRan.Load(), false)
		expect(t, env.tokenCalls.Load(), int32(0))
	})

	t.Run("absent aud is accepted without consulting the handler", func(t *testing.T) {
		env := setupAudienceTest(t, nil, "")
		MissingAudienceHandler = func(w http.ResponseWriter, r *http.Request) error {
			t.Error("MissingAudienceHandler must not run while AcceptedAudiences is empty")
			return nil
		}
		rec := secureRequest(audienceToken(t, valid, nil), "", false)
		expect(t, rec.Code, http.StatusOK)
		expect(t, env.handlerRan.Load(), true)
	})

	t.Run("aud that differs is refreshed when a refresh token is present", func(t *testing.T) {
		// Documents the gap that AcceptedAudiences closes: today a mismatch is
		// treated like any other validation failure and triggers a refresh.
		env := setupAudienceTest(t, nil, audienceToken(t, valid, nil))
		rec := secureRequest(audienceToken(t, valid, "https://other.example.com"), "refresh-token", false)
		expect(t, rec.Code, http.StatusOK)
		expect(t, env.tokenCalls.Load(), int32(1))
	})
}

// TestAuthMiddlewareAcceptedAudiences covers the opted-in behaviour.
func TestAuthMiddlewareAcceptedAudiences(t *testing.T) {
	accepted := []string{"https://api.example.com", "my-service"}
	valid := time.Now().Add(time.Minute)
	expired := time.Now().Add(-time.Minute)

	t.Run("matching aud is accepted", func(t *testing.T) {
		env := setupAudienceTest(t, accepted, "")
		rec := secureRequest(audienceToken(t, valid, "my-service"), "", false)
		expect(t, rec.Code, http.StatusOK)
		expect(t, env.handlerRan.Load(), true)
		expect(t, env.tokenCalls.Load(), int32(0))
	})

	t.Run("mismatched aud is rejected and never refreshed", func(t *testing.T) {
		env := setupAudienceTest(t, accepted, audienceToken(t, valid, "my-service"))
		rec := secureRequest(audienceToken(t, valid, "https://other.example.com"), "refresh-token", false)
		expect(t, rec.Code, http.StatusUnauthorized)
		expect(t, env.handlerRan.Load(), false)
		expect(t, env.tokenCalls.Load(), int32(0))
		if !strings.Contains(rec.Body.String(), "invalid audience") {
			t.Fatalf("got body %q, expected it to mention an invalid audience", rec.Body.String())
		}
	})

	t.Run("mismatched aud on a browser navigation is rejected not redirected", func(t *testing.T) {
		// Redirecting would loop: the provider would mint another token with the
		// same audience.
		env := setupAudienceTest(t, accepted, "")
		rec := secureRequest(audienceToken(t, valid, "https://other.example.com"), "refresh-token", true)
		expect(t, rec.Code, http.StatusUnauthorized)
		expect(t, rec.Header().Get("Location"), "")
		expect(t, env.tokenCalls.Load(), int32(0))
	})

	t.Run("absent aud reaches MissingAudienceHandler", func(t *testing.T) {
		env := setupAudienceTest(t, accepted, "")
		var called atomic.Bool
		MissingAudienceHandler = func(w http.ResponseWriter, r *http.Request) error {
			called.Store(true)
			return nil
		}
		rec := secureRequest(audienceToken(t, valid, nil), "", false)
		expect(t, rec.Code, http.StatusOK)
		expect(t, called.Load(), true)
		expect(t, env.handlerRan.Load(), true)
	})

	t.Run("absent aud is accepted by the default handler", func(t *testing.T) {
		env := setupAudienceTest(t, accepted, "")
		rec := secureRequest(audienceToken(t, valid, nil), "", false)
		expect(t, rec.Code, http.StatusOK)
		expect(t, env.handlerRan.Load(), true)
	})

	t.Run("absent aud is rejected once the handler returns an error", func(t *testing.T) {
		for _, navigation := range []bool{false, true} {
			env := setupAudienceTest(t, accepted, "")
			MissingAudienceHandler = func(w http.ResponseWriter, r *http.Request) error {
				return UnauthorizedErr("missing audience")
			}
			rec := secureRequest(audienceToken(t, valid, nil), "", navigation)
			expect(t, rec.Code, http.StatusUnauthorized)
			expect(t, rec.Header().Get("Location"), "")
			expect(t, env.handlerRan.Load(), false)
		}
	})

	t.Run("mismatch is reported through UnauthorizedHandler", func(t *testing.T) {
		setupAudienceTest(t, accepted, "")
		oldHandler := UnauthorizedHandler
		var details atomic.Value
		UnauthorizedHandler = func(w http.ResponseWriter, r *http.Request, d string) error {
			details.Store(d)
			return ForbiddenErr("%s", d)
		}
		defer func() { UnauthorizedHandler = oldHandler }()

		rec := secureRequest(audienceToken(t, valid, "https://other.example.com"), "", false)
		expect(t, rec.Code, http.StatusForbidden)
		got, _ := details.Load().(string)
		if !strings.Contains(got, "invalid audience") {
			t.Fatalf("got details %q, expected them to mention an invalid audience", got)
		}
	})

	t.Run("array aud is accepted when one element matches", func(t *testing.T) {
		env := setupAudienceTest(t, accepted, "")
		rec := secureRequest(audienceToken(t, valid, []string{"https://other.example.com", "my-service"}), "", false)
		expect(t, rec.Code, http.StatusOK)
		expect(t, env.handlerRan.Load(), true)
	})

	t.Run("array aud is rejected when no element matches", func(t *testing.T) {
		env := setupAudienceTest(t, accepted, "")
		rec := secureRequest(audienceToken(t, valid, []string{"a", "b"}), "", false)
		expect(t, rec.Code, http.StatusUnauthorized)
		expect(t, env.tokenCalls.Load(), int32(0))
	})

	t.Run("expired token refreshed to a matching aud is accepted", func(t *testing.T) {
		fresh := audienceToken(t, valid, "my-service")
		env := setupAudienceTest(t, accepted, fresh)
		rec := secureRequest(audienceToken(t, expired, "my-service"), "refresh-token", false)
		expect(t, rec.Code, http.StatusOK)
		expect(t, env.tokenCalls.Load(), int32(1))
		expect(t, CookieByName(rec.Result().Cookies(), AccessTokenCookie).Value, fresh)
		expect(t, CookieByName(rec.Result().Cookies(), RefreshTokenCookie).Value, "rotated-refresh-token")
	})

	t.Run("expired token refreshed to a wrong aud is rejected", func(t *testing.T) {
		env := setupAudienceTest(t, accepted, audienceToken(t, valid, "https://other.example.com"))
		rec := secureRequest(audienceToken(t, expired, "my-service"), "refresh-token", false)
		expect(t, rec.Code, http.StatusUnauthorized)
		expect(t, env.handlerRan.Load(), false)
		expect(t, env.tokenCalls.Load(), int32(1))
	})

	t.Run("unreadable aud is rejected", func(t *testing.T) {
		env := setupAudienceTest(t, accepted, "")
		rec := secureRequest(audienceToken(t, valid, 42), "", false)
		expect(t, rec.Code, http.StatusUnauthorized)
		expect(t, env.handlerRan.Load(), false)
	})

	t.Run("expired token without a refresh token still redirects a navigation", func(t *testing.T) {
		setupAudienceTest(t, accepted, "")
		rec := secureRequest(audienceToken(t, expired, "my-service"), "", true)
		expect(t, rec.Code, http.StatusTemporaryRedirect)
		location, err := url.Parse(rec.Header().Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		expect(t, location.Host, "identity.example.com")
	})
}

func TestJWTAudiences(t *testing.T) {
	valid := time.Now().Add(time.Minute)
	for _, tt := range []struct {
		name    string
		aud     any
		want    []string
		wantErr bool
	}{
		{name: "absent claim", aud: nil},
		{name: "empty string", aud: ""},
		{name: "null", aud: json.RawMessage("null")},
		{name: "empty array", aud: []string{}},
		{name: "single string", aud: "my-service", want: []string{"my-service"}},
		{name: "array", aud: []string{"x", "y"}, want: []string{"x", "y"}},
		{name: "number", aud: 42, wantErr: true},
		{name: "mixed array", aud: []any{"x", 1}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jwtAudiences(audienceToken(t, valid, tt.aud))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %v, expected an error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("got %v, expected %v", got, tt.want)
			}
		})
	}

	t.Run("malformed token", func(t *testing.T) {
		if _, err := jwtAudiences("a.b"); err == nil {
			t.Fatal("expected an error for a token that is not three parts")
		}
	})

	t.Run("payload that is not base64", func(t *testing.T) {
		if _, err := jwtAudiences("a.!!!.c"); err == nil {
			t.Fatal("expected an error for a payload that cannot be decoded")
		}
	})
}
