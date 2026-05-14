package mux

import (
	"fmt"
	"net/http"
	"strings"

	"go.alis.build/iam/v3"
	"google.golang.org/api/idtoken"
)

var (
	// ProjectID is the ALIS project identifier for the running service.
	//
	// It is read from ALIS_OS_PROJECT during package init and is used to derive
	// the expected environment service account.
	ProjectID = RequiredEnv("ALIS_OS_PROJECT")

	// EnvironmentServiceAccountEmail is the only service account accepted by system routes.
	//
	// System middleware validates incoming Google ID tokens and requires the
	// token email claim to match this address.
	EnvironmentServiceAccountEmail = fmt.Sprintf("alis-build@%s.iam.gserviceaccount.com", ProjectID)
)

func systemMiddleware(w http.ResponseWriter, r *http.Request, handler Func) error {
	authorizationHdr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	idToken, err := idtoken.Validate(r.Context(), authorizationHdr, "")
	if err != nil {
		return UnauthorizedErr("invalid google authorization header: %v", err)
	}
	if idToken.Claims["email"].(string) != EnvironmentServiceAccountEmail {
		return UnauthorizedErr("not environment service account")
	}

	identity := iam.MustFromJWT(authorizationHdr)
	r = r.WithContext(identity.Context(r.Context()))
	return handler(w, r)
}

// SystemHandle registers a system-authenticated route on the package-level mux.
//
// The system middleware runs before any middlewares supplied by the caller. It
// validates the bearer token as a Google ID token, requires it to belong to the
// environment service account, and stores the IAM identity in the request
// context before invoking handleFunc.
func SystemHandle(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle(pattern, handleFunc, append(
		[]Middleware{systemMiddleware}, middlewares...,
	)...)
}

// SystemHandleGRPC registers a system-authenticated gRPC-compatible handler.
//
// It is equivalent to HandleGRPC, but validates the system bearer token before
// checking whether the request should be served by grpcHandler.
func SystemHandleGRPC(grpcHandler http.Handler, middlewares ...Middleware) {
	SystemHandle("POST /", func(w http.ResponseWriter, r *http.Request) error {
		if !IsGRPCRequest(r) {
			return NotFoundErr("request did not match a REST route or gRPC request")
		}
		grpcHandler.ServeHTTP(w, r)
		return nil
	}, middlewares...)
}

// SystemOptions registers a system-authenticated OPTIONS route for pattern.
//
// It is equivalent to calling SystemHandle with "OPTIONS " prefixed to pattern.
func SystemOptions(pattern string, handleFunc Func, middlewares ...Middleware) {
	SystemHandle("OPTIONS "+pattern, handleFunc, middlewares...)
}

// SystemGet registers a system-authenticated GET route for pattern.
//
// It is equivalent to calling SystemHandle with "GET " prefixed to pattern.
func SystemGet(pattern string, handleFunc Func, middlewares ...Middleware) {
	SystemHandle("GET "+pattern, handleFunc, middlewares...)
}

// SystemPost registers a system-authenticated POST route for pattern.
//
// It is equivalent to calling SystemHandle with "POST " prefixed to pattern.
func SystemPost(pattern string, handleFunc Func, middlewares ...Middleware) {
	SystemHandle("POST "+pattern, handleFunc, middlewares...)
}

// SystemPatch registers a system-authenticated PATCH route for pattern.
//
// It is equivalent to calling SystemHandle with "PATCH " prefixed to pattern.
func SystemPatch(pattern string, handleFunc Func, middlewares ...Middleware) {
	SystemHandle("PATCH "+pattern, handleFunc, middlewares...)
}

// SystemPut registers a system-authenticated PUT route for pattern.
//
// It is equivalent to calling SystemHandle with "PUT " prefixed to pattern.
func SystemPut(pattern string, handleFunc Func, middlewares ...Middleware) {
	SystemHandle("PUT "+pattern, handleFunc, middlewares...)
}

// SystemDelete registers a system-authenticated DELETE route for pattern.
//
// It is equivalent to calling SystemHandle with "DELETE " prefixed to pattern.
func SystemDelete(pattern string, handleFunc Func, middlewares ...Middleware) {
	SystemHandle("DELETE "+pattern, handleFunc, middlewares...)
}
