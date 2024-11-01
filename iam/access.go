package iam

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/google/uuid"
	"go.alis.build/alog"
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

	// Concurrency safe storage of policies that are always checked
	policies *sync.Map

	// Skip authorization. No auth required if requester is super admin or the auth is already claimed.
	skipAuth bool

	// The ctx which is applicable for the duration of the grpc method call.
	ctx context.Context

	// A cache to store the result of the member resolver function
	memberCache *sync.Map

	// A wait group to track any background policy fetches before checking access.
	wg *sync.WaitGroup

	// A generic cache that group resolvers can use to store data used to resolve group membership.
	// There is no need to store final results in this cache, as these are automatically cached by the Authorizer.
	// An example use case is to store the result of a database query to avoid duplicate queries if a query could
	// resolve more than one group membership.
	Cache *sync.Map

	// The batch authorizer, if any, that this authorizer belongs to.
	// Batch authorizer is used as a shared cache for policies, group memberships and the generic Cache.
	batchAuthorizer *BatchAuthorizer
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
		iam:         i,
		policies:    &sync.Map{},
		memberCache: &sync.Map{},
		wg:          &sync.WaitGroup{},
		Cache:       &sync.Map{},
	}

	// Get identity from context
	identity, err := ExtractIdentityFromCtx(ctx, i.deploymentServiceAccountEmail)
	if err != nil {
		return nil, ctx, err
	}
	authorizer.Identity = identity

	// Skip auth if the identity is the deployment service account
	if authorizer.Identity.isDeploymentServiceAccount {
		authorizer.skipAuth = true
	}

	// Skip auth if the identity is a super admin. Normally deployment service account is the only super admin.
	if i.superAdmins[authorizer.Identity.PolicyMember()] {
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

// Policies returns the list of policies against which any access will be validated.
func (a *Authorizer) Policies() []*iampb.Policy {
	// first wait for any async policy fetches to complete
	a.wg.Wait()

	policies := []*iampb.Policy{}
	a.policies.Range(func(key, value interface{}) bool {
		policies = append(policies, value.(*iampb.Policy))
		return true
	})
	return policies
}

// AuthorizeRpc checks if the Identity has access to the current rpc method based on all the provided policies.
// This method returns an gRPC compliant error message and should be handled accordingly and is intented to be used by
// rpc methods.
func (a *Authorizer) AuthorizeRpc() error {
	if !a.HasAccess(a.Method) {
		return status.Errorf(codes.PermissionDenied, "permission denied: %s", a.Method)
	}
	return nil
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

	// check if the permission is an open permission
	if a.iam.openPermissions[permission] {
		return true
	}

	// Iterate through Policies and grant access if member found in role that grants access
	policiesToCheck := append(a.Policies(), policies...)
	for _, policy := range policiesToCheck {
		// Now iterate through the bindings
		for _, binding := range policy.Bindings {
			// If the binding role has the relevant permission
			if a.iam.RoleHasPermission(binding.Role, permission) {
				// Check whether the identity is present in the policy members.
				for _, member := range binding.Members {
					if member == a.Identity.PolicyMember() {
						return true
					} else if a.IsGroupMember(member) {
						return true
					}
				}
			}
		}
	}

	return false
}

// Returns whether the requester has the specified role in the list of specified policies.
// Does not look in the Policies stored in the Authorizer, but rather the provided policies.
func (a *Authorizer) HasRole(policies []*iampb.Policy, role string) bool {
	role = ensureCorrectRoleName(role)
	for _, policy := range policies {
		// Now iterate through the bindings
		for _, binding := range policy.Bindings {
			// accomodating legacy bindings where role was either just roleId
			// or alis-build role name, e.g. organisations/*/products/*/roles/*
			bindingRole := ensureCorrectRoleName(binding.Role)
			if bindingRole == role {
				// Check whether the identity is present in the policy members.
				for _, policyMember := range binding.Members {
					if a.Identity.PolicyMember() == policyMember {
						return true
					} else if a.IsGroupMember(policyMember) {
						return true
					}
				}
			}
		}
	}
	return false
}

// Returns whether identity is a member of the specified iam group.
func (a *Authorizer) IsGroupMember(group string) bool {
	parts := strings.Split(group, ":")
	groupType := parts[0]
	if groupType == "user" || groupType == "serviceAccount" {
		return false
	}
	groupId := ""
	if len(parts) > 1 {
		groupId = parts[1]

		// builtin domain groups
		if groupType == "domain" {
			if strings.HasSuffix(a.Identity.email, "@"+parts[1]) {
				return true
			}
		}
	}
	if resolver, ok := a.iam.memberResolver[groupType]; ok {
		if isMember, ok := a.memberCache.Load(group); ok {
			return isMember.(bool)
		}
		isMember := resolver(a.ctx, groupType, groupId, a)
		a.memberCache.Store(group, isMember)
		return isMember
	}
	return false
}

// AddPolicy adds a policy to the list of policies against which any access will be validated.
// Do not add policies that should not be evaluated in downstream access checks.
// To add a policy that should only be evaluated for a specific access check, use the optional policies parameter in the HasAccess method.
// Will do nothing if identity is deployment service account or auth is already claimed.
// Auth is claimed if another method on this server is calling the method where this authorizer lives.
func (a *Authorizer) AddPolicy(policy *iampb.Policy) {
	// do nothing if auth skipped
	if a.skipAuth {
		return
	}

	// do nothing if policy is nil
	if policy == nil {
		return
	}

	// add the policy to the list of policies
	randomUniqueKey := uuid.New().String()
	a.policies.Store(randomUniqueKey, policy)
}

// AddIdentityPolicy adds a policy from the underlying Identity to the list of policies against which access will be validated.
// If the policy is not present in the Identity's jwt and a usersClient is provided, it will fetch the policy asynchronously.
// Will do nothing if identity is deployment service account or auth is already claimed.
// Auth is claimed if another method on this server is calling the method where this authorizer lives.
func (a *Authorizer) AddIdentityPolicy() {
	// do nothing if auth skipped
	if a.skipAuth {
		return
	}
	// add the policy if it exists
	if a.Identity.policy != nil {
		a.AddPolicy(a.Identity.policy)
	} else {
		if a.iam.UsersServer != nil {
			// async fetch the policy
			fetchFunc := func() *iampb.Policy {
				req := &iampb.GetIamPolicyRequest{
					Resource: a.Identity.UserName(),
				}
				policy, err := a.iam.UsersServer.GetIamPolicy(a.ctx, req)
				if err != nil {
					alog.Alertf(a.ctx, "error fetching policy for user %s: %v", a.Identity.UserName(), err)
					return nil
				}
				return policy
			}
			a.AsyncAddPolicy(a.Identity.UserName(), fetchFunc)
		} else if a.iam.UsersClient != nil {
			// async fetch the policy
			fetchFunc := func() *iampb.Policy {
				req := &iampb.GetIamPolicyRequest{
					Resource: a.Identity.UserName(),
				}
				policy, err := a.iam.UsersClient.GetIamPolicy(a.ctx, req)
				if err != nil {
					alog.Alertf(a.ctx, "error fetching policy for user %s: %v", a.Identity.UserName(), err)
					return nil
				}
				return policy
			}
			a.AsyncAddPolicy(a.Identity.UserName(), fetchFunc)
		}
	}
}

// Asynchronously fetches a policy on another server using a Grpc Client's GetIamPolicy method.
// Adds the fetched policy to the list of policies against which access will be validated.
// Will do nothing if identity is deployment service account or auth is already claimed.
// Auth is claimed if another method on this server is calling the method where this authorizer lives.
func (a *Authorizer) AddPolicyFromClientRpc(resource string, clientGetIamPolicyMethod func(ctx context.Context, req *iampb.GetIamPolicyRequest, opts ...grpc.CallOption) (*iampb.Policy, error)) {
	// do nothing if auth skipped
	if a.skipAuth {
		return
	}

	// async fetch the policy
	fetchFunc := func() *iampb.Policy {
		req := &iampb.GetIamPolicyRequest{
			Resource: resource,
		}
		policy, err := clientGetIamPolicyMethod(a.ctx, req)
		if err != nil {
			alog.Alertf(a.ctx, "error fetching policy from client for %s: %v", resource, err)
			return nil
		}
		return policy
	}
	a.AsyncAddPolicy(resource, fetchFunc)
}

// Asynchronously fetches a policy from a locally implemented service's GetIamPolicy method.
// Adds the fetched policy to the list of policies against which access will be validated.
// Will do nothing if identity is deployment service account or auth is already claimed.
// Auth is claimed if another method on this server is calling the method where this authorizer lives.
func (a *Authorizer) AddPolicyFromServerRpc(resource string, serverGetIamPolicyMethod func(ctx context.Context, req *iampb.GetIamPolicyRequest) (*iampb.Policy, error)) {
	// do nothing if auth skipped
	if a.skipAuth {
		return
	}

	// async fetch the policy
	fetchFunc := func() *iampb.Policy {
		req := &iampb.GetIamPolicyRequest{
			Resource: resource,
		}
		policy, err := serverGetIamPolicyMethod(a.ctx, req)
		if err != nil {
			alog.Alertf(a.ctx, "error fetching policy from server for %s: %v", resource, err)
			return nil
		}
		return policy
	}
	a.AsyncAddPolicy(resource, fetchFunc)
}

// This function should rarely be used directly.
// Rather use the AddIdentityPolicy, AddPolicyFromClientRpc, or AddPolicyFromServerRpc functions.
// Asynchronously fetches a policy using the provided fetch function and adds it to the list of policies against which access will be validated.
func (a *Authorizer) AsyncAddPolicy(resource string, fetchFunc func() *iampb.Policy) {
	// do nothing if auth skipped
	if a.skipAuth {
		return
	}

	// async fetch the policy
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()

		// if part of batch authorizer, use the batch authorizer's async fetch policy to benefit from caching
		if a.batchAuthorizer != nil {
			policy := a.batchAuthorizer.asyncFetchPolicy(resource, fetchFunc)
			if policy != nil {
				a.AddPolicy(policy)
			}
		} else {
			policy := fetchFunc()
			if policy != nil {
				a.AddPolicy(policy)
			}
		}
	}()
}
