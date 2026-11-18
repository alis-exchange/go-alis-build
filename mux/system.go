package mux

import (
	"fmt"
	"net/http"
	"slices"
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

	// SystemAudiences lists the aud claim values system routes accept in Google
	// ID tokens from the environment service account.
	//
	// When empty, which is the default, a token minted for any audience is
	// accepted, as before. That means a token the environment service account
	// minted for some other recipient, such as another service, a Cloud Tasks
	// or Pub/Sub push target, or an outbound webhook, is also accepted here if
	// it leaks, for the rest of its lifetime.
	//
	// When set, the token's aud must equal one of these values exactly, or the
	// request is rejected with a 401 that names the audience it carried. There
	// is no origin or prefix matching. A Google ID token carries whatever
	// audience its caller asked for, so list every one in use:
	//
	//   - Each URL callers dial. A Cloud Run service has two run.app URLs, the
	//     hashed one and https://SERVICE-PROJECT_NUMBER.REGION.run.app, plus
	//     any custom or load balancer domain.
	//   - Pub/Sub push subscriptions, Cloud Tasks and Cloud Scheduler jobs
	//     without an explicit audience: they default it to the full target
	//     URL, path included, such as https://svc-abc123-ew.a.run.app/events.
	//     Setting an explicit audience on them keeps the list short.
	//
	// Set this before serving requests.
	SystemAudiences []string

	// validateIDToken validates a Google ID token. It is a variable so tests
	// can stand in for Google's signing keys.
	validateIDToken = idtoken.Validate
)

// systemMiddleware authenticates system routes. It accepts a bearer Google ID
// token whose signature validates, whose email claim is
// EnvironmentServiceAccountEmail and, when SystemAudiences is set, whose
// audience is listed there. A token without an email claim is rejected like
// any other account.
func systemMiddleware(w http.ResponseWriter, r *http.Request, handler Func) error {
	authorizationHdr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	idToken, err := validateIDToken(r.Context(), authorizationHdr, "")
	if err != nil {
		return UnauthorizedErr("invalid google authorization header: %v", err)
	}
	email, _ := idToken.Claims["email"].(string)
	if email != EnvironmentServiceAccountEmail {
		return UnauthorizedErr("not environment service account")
	}
	if len(SystemAudiences) > 0 && !slices.Contains(SystemAudiences, idToken.Audience) {
		return UnauthorizedErr("system token audience %q is not in SystemAudiences", idToken.Audience)
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
//
// Callers mint the token with the environment service account for an
// audience, normally the URL they dial. Any audience is accepted unless
// SystemAudiences is set, which is what stops a token minted for a different
// recipient from being replayed here; see it for which audiences to list.
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
