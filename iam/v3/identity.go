package iam

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	internaljwt "go.alis.build/iam/v3/internal/jwt"
	"google.golang.org/protobuf/proto"
)

const (
	AuthHeader                = "authorization"
	AlisUserEmailHeader       = "x-alis-user-email"
	AlisUserIDHeader          = "x-alis-user-id"
	AlisIdentityContextHeader = "x-alis-identity-context"
	groupTypeUser             = "user"
	groupTypeServiceAccount   = "serviceAccount"
	groupTypeDomain           = "domain"
	groupTypeGroup            = "group"
)

type Identity struct {
	token                      string
	id                         string
	email                      string
	isDeploymentServiceAccount bool
	policy                     *iampb.Policy
	groupIDs                   []string
	accounts                   map[string]*internaljwt.Account
}

// RequestIdentity describes both the authenticated transport principal and the
// effective caller on whose behalf authorization should run.
type RequestIdentity struct {
	Caller        *Identity
	Authenticated *Identity
}

// IdentityOption configures an Identity created with NewIdentity.
type IdentityOption func(*Identity)

type assertedIdentityContext struct {
	Policy string `json:"policy,omitempty"`
}

// WithIdentityPolicy attaches an IAM policy to an Identity created via
// NewIdentity.
func WithIdentityPolicy(policy *iampb.Policy) IdentityOption {
	return func(identity *Identity) {
		identity.policy = policy
	}
}

// NewIdentity constructs an explicit identity for trusted internal use, such as
// when a gateway has already authenticated a user and needs to assert that
// identity to downstream services.
func NewIdentity(id string, email string, opts ...IdentityOption) *Identity {
	identity := &Identity{
		id:    id,
		email: email,
	}
	for _, opt := range opts {
		opt(identity)
	}
	return identity
}

// WithRequestIdentity attaches a resolved RequestIdentity to ctx.
func WithRequestIdentity(ctx context.Context, identity *RequestIdentity) context.Context {
	return context.WithValue(ctx, requestIdentityKey, identity)
}

// RequestIdentityFromContext returns a RequestIdentity previously attached to
// ctx.
func RequestIdentityFromContext(ctx context.Context) (*RequestIdentity, bool) {
	identity, ok := ctx.Value(requestIdentityKey).(*RequestIdentity)
	return identity, ok
}

// ID returns the caller's stable user or service-account identifier.
func (i *Identity) ID() string {
	return i.id
}

// Email returns the caller's email address.
func (i *Identity) Email() string {
	return i.email
}

// Token returns the original bearer token when the identity came directly from
// an authenticated request.
func (i *Identity) Token() string {
	return i.token
}

// Policy returns the policy embedded on the identity, if any.
func (i *Identity) Policy() *iampb.Policy {
	return i.policy
}

// GroupIDs returns the group IDs present on the direct caller token, if any.
func (i *Identity) GroupIDs() []string {
	return append([]string(nil), i.groupIDs...)
}

// Accounts returns the account context present on the direct caller token, if
// any.
func (i *Identity) Accounts() map[string]*internaljwt.Account {
	if i.accounts == nil {
		i.accounts = map[string]*internaljwt.Account{}
	}
	return i.accounts
}

// IsDeploymentServiceAccount reports whether the identity is the configured
// deployment service account.
func (i *Identity) IsDeploymentServiceAccount() bool {
	return i.isDeploymentServiceAccount
}

// IsServiceAccount reports whether the identity email represents a service
// account.
func (i *Identity) IsServiceAccount() bool {
	return strings.HasSuffix(i.email, ".gserviceaccount.com")
}

// IsGoogleIdentity reports whether the identity subject looks like a Google
// user ID.
func (i *Identity) IsGoogleIdentity() bool {
	if i.id == "" {
		return false
	}
	return '0' <= i.id[0] && i.id[0] <= '9'
}

// PolicyMember returns the IAM policy member string for the identity.
func (i *Identity) PolicyMember() string {
	if i.IsServiceAccount() {
		return "serviceAccount:" + i.email
	}
	return "user:" + i.id
}

// UserName returns the canonical IAM user resource name for the identity.
func (i *Identity) UserName() string {
	return "users/" + i.id
}

// AssertedHeaders serializes the identity into the trusted internal headers
// used by v3 for caller assertion across service boundaries.
func (i *Identity) AssertedHeaders() (http.Header, error) {
	if i == nil {
		return nil, fmt.Errorf("identity is required")
	}
	if i.id == "" || i.email == "" {
		return nil, fmt.Errorf("identity id and email are required")
	}

	headers := http.Header{}
	headers.Set(AlisUserIDHeader, i.ID())
	headers.Set(AlisUserEmailHeader, i.Email())

	payload := &assertedIdentityContext{}
	if policy := i.Policy(); policy != nil {
		policyBytes, err := proto.Marshal(policy)
		if err != nil {
			return nil, fmt.Errorf("marshal identity policy: %w", err)
		}
		payload.Policy = base64.StdEncoding.EncodeToString(policyBytes)
	}
	if payload.Policy != "" {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal asserted identity context: %w", err)
		}
		headers.Set(AlisIdentityContextHeader, base64.StdEncoding.EncodeToString(payloadBytes))
	}

	return headers, nil
}

