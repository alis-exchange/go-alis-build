package mux

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"go.alis.build/alog"
)

var (
	mux     = http.NewServeMux()
	gateway Middleware
)

type (
	// Func is an HTTP handler function that reports failures as errors.
	//
	// When a registered Func returns a *Error, mux writes that error's status
	// code and message to the response. Any other non-nil error is logged and
	// returned to the client as a generic 500 Internal Server Error.
	Func func(w http.ResponseWriter, r *http.Request) error

	// Middleware wraps a Func with request processing that can run before or
	// after the handler.
	//
	// Middleware should call handler when the request should continue through
	// the chain. Returning an error stops the chain and lets mux convert the
	// error into an HTTP response.
	Middleware func(w http.ResponseWriter, r *http.Request, handler Func) error
)

// AddGateway installs a package-wide gateway middleware.
//
// The gateway wraps every registered route after route-specific middleware has
// been composed. Passing nil disables the gateway. Only one gateway is active at
// a time; later calls replace the previous gateway.
func AddGateway(gw Middleware) {
	gateway = gw
}

// Handle registers handleFunc for pattern on the package-level ServeMux.
//
// The pattern is passed directly to http.ServeMux.HandleFunc, so it supports the
// standard library's method-aware patterns such as "GET /users". Middlewares are
// executed in the order provided before handleFunc is invoked.
func Handle(pattern string, handleFunc Func, middlewares ...Middleware) {
	mux.HandleFunc(pattern, handler(handleFunc, middlewares...))
}

// HandleHTTP registers an http.Handler for pattern on the package-level ServeMux.
//
// This is useful when an existing component already exposes a standard
// http.Handler instead of a mux.Func, such as a grpc.Server, generated REST
// gateway, long-running operation handler, or another ServeMux. The handler is
// adapted into a Func, so route-specific middleware and any package-wide gateway
// installed with AddGateway still run before the handler serves the request.
//
// For mixed REST and gRPC servers, register REST routes on their specific
// patterns and mount the gRPC server on a broad HTTP/2 pattern such as "POST /".
// ListenAndServe accepts unencrypted HTTP/2, which allows cleartext HTTP/2 gRPC
// requests and ordinary HTTP/1.1 REST requests to share the same listener.
func HandleHTTP(pattern string, httpHandler http.Handler, middlewares ...Middleware) {
	Handle(pattern, func(w http.ResponseWriter, r *http.Request) error {
		httpHandler.ServeHTTP(w, r)
		return nil
	}, middlewares...)
}

// HandleGRPC registers a gRPC-compatible http.Handler on the package-level ServeMux.
//
// grpc.Server implements http.Handler for HTTP/2 requests, so callers can pass a
// configured grpc.Server without this package depending on google.golang.org/grpc.
// The handler is mounted at "POST /", which matches gRPC method calls such as
// POST /package.Service/Method. More specific REST routes registered on the same
// mux take precedence over this broad fallback pattern.
//
// The registered handler still runs through route-specific middleware and any
// package-wide gateway installed with AddGateway. ListenAndServe accepts
// unencrypted HTTP/2, allowing cleartext HTTP/2 gRPC calls and HTTP/1.1 REST
// calls to share the same listener.
func HandleGRPC(grpcHandler http.Handler, middlewares ...Middleware) {
	Handle("POST /", func(w http.ResponseWriter, r *http.Request) error {
		if !IsGRPCRequest(r) {
			return NotFoundErr("request did not match a REST route or gRPC request")
		}
		grpcHandler.ServeHTTP(w, r)
		return nil
	}, middlewares...)
}

// HandleGRPCWeb registers a gRPC-Web-compatible http.Handler on the package-level ServeMux.
//
// The handler is mounted on "POST /" for gRPC-Web calls and "OPTIONS /" for
// gRPC-Web CORS preflight requests. It is intended for handlers produced by a
// gRPC-Web adapter such as an improbable grpcweb.WrappedGrpcServer, while keeping
// this package independent from any specific gRPC-Web implementation.
//
// More specific REST routes registered on the same mux take precedence over
// these broad fallback patterns. Requests that reach the fallback patterns but
// do not look like gRPC-Web requests receive a 404 response.
func HandleGRPCWeb(grpcWebHandler http.Handler, middlewares ...Middleware) {
	Handle("POST /", func(w http.ResponseWriter, r *http.Request) error {
		if !IsGRPCWebRequest(r) {
			return NotFoundErr("request did not match a REST route or gRPC-Web request")
		}
		grpcWebHandler.ServeHTTP(w, r)
		return nil
	}, middlewares...)
	Handle("OPTIONS /", func(w http.ResponseWriter, r *http.Request) error {
		if !IsGRPCWebRequest(r) {
			return NotFoundErr("request did not match a REST route or gRPC-Web preflight request")
		}
		grpcWebHandler.ServeHTTP(w, r)
		return nil
	}, middlewares...)
}

