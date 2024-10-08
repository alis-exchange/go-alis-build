package jwt

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Payload represents a decoded payload of an ID Token.
type Payload struct {
	Issuer   string                 `json:"iss"`
	Audience string                 `json:"aud"`
	Expires  int64                  `json:"exp"`
	IssuedAt int64                  `json:"iat"`
	Subject  string                 `json:"sub,omitempty"`
	Email    string                 `json:"email"`
	Claims   map[string]interface{} `json:"-"`
}

// jwt represents the segments of a jwt and exposes convenience methods for
// working with the different segments.
type jwt struct {
	header    string
	payload   string
	signature string
}

// jwtHeader represents a parted jwt's header segment.
type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

// ParsePayload parses the given token and returns its payload.
//
// Warning: This function does not validate the token prior to parsing it.
//
// ParsePayload is primarily meant to be used to inspect a token's payload. This is
// useful when validation fails and the payload needs to be inspected.
func ParsePayload(idToken string) (*Payload, error) {
	jwt, err := parseJWT(idToken)
	if err != nil {
		return nil, err
	}
	return jwt.parsedPayload()
}

func parseJWT(idToken string) (*jwt, error) {
	segments := strings.Split(idToken, ".")
	if len(segments) != 3 {
		return nil, fmt.Errorf("jwt: invalid token, token must have three segments, found %d", len(segments))
	}
	return &jwt{
		header:    segments[0],
		payload:   segments[1],
		signature: segments[2],
	}, nil
}

// decodedHeader base64 decodes the header segment.
func (j *jwt) decodedHeader() ([]byte, error) {
	dh, err := decode(j.header)
	if err != nil {
		return nil, fmt.Errorf("jwt: unable to decode JWT header: %w", err)
	}
	return dh, nil
}

// decodedPayload base64 payload the header segment.
func (j *jwt) decodedPayload() ([]byte, error) {
	p, err := decode(j.payload)
	if err != nil {
		return nil, fmt.Errorf("jwt: unable to decode JWT payload: %w", err)
	}
	return p, nil
}

// decodedPayload base64 payload the header segment.
func (j *jwt) decodedSignature() ([]byte, error) {
	p, err := decode(j.signature)
	if err != nil {
		return nil, fmt.Errorf("jwt: unable to decode JWT signature: %w", err)
	}
	return p, nil
}

// parsedHeader returns a struct representing a JWT header.
func (j *jwt) parsedHeader() (jwtHeader, error) {
	var h jwtHeader
	dh, err := j.decodedHeader()
	if err != nil {
		return h, err
	}
	err = json.Unmarshal(dh, &h)
	if err != nil {
		return h, fmt.Errorf("jwt: unable to unmarshal JWT header: %w", err)
	}
	return h, nil
}

// parsedPayload returns a struct representing a JWT payload.
func (j *jwt) parsedPayload() (*Payload, error) {
	var p Payload
	dp, err := j.decodedPayload()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(dp, &p); err != nil {
		return nil, fmt.Errorf("jwt: unable to unmarshal JWT payload: %w", err)
	}
	if err := json.Unmarshal(dp, &p.Claims); err != nil {
		return nil, fmt.Errorf("jwt: unable to unmarshal JWT payload claims: %w", err)
	}
	return &p, nil
}

// hashedContent gets the SHA256 checksum for verification of the JWT.
func (j *jwt) hashedContent() []byte {
	signedContent := j.header + "." + j.payload
	hashed := sha256.Sum256([]byte(signedContent))
	return hashed[:]
}

func (j *jwt) String() string {
	return fmt.Sprintf("%s.%s.%s", j.header, j.payload, j.signature)
}

func decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// ValidateRegex validates an argument and returns an error if not valid
func ValidateRegex(name string, value string, regex string) error {
	if !regexp.MustCompile(regex).MatchString(value) {
		return fmt.Errorf("%s (%s) is not of the right format: %s", name, value, regex)
	}
	return nil
}