// StripAssertedIdentityHeaders removes all trusted internal caller assertion
// headers so an edge service can safely overwrite them before proxying
// downstream.
func StripAssertedIdentityHeaders(headers http.Header) {
	headers.Del(AlisUserIDHeader)
	headers.Del(AlisUserEmailHeader)
	headers.Del(AlisIdentityContextHeader)
}

// ExtractRequestIdentity resolves the authenticated principal from the request
// and, when trusted asserted headers are present, the effective caller.
func (iam *IAM) ExtractRequestIdentity(ctx context.Context) (*RequestIdentity, error) {
	if existing, ok := RequestIdentityFromContext(ctx); ok && existing != nil {
		return existing, nil
	}

	ctx = ContextWithGRPCMetadata(ctx)
	md, ok := RequestMetadataFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("request metadata not found in context")
	}

	authenticated, err := parseIdentityFromHeaders(md.Headers, AuthHeader, iam.deploymentServiceAccountEmail)
	if err != nil {
		return nil, fmt.Errorf("extract authenticated identity: %w", err)
	}

	caller := authenticated
	if iam.superAdmins[authenticated.PolicyMember()] {
		asserted, assertedErr := parseAssertedIdentityFromHeaders(md.Headers, iam.deploymentServiceAccountEmail)
		if assertedErr == nil {
			caller = asserted
		}
	}

	requestIdentity := &RequestIdentity{
		Caller:        caller,
		Authenticated: authenticated,
	}
	return requestIdentity, nil
}

func parseIdentityFromHeaders(headers map[string][]string, header string, deploymentServiceAccountEmail string) (*Identity, error) {
	values := headers[strings.ToLower(header)]
	if len(values) == 0 {
		return nil, fmt.Errorf("header %q not found", header)
	}

	token := strings.TrimSpace(values[0])
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimPrefix(token, "bearer ")
	if token == "" {
		return nil, fmt.Errorf("header %q does not contain a token", header)
	}

	payload, err := internaljwt.ParsePayload(token)
	if err != nil {
		return nil, fmt.Errorf("parse jwt payload from %q: %w", header, err)
	}

	identity := &Identity{
		token:                      token,
		id:                         payload.Subject,
		email:                      payload.Email,
		groupIDs:                   append([]string(nil), payload.Groups...),
		accounts:                   payload.Accounts,
		isDeploymentServiceAccount: payload.Email == deploymentServiceAccountEmail,
	}

	if !identity.IsServiceAccount() {
		policyString, ok := payload.Claims["policy"].(string)
		if ok && payload.Issuer == "alis.build" {
			policyBytes, err := base64.StdEncoding.DecodeString(policyString)
			if err != nil {
				return nil, fmt.Errorf("decode policy claim: %w", err)
			}
			if len(policyBytes) > 0 {
				policy := &iampb.Policy{}
				if err := proto.Unmarshal(policyBytes, policy); err != nil {
					return nil, fmt.Errorf("unmarshal policy claim: %w", err)
				}
				identity.policy = policy
			}
		}
	}

	return identity, nil
}

func parseAssertedIdentityFromHeaders(headers map[string][]string, deploymentServiceAccountEmail string) (*Identity, error) {
	id := firstHeaderValue(headers, AlisUserIDHeader)
	email := firstHeaderValue(headers, AlisUserEmailHeader)
	if id == "" || email == "" {
		return nil, fmt.Errorf("asserted identity headers not found")
	}

	identity := &Identity{
		id:                         id,
		email:                      email,
		isDeploymentServiceAccount: email == deploymentServiceAccountEmail,
	}

	if contextValue := firstHeaderValue(headers, AlisIdentityContextHeader); contextValue != "" {
		payloadBytes, err := base64.StdEncoding.DecodeString(contextValue)
		if err != nil {
			return nil, fmt.Errorf("decode asserted identity context: %w", err)
		}

		payload := &assertedIdentityContext{}
		if err := json.Unmarshal(payloadBytes, payload); err != nil {
			return nil, fmt.Errorf("unmarshal asserted identity context: %w", err)
		}

		if payload.Policy != "" {
			policyBytes, err := base64.StdEncoding.DecodeString(payload.Policy)
			if err != nil {
				return nil, fmt.Errorf("decode asserted policy: %w", err)
			}
			if len(policyBytes) > 0 {
				policy := &iampb.Policy{}
				if err := proto.Unmarshal(policyBytes, policy); err != nil {
					return nil, fmt.Errorf("unmarshal asserted policy: %w", err)
				}
				identity.policy = policy
			}
		}
	}

	return identity, nil
}

func firstHeaderValue(headers map[string][]string, header string) string {
	values := headers[strings.ToLower(header)]
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}
