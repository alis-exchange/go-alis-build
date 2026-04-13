package jwt

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type Seat struct {
	Plan int32 `json:"plan"`
	Seat int32 `json:"seat"`
}

type Account struct {
	Seats map[int32]*Seat `json:"seats"`
}

// Payload represents the decoded payload of a JWT.
type Payload struct {
	Issuer   string                 `json:"iss"`
	Audience string                 `json:"aud"`
	Expires  int64                  `json:"exp"`
	IssuedAt int64                  `json:"iat"`
	Subject  string                 `json:"sub,omitempty"`
	Email    string                 `json:"email"`
	Groups   []string               `json:"groups"`
	Claims   map[string]interface{} `json:"-"`
	Accounts map[string]*Account    `json:"accounts"`
}

func ParsePayload(token string) (*Payload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("jwt: token must have three segments, found %d", len(parts))
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode payload: %w", err)
	}

	payload := &Payload{}
	if err := json.Unmarshal(payloadBytes, payload); err != nil {
		return nil, fmt.Errorf("jwt: unmarshal payload: %w", err)
	}
	if err := json.Unmarshal(payloadBytes, &payload.Claims); err != nil {
		return nil, fmt.Errorf("jwt: unmarshal claims: %w", err)
	}

	return payload, nil
}
