// Package authn is used to identify requesters
package authn

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	AuthURL     string
	TokenURL    string
	JWKSURL     string
	ID          string
	Secret      string
	CallbackURL string

	// Set to true if you store the tokens in your database for connections to OTHER services.
	// This will skip fetching the public keys from the JWKSURL, speeding up the authentication process.
	// Do NOT set to true if the tokens are provided by the client in order to access YOUR service.
	SkipSignatureValidation bool
	keys                    sync.Map
}

func (c *Client) AuthorizeURL(state string) string {
	url := fmt.Sprintf("%s?redirect_uri=%s&state=%s", c.AuthURL, c.CallbackURL, state)
	if c.ID != "" {
		url += "&client_id=" + c.ID
	}
	return url
}

// ExchangeCode exchanges an authorization code for access and refresh tokens
func (c *Client) ExchangeCode(code string) (*Tokens, error) {
	return c.postToken(authorizationCode, code)
}

func (c *Client) Refresh(tokens *Tokens) error {
	newTokens, err := c.postToken(refreshToken, tokens.RefreshToken)
	if err != nil {
		return err
	}
	tokens.AccessToken = newTokens.AccessToken
	tokens.RefreshToken = newTokens.RefreshToken
	return nil
}

type grantType string

const (
	refreshToken      grantType = "refresh_token"
	authorizationCode grantType = "authorization_code"
)

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (c *Client) postToken(grantType grantType, grant string) (*Tokens, error) {
	// build request body
	type bodyType struct {
		GrantType    string `json:"grant_type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Code         string `json:"code,omitempty"`
		RedirectURI  string `json:"redirect_uri"`
		RefreshToken string `json:"refresh_token,omitempty"`
	}
	body := &bodyType{
		GrantType:    string(grantType),
		ClientID:     c.ID,
		ClientSecret: c.Secret,
	}
	switch grantType {
	case refreshToken:
		body.RefreshToken = grant
	case authorizationCode:
		body.Code = grant
		body.RedirectURI = c.CallbackURL
	}
	bytesBuffer := bytes.NewBuffer(nil)
	jsonEncoder := json.NewEncoder(bytesBuffer)
	if err := jsonEncoder.Encode(body); err != nil {
		return nil, fmt.Errorf("encoding body: %w", err)
	}

	// make request
	req, err := http.NewRequest(http.MethodPost, c.TokenURL, bytesBuffer)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	// handle error response
	if resp.StatusCode != 200 {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}
		return nil, fmt.Errorf("%d: %s", resp.StatusCode, bodyBytes)
	}

	// handle success response
	defer resp.Body.Close()
	tokens := &Tokens{}
	if err := json.NewDecoder(resp.Body).Decode(tokens); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return tokens, nil
}

// Authenticate refreshes the user's access token if its invalid/expired and returns true if it was refreshed.
func (c *Client) Authenticate(tokens *Tokens, now time.Time) (bool, error) {
	if err := c.ValidateToken(tokens.AccessToken, now); err != nil {
		if err := c.Refresh(tokens); err != nil {
			return false, err
		}
		return true, nil
	}

	// no refresh needed
	return false, nil
}

// ValidateToken validates the token and returns an error if it is invalid.
func (c *Client) ValidateToken(token string, now time.Time) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("invalid token format, expect {hdr}.{body}.{sig}")
	}

	if !c.SkipSignatureValidation {
		if err := c.validateSignature(parts[0], parts[1], parts[2], now); err != nil {
			return err
		}
	}

	// decode payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("failed to decode payload: %w", err)
	}

	// unmarshal payload into Jwt struct
	type Jwt struct {
		Exp int64 `json:"exp"`
	}
	var jwt Jwt
	if err := json.Unmarshal(payload, &jwt); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// ensure payload has not expired
	if jwt.Exp < now.Unix() {
		return errors.New("token has expired")
	}

	return nil
}

func (c *Client) validateSignature(hdr, body, sig string, now time.Time) error {
	// decode base64 header
	header, err := base64.RawURLEncoding.DecodeString(hdr)
	if err != nil {
		return fmt.Errorf("failed to decode header: %w", err)
	}
	var headerMap map[string]any
	if err := json.Unmarshal(header, &headerMap); err != nil {
		return fmt.Errorf("failed to unmarshal header: %w", err)
	}

	// extract key id from decoded header
	kid, ok := headerMap["kid"].(string)
	if !ok {
		return errors.New("missing kid in header")
	}

	// load key
	key, ok := c.keys.Load(kid)
	if !ok {
		c.SyncPublicKeys(now) // sync incase missing
		if key, ok = c.keys.Load(kid); !ok {
			return fmt.Errorf("key not found for kid: %s", kid)
		}
	}

	// decode jwt signature
	signature, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// validate signature
	hashed := sha256.Sum256([]byte(hdr + "." + body))
	if err := rsa.VerifyPKCS1v15(key.(*rsa.PublicKey), crypto.SHA256, hashed[:], signature); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	return nil
}

// SyncPublicKeys syncs the keys if one/more is missing.
func (c *Client) SyncPublicKeys(now time.Time) {
	// skip if all keys already loaded
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	var requireSync bool
	if _, ok := c.keys.Load(today); !ok {
		requireSync = true
	}
	if _, ok := c.keys.Load(yesterday); !ok {
		requireSync = true
	}
	if !requireSync {
		return
	}

	// fetch keys from endpoint
	resp, err := http.Get(c.JWKSURL)
	if err != nil {
		log.Printf("failed to get public keys: %v", err)
		return
	}
	defer resp.Body.Close()

	type JWKS struct {
		Keys []struct {
			Kid string `json:"kid"`
			Mod string `json:"mod"`
			Exp string `json:"exp"`
		} `json:"keys"`
	}

	jwks := &JWKS{}
	if err := json.NewDecoder(resp.Body).Decode(jwks); err != nil {
		log.Printf("failed to decode public keys: %v", err)
		return
	}

	for _, key := range jwks.Keys {
		n, err := base64.RawURLEncoding.DecodeString(key.Mod)
		if err != nil {
			log.Printf("failed to decode modulus: %v", err)
			continue
		}
		e, err := base64.RawURLEncoding.DecodeString(key.Exp)
		if err != nil {
			log.Printf("failed to decode exponent: %v", err)
			continue
		}
		c.keys.Store(key.Kid, &rsa.PublicKey{
			N: new(big.Int).SetBytes(n),
			E: int(new(big.Int).SetBytes(e).Int64()),
		})
	}
}
