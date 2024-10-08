package iam

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ctxKey string

const (
	claimedKey ctxKey = "x-alis-authz-claimed"
)

// Authorizer is responsible for the Authorization (i.e. Access) part of the IAM service and
// lives for the duration of a grpc method call. It is used to authorize the requester while
// providing access to the policy cache and the member cache to prevent redundant calls.
type Authorizer struct {
	// The Identity and Access object with details on the underlyging roles, service account, super Admin, etc.
	iam *IAM

	// The Identity
	Identity *Identity

	// The rpc method
	// Format: /package.service/method
	Method string

	// Policies applicable for the method.
	Policies []*iampb.Policy

	// Skip authorization. No auth required if requester is super admin or the auth is already claimed.
	skipAuth bool

	// The ctx which is applicable for the duration of the grpc method call.
	ctx context.Context
}

/*
NewAuthorizer creates a new authorizer which will live for the duration of the grpc method call.

It initializes an Authorizer with the provided IAM instance and performs the following actions:

  - Identity Extraction: Extracts the Identity from the context, which includes information about the caller's authentication status and associated policies.
  - Policy Initialization: If the extracted Identity has an associated policy, it is added to the Authorizer's list of policies.
  - Super Admin Handling: Checks if the Identity represents a super admin. If so, authorization checks are skipped.
  - Authorization Claiming:  Marks the authorization as 'claimed' in the context to prevent redundant authorization checks within the same call. If the authorization is already claimed, it skips further checks.
  - Method Extraction: Extracts the gRPC method name from the context, which is crucial for determining the required permissions. If the method cannot be extracted and the caller is not a super admin, it returns an error.

The resulting Authorizer is then returned, ready to be used for access control decisions.

Parameters:

	ctx: The context containing information about the gRPC call, including the Identity and the method being invoked.

Returns:

  - *Authorizer: A new Authorizer instance populated with the extracted Identity, method, and policies.
  - error: An error if any of the extraction or initialization steps fail.
*/
func (i *IAM) NewAuthorizer(ctx context.Context) (*Authorizer, context.Context, error) {
	authorizer := &Authorizer{
		iam:      i,
		Policies: []*iampb.Policy{},
	}

	// First, we'll extract the Identity from the context
	identity, err := ExtractIdentityFromCtx(ctx, i.deploymentServiceAccountEmail)
	if err != nil {
		return nil, ctx, err
	}

	authorizer.Identity = identity
	if authorizer.Identity.isDeploymentServiceAccount {
		authorizer.skipAuth = true
	}

	// claim if not claimed, otherwise do not require auth
	_, ok := ctx.Value(claimedKey).(bool)
	if !ok {
		ctx = context.WithValue(ctx, claimedKey, true)
	} else {
		authorizer.skipAuth = true
	}
	authorizer.ctx = ctx

	// extract method from context
	method, ok := grpc.Method(ctx)
	if !ok {
		if !authorizer.skipAuth {
			return nil, ctx, fmt.Errorf("rpc method not found in context")
		}
	}
	authorizer.Method = method

	return authorizer, ctx, nil
}

// AuthorizeRpc checks if the Identity has access to the current rpc method based on all the provided policies.
// This method returns an gRPC compliant error message and should be handled accordingly and is intented to be used by
// rpc methods.
func (a *Authorizer) AuthorizeRpc() error {
	// Skip authorization if the IAM package is globally disabled.
	if a.iam.disabled {
		return nil
	}

	// if no auth is required, return true
	if a.skipAuth {
		return nil
	}

	// Iterate through the policies and grant access if found
	for _, policy := range a.Policies {
		// Now iterate through the bindings
		for _, binding := range policy.Bindings {
			// If the binding role has the relevant permission
			if a.iam.RoleHasPermission(binding.Role, a.Method) {
				// Check whether the identity is present in the policy members.
				for _, member := range binding.Members {
					switch {
					case strings.HasPrefix(member, "user:") || strings.HasPrefix(member, "serviceAccount:"):
						// Check for exact member entry
						if member == a.Identity.PolicyMember() {
							return nil
						}
					case strings.HasPrefix(member, "domain:"):
						// The domain that represents all the users of that domain.
						emailDomain := strings.Split(a.Identity.email, "@")[1] // user:joe@mywebsite.com -> mywebsite.com
						memberDomain := strings.Split(member, ":")[1]          // domain:mywebsite.com -> mywebsite.com
						if emailDomain == memberDomain {
							return nil
						}
					case member == "allAuthenticatedUsers":
						// A special identifier that represents anyone
						// who is authenticated with an approved Identity Provider
						return nil
					default:
						// Still need to provide support for 'groups:'
					}
				}
			}
		}
	}

	return status.Errorf(codes.PermissionDenied, "permission denied: %s", a.Method)
}

// HasAccess checks if the requester has access to the current method based on all the
// underlying policies as well as the optionally provided policies
func (a *Authorizer) HasAccess(permission string, policies ...*iampb.Policy) bool {
	// Skip authorization if the IAM package is globally disabled.
	if a.iam.disabled {
		return true
	}

	// if no auth is required, return true
	if a.skipAuth {
		return true
	}

	// Handle any additional Policies.
	if len(policies) > 0 {
		for _, policy := range policies {
			// Now iterate through the bindings
			for _, binding := range policy.Bindings {
				// If the binding role has the relevant permission
				if a.iam.RoleHasPermission(binding.Role, permission) {
					// Check whether the identity is present in the policy members.
					for _, member := range binding.Members {
						if member == a.Identity.PolicyMember() {
							return true
						}
					}
				}
			}
		}
	}

	// Iterate through existing Policies, likely to refer to Policies on parent resources.
	for _, policy := range a.Policies {
		// Now iterate through the bindings
		for _, binding := range policy.Bindings {
			// If the binding role has the relevant permission
			if a.iam.RoleHasPermission(binding.Role, permission) {
				// Check whether the identity is present in the policy members.
				for _, member := range binding.Members {
					if member == a.Identity.PolicyMember() {
						return true
					}
				}
			}
		}
	}

	return false
}

// AddPolicy adds a policy to the list of policies against which access will be validated.
func (a *Authorizer) AddPolicy(policy *iampb.Policy) {
	a.Policies = append(a.Policies, policy)
}

// AddIdentityPolicy adds a policy from the underlying Identity to the list of policies against which access will be validated.
func (a *Authorizer) AddIdentityPolicy() {
	if a.Identity.policy != nil {
		a.Policies = append(a.Policies, a.Identity.policy)
	}
}