// HandleGRPCAndWeb registers native gRPC and gRPC-Web handlers on one fallback route.
//
// Both native gRPC and gRPC-Web use broad POST request paths such as
// /package.Service/Method. Registering them separately would register "POST /"
// twice, which http.ServeMux rejects. This helper registers the broad POST
// fallback once, dispatches native gRPC requests to grpcHandler, dispatches
// gRPC-Web requests to grpcWebHandler, and registers "OPTIONS /" for gRPC-Web
// preflight requests.
//
// More specific REST routes registered on the same mux take precedence over the
// broad fallback patterns.
func HandleGRPCAndWeb(grpcHandler http.Handler, grpcWebHandler http.Handler, middlewares ...Middleware) {
	Handle("POST /", func(w http.ResponseWriter, r *http.Request) error {
		switch {
		case IsGRPCRequest(r):
			grpcHandler.ServeHTTP(w, r)
		case IsGRPCWebRequest(r):
			grpcWebHandler.ServeHTTP(w, r)
		default:
			return NotFoundErr("request did not match a REST route, gRPC request, or gRPC-Web request")
		}
		return nil
	}, middlewares...)
	Handle("OPTIONS /", func(w http.ResponseWriter, r *http.Request) error {
		if !IsGRPCWebRequest(r) {
			return NotFoundErr("request did not match a REST route or gRPC-Web preflight request")
		}
		grpcWebHandler.ServeHTTP(w, r)
		return nil
	}, middlewares...)
}

// Options registers an OPTIONS route for pattern.
//
// It is equivalent to calling Handle with "OPTIONS " prefixed to pattern.
func Options(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("OPTIONS "+pattern, handleFunc, middlewares...)
}

// Get registers a GET route for pattern.
//
// It is equivalent to calling Handle with "GET " prefixed to pattern.
func Get(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("GET "+pattern, handleFunc, middlewares...)
}

// Post registers a POST route for pattern.
//
// It is equivalent to calling Handle with "POST " prefixed to pattern.
func Post(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("POST "+pattern, handleFunc, middlewares...)
}

// Patch registers a PATCH route for pattern.
//
// It is equivalent to calling Handle with "PATCH " prefixed to pattern.
func Patch(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("PATCH "+pattern, handleFunc, middlewares...)
}

// Put registers a PUT route for pattern.
//
// It is equivalent to calling Handle with "PUT " prefixed to pattern.
func Put(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("PUT "+pattern, handleFunc, middlewares...)
}

// Delete registers a DELETE route for pattern.
//
// It is equivalent to calling Handle with "DELETE " prefixed to pattern.
func Delete(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("DELETE "+pattern, handleFunc, middlewares...)
}

func handler(handler Func, middlewares ...Middleware) func(w http.ResponseWriter, r *http.Request) {
	var mws []Middleware
	mws = append(mws, middlewares...)

	for i := len(mws) - 1; i >= 0; i-- {
		mw := mws[i]
		next := handler
		handler = func(w http.ResponseWriter, r *http.Request) error {
			return mw(w, r, next)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var err error
		if gateway != nil {
			err = gateway(w, r, handler)
		} else {
			err = handler(w, r)
		}

		invokeInfo := fmt.Sprintf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start))
		if err != nil {
			alog.Warnf(r.Context(), "%s: %s", invokeInfo, err.Error())
			if httpErr, ok := errors.AsType[*Error](err); ok {
				http.Error(w, httpErr.Msg, httpErr.Code)
			} else {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		} else {
			alog.Debug(r.Context(), invokeInfo)
		}
	}
}

// ListenAndServe starts an HTTP server on addr using the package-level mux.
//
// The server accepts unencrypted HTTP/2 in addition to HTTP/1.1, which allows
// the same listener to serve ordinary HTTP handlers and local gRPC-style
// traffic. The returned error is the result of http.Server.ListenAndServe.
func ListenAndServe(addr string) error {
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	server := &http.Server{
		Addr:      addr,
		Handler:   mux,
		Protocols: protocols,
	}
	return server.ListenAndServe()
}

