package mux

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
	"go.alis.build/iam/v3/authn"
)

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
