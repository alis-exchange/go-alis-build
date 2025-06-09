package iam

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.alis.build/alog"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
	"google.golang.org/grpc/metadata"
)

type Authenticator struct {
	authHost   string
	publicKeys *sync.Map
}

const ForwardedHostHeader = "x-forwarded-host"

// Creates a new Authenticator instance that can be used to authenticate users
// via the alis-build managed iam service. If authHost is empty,authHost =
// "https://iam-v1-{runHash}.run.app:443".
func NewAuthenticator(authHost string) *Authenticator {
	if authHost == "" {
		runHash := os.Getenv("ALIS_RUN_HASH")
		if runHash == "" {
			alog.Fatalf(context.Background(), "ALIS_RUN_HASH not set")
		}
		authHost = fmt.Sprintf("https://iam-v1-%s.run.app:443", runHash)
	}
	an := &Authenticator{
		authHost:   authHost,
		publicKeys: &sync.Map{},
	}
	return an
}

// Returns whether the request has "/auth" as the prefix.
func (h *Authenticator) IsAuthRequest(req *http.Request) bool {
	path := req.URL.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.HasPrefix(path, "/auth")
}

// Reverse proxies to the authHost for all requests that have "/auth" as the prefix.
// You must first check if the request is an auth request using IsAuthRequest.
func (h *Authenticator) HandleAuthRequest(resp http.ResponseWriter, req *http.Request) {
	// make hit and return response body and cookie headers
	endpoint := fmt.Sprintf("%s%s", h.authHost, req.URL.Path)
	if req.URL.RawQuery != "" {
		endpoint += "?" + req.URL.RawQuery
	}

	h.reverseProxyToEndpoint(resp, req, endpoint)
}

// Returns whether user is authenticated.
// First checks if there is a valid access token cookie and attaches it as an Authorization header.
// If above failed, it checks for a refresh_token cookie and tries to do a refresh on the fly by hitting /auth/refresh.
// Lastly, it checks directly for a Authorization header.
func (h *Authenticator) IsAuthenticated(resp http.ResponseWriter, req *http.Request) bool {
	tryToRefresh := func() bool {
		// try to refresh token by making get request to /auth/refresh
		refreshTokenCookie, err := req.Cookie("refresh_token")
		if err == nil && refreshTokenCookie.Value != "" {
			refreshReq, err := http.NewRequest("GET", fmt.Sprintf("%s/auth/refresh", h.authHost), nil)
			if err != nil {
				http.Error(resp, err.Error(), http.StatusServiceUnavailable)
				return true
			}
			refreshReq.Header.Add(ForwardedHostHeader, req.Host)
			refreshReq.AddCookie(refreshTokenCookie)
			client := &http.Client{}
			refreshResp, err := client.Do(refreshReq)
			if err != nil {
				http.Error(resp, err.Error(), http.StatusServiceUnavailable)
				return false
			}

			// copy cookies and check for access_token
			hasNewValidAccessToken := false
			var accessT string

			if refreshResp != nil && refreshResp.Request != nil && refreshResp.Request.Response != nil {
				for _, c := range refreshResp.Request.Response.Cookies() {
					http.SetCookie(resp, c)
					if c.Name == "access_token" {
						if err := h.validateJwt(c.Value); err == nil {
							hasNewValidAccessToken = true
							accessT = c.Value
						}
					}
				}
			} else {
				alog.Warnf(req.Context(), "empty refresh resp")
			}
			if hasNewValidAccessToken {
				req.Header.Del(AuthHeader)
				req.Header.Add(AuthHeader, "Bearer "+accessT)
				return true
			}
		}
		return false
	}

	// directly check for Authorization header
	atFromAuthHeader := req.Header.Get(AuthHeader)
	if atFromAuthHeader != "" {
		err := h.validateJwt(atFromAuthHeader)
		if err == nil {
			req.Header.Del(AuthHeader)
			req.Header.Add(AuthHeader, "Bearer "+atFromAuthHeader)
			return true
		} else {
			return tryToRefresh()
		}
	} else {
		// ensure all requests not going to /auth have a valid access token
		accessTokenCookie, err := req.Cookie("access_token")
		if err != nil || accessTokenCookie.Value == "" {
			return tryToRefresh()
		} else {
			// validate access token
			err = h.validateJwt(accessTokenCookie.Value)
			if err == nil {
				req.Header.Del(AuthHeader)
				req.Header.Add(AuthHeader, "Bearer "+accessTokenCookie.Value)
				return true
			} else {
				return tryToRefresh()
			}
		}
	}
}

// Redirects to /auth/signin and sets the post_auth_redirect_uri cookie to the current request URI so that the user can
// be redirected back after signing in.
//   - resp: the http.ResponseWriter
//   - req: the http.Request
//   - postAuthRedirectUri: where to go after signing in. Use req.RequestURI to redirect back to the current page.
func (h *Authenticator) RedirectToSignIn(resp http.ResponseWriter, req *http.Request, postAuthRedirectUri string) {
	http.SetCookie(resp, &http.Cookie{
		Name:  "post_auth_redirect_uri",
		Value: postAuthRedirectUri,
	})
	http.Redirect(resp, req, "/auth/signin", http.StatusTemporaryRedirect)
}

