package mux

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"go.alis.build/alog"
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

	// AcceptedAudiences lists the aud claim values this service accepts in IAM
	// access tokens.
	//
	// When empty, which is the default, a populated aud claim must equal
	// RequestHost(r) for the request being served. That is the historical
	// behaviour. It compares against the client-controlled Host header, and it
	// forces an identity provider to omit aud entirely, because a token minted
	// for one service is rejected by every other host.
	//
	// When set, a populated aud must contain one of these values, compared
	// exactly, and RequestHost is no longer consulted. Both a string aud and an
	// RFC 7519 array of strings are accepted, and an array matches when any of
	// its elements does. A token carrying no aud is still accepted and reported
	// through MissingAudienceHandler. A token whose aud does not match is never
	// refreshed, and a refreshed token is checked in turn.
	//
	// Set this before serving requests. Migration order matters: every resource
	// server has to be running a mux that accepts the audience before the
	// identity provider starts populating aud, because earlier versions reject
	// any aud other than their own origin.
	AcceptedAudiences []string

	// MissingAudienceHandler is called when AcceptedAudiences is set and the
	// authenticated access token carries no aud claim.
	//
	// The default logs and accepts the request, which holds the deprecation
	// window open while the identity provider is migrated to populating aud. The
	// log line is the signal for timing that cutover: it shows which services
	// have adopted AcceptedAudiences, and later whether any token without an
	// audience is still in circulation. Return an error, such as
	// UnauthorizedErr("missing audience"), to close the window. The error is
	// returned to the client as-is and never starts a login redirect.
	//
	// It is never called while AcceptedAudiences is empty.
	MissingAudienceHandler = func(w http.ResponseWriter, r *http.Request) error {
		alog.Infof(r.Context(), "access token accepted without aud claim: %s %s", r.Method, r.URL.Path)
		return nil
	}

	// AuthClient is the authentication client used by the built-in auth handlers.
	//
	// It is initialized from IDENTITY_SERVICE_URL during package init. Tests or
	// services with custom authentication plumbing may replace it before serving
	// requests.
	AuthClient *authn.Client

	// What to do for unauthorized requests. By default this just returns a 401 status.
	UnauthorizedHandler = func(w http.ResponseWriter, r *http.Request, details string) error {
		return UnauthorizedErr("%s", details)
	}

	PostAuthMiddleware Middleware = func(w http.ResponseWriter, r *http.Request, handler Func) error {
		return handler(w, r)
	}
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
	var refreshed bool
	var err error
	if len(AcceptedAudiences) == 0 {
		refreshed, err = AuthClient.AuthenticateWithAudience(tokens, time.Now(), RequestHost(r))
	} else {
		// The audience is enforced by checkAudience below, against the token this
		// request ends up using. Leaving it out here keeps a token with the wrong
		// audience from being refreshed into an accepted one.
		refreshed, err = AuthClient.Authenticate(tokens, time.Now())
	}
	if err != nil {
		if !IsBrowserNavigationRequest(r) {
			return UnauthorizedHandler(w, r, err.Error())
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
		setAuthCookie(w, r, AccessTokenCookie, tokens.AccessToken, 400*24*3600)
		setAuthCookie(w, r, RefreshTokenCookie, tokens.RefreshToken, 400*24*3600)
	}

	// enforce the configured audiences against the token this request uses,
	// which is the refreshed one if a refresh happened
	if len(AcceptedAudiences) > 0 {
		if err := checkAudience(w, r, tokens.AccessToken); err != nil {
			return err
		}
	}

	// set identity in context
	identity := iam.MustFromJWT(tokens.AccessToken)
	r = r.WithContext(identity.Context(r.Context()))
	return PostAuthMiddleware(w, r, handler)
}

