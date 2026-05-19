package mux

import (
	"net/http"
	"strings"
	"time"

	"go.alis.build/iam/v3"
	"go.alis.build/iam/v3/authn"
)

const (
	// AuthCallbackPath is the route path registered for OAuth callback handling.
	//
	// The package registers a GET handler for this path during init.
	AuthCallbackPath string = "/auth/callback"

	// LogoutPath is the route path registered for clearing authentication cookies.
	//
	// The package registers a GET handler for this path during init.
	LogoutPath string = "/auth/logout"
)

var (
	// PostAuthRedirectCookie is the cookie name reserved for post-login redirects.
	//
	// Callers may change this value before handling requests if they need to
	// align cookie names across services.
	PostAuthRedirectCookie string = "post_auth_redirect_uri"

	// AccessTokenCookie is the cookie name used to store the IAM access token.
	//
	// Callers may change this value before handling requests if their service
	// uses a different cookie naming convention.
	AccessTokenCookie string = "access_token"

	// RefreshTokenCookie is the cookie name used to store the IAM refresh token.
	//
	// Callers may change this value before handling requests if their service
	// uses a different cookie naming convention.
	RefreshTokenCookie string = "refresh_token"

	// AuthCookiesDomain is the Domain attribute applied to authentication cookies.
	//
	// The default empty string leaves the cookie host-only. Set this before
	// serving requests when cookies should be shared across subdomains.
	AuthCookiesDomain string = ""

	// AuthClient is the authentication client used by the built-in auth handlers.
	//
	// It is initialized from IDENTITY_SERVICE_URL during package init. Tests or
	// services with custom authentication plumbing may replace it before serving
	// requests.
	AuthClient *authn.Client

	// WWWAuthenticateHeader is the optional 'WWW-Authenticate' response header value
	// written with non-browser unauthorized requests.
	WWWAuthenticateHeader = ""
)

func init() {
	identityServiceURL := RequiredEnv("IDENTITY_SERVICE_URL")
	AuthClient = authn.NewClient(identityServiceURL)
	Get(AuthCallbackPath, callbackHandle)
	Get(LogoutPath, logoutHandle)
}

// authMiddleware authenticates a request before invoking handler.
//
// It reads tokens from the configured auth cookies, falls back to the bearer
// Authorization header for the access token, and refreshes auth cookies when the
// authentication client rotates tokens. When authentication fails, browser page
// navigations are redirected to the identity service authorization URL with the
// original path and query as state. Non-navigation requests, including API calls
// from browsers, receive a 401 Unauthorized error instead of a redirect.
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
			if WWWAuthenticateHeader != "" {
				w.Header().Set("WWW-Authenticate", WWWAuthenticateHeader)
			}
			return UnauthorizedErr("%s", err.Error())
		}
		fullPath := r.URL.Path
		if r.URL.RawQuery != "" {
			fullPath += "?" + r.URL.RawQuery
		}
		callbackURI := RequestHost(r) + AuthCallbackPath
		login, err := AuthClient.StartLogin(w, r, callbackURI, fullPath)
		if err != nil {
			return InternalServerErr("Failed to start login: %s", err.Error())
		}
		http.Redirect(w, r, login.URL, http.StatusTemporaryRedirect)
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
	callbackURI := RequestHost(r) + AuthCallbackPath
	tokens, returnTo, err := AuthClient.CompleteLogin(w, r, callbackURI)
	if err != nil {
		return BadRequestErr("Failed to complete login: %s", err.Error())
	}
	setAuthCookie(w, AccessTokenCookie, tokens.AccessToken, 400*24*3600)
	setAuthCookie(w, RefreshTokenCookie, tokens.RefreshToken, 400*24*3600)

	// redirect to post auth redirect uri
	http.Redirect(w, r, returnTo, http.StatusTemporaryRedirect)
	return nil
}

func logoutHandle(w http.ResponseWriter, r *http.Request) error {
	ClearAuthCookies(w)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	return nil
}

// ClearAuthCookies expires the access and refresh token cookies.
//
// It uses the currently configured AccessTokenCookie, RefreshTokenCookie, and
// AuthCookiesDomain values. The default logout handler calls this function, and
// custom handlers can call it when they need to terminate a browser session.
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

