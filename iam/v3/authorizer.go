package iam

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"cloud.google.com/go/iam/apiv1/iampb"
)

type Authorizer struct {
	iam         *IAM
	ctx         context.Context
	permission  string
	request     *RequestIdentity
	policies    []*iampb.Policy
	memberCache sync.Map
	Cache       sync.Map
}

// PermissionDeniedError is returned when the effective caller does not have the
// requested permission.
type PermissionDeniedError struct {
	Permission    string
	Caller        string
	Authenticated string
}

// Error implements the error interface.
func (e *PermissionDeniedError) Error() string {
	return fmt.Sprintf(
		"permission denied: permission=%s caller=%s authenticated=%s",
		e.Permission,
		e.Caller,
		e.Authenticated,
	)
}

// NewAuthorizer constructs an Authorizer for a single permission check.
func (i *IAM) NewAuthorizer(ctx context.Context, permission string) (*Authorizer, error) {
	if permission == "" {
		return nil, fmt.Errorf("permission is required")
	}

	requestIdentity, err := i.ExtractRequestIdentity(ctx)
	if err != nil {
		return nil, err
	}

	return &Authorizer{
		iam:        i,
		ctx:        WithRequestIdentity(ctx, requestIdentity),
		permission: permission,
		request:    requestIdentity,
		policies:   []*iampb.Policy{},
	}, nil
}

// RequestIdentity returns the authenticated and effective caller identities
// resolved for this authorization flow.
func (a *Authorizer) RequestIdentity() *RequestIdentity {
	return a.request
}

// Caller returns the effective caller used for authorization decisions.
func (a *Authorizer) Caller() *Identity {
	return a.request.Caller
}

// Authenticated returns the principal that authenticated the transport request.
func (a *Authorizer) Authenticated() *Identity {
	return a.request.Authenticated
}

// AddPolicy appends a policy to the set evaluated for access checks.
func (a *Authorizer) AddPolicy(policy *iampb.Policy) {
	if policy == nil {
		return
	}
	a.policies = append(a.policies, policy)
}

// AddIdentityPolicy adds the caller's own user policy, loading it via the
// configured PolicyFetcher when necessary.
func (a *Authorizer) AddIdentityPolicy() error {
	if a.Caller() == nil || a.Caller().IsServiceAccount() {
		return nil
	}
	if policy := a.Caller().Policy(); policy != nil {
		a.AddPolicy(policy)
		return nil
	}
	if a.iam.policyFetcher == nil {
		return nil
	}
	return a.AddPolicyFromResource(a.Caller().UserName())
}

// AddPolicyFromResource loads and adds the policy for a specific resource.
func (a *Authorizer) AddPolicyFromResource(resource string) error {
	if a.iam.policyFetcher == nil {
		return fmt.Errorf("policy fetcher not configured")
	}
	policy, err := a.iam.policyFetcher.GetPolicy(a.ctx, resource)
	if err != nil {
		return err
	}
	a.AddPolicy(policy)
	return nil
}

// HasAccess reports whether the caller has the configured permission.
func (a *Authorizer) HasAccess(extraPolicies ...*iampb.Policy) bool {
	if a.iam.disabled {
		return true
	}

	caller := a.Caller()
	if caller == nil {
		return false
	}

	if a.iam.superAdmins[caller.PolicyMember()] {
		return true
	}

	if a.iam.openPermissions[a.permission] {
		return true
	}

	policies := append([]*iampb.Policy{}, a.policies...)
	policies = append(policies, extraPolicies...)
	for _, policy := range policies {
		if policy == nil {
			continue
		}
		for _, binding := range policy.GetBindings() {
			if !a.iam.RoleHasPermission(binding.Role, a.permission) {
				continue
			}
			for _, member := range binding.Members {
				if member == caller.PolicyMember() || a.IsGroupMember(member) {
					return true
				}
			}
		}
	}

	return false
}

// Require returns an error when the caller does not have the configured
// permission.
func (a *Authorizer) Require(extraPolicies ...*iampb.Policy) error {
	if a.HasAccess(extraPolicies...) {
		return nil
	}

	caller := ""
	if a.Caller() != nil {
		caller = a.Caller().PolicyMember()
	}

	authenticated := ""
	if a.Authenticated() != nil {
		authenticated = a.Authenticated().PolicyMember()
	}

	return &PermissionDeniedError{
		Permission:    a.permission,
		Caller:        caller,
		Authenticated: authenticated,
	}
}

// HasRole reports whether the caller has the supplied role in the provided
// policies.
func (a *Authorizer) HasRole(role string, policies ...*iampb.Policy) bool {
	role = ensureCorrectRoleName(role)
	for _, policy := range policies {
		if policy == nil {
			continue
		}
		for _, binding := range policy.GetBindings() {
			if ensureCorrectRoleName(binding.Role) != role {
				continue
			}
			for _, member := range binding.Members {
				if member == a.Caller().PolicyMember() || a.IsGroupMember(member) {
					return true
				}
			}
		}
	}
	return false
}

// IsGroupMember reports whether the caller belongs to the supplied IAM group
// member string.
func (a *Authorizer) IsGroupMember(group string) bool {
	if cached, ok := a.memberCache.Load(group); ok {
		return cached.(bool)
	}

	parts := strings.Split(group, ":")
	groupType := parts[0]
	if groupType == groupTypeUser || groupType == groupTypeServiceAccount {
		return false
	}

	groupID := ""
	if len(parts) > 1 {
		groupID = parts[1]
	}

	switch groupType {
	case groupTypeDomain:
		isMember := strings.HasSuffix(a.Caller().Email(), "@"+groupID)
		a.memberCache.Store(group, isMember)
		return isMember
	case groupTypeGroup:
		isMember := slices.Contains(a.Caller().GroupIDs(), groupID)
		a.memberCache.Store(group, isMember)
		return isMember
	}

	resolver, ok := a.iam.memberResolver[groupType]
	if !ok {
		a.memberCache.Store(group, false)
		return false
	}

	isMember := resolver(a.ctx, groupType, groupID, a)
	a.memberCache.Store(group, isMember)
	return isMember
}
