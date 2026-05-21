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
