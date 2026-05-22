package authn

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	loginTransactionCookieName = "alis_authn_login"
	loginTransactionTTL        = 10 * time.Minute
)

type LoginTransaction struct {
	URL      string
	State    string
	Nonce    string
	ReturnTo string
}

type loginTransactionCookie struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	ReturnTo string `json:"return_to"`
	Expires  int64  `json:"expires"`
}

// GenerateState returns a URL-safe random value suitable for an OAuth/OIDC state parameter.
func GenerateState() (string, error) {
	return randomURLValue(32)
}

// GenerateNonce returns a URL-safe random value suitable for an OIDC nonce parameter.
func GenerateNonce() (string, error) {
	return randomURLValue(32)
}

// StartLogin creates a short-lived login transaction cookie and returns the authorization URL.
func (c *Client) StartLogin(w http.ResponseWriter, r *http.Request, redirectURI, returnTo string) (*LoginTransaction, error) {
	state, err := GenerateState()
	if err != nil {
		return nil, err
	}
	nonce, err := GenerateNonce()
	if err != nil {
		return nil, err
	}

	transaction := &loginTransactionCookie{
		State:    state,
		Nonce:    nonce,
		ReturnTo: returnTo,
		Expires:  time.Now().Add(loginTransactionTTL).Unix(),
	}
	if err := setLoginTransactionCookie(w, r, transaction); err != nil {
		return nil, err
	}

	return &LoginTransaction{
		URL:      c.AuthorizeURL(redirectURI, state, nonce),
		State:    state,
		Nonce:    nonce,
		ReturnTo: returnTo,
	}, nil
}

// CompleteLogin validates the login transaction, exchanges the authorization code.
// Does not validate the ID token nonce: use ValidateIDTokenNonce to validate the nonce.
func (c *Client) CompleteLogin(w http.ResponseWriter, r *http.Request, redirectURI string) (*Tokens, string, error) {
	if authErr := r.URL.Query().Get("error"); authErr != "" {
		return nil, "", fmt.Errorf("authorization failed: %s", authErr)
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		return nil, "", errors.New("missing code")
	}
	state := r.URL.Query().Get("state")
	if state == "" {
		return nil, "", errors.New("missing state")
	}

	transaction, err := readLoginTransactionCookie(r)
	if err != nil {
		return nil, "", err
	}
	clearLoginTransactionCookie(w, r)
	if time.Now().Unix() > transaction.Expires {
		return nil, "", errors.New("login transaction expired")
	}
	if subtle.ConstantTimeCompare([]byte(state), []byte(transaction.State)) != 1 {
		return nil, "", errors.New("state mismatch")
	}

	tokens, err := c.ExchangeCode(redirectURI, code)
	if err != nil {
		return nil, "", err
	}
	return tokens, transaction.ReturnTo, nil
}

func randomURLValue(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random value: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func setLoginTransactionCookie(w http.ResponseWriter, r *http.Request, transaction *loginTransactionCookie) error {
	b, err := json.Marshal(transaction)
	if err != nil {
		return fmt.Errorf("encoding login transaction: %w", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     loginTransactionCookieName,
		Value:    base64.RawURLEncoding.EncodeToString(b),
		Path:     "/",
		MaxAge:   int(loginTransactionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func readLoginTransactionCookie(r *http.Request) (*loginTransactionCookie, error) {
	cookie, err := r.Cookie(loginTransactionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, errors.New("missing login transaction")
	}
	b, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil, fmt.Errorf("decoding login transaction: %w", err)
	}
	transaction := &loginTransactionCookie{}
	if err := json.Unmarshal(b, transaction); err != nil {
		return nil, fmt.Errorf("decoding login transaction: %w", err)
	}
	return transaction, nil
}

func clearLoginTransactionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     loginTransactionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func secureCookie(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https"
}