// Reverse proxies to /auth/denied which shows a message in the line of "Not found or you don't have access".
func (h *Authenticator) HandleNotFoundOrAccessDenied(resp http.ResponseWriter, req *http.Request) {
	endpoint := fmt.Sprintf("%s/auth/denied", h.authHost)
	h.reverseProxyToEndpoint(resp, req, endpoint)
}

// Implements a basic reverse proxy to the authHost that forwards the incoming request to the authHost and copies the
// response headers, body and cookies back to the client.
func (h *Authenticator) reverseProxyToEndpoint(resp http.ResponseWriter, req *http.Request, endpoint string) {
	// Create a new request using http
	authReq, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// add headers
	authReq.Header.Add(ForwardedHostHeader, req.Host)
	endpointHost := strings.TrimPrefix(endpoint, "https://")
	endpointHost = strings.TrimPrefix(endpointHost, "http://")
	audience := "https://" + strings.Split(endpointHost, ":")[0]
	tokenSource, err := idtoken.NewTokenSource(req.Context(), audience, option.WithAudiences(audience))
	if err != nil {
		http.Error(resp, "creating token source", http.StatusInternalServerError)
		return
	}
	token, err := tokenSource.Token()
	if err != nil {
		http.Error(resp, "getting token", http.StatusInternalServerError)
		return
	}
	token.SetAuthHeader(authReq)

	// set cookies
	for _, c := range req.Cookies() {
		authReq.AddCookie(c)
	}

	// Send req using http Client
	client := &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	authResp, err := client.Do(authReq)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer authResp.Body.Close()

	// copy cookies
	for _, c := range authResp.Cookies() {
		http.SetCookie(resp, c)
	}

	// copy non-cookie headers; also ignore Content-Length as it will be recalculated by the http library
	for k, v := range authResp.Header {
		if k == "Set-Cookie" {
			continue
		}
		if k == "Cookie" {
			continue
		}
		if k == "Content-Length" {
			continue
		}
		resp.Header().Set(k, v[0])
	}

	// copy status code
	resp.WriteHeader(authResp.StatusCode)

	// copy body
	_, err = io.Copy(resp, authResp.Body)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Returns an error if the token is invalid
func (h *Authenticator) validateJwt(accessToken string) error {
	// validate token.Signature against publicKey
	_, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		// get kid from headers
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("kid not found")
		}

		todayKey := time.Now().UTC().Format("2006-01-02")
		yesterdayKey := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

		if kid == todayKey || kid == yesterdayKey {
			publicKey, ok := h.publicKeys.Load(kid)
			if !ok {
				endpointHost := strings.TrimPrefix(h.authHost, "https://")
				endpointHost = strings.TrimPrefix(endpointHost, "http://")
				audience := "https://" + strings.Split(endpointHost, ":")[0]
				tokenSource, err := idtoken.NewTokenSource(context.Background(), audience, option.WithAudiences(audience))
				if err != nil {
					return nil, fmt.Errorf("creating token source: %w", err)
				}
				token, err := tokenSource.Token()
				if err != nil {
					return nil, fmt.Errorf("getting token: %w", err)
				}
				authReq, err := http.NewRequest("GET", h.authHost+"/auth/keys", nil)
				if err != nil {
					return nil, fmt.Errorf("failed to create request: %w", err)
				}
				token.SetAuthHeader(authReq)

				// get keys from /auth/keys
				keysResp, err := http.DefaultClient.Do(authReq)
				if err != nil {
					return nil, fmt.Errorf("failed to get public keys: %w", err)
				}
				type Key struct {
					Kid string `json:"kid"`
					Key string `json:"key"`
				}
				var keys []Key
				err = json.NewDecoder(keysResp.Body).Decode(&keys)
				if err != nil {
					return nil, fmt.Errorf("failed to decode public keys: %w", err)
				}
				for _, key := range keys {
					keyBytes := []byte(key.Key)
					block, _ := pem.Decode(keyBytes)
					if block == nil {
						return nil, fmt.Errorf("failed to decode public key")
					}
					pubKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
					if err != nil {
						return nil, fmt.Errorf("failed to parse public key: %w", err)
					}
					h.publicKeys.Store(key.Kid, pubKey)
				}

				publicKey, ok = h.publicKeys.Load(kid)
				if !ok {
					return nil, fmt.Errorf("public key not found")
				}
			}
			return publicKey, nil
		} else {
			return nil, fmt.Errorf("kid not found")
		}
	})
	return err
}

// ForwardAuthorizationHeader forwards the Authorization header in the incoming ctx to the outgoing ctx.
// Use this at the very top of your unary and streaming interceptors in the context of a gRPC web server.
//   - ctx: the context
//   - allowMissingAuthHeader: whether to not return an error if the Authorization header is missing. Only set to true
//     if you have checked for authentication when required in the http server layer and you have some methods that
//     do not require authentication.
func (h *Authenticator) ForwardAuthorizationHeader(ctx context.Context, allowMissingAuthHeader bool) (context.Context, error) {
	// forward authorization header as metadata in x-alis-forwarded-authorization
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		accessToken := md.Get(AuthHeader)
		if len(accessToken) > 0 {
			ctx = metadata.AppendToOutgoingContext(ctx, AlisForwardingHeader, accessToken[0])
		} else {
			if allowMissingAuthHeader {
				return ctx, nil
			}
			return ctx, fmt.Errorf("authorization header not found")
		}
	} else {
		if allowMissingAuthHeader {
			return ctx, nil
		}
		return ctx, fmt.Errorf("metadata not found")
	}
	return ctx, nil
}
