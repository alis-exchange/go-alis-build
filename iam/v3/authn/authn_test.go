package authn

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
)

var testClient = NewClient("https://identity.alisx.com")

func expect[T comparable](t *testing.T, got, expected T) {
	if got != expected {
		t.Fatalf("got %v, expected %v", got, expected)
	}
}

func TestAuthorizeURL(t *testing.T) {
	client := NewClient("https://identity.alisx.com")
	client.ID = "client-id"

	authURL := client.AuthorizeURL("https://app.example.com/auth/callback", "/dashboard", "nonce-value")
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}

	expect(t, parsed.Scheme, "https")
	expect(t, parsed.Host, "identity.alisx.com")
	expect(t, parsed.Path, "/authorize")
	expect(t, parsed.Query().Get("redirect_uri"), "https://app.example.com/auth/callback")
	expect(t, parsed.Query().Get("state"), "/dashboard")
	expect(t, parsed.Query().Get("client_id"), "client-id")
	expect(t, parsed.Query().Get("nonce"), "nonce-value")
}

func TestLoginTransaction(t *testing.T) {
	var tokenRequest struct {
		GrantType   string `json:"grant_type"`
		Code        string `json:"code"`
		RedirectURI string `json:"redirect_uri"`
	}
	var transaction *LoginTransaction
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&tokenRequest); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(&Tokens{
			AccessToken:  testJWT(t, map[string]any{"exp": time.Now().Add(time.Minute).Unix()}),
			RefreshToken: "refresh-token",
			IDToken:      testJWT(t, map[string]any{"exp": time.Now().Add(time.Minute).Unix(), "nonce": transaction.Nonce}),
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer tokenServer.Close()

	client := NewClient("https://identity.alisx.com")
	client.TokenURL = tokenServer.URL
	client.SkipSignatureValidation = true

	startReq := httptest.NewRequest(http.MethodGet, "https://app.example.com/login", nil)
	startResp := httptest.NewRecorder()
	var err error
	transaction, err = client.StartLogin(startResp, startReq, "https://app.example.com/auth/callback", "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	if transaction.State == "" {
		t.Fatal("missing state")
	}
	if transaction.Nonce == "" {
		t.Fatal("missing nonce")
	}

	authURL, err := url.Parse(transaction.URL)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, authURL.Query().Get("state"), transaction.State)
	expect(t, authURL.Query().Get("nonce"), transaction.Nonce)
	expect(t, authURL.Query().Get("redirect_uri"), "https://app.example.com/auth/callback")

	cookies := startResp.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, expected 1", len(cookies))
	}
	expect(t, cookies[0].Name, loginTransactionCookieName)
	if !cookies[0].HttpOnly {
		t.Fatal("expected HttpOnly cookie")
	}
	if !cookies[0].Secure {
		t.Fatal("expected Secure cookie")
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "https://app.example.com/auth/callback?code=auth-code&state="+url.QueryEscape(transaction.State), nil)
	callbackReq.AddCookie(cookies[0])
	callbackResp := httptest.NewRecorder()
	tokens, returnTo, err := client.CompleteLogin(callbackResp, callbackReq, "https://app.example.com/auth/callback")
	if err != nil {
		t.Fatal(err)
	}
	expect(t, tokens.RefreshToken, "refresh-token")
	expect(t, returnTo, "/dashboard")
	expect(t, tokenRequest.GrantType, "authorization_code")
	expect(t, tokenRequest.Code, "auth-code")
	expect(t, tokenRequest.RedirectURI, "https://app.example.com/auth/callback")

	clearCookies := callbackResp.Result().Cookies()
	if len(clearCookies) != 1 {
		t.Fatalf("got %d clear cookies, expected 1", len(clearCookies))
	}
	expect(t, clearCookies[0].Name, loginTransactionCookieName)
	expect(t, clearCookies[0].MaxAge, -1)
}

func TestCompleteLoginRejectsNonceMismatch(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(&Tokens{
			AccessToken:  testJWT(t, map[string]any{"exp": time.Now().Add(time.Minute).Unix()}),
			RefreshToken: "refresh-token",
			IDToken:      testJWT(t, map[string]any{"exp": time.Now().Add(time.Minute).Unix(), "nonce": "wrong-nonce"}),
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer tokenServer.Close()

	client := NewClient("https://identity.alisx.com")
	client.TokenURL = tokenServer.URL
	client.SkipSignatureValidation = true

	startReq := httptest.NewRequest(http.MethodGet, "https://app.example.com/login", nil)
	startResp := httptest.NewRecorder()
	transaction, err := client.StartLogin(startResp, startReq, "https://app.example.com/auth/callback", "/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "https://app.example.com/auth/callback?code=auth-code&state="+url.QueryEscape(transaction.State), nil)
	callbackReq.AddCookie(startResp.Result().Cookies()[0])
	_, _, err = client.CompleteLogin(httptest.NewRecorder(), callbackReq, "https://app.example.com/auth/callback")
	if err == nil {
		t.Fatal("expected nonce mismatch")
	}
}

func TestAuthFlow(t *testing.T) {
	done := atomic.Bool{}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/callback" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			t.Fatal("missing code")
			return
		}
		if state != "mycustomstate" {
			w.WriteHeader(http.StatusBadRequest)
			t.Fatal("invalid state")
			return
		}

		// test exchange code
		tokens, err := testClient.ExchangeCode(server.URL+"/auth/callback", code)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			t.Fatal(err)
			return
		}

		// test identity extraction from token
		identity := iam.MustFromJWT(tokens.AccessToken)
		expect(t, identity.Type, iam.User)

		// test refresh
		if err = testClient.Refresh(tokens); err != nil {
			t.Fatal(err)
		}

		// test validate token
		if err = testClient.ValidateToken(tokens.AccessToken, time.Now()); err != nil {
			t.Fatal(err)
		}

		// test authenticate
		refreshed, err := testClient.Authenticate(tokens, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		expect(t, refreshed, false)
		tokens.AccessToken = ""
		refreshed, err = testClient.Authenticate(tokens, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		expect(t, refreshed, true)
		w.WriteHeader(http.StatusOK)
		done.Store(true)
	}))
	defer server.Close()

	// open user's browser and wait for callback
	url := testClient.AuthorizeURL(server.URL+"/auth/callback", "mycustomstate")
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
