package jwt

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Payload represents the decoded payload of a JWT.
type Payload struct {
	// Issuer identifies the principal that issued the token.
	Issuer string `json:"iss"`
	// Audience identifies the intended recipient of the token.
	Audience string `json:"aud"`
	// Expires is the token expiration time as a Unix timestamp.
	Expires int64 `json:"exp"`
	// IssuedAt is the token issuance time as a Unix timestamp.
	IssuedAt int64 `json:"iat"`
	// Subject is the stable user or service account identifier for the token
	// subject.
	Subject string `json:"sub,omitempty"`
	// Email is the email address associated with the token subject.
	Email string `json:"email"`
	// Groups lists group identifiers asserted on the token.
	Groups []string `json:"groups"`
	// Claims contains the full decoded claim set, including claims not modeled
	// by the typed fields above.
	Claims map[string]interface{} `json:"-"`
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
