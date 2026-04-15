package mux

import (
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/api/idtoken"
)

var (
	ProjectID                      = RequiredEnv("ALIS_OS_PROJECT")
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
	return handler(w, r)
}

func SystemHandle(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle(pattern, handleFunc, append(
		[]Middleware{systemMiddleware}, middlewares...,
	)...)
}

func SystemOptions(pattern string, handleFunc Func, middlewares ...Middleware) {
	SystemHandle("OPTIONS "+pattern, handleFunc, middlewares...)
}

func SystemGet(pattern string, handleFunc Func, middlewares ...Middleware) {
	SystemHandle("GET "+pattern, handleFunc, middlewares...)
}

func SystemPost(pattern string, handleFunc Func, middlewares ...Middleware) {
	SystemHandle("POST "+pattern, handleFunc, middlewares...)
}

func SystemPatch(pattern string, handleFunc Func, middlewares ...Middleware) {
	SystemHandle("PATCH "+pattern, handleFunc, middlewares...)
}

func SystemPut(pattern string, handleFunc Func, middlewares ...Middleware) {
	SystemHandle("PUT "+pattern, handleFunc, middlewares...)
}

func SystemDelete(pattern string, handleFunc Func, middlewares ...Middleware) {
	SystemHandle("DELETE "+pattern, handleFunc, middlewares...)
}
