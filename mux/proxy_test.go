package mux

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Use two httptest servers to test the proxy.
// The first server is the proxy, the second is the target.

func TestHTTPProxy(t *testing.T) {
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, world!"))
	}))
	defer targetServer.Close()
	targetServerPort := targetServer.Listener.Addr().(*net.TCPAddr).Port
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
	}))
	defer proxyServer.Close()
	proxyHandler := HTTPProxy(targetServerPort)
	Get("/proxy", proxyHandler)
	resp, err := http.Get(proxyServer.URL + "/proxy")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}
}
