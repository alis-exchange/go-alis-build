// Package mux provides a simple http.ServeMux wrapper.
package mux

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"go.alis.build/alog"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var (
	mux     = http.NewServeMux()
	gateway Middleware
)

type (
	Func       func(w http.ResponseWriter, r *http.Request) error
	Middleware func(w http.ResponseWriter, r *http.Request, handler Func) error
)

func AddGateway(gw Middleware) {
	gateway = gw
}

func Handle(pattern string, handleFunc Func, middlewares ...Middleware) {
	mux.HandleFunc(pattern, handler(handleFunc, middlewares...))
}

func Options(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("OPTIONS "+pattern, handleFunc, middlewares...)
}

func Get(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("GET "+pattern, handleFunc, middlewares...)
}

func Post(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("POST "+pattern, handleFunc, middlewares...)
}

func Patch(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("PATCH "+pattern, handleFunc, middlewares...)
}

func Put(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle("PUT "+pattern, handleFunc, middlewares...)
}

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

func ListenAndServe(addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}
	return server.ListenAndServe()
}

var OnCloudRun = os.Getenv("K_SERVICE") != ""

// RequestHost returns the host of the request, e.g. http://localhost:8080 or "https://example.com"
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

// CookieIfExists returns the value of the cookie with the given name if it exists, or an empty string if it does not.
func CookieIfExists(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

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

// RequiredEnv returns the value of the required environment variable with the given name.
// If the variable is not set, it panics with an error message.
func RequiredEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic("Required environment variable " + name + " is not set")
	}
	return value
}
