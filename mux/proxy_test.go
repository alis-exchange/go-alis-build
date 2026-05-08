package mux

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestHandleHTTP(t *testing.T) {
	mux = http.NewServeMux()
	gateway = nil

	HandleHTTP("GET /raw-handler", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-middleware") != "ran" {
			t.Fatal("middleware did not run before raw handler")
		}
		w.WriteHeader(http.StatusAccepted)
	}), func(w http.ResponseWriter, r *http.Request, next Func) error {
		r.Header.Set("x-middleware", "ran")
		return next(w, r)
	})

	req := httptest.NewRequest(http.MethodGet, "/raw-handler", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("unexpected status code: %d", rec.Code)
	}
}

func TestHandleGRPC(t *testing.T) {
	mux = http.NewServeMux()
	gateway = nil

	Post("/rest", func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})
	HandleGRPC(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	grpcReq := httptest.NewRequest(http.MethodPost, "/package.Service/Method", nil)
	grpcReq.ProtoMajor = 2
	grpcReq.ProtoMinor = 0
	grpcReq.Header.Set("Content-Type", "application/grpc")
	grpcRec := httptest.NewRecorder()
	mux.ServeHTTP(grpcRec, grpcReq)
	if grpcRec.Code != http.StatusAccepted {
		t.Fatalf("unexpected grpc status code: %d", grpcRec.Code)
	}

	restReq := httptest.NewRequest(http.MethodPost, "/rest", nil)
	restRec := httptest.NewRecorder()
	mux.ServeHTTP(restRec, restReq)
	if restRec.Code != http.StatusOK {
		t.Fatalf("unexpected rest status code: %d", restRec.Code)
	}

	unmatchedRESTReq := httptest.NewRequest(http.MethodPost, "/unmatched-rest", nil)
	unmatchedRESTReq.Header.Set("Content-Type", "application/json")
	unmatchedRESTRec := httptest.NewRecorder()
	mux.ServeHTTP(unmatchedRESTRec, unmatchedRESTReq)
	if unmatchedRESTRec.Code != http.StatusNotFound {
		t.Fatalf("unexpected unmatched rest status code: %d", unmatchedRESTRec.Code)
	}
	if !strings.Contains(unmatchedRESTRec.Body.String(), "request did not match a REST route or gRPC request") {
		t.Fatalf("unexpected unmatched rest body: %q", unmatchedRESTRec.Body.String())
	}
}

func TestHandleGRPCWeb(t *testing.T) {
	mux = http.NewServeMux()
	gateway = nil

	HandleGRPCWeb(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	grpcWebReq := httptest.NewRequest(http.MethodPost, "/package.Service/Method", nil)
	grpcWebReq.Header.Set("Content-Type", "application/grpc-web+proto")
	grpcWebRec := httptest.NewRecorder()
	mux.ServeHTTP(grpcWebRec, grpcWebReq)
	if grpcWebRec.Code != http.StatusAccepted {
		t.Fatalf("unexpected grpc-web status code: %d", grpcWebRec.Code)
	}

	preflightReq := httptest.NewRequest(http.MethodOptions, "/package.Service/Method", nil)
	preflightReq.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflightReq.Header.Set("Access-Control-Request-Headers", "content-type,x-grpc-web")
	preflightRec := httptest.NewRecorder()
	mux.ServeHTTP(preflightRec, preflightReq)
	if preflightRec.Code != http.StatusAccepted {
		t.Fatalf("unexpected grpc-web preflight status code: %d", preflightRec.Code)
	}

	unmatchedReq := httptest.NewRequest(http.MethodPost, "/unmatched-rest", nil)
	unmatchedReq.Header.Set("Content-Type", "application/json")
	unmatchedRec := httptest.NewRecorder()
	mux.ServeHTTP(unmatchedRec, unmatchedReq)
	if unmatchedRec.Code != http.StatusNotFound {
		t.Fatalf("unexpected unmatched status code: %d", unmatchedRec.Code)
	}
}

func TestHandleGRPCAndWeb(t *testing.T) {
	mux = http.NewServeMux()
	gateway = nil

	Post("/rest", func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})
	HandleGRPCAndWeb(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}),
	)

	grpcReq := httptest.NewRequest(http.MethodPost, "/package.Service/Method", nil)
	grpcReq.ProtoMajor = 2
	grpcReq.ProtoMinor = 0
	grpcReq.Header.Set("Content-Type", "application/grpc")
	grpcRec := httptest.NewRecorder()
	mux.ServeHTTP(grpcRec, grpcReq)
	if grpcRec.Code != http.StatusAccepted {
		t.Fatalf("unexpected grpc status code: %d", grpcRec.Code)
	}

	grpcWebReq := httptest.NewRequest(http.MethodPost, "/package.Service/Method", nil)
	grpcWebReq.Header.Set("Content-Type", "application/grpc-web+json")
	grpcWebRec := httptest.NewRecorder()
	mux.ServeHTTP(grpcWebRec, grpcWebReq)
	if grpcWebRec.Code != http.StatusCreated {
		t.Fatalf("unexpected grpc-web status code: %d", grpcWebRec.Code)
	}

	restReq := httptest.NewRequest(http.MethodPost, "/rest", nil)
	restRec := httptest.NewRecorder()
	mux.ServeHTTP(restRec, restReq)
	if restRec.Code != http.StatusOK {
		t.Fatalf("unexpected rest status code: %d", restRec.Code)
	}
}
