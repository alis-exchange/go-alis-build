package authz

import (
	"context"
	"encoding/base64"
	"slices"
	"strings"
	"sync"

	"cloud.google.com/go/iam/apiv1/iampb"
	"go.alis.build/alog"
	"go.alis.build/authz/internal/jwt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	openIam "open.alis.services/protobuf/alis/open/iam/v1"
)

const (
	// One of the headers that cloudrun uses to send the JWT token of the authorized requester
	AuthHeader = "authorization"
	// One of the headers that cloudrun uses to send the JWT token of the authorized requester
	ServerlessAuthHeader = "x-serverless-authorization"
	// The header that this package uses to forward the JWT token of the authorized requester
	AuthzForwardingHeader = "x-alis-forwarded-authorization"
	// The header that Google Cloud ESPv2 proxy uses to forward the JWT token of the authorized requester
	ProxyForwardingHeader = "x-forwarded-authorization"
	// The header that Google Cloud IAP uses to forward the JWT token of the authorized requester
	IAPJWTAssertionHeader = "x-goog-iap-jwt-assertion"
)

type Requester struct {
	// The original jwt token
	jwt string
	// The requester id e.g. 123456789, m-0000-0000-9245-2134
	id string
	// The requester email e.g. john@gmail.com or alis-build@...gserviceaccount.com
	email string

	// Whether the requester is a super admin
	isSuperAdmin bool

	// The iam policy on the user resource of the requester.
	// Not applicable for service accounts
	policy *iampb.Policy

	// The authorizer that is used to authorize the requester
	az *Authorizer

	// A cache to store the result of the member resolver function
	memberCache *sync.Map
}

// Returns the requester making the request or for whom the request is being forwarded by a super admin.
func newRequesterFromCtx(ctx context.Context, deploymentServiceAccountEmail string) *Requester {
	// Looks in the specified header for a JWT token and extracts the requester from it.
	// Returns nil if no JWT token was found in the header.
	// Returns an error if the JWT token is invalid.
	getRequesterFromJwtHeader := func(ctx context.Context, header string, deploymentServiceAccountEmail string) (*Requester, error) {
		requester := &Requester{
			memberCache: &sync.Map{},
		}

		// Retrieve the metadata from the context.
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, nil
		}

		if len(md.Get(header)) > 0 {
			// Get token from header
			token := strings.TrimPrefix(md.Get(header)[0], "Bearer ")
			token = strings.TrimPrefix(token, "bearer ")

			// Extract token payload
			payload, err := jwt.ParsePayload(token)
			if err != nil {
				return nil, status.Errorf(codes.Unauthenticated, "%s", err)
			}

			requester.jwt = token
			requester.id = payload.Subject
			requester.email = payload.Email

			if !requester.IsServiceAccount() {
				// policy is base64 encoded version of the bytes of the policy
				policyString, ok := payload.Claims["iam_policy"].(string)
				if ok && payload.Issuer == "alis.build" {
					policyBytes, err := base64.StdEncoding.DecodeString(policyString)
					if err != nil {
						return nil, status.Errorf(codes.Unauthenticated, "unable to decode jwt iam policy: %s", err)
					}
					if len(policyBytes) > 0 {
						policy := &iampb.Policy{}
						err = proto.Unmarshal(policyBytes, policy)
						if err != nil {
							return nil, status.Errorf(codes.Unauthenticated, "unable to unmarshal jwt iam policy: %s", err)
						}
					}
				}
			}
			requester.isSuperAdmin = requester.email == deploymentServiceAccountEmail

			return requester, nil

		} else {
			return nil, nil
		}
	}

	// first get the principal in one of the non-forwarded auth headers
	principal, err := getRequesterFromJwtHeader(ctx, AuthHeader, deploymentServiceAccountEmail)
	if principal == nil && err == nil {
		principal, err = getRequesterFromJwtHeader(ctx, ServerlessAuthHeader, deploymentServiceAccountEmail)
	}
	if err != nil {
		alog.Alertf(ctx, "unable to retrieve metadata from the request header: %s", err)
		return nil
	}
	if principal == nil {
		// if no principal is found, return a super admin
		return &Requester{
			email:        deploymentServiceAccountEmail,
			isSuperAdmin: true,
		}
	}

	// if principal is a service account ending on "@gcp-sa-iap.iam.gserviceaccount.com", trust IAPJWTAssertionHeader
	if principal.IsServiceAccount() && strings.HasSuffix(principal.email, "@gcp-sa-iap.iam.gserviceaccount.com") {
		principal, err = getRequesterFromJwtHeader(ctx, IAPJWTAssertionHeader, deploymentServiceAccountEmail)
		if err != nil {
			alog.Alertf(ctx, "unable to retrieve forwarded principal from the IAP request header: %s", err)
			return nil
		}
		return principal
	}

	// if principal is a super admin, check for forwarded principal
	if principal.isSuperAdmin {
		for _, header := range []string{AuthzForwardingHeader, ProxyForwardingHeader} {
			forwardedPrincipal, err := getRequesterFromJwtHeader(ctx, header, deploymentServiceAccountEmail)
			if err != nil {
				alog.Alertf(ctx, "unable to retrieve forwarded principal from the request header: %s", err)
				return nil
			}
			if forwardedPrincipal != nil {
				return forwardedPrincipal
			}
		}
	}

	return principal
}

