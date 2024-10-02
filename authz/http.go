package authz

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

	"github.com/golang-jwt/jwt"
	"google.golang.org/grpc/metadata"
)

type HttpAuthorizer struct {
	authHost   string
	publicKeys *sync.Map
}

// Creates a new HttpAuthorizer with the given authHost.
// Example authHost: "https://iam-auth-123456789.europe-west1.run.app".
func NewHttpAuthorizer(authHost string) *HttpAuthorizer {
	return &HttpAuthorizer{
		authHost:   strings.TrimSuffix(authHost, "/"),
		publicKeys: &sync.Map{},
	}
}

// Reverse proxies /auth/* requests to the authHost and validates the access_token cookie
// set by the authHost for all other requests.
// If the access token is valid, it also adds it as a header to the request.
//
// Returns true if the request was handled, in which case you should return from the handler.
func (h *HttpAuthorizer) HandleAuth(resp http.ResponseWriter, req *http.Request) bool {
	// Handle /auth requests
	if strings.HasPrefix(req.URL.Path, "/auth") {
		// make hit and return response body and cookie headers
		endpoint := fmt.Sprintf("%s%s", h.authHost, req.URL.Path)
		if req.URL.RawQuery != "" {
			endpoint += "?" + req.URL.RawQuery
		}

		// Create a new request using http
		authReq, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			http.Error(resp, err.Error(), http.StatusServiceUnavailable)
			return true
		}

		// add header
		authReq.Header.Add("x-forwarded-host", req.Host)

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
			return true
		}
		defer authResp.Body.Close()

		// // copy cookies
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
			return true
		}
		return true
	}

	// ensure all requests not going to /auth have a valid access token
	accessTokenCookie, err := req.Cookie("access_token")
	if err != nil || accessTokenCookie.Value == "" {
		http.Redirect(resp, req, "/auth/refresh", http.StatusTemporaryRedirect)
		return true
	} else {
		// validate access token
		err = h.validateJwt(accessTokenCookie.Value)
		if err != nil {
			http.Redirect(resp, req, "/auth/refresh", http.StatusTemporaryRedirect)
			return true
		}
	}

	// add access token as header
	req.Header.Add("Authorization", "Bearer "+accessTokenCookie.Value)

	// return false if request was not handled
	return false
}

// Returns an error if the token is invalid
func (h *HttpAuthorizer) validateJwt(accessToken string) error {
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

// Forwards the Authorization header in the incoming ctx to the outgoing ctx.
// Use this at the very top of your unary and streaming interceptors.
func (h *HttpAuthorizer) ForwardAuthorizationHeader(ctx context.Context) (context.Context, error) {
	// forward authorization header as metadata in x-alis-forwarded-authorization
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		accessToken := md.Get("authorization")
		if len(accessToken) > 0 {
			ctx = metadata.AppendToOutgoingContext(ctx, "x-alis-forwarded-authorization", accessToken[0])
		} else {
			return ctx, fmt.Errorf("authorization header not found")
		}
	} else {
		return ctx, fmt.Errorf("metadata not found")
	}
	return ctx, nil
}
