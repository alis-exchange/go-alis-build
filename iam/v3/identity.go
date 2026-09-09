// Package iam provides an identity which is shared by the authn and authz packages.
package iam

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
		Type                Type                // Type of the identity
		ID                  string              `json:"sub"`      // E.g. "1934872948" or "alis-build@my-project.iam.gserviceaccount.com"
		Email               string              `json:"email"`    // E.g. "john@example.com" or "alis-build@myproject.iam.gserviceaccount.com"
		Accounts            map[string]*Account `json:"accounts"` // User's seats in their accounts
		GroupIDs            []string            `json:"groups"`   // IDs of the groups the user belongs to
		Policy              string              `json:"policy"`   // Base64 encoded iam policy
		Exp                 int64               `json:"exp"`      // Expiration time in seconds since epoch. Only used for validating tokens.
		App                 string              `json:"app"`      // Client ID (if any) of the registered third party app.
		Scopes              []string            `json:"scopes"`   // Set of scopes that the third party app has been granted.
		ActiveIdeateAccount *IdeateAccount      `json:"active_ideate_account"`
		ActiveBuildAccount  *BuildAccount       `json:"active_build_account"`
		// AuthzRoles is the set of roles a scoped credential, such as a personal
		// API key, is limited to. The library only carries it. Only the service
		// that issued the credential may interpret or enforce it.
		//
		// The values are that issuer's own role vocabulary and nothing more.
		// Role ids are unnamespaced strings registered into a process-local map
		// by [go.alis.build/iam/v3/authz.AddRolePermissions], so the same string
		// routinely means different permissions in different services, and no
		// two services need agree. A service that trims its own roles against an
		// allowlist minted elsewhere is comparing strings that were never in the
		// same vocabulary: it denies most of what it should allow and grants
		// whatever happens to collide.
		//
		// "roles/open" is the near-certain collision, because it is the
		// conventional name for the any-signed-in-user role and so nearly every
		// service defines one. Real definitions in production today range from
		// reading your own profile, to deleting your own account, to creating
		// and batch-deleting another domain's resources. An allowlist naming it
		// means something different in every process that reads it.
		//
		// So carriage is all this field can safely be. Treat an allowlist minted
		// by another service as opaque: forward it untouched, or ignore it. If a
		// credential must be scoped across a service boundary, that has to be
		// expressed in something both ends define, such as a policy, not in
		// these ids.
		//
		// nil (claim absent or null) means unrestricted. A non-nil empty slice
		// (the claim present as []) means restricted to no roles at all, which is
		// the most restricted credential there is. Never normalise empty to nil
		// and never add omitempty: collapsing the two turns the most restricted
		// credential into the least restricted one.
		AuthzRoles []string `json:"authz_roles"`
		// Restricted marks a credential that may never exceed the roles it
		// explicitly carries: the system and admin bypass in IsPrivileged and the
		// open role bypass in the authz package do not apply to it. Absent means
		// false, so existing callers see no change.
		//
		// IsSystem and IsAdmin still report who the principal is; only
		// IsPrivileged is affected. This is also distinct from any issuer claim
		// describing how the holder authenticated, such as cred: authorization
		// must key off Restricted, not off the credential kind.
		//
		// Restriction is fail-open across versions. A consumer older than v3.10.0
		// ignores this claim and grants the credential its principal's full
		// authority, so issuers have to keep their own guards until every
		// consumer has upgraded.
		Restricted bool `json:"restricted"`
	}
	IdeateAccount struct {
		AccountID                string  `json:"account_id"`
		IdeateAccountCreditLimit float64 `json:"ideate_account_credit_limit"`
		IdeateUserCreditLimit    float64 `json:"ideate_user_credit_limit"`
	}
	BuildAccount struct {
		AccountID string `json:"account_id"`
	}
	Type string
	Seat struct {
		Plan int32 `json:"plan"`
		Seat int32 `json:"seat"`
	}
	Account struct {
		Seats                    map[int32]*Seat `json:"seats"`
		IdeateAccountCreditLimit float64         `json:"ideate_account_credit_limit"`
		IdeateUserCreditLimit    float64         `json:"ideate_user_credit_limit"`
	}
	ctxKey string
)

// PolicyMember returns the member to use in iam policy bindings.
// E.g. "user:1234129384" or "serviceAccount:alis-build@myproject.iam.gserviceaccount.com"
func (i *Identity) PolicyMember() string {
	switch i.Type {
	case System:
		return "system"
	case ServiceAccount:
		return string(i.Type) + ":" + i.Email
	}
	return string(i.Type) + ":" + i.ID
}

// User returns the User resource name for identities that have one.
// E.g. "users/1234129384"
func (i *Identity) User() string {
	if i.Type == User {
		return "users/" + i.ID
	}
	return ""
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

// MustFromContext returns the Identity stored in ctx.
//
// It panics if ctx does not contain an Identity value.
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

// MustUnmarshal parses data as an Identity.
//
// It panics if data cannot be unmarshalled as an Identity.
func MustUnmarshal(data []byte) *Identity {
	identity, err := Unmarshal(data)
	if err != nil {
		panic(fmt.Sprintf("identity.MustUnmarshal: %v", err))
	}
	return identity
}

// LegacyOutgoingMetadata returns a derived context with the identity value in it.
// Enables downstream gRPC services in the same environment to identify the requester.
// Does the same as OutgoingMetadata but also adds "x-alis-forwarded-authorization" with a
// derived jwt token.
func (i *Identity) LegacyOutgoingMetadata(ctx context.Context) context.Context {
	ctx = i.OutgoingMetadata(ctx)
	ctx = metadata.AppendToOutgoingContext(ctx, "x-alis-forwarded-authorization", i.UnsignedJWT(ctx))
	return ctx
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

// MustFromIncomingMetadata returns the Identity stored in incoming gRPC metadata.
//
// It panics if the metadata does not contain an Identity value or the value
// cannot be unmarshalled as an Identity.
func MustFromIncomingMetadata(ctx context.Context) *Identity {
	identity, err := FromIncomingMetadata(ctx)
	if err != nil {
		panic(fmt.Sprintf("identity.MustFromIncomingMetadata: %v", err))
	}
	return identity
}

func (i *Identity) UnsignedJWT(ctx context.Context) string {
	jsonIdentity, err := json.Marshal(i)
	if err != nil {
		panic(err) // impossible
	}
	identityBase64 := base64.RawURLEncoding.EncodeToString(jsonIdentity)
	headJSON := []byte(`{"alg":"none"}`)
	headB64 := base64.RawURLEncoding.EncodeToString(headJSON)
	return fmt.Sprintf("%s.%s.###", headB64, identityBase64)
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

// MustFromJWT decodes jwt and returns the Identity represented by its payload.
//
// It panics if jwt is malformed, its payload cannot be decoded, or the payload
// cannot be unmarshalled as an Identity.
func MustFromJWT(jwt string) *Identity {
	identity, err := FromJWT(jwt)
	if err != nil {
		panic(fmt.Sprintf("identity.MustFromJWT: %v", err))
	}
	return identity
}
