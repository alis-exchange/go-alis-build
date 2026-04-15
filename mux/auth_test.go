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
)

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
