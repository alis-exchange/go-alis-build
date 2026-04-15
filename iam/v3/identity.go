// Package auth provides an identity which is shared by the authn and authz packages.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/metadata"
)

const (
	User           Type   = "user"
	ServiceAccount Type   = "serviceAccount"
	System         Type   = "system" // can do everything
	identityCtxKey ctxKey = "x-alis-identity"
)

type (
	Identity struct {
		Type     Type              // Type of the identity
		ID       string            `json:"sub"`      // E.g. "1934872948" or "alis-build@my-project.iam.gserviceaccount.com"
		Email    string            `json:"email"`    // E.g. "john@example.com" or "alis-build@myproject.iam.gserviceaccount.com"
		Accounts map[string]*Seats `json:"accounts"` // User's seats in their accounts
		GroupIDs []string          `json:"groups"`   // IDs of the groups the user belongs to
		Policy   string            `json:"policy"`   // Base64 encoded iam policy
		Exp      int64             `json:"exp"`      // Expiration time in seconds since epoch. Only used for validating tokens.
		App      string            `json:"app"`      // Client ID (if any) of the registered third party app.
		Scopes   []string          `json:"scopes"`   // Set of scopes that the third party app has been granted.
	}
	Type string
	Seat struct {
		Plan int32
		Seat int32
	}
	Seats  map[string]*Seat
	ctxKey string
)

// PolicyMember returns the member to use in iam policy bindings.
// E.g. "user:1234129384" or "serviceAccount:alis-build@myproject.iam.gserviceaccount.com"
func (i *Identity) PolicyMember() string {
	if i.Type == ServiceAccount {
		return string(i.Type) + ":" + i.Email
	}
	return string(i.Type) + ":" + i.ID
}

// Context returns a derived context with the identity value in it to use locally.
// Use OutgoingMetadata if you want remote services to identify the requester.
// You can use Context and OutgoingMetadata together.
func (i *Identity) Context(ctx context.Context) context.Context {
	return context.WithValue(ctx, identityCtxKey, i)
}

// FromContext returns the Identity inside the given ctx, if any.
func FromContext(ctx context.Context) (*Identity, error) {
	ctxValue := ctx.Value(identityCtxKey)
	if ctxValue == nil {
		return nil, errors.New("no Identity found in ctx")
	}
	identity, ok := ctxValue.(*Identity)
	if !ok || identity == nil {
		return nil, errors.New("no Identity found in ctx")
	}
	identity.checkIfSystem()
	return identity, nil
}

// MustFromContext does the same as FromContext, but panics on an error.
func MustFromContext(ctx context.Context) *Identity {
	identity, err := FromContext(ctx)
	if err != nil {
		panic(fmt.Sprintf("identity.MustFromContext: %v", err))
	}
	return identity
}

// Marshal returns the bytes representation of the identity.
func (i *Identity) Marshal() []byte {
	if i == nil {
		return nil
	}
	data, err := json.Marshal(i)
	if err != nil {
		panic(err) // impossible
	}
	return data
}

// Unmarshal returns the identity represented by the bytes.
func Unmarshal(data []byte) (*Identity, error) {
	var identity Identity
	if err := json.Unmarshal(data, &identity); err != nil {
		return nil, err
	}
	identity.checkIfSystem()
	return &identity, nil
}

// MustUnmarshal does the same as [Unmarshal], but panics on an error.
func MustUnmarshal(data []byte) *Identity {
	identity, err := Unmarshal(data)
	if err != nil {
		panic(fmt.Sprintf("identity.MustUnmarshal: %v", err))
	}
	return identity
}

// OutgoingMetadata returns a derived context with the identity value in it.
// Enables downstream gRPC services in the same environment to identify the requester.
func (i *Identity) OutgoingMetadata(ctx context.Context) context.Context {
	if i == nil {
		return ctx
	}
	value := string(i.Marshal())
	return metadata.AppendToOutgoingContext(ctx, string(identityCtxKey), value)
}

// FromIncomingMetadata returns the Identity inside the given gRPC context, if any.
func FromIncomingMetadata(ctx context.Context) (*Identity, error) {
	values := metadata.ValueFromIncomingContext(ctx, string(identityCtxKey))
	if len(values) == 0 {
		return nil, errors.New("no identity value found in incoming metadata")
	}
	data := []byte(values[len(values)-1]) // use last appended value
	identity, err := Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling incoming metadata: %v", err)
	}
	identity.checkIfSystem()
	return identity, nil
}

// MustFromIncomingMetadata does the same as FromIncomingMetadata, but panics on an error.
func MustFromIncomingMetadata(ctx context.Context) *Identity {
	identity, err := FromIncomingMetadata(ctx)
	if err != nil {
		panic(fmt.Sprintf("identity.MustFromIncomingMetadata: %v", err))
	}
	return identity
}

// FromJWT decodes and unmarshals the given jwt into an Identity.
func FromJWT(jwt string) (*Identity, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format, expect {hdr}.{body}.{sig}")
	}

	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	var identity Identity
	if err := json.Unmarshal(body, &identity); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	if strings.HasSuffix(identity.Email, ".iam.gserviceaccount.com") {
		identity.Type = ServiceAccount
	} else {
		identity.Type = User
	}

	identity.checkIfSystem()
	return &identity, nil
}

// MustFromJWT does the same as FromJWT, but panics on an error.
func MustFromJWT(jwt string) *Identity {
	identity, err := FromJWT(jwt)
	if err != nil {
		panic(fmt.Sprintf("identity.MustFromJWT: %v", err))
	}
	return identity
}
