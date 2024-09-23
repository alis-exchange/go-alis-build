package authz

import (
	"context"
	"sync"

	"cloud.google.com/go/iam/apiv1/iampb"
	"go.alis.build/alog"
	"google.golang.org/grpc"
)

type ctxKey string

const (
	claimdKey ctxKey = "x-alis-authz-claimed"
)

// An authorizer lives for the duration of a grpc method call and is used to authorize the requester while
// providing access to the policy cache and the member cache to prevent redundant calls.
type Authorizer struct {
	// The server authorizer
	authorizer *ServerAuthorizer
	// The rpc method
	// Format: /package.service/method
	Method string
	// The Requester
	Requester *Requester
	// Whether authorization is required. No auth required if requester is super admin or the auth is already claimed.
	requireAuth bool

	// The policy cache
	policyCache sync.Map

	// The ctx
	ctx context.Context
}

// Creates a new authorizer which will live for the duration of the grpc method call.
func (s *ServerAuthorizer) Authorizer(ctx context.Context) (*Authorizer, context.Context) {
	requireAuth := true

	requester := newRequesterFromCtx(ctx, s.deploymentServiceAccountEmail)

	if requester.isSuperAdmin {
		requireAuth = false
	}

	// claim if not claimed, otherwise do not require auth
	_, ok := ctx.Value(claimdKey).(bool)
	if !ok {
		ctx = context.WithValue(ctx, claimdKey, true)
	} else {
		requireAuth = false
	}

	// extract method from context
	method, ok := grpc.Method(ctx)
	if !ok {
		if requireAuth {
			alog.Fatalf(ctx, "rpc method not found in context")
		}
	}

	return &Authorizer{
		authorizer:  s,
		Method:      method,
		Requester:   requester,
		requireAuth: requireAuth,
		policyCache: sync.Map{},

		ctx: ctx,
	}, ctx
}

// Checks if requester has access to the current method based on the provided policies.
func (s *Authorizer) HasMethodAccess(policies []*iampb.Policy) bool {
	roles := s.authorizer.GetRolesThatGrantAccess(s.Method)
	return s.Requester.HasRole(roles.ids, policies)
}

// Checks if the requester has the specified permission in the provided policies.
func (s *Authorizer) HasPermission(permission string, policies []*iampb.Policy) bool {
	roles := s.authorizer.GetRolesThatGrantAccess(permission)
	return s.Requester.HasRole(roles.ids, policies)
}

// Get the cached policy (if any) for the given resource in this authorizer.
func (r *Authorizer) cachedPolicy(resource string) *iampb.Policy {
	if policy, ok := r.policyCache.Load(resource); ok {
		return policy.(*iampb.Policy)
	}
	return nil
}

// Cache the policy for the given resource in this authorizer.
func (r *Authorizer) cachePolicy(resource string, policy *iampb.Policy) {
	r.policyCache.Store(resource, policy)
}

// Returns a grpc error for this authorizer's method with the PermissionDenied code and an appropriate message.
func (r *Authorizer) RpcPermissionDeniedError(roles []string, resources ...string) error {
	return PermissionDeniedError(r.Method, roles, resources...)
}