func (r *Requester) Id() string {
	return r.id
}

func (r *Requester) Email() string {
	return r.email
}

func (r *Requester) Jwt() string {
	return r.jwt
}

func (r *Requester) IsSuperAdmin() bool {
	return r.isSuperAdmin
}

func (r *Requester) Policy() *iampb.Policy {
	return r.policy
}

// Returns whether the requester used a google identity to authenticate.
func (r *Requester) IsGoogleIdentity() bool {
	// if first char of id is a number, it is a google identity
	return '0' <= r.id[0] && r.id[0] <= '9'
}

// Returns whether the requester is a service account.
func (r *Requester) IsServiceAccount() bool {
	return strings.HasSuffix(r.email, "@gserviceaccount.com")
}

// Returns the policy member string of the requester.
// E.g. user:123456789 or serviceAccount:alis-build@...
func (r *Requester) PolicyMember() string {
	if r.IsServiceAccount() {
		return "serviceAccount:" + r.email
	} else {
		return "user:" + r.id
	}
}

// Returns the user name of the requester.
// Format: users/{userId}
func (r *Requester) UserName() string {
	return "users/" + r.id
}

func (r *Requester) HasRole(roleIds []string, policies []*iampb.Policy) bool {
	// if any role is an open role, return true
	trimmedRoleIds := map[string]bool{}
	for _, roleId := range roleIds {
		roleIdParts := strings.Split(roleId, "/")
		roleId = roleIdParts[len(roleIdParts)-1]
		if r.az.server_authorizer.openRoles[roleId] {
			return true
		}
		trimmedRoleIds[roleId] = true
	}

	// if the requester is a member of any of the roles, return true
	for _, policy := range policies {
		if policy == nil {
			continue
		}
		for _, binding := range policy.Bindings {
			bindingRoleParts := strings.Split(binding.Role, "/")
			roleId := bindingRoleParts[len(bindingRoleParts)-1]
			if trimmedRoleIds[roleId] {
				for _, member := range binding.Members {
					if r.IsMember(member) {
						return true
					}
				}
			}
		}
	}
	return false
}

// Returns whether the requester is the same as the specified policy member or is a member
// of the specified policy member if its a group.
func (r *Requester) IsMember(policyMember string) bool {
	if policyMember == r.PolicyMember() {
		return true
	}
	parts := strings.Split(policyMember, ":")
	groupType := parts[0]
	if groupType == "user" || groupType == "serviceAccount" {
		return false
	}
	groupId := ""
	if len(parts) > 1 {
		groupId = parts[1]

		// builtin domain groups
		if groupType == "domain" {
			if strings.HasSuffix(r.email, "@"+parts[1]) {
				return true
			}
		}
	}
	if resolver, ok := r.az.server_authorizer.memberResolver[groupType]; ok {
		if isMember, ok := r.memberCache.Load(policyMember); ok {
			return isMember.(bool)
		}
		isMember := resolver(r.az.ctx, groupType, groupId, r.az)
		r.memberCache.Store(policyMember, isMember)
		return isMember
	}
	return false
}

// Returns the PolicySource of the requester.
// If usersClient is not nil, it will be used to fetch the policy from the Users service.
// If the user's provided JWT token contains a valid policy claim, it will be used instead of fetching the policy.
func (r *Requester) policySource(usersClient openIam.UsersServiceClient) *PolicySource {
	if r.IsServiceAccount() {
		return nil
	} else if r.policy != nil {
		return &PolicySource{
			Resource: r.UserName(),
			Getter: func(ctx context.Context) (*iampb.Policy, error) {
				return r.policy, nil
			},
		}
	} else if usersClient != nil {
		getter := func(ctx context.Context) (*iampb.Policy, error) {
			return usersClient.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
				Resource: r.UserName(),
			})
		}
		return &PolicySource{
			Resource: r.UserName(),
			Getter:   getter,
		}
	} else {
		return nil
	}
}

// Returns the PolicySources of the requester if resourceTypes contain "alis.open.iam.v1.User"
// If usersClient is not nil, it will be used to fetch the policy from the Users service.
// If the user's provided JWT token contains a valid policy claim, it will be used instead of fetching the policy.
func (r *Requester) PolicySources(usersClient openIam.UsersServiceClient, resourceTypes []string) []*PolicySource {
	if !slices.Contains(resourceTypes, "alis.open.iam.v1.User") {
		return []*PolicySource{}
	}
	policySource := r.policySource(usersClient)
	if policySource != nil {
		return []*PolicySource{policySource}
	}
	return []*PolicySource{}
}