// OnCloudRun reports whether the process appears to be running on Cloud Run.
//
// It is initialized from the K_SERVICE environment variable at package init
// time and is used by RequestHost when deciding whether a request should be
// treated as HTTPS behind Cloud Run's proxy.
var OnCloudRun = os.Getenv("K_SERVICE") != ""

// RequestHost returns the request's scheme and host.
//
// HTTPS is returned for TLS requests, Cloud Run requests, and ngrok hosts. All
// other requests are treated as HTTP. The returned value is suitable for
// building absolute callback URLs such as "http://localhost:8080/auth/callback".
func RequestHost(r *http.Request) string {
	if r.TLS != nil {
		return "https://" + r.Host
	}
	if OnCloudRun {
		return "https://" + r.Host
	}
	if strings.Contains(r.Host, ".ngrok") {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}

// CookieIfExists returns the value of the named cookie.
//
// If the cookie is not present or cannot be parsed, CookieIfExists returns an
// empty string. It does not distinguish between a missing cookie and a cookie
// whose value is the empty string.
func CookieIfExists(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// IsBrowserNavigationRequest reports whether r looks like a top-level page load.
//
// It returns true only for GET requests that either include
// Sec-Fetch-Mode: navigate or accept text/html. Authentication middleware uses
// this to decide whether to redirect the user into an interactive login flow or
// return an ordinary unauthorized error for API clients. This keeps unauthenticated
// browser page loads user-friendly while preserving predictable 401 responses for
// fetch/XHR calls, form submissions, and other non-navigation requests.
func IsBrowserNavigationRequest(r *http.Request) bool {
	// Only top-level browser navigations should trigger the login redirect flow.
	// API calls should get a normal unauthorized response instead, even when they
	// come from a browser tab.
	if r.Method != http.MethodGet {
		return false
	}
	// Modern browsers mark page navigations explicitly.
	if r.Header.Get("Sec-Fetch-Mode") == "navigate" {
		return true
	}
	// Fallback for clients that do not send Sec-Fetch-* headers but still ask
	// for an HTML document, which usually means "load a page".
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// IsGRPCRequest reports whether r looks like a standard gRPC request.
//
// It returns true for HTTP/2 POST requests whose Content-Type is application/grpc
// or an application/grpc subtype such as application/grpc+proto. HandleGRPC uses
// this guard because it is mounted on the broad "POST /" pattern, which would
// otherwise catch unmatched REST POST requests.
func IsGRPCRequest(r *http.Request) bool {
	contentType := requestContentType(r)
	return r.Method == http.MethodPost &&
		r.ProtoMajor == 2 &&
		(contentType == "application/grpc" || strings.HasPrefix(contentType, "application/grpc+"))
}

// IsGRPCWebRequest reports whether r looks like a standard gRPC-Web request.
//
// It returns true for POST requests whose Content-Type is application/grpc-web
// or application/grpc-web-text, including subtypes such as application/grpc-web+proto
// and application/grpc-web-text+proto. It also returns true for CORS preflight
// requests that ask to POST with gRPC-Web headers, allowing HandleGRPCWeb and
// HandleGRPCAndWeb to route browser preflights to the gRPC-Web adapter.
func IsGRPCWebRequest(r *http.Request) bool {
	if r.Method == http.MethodPost {
		contentType := requestContentType(r)
		return contentType == "application/grpc-web" ||
			strings.HasPrefix(contentType, "application/grpc-web+") ||
			contentType == "application/grpc-web-text" ||
			strings.HasPrefix(contentType, "application/grpc-web-text+")
	}
	if r.Method != http.MethodOptions || r.Header.Get("Access-Control-Request-Method") != http.MethodPost {
		return false
	}
	requestHeaders := strings.ToLower(r.Header.Get("Access-Control-Request-Headers"))
	return strings.Contains(requestHeaders, "x-grpc-web")
}

func requestContentType(r *http.Request) string {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if i := strings.Index(contentType, ";"); i >= 0 {
		contentType = contentType[:i]
	}
	return strings.TrimSpace(contentType)
}

// RequiredEnv returns the value of a required environment variable.
//
// It panics when the variable is unset or set to the empty string. Use this for
// configuration that must be present before the package can serve requests.
func RequiredEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic("Required environment variable " + name + " is not set")
	}
	return value
}
