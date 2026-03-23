package authproxy

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.alis.build/iam/v2"
	"google.golang.org/grpc/metadata"
)

type AuthProxy struct {
	authHost              string
	publicKeys            *sync.Map
	publicPrefixes        []string
	publicExacts          map[string]bool
	fixedPostAuthRedirect string
}

// AlisForwardedHostHeader ia the header used to forward the host with the
const ForwardedHostHeader = "x-forwarded-host"

// Creates a new AuthProxy with the given authHost.
// Example authHost: "https://iam-auth-123456789.europe-west1.run.app".
func New(authHost string) *AuthProxy {
	return &AuthProxy{
		authHost:   strings.TrimSuffix(authHost, "/"),
		publicKeys: &sync.Map{},
		publicExacts: map[string]bool{
			"/favicon.ico": true,
		},
	}
}

// Exclude paths from authentication, i.e. no access token is required for these paths.
// You can specify exact paths or paths with a wildcard (*) at the end.
// favicon.ico is by default a public path.
func (h *AuthProxy) WithPublicPaths(paths ...string) {
	for _, path := range paths {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if strings.HasSuffix(path, "*") {
			h.publicPrefixes = append(h.publicPrefixes, strings.TrimSuffix(path, "*"))
		} else {
			h.publicExacts[path] = true
		}
	}
}

// Exclude favicon.ico from public paths, as its default behavior is to be public.
func (h *AuthProxy) WithPrivateFavicon() {
	h.publicExacts["/favicon.ico"] = false
}

// Hardcodes the path to redirect to after authentication in stead of using the request URI.
func (h *AuthProxy) WithFixedPostAuthRedirect(path string) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	h.fixedPostAuthRedirect = path
}

// Reverse proxies /auth/* requests to the authHost and validates the access_token cookie
// set by the authHost for all other requests.
// If the access token is valid, it also adds it as a header to the request.
//
// Returns true if the request was handled, in which case you should return from the handler.
func (h *AuthProxy) HandleAuth(resp http.ResponseWriter, req *http.Request) bool {
	// Handle /auth requests
	path := req.URL.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if strings.HasPrefix(path, "/auth") {
		// make hit and return response body and cookie headers
		endpoint := fmt.Sprintf("%s%s", h.authHost, req.URL.Path)
		if req.URL.RawQuery != "" {
			endpoint += "?" + req.URL.RawQuery
		}

		h.reverseProxyToEndpoint(resp, req, endpoint)

		return true

	}

	// Handle public paths
	if h.publicExacts[path] {
		return false
	}
	for _, prefix := range h.publicPrefixes {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}

	// ensure all requests not going to /auth have a valid access token
	accessTokenCookie, err := req.Cookie("access_token")
	if err != nil || accessTokenCookie.Value == "" {
		if h.fixedPostAuthRedirect != "" {
			http.SetCookie(resp, &http.Cookie{
				Name:  "post_auth_redirect_uri",
				Value: h.fixedPostAuthRedirect,
				Path:  "/",
			})
		} else {
			http.SetCookie(resp, &http.Cookie{
				Name:  "post_auth_redirect_uri",
				Value: req.RequestURI,
				Path:  "/",
			})
		}

		http.Redirect(resp, req, "/auth/refresh", http.StatusTemporaryRedirect)
		return true
	} else {
		// validate access token
		err = h.validateJwt(accessTokenCookie.Value)
		if err != nil {
			if h.fixedPostAuthRedirect != "" {
				http.SetCookie(resp, &http.Cookie{
					Name:  "post_auth_redirect_uri",
					Value: h.fixedPostAuthRedirect,
					Path:  "/",
				})
			} else {
				http.SetCookie(resp, &http.Cookie{
					Name:  "post_auth_redirect_uri",
					Value: req.RequestURI,
					Path:  "/",
				})
			}
			http.Redirect(resp, req, "/auth/refresh", http.StatusTemporaryRedirect)
			return true
		}
	}

	// add access token as header
	req.Header.Del(iam.AuthHeader)
	req.Header.Add(iam.AuthHeader, "Bearer "+accessTokenCookie.Value)

	// return false if request was not handled
	return false
}

// Reverse proxies to /auth/denied which shows a message in the line of "Not found or you don't have access".
func (h *AuthProxy) HandleNotFoundOrAccessDenied(resp http.ResponseWriter, req *http.Request) {
	endpoint := fmt.Sprintf("%s/auth/denied", h.authHost)
	h.reverseProxyToEndpoint(resp, req, endpoint)
}

func (h *AuthProxy) reverseProxyToEndpoint(resp http.ResponseWriter, req *http.Request, endpoint string) {
	// Create a new request using http
	authReq, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// add header
	authReq.Header.Add(ForwardedHostHeader, req.Host)

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

	// copy headers but change Cookie to Set-Cookie
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
func (h *AuthProxy) validateJwt(accessToken string) error {
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
				// get keys from /auth/keys
				keysResp, err := http.Get(h.authHost + "/auth/keys")
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
// Use this at the very top of your unary and streaming interceptors in the context of a gRPC server
func ForwardAuthorizationHeader(ctx context.Context) (context.Context, error) {
	// forward authorization header as metadata in x-alis-forwarded-authorization
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		accessToken := md.Get(iam.AuthHeader)
		if len(accessToken) > 0 {
			ctx = metadata.AppendToOutgoingContext(ctx, iam.AlisForwardingHeader, accessToken[0])
		} else {
			return ctx, fmt.Errorf("authorization header not found")
		}
	} else {
		return ctx, fmt.Errorf("metadata not found")
	}
	return ctx, nil
}
