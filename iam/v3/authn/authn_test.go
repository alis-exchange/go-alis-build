package authn

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