// AuthenticatedHandle registers an authenticated route on the package-level mux.
//
// The auth middleware runs before any middlewares supplied by the caller. It
// authenticates access and refresh tokens from cookies, falls back to a bearer
// Authorization header for the access token, refreshes cookies when needed, and
// stores the IAM identity in the request context before invoking handleFunc.
//
// If authentication fails for a browser navigation request, the middleware
// redirects the user to the identity service authorization URL. The redirect uses
// RequestHost plus AuthCallbackPath as the OAuth callback URI and preserves the
// requested path and query in the authorization state so the callback can return
// the browser to the originally requested page. Requests that do not look like
// top-level browser navigations receive UnauthorizedErr instead, which avoids
// converting API calls into HTML login redirects.
func AuthenticatedHandle(pattern string, handleFunc Func, middlewares ...Middleware) {
	Handle(pattern, handleFunc, append(
		[]Middleware{authMiddleware}, middlewares...,
	)...)
}

// AuthenticatedHandleHTTP registers an authenticated http.Handler on the package-level mux.
//
// It adapts httpHandler into a Func and applies the same auth middleware used by
// AuthenticatedHandle before running any caller-supplied middlewares. Use this
// for generated REST gateways, nested ServeMux values, or other standard
// http.Handler implementations that should use the browser/session
// authentication flow.
func AuthenticatedHandleHTTP(pattern string, httpHandler http.Handler, middlewares ...Middleware) {
	AuthenticatedHandle(pattern, func(w http.ResponseWriter, r *http.Request) error {
		httpHandler.ServeHTTP(w, r)
		return nil
	}, middlewares...)
}

// AuthenticatedHandleGRPCWeb registers an authenticated gRPC-Web handler.
//
// POST requests are authenticated with the same browser/session auth middleware
// used by AuthenticatedHandle and are served only when they look like gRPC-Web
// requests. OPTIONS preflight requests are not authenticated, because browsers
// must receive the gRPC-Web adapter's CORS preflight response before sending the
// authenticated POST request.
func AuthenticatedHandleGRPCWeb(grpcWebHandler http.Handler, middlewares ...Middleware) {
	AuthenticatedHandle("POST /", func(w http.ResponseWriter, r *http.Request) error {
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

// AuthenticatedOptions registers an authenticated OPTIONS route for pattern.
//
// It is equivalent to calling AuthenticatedHandle with "OPTIONS " prefixed to
// pattern.
func AuthenticatedOptions(pattern string, handleFunc Func, middlewares ...Middleware) {
	AuthenticatedHandle("OPTIONS "+pattern, handleFunc, middlewares...)
}

// AuthenticatedGet registers an authenticated GET route for pattern.
//
// It is equivalent to calling AuthenticatedHandle with "GET " prefixed to
// pattern.
func AuthenticatedGet(pattern string, handleFunc Func, middlewares ...Middleware) {
	AuthenticatedHandle("GET "+pattern, handleFunc, middlewares...)
}

// AuthenticatedPost registers an authenticated POST route for pattern.
//
// It is equivalent to calling AuthenticatedHandle with "POST " prefixed to
// pattern.
func AuthenticatedPost(pattern string, handleFunc Func, middlewares ...Middleware) {
	AuthenticatedHandle("POST "+pattern, handleFunc, middlewares...)
}

// AuthenticatedPatch registers an authenticated PATCH route for pattern.
//
// It is equivalent to calling AuthenticatedHandle with "PATCH " prefixed to
// pattern.
func AuthenticatedPatch(pattern string, handleFunc Func, middlewares ...Middleware) {
	AuthenticatedHandle("PATCH "+pattern, handleFunc, middlewares...)
}

// AuthenticatedPut registers an authenticated PUT route for pattern.
//
// It is equivalent to calling AuthenticatedHandle with "PUT " prefixed to
// pattern.
func AuthenticatedPut(pattern string, handleFunc Func, middlewares ...Middleware) {
	AuthenticatedHandle("PUT "+pattern, handleFunc, middlewares...)
}

// AuthenticatedDelete registers an authenticated DELETE route for pattern.
//
// It is equivalent to calling AuthenticatedHandle with "DELETE " prefixed to
// pattern.
func AuthenticatedDelete(pattern string, handleFunc Func, middlewares ...Middleware) {
	AuthenticatedHandle("DELETE "+pattern, handleFunc, middlewares...)
}
