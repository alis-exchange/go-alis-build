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
	server_authorizer *ServerAuthorizer
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
	a := &Authorizer{
		server_authorizer: s,
		requireAuth:       requireAuth,
		policyCache:       sync.Map{},
		ctx:               ctx,
	}

	requester := newRequesterFromCtx(ctx, s.deploymentServiceAccountEmail)
	requester.az = a

	if requester.isSuperAdmin {
		a.requireAuth = false
	}

	// claim if not claimed, otherwise do not require auth
	_, ok := ctx.Value(claimdKey).(bool)
	if !ok {
		ctx = context.WithValue(ctx, claimdKey, true)
	} else {
		a.requireAuth = false
	}

	// extract method from context
	method, ok := grpc.Method(ctx)
	if !ok {
		if a.requireAuth {
			alog.Fatalf(ctx, "rpc method not found in context")
		}
	}
	a.Method = method

	return a, ctx
}

// Checks if requester has access to the current method based on the provided policies.
func (a *Authorizer) HasMethodAccess(policies []*iampb.Policy) bool {
	// if no auth is required, return true
	if !a.requireAuth {
		return true
	}

	// check if users has a role that gives them the permission
	roleIds := a.server_authorizer.permissionRoles[a.Method]
	return a.Requester.HasRole(roleIds, policies)
}

// Checks if the requester has the specified permission in the provided policies.
func (a *Authorizer) HasPermission(permission string, policies []*iampb.Policy) bool {
	// if no auth is required, return true
	if !a.requireAuth {
		return true
	}

	// check if users has a role that gives them the permission
	roleIds := a.server_authorizer.permissionRoles[permission]
	return a.Requester.HasRole(roleIds, policies)
}

// Returns a grpc error for this authorizer's method with the PermissionDenied code and an appropriate message.
func (a *Authorizer) PermissionDeniedError(resources ...string) error {
	return a.server_authorizer.PermissionDeniedError(a.Method, resources...)
}

// Get the cached policy (if any) for the given resource in this authorizer.
func (a *Authorizer) cachedPolicy(resource string) *iampb.Policy {
	if policy, ok := a.policyCache.Load(resource); ok {
		return policy.(*iampb.Policy)
	}
	return nil
}

// Cache the policy for the given resource in this authorizer.
func (a *Authorizer) cachePolicy(resource string, policy *iampb.Policy) {
	a.policyCache.Store(resource, policy)
}