// checkAudience enforces AcceptedAudiences against the aud claim of token.
//
// A mismatch is rejected through UnauthorizedHandler for every kind of request,
// browser navigations included. Logging in again cannot repair a mismatch,
// because the identity provider would mint another token with the same
// audience, so redirecting would bounce the browser between the two services
// indefinitely. A token with no aud claim is left to MissingAudienceHandler.
func checkAudience(w http.ResponseWriter, r *http.Request, token string) error {
	audiences, err := jwtAudiences(token)
	if err != nil {
		return UnauthorizedHandler(w, r, err.Error())
	}
	if len(audiences) == 0 {
		return MissingAudienceHandler(w, r)
	}
	for _, audience := range audiences {
		if slices.Contains(AcceptedAudiences, audience) {
			return nil
		}
	}
	return UnauthorizedHandler(w, r, fmt.Sprintf("invalid audience %q", audiences))
}

// jwtAudiences returns the aud claim of token.
//
// RFC 7519 allows aud to be a single string or an array of strings, and both
// are returned as a slice. A token with no aud claim, or an empty one, returns
// nil without an error so the caller can treat it as absent. Any other shape is
// an error, so an audience that cannot be read is rejected rather than ignored.
// The signature is not checked here; the token must be validated first.
func jwtAudiences(token string) ([]string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format, expect {hdr}.{body}.{sig}")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}
	var claims struct {
		Aud json.RawMessage `json:"aud"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	if len(claims.Aud) == 0 {
		return nil, nil
	}
	// A JSON null unmarshals into the string form without error and leaves it
	// empty, so it is treated as absent alongside "".
	var single string
	if err := json.Unmarshal(claims.Aud, &single); err == nil {
		if single == "" {
			return nil, nil
		}
		return []string{single}, nil
	}
	var many []string
	if err := json.Unmarshal(claims.Aud, &many); err != nil {
		return nil, fmt.Errorf("invalid aud claim: %w", err)
	}
	if len(many) == 0 {
		return nil, nil
	}
	return many, nil
}

func callbackHandle(w http.ResponseWriter, r *http.Request) error {
	callbackURI := RequestHost(r) + AuthCallbackPath
	tokens, returnTo, err := AuthClient.CompleteLogin(w, r, callbackURI)
	if err != nil {
		return BadRequestErr("Failed to complete login: %s", err.Error())
	}
	setAuthCookie(w, r, AccessTokenCookie, tokens.AccessToken, 400*24*3600)
	setAuthCookie(w, r, RefreshTokenCookie, tokens.RefreshToken, 400*24*3600)

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
	setAuthCookie(w, nil, AccessTokenCookie, "", -1)
	setAuthCookie(w, nil, RefreshTokenCookie, "", -1)
}

func setAuthCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Domain:   AuthCookiesDomain,
		Path:     "/",
		HttpOnly: true,
		Secure:   shouldSecureAuthCookies(r),
		MaxAge:   maxAge,
		SameSite: http.SameSiteLaxMode,
	})
}

func shouldSecureAuthCookies(r *http.Request) bool {
	// Cookie Secure must match the scheme the browser is using for this app.
	// Local development commonly runs on plain http://localhost, and browsers
	// will not store Secure cookies set by an HTTP response. If we mark those
	// callback cookies as Secure, the callback succeeds, redirects to the app,
	// and the next request starts a fresh login because no auth cookies were
	// persisted.
	if r == nil {
		// Callers such as ClearAuthCookies do not have the request available.
		// Default to secure in that case so cookie writes without request context
		// preserve the safest production behavior.
		return true
	}

	// RequestHost centralizes the scheme decision for TLS, Cloud Run, ngrok, and
	// local HTTP. Reusing it keeps the cookie policy aligned with the callback
	// URL we send to the identity service for the same request.
	return strings.HasPrefix(RequestHost(r), "https://")
}

// AuthenticatedHandle registers an authenticated route on the package-level mux.
//
// The auth middleware runs before any middlewares supplied by the caller. It
// authenticates access and refresh tokens from cookies, falls back to a bearer
// Authorization header for the access token, refreshes cookies when needed, and
// stores the IAM identity in the request context before invoking handleFunc.
//
// When AcceptedAudiences is set, the aud claim of the token the request ends up
// using is checked after any refresh. See AcceptedAudiences and
// MissingAudienceHandler.
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
