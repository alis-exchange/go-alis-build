package mux

import (
	"net/http"
	"strings"
	"time"

	"go.alis.build/iam/v3"
	"go.alis.build/iam/v3/authn"
)

const (
	AuthCallbackPath string = "/auth/callback"
	LogoutPath       string = "/auth/logout"
)

var (
	PostAuthRedirectCookie string        = "post_auth_redirect_uri" // You can change this
	AccessTokenCookie      string        = "access_token"           // You can change this
	RefreshTokenCookie     string        = "refresh_token"          // You can change this
	AuthCookiesDomain      string        = ""                       // You can change this
	AuthClient             *authn.Client                            // You can change this
)

func init() {
	identityServiceURL := RequiredEnv("IDENTITY_SERVICE_URL")
	AuthClient = authn.NewClient(identityServiceURL)
	Get(AuthCallbackPath, callbackHandle)
	Get(LogoutPath, logoutHandle)
}

func authMiddleware(w http.ResponseWriter, r *http.Request, handler Func) error {
	// extract tokens
	tokens := &authn.Tokens{
		AccessToken:  CookieIfExists(r, AccessTokenCookie),
		RefreshToken: CookieIfExists(r, RefreshTokenCookie),
	}
	if tokens.AccessToken == "" {
		tokens.AccessToken = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}

	// authenticate
	refreshed, err := AuthClient.Authenticate(tokens, time.Now())
	if err != nil {
		if !IsBrowserNavigationRequest(r) {
			return UnauthorizedErr("%s", err.Error())
		}
		fullPath := r.URL.Path
		if r.URL.RawQuery != "" {
			fullPath += "?" + r.URL.RawQuery
		}
		callbackURI := RequestHost(r) + AuthCallbackPath
		redirectURI := AuthClient.AuthorizeURL(callbackURI, fullPath)
		http.Redirect(w, r, redirectURI, http.StatusTemporaryRedirect)
		return nil
	}

	// save refreshed tokens if any
	if refreshed {
		setAuthCookie(w, AccessTokenCookie, tokens.AccessToken, 400*24*3600)
		setAuthCookie(w, RefreshTokenCookie, tokens.RefreshToken, 400*24*3600)
	}

	// set identity in context
	identity := iam.MustFromJWT(tokens.AccessToken)
	r = r.WithContext(identity.Context(r.Context()))
	return handler(w, r)
}

func callbackHandle(w http.ResponseWriter, r *http.Request) error {
	// get query params
	code := r.URL.Query().Get("code")
	if code == "" {
		return BadRequestErr("Missing code")
	}
	state := r.URL.Query().Get("state")
	if state == "" {
		return BadRequestErr("Missing state")
	}

	// exchange code for tokens and set cookies
	callbackURI := RequestHost(r) + "/auth/callback"
	tokens, err := AuthClient.ExchangeCode(callbackURI, code)
	if err != nil {
		return InternalServerErr("Failed to exchange code: %s", err.Error())
	}
	setAuthCookie(w, AccessTokenCookie, tokens.AccessToken, 400*24*3600)
	setAuthCookie(w, RefreshTokenCookie, tokens.RefreshToken, 400*24*3600)

	// redirect to post auth redirect uri
	http.Redirect(w, r, state, http.StatusTemporaryRedirect)
	return nil
}

func logoutHandle(w http.ResponseWriter, r *http.Request) error {
	ClearAuthCookies(w)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	return nil
}

// ClearAuthCookies clears the access and refresh tokens to effectively log out the user.
// Used by the default logout handler, but can also be used by your own handlers.
func ClearAuthCookies(w http.ResponseWriter) {
	setAuthCookie(w, AccessTokenCookie, "", -1)
	setAuthCookie(w, RefreshTokenCookie, "", -1)
}

func setAuthCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Domain:   AuthCookiesDomain,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   maxAge,
		SameSite: http.SameSiteLaxMode,
	})
}

func AuthenticatedHandle(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle(pattern, handleFunc, append(
		[]Middleware{authMiddleware}, middlewares...,
	)...)
}

func AuthenticatedOptions(pattern string, handleFunc Func, middlewares ...Middleware) {
	AuthenticatedHandle("OPTIONS "+pattern, handleFunc, middlewares...)
}

func AuthenticatedGet(pattern string, handleFunc Func, middlewares ...Middleware) {
	AuthenticatedHandle("GET "+pattern, handleFunc, middlewares...)
}

func AuthenticatedPost(pattern string, handleFunc Func, middlewares ...Middleware) {
	AuthenticatedHandle("POST "+pattern, handleFunc, middlewares...)
}

func AuthenticatedPatch(pattern string, handleFunc Func, middlewares ...Middleware) {
	AuthenticatedHandle("PATCH "+pattern, handleFunc, middlewares...)
}

func AuthenticatedPut(pattern string, handleFunc Func, middlewares ...Middleware) {
	AuthenticatedHandle("PUT "+pattern, handleFunc, middlewares...)
}

func AuthenticatedDelete(pattern string, handleFunc Func, middlewares ...Middleware) {
	AuthenticatedHandle("DELETE "+pattern, handleFunc, middlewares...)
}
