package authz

import (
	"context"
	"strings"
	"sync"

	"cloud.google.com/go/iam/apiv1/iampb"
	"go.alis.build/alog"
	"golang.org/x/exp/maps"
	"google.golang.org/grpc"
	openIam "open.alis.services/protobuf/alis/open/iam/v1"
)

type ctxKey string

const (
	claimdKey ctxKey = "x-alis-authz-claimed"
)

// A server authorizer is setup once per grpc server and contains static information about the roles, permissions
// and functions to resolve group memberships.
type ServerAuthorizer struct {
	// the email of the deployment service account
	deploymentServiceAccountEmail string
	// permissions => list of roles
	permissionRoles map[string][]string
	// role => list of resource types
	roleResourceTypes map[string][]string
	// the set of open roles, which any user has
	openRoles map[string]bool

	// the function per group type that resolves whether a requester is a member of a group
	memberResolver map[string](func(ctx context.Context, groupType string, groupId string, az *Authorizer) bool)
}

// Create a new server authorizer from the given roles and deployment service account email.
func NewServerAuthorizer(roles []*openIam.Role, deploymentServiceAccountEmail string) *ServerAuthorizer {
	// check if deployment service account email is valid
	if !strings.HasSuffix(deploymentServiceAccountEmail, ".gserviceaccount.com") {
		alog.Fatalf(context.Background(), "could not create authz.ServerAuthorizer, because deployment service account did not end with .gserviceaccount.com")
	}
	s := &ServerAuthorizer{
		deploymentServiceAccountEmail: deploymentServiceAccountEmail,
		permissionRoles:               make(map[string][]string),
		roleResourceTypes:             make(map[string][]string),
		openRoles:                     make(map[string]bool),
		memberResolver:                make(map[string](func(ctx context.Context, groupType string, groupId string, rpcAuthz *Authorizer) bool)),
	}

	permissionsMap := map[string]map[string]bool{}
	resourceTypes := map[string]map[string]bool{}
	for _, role := range roles {
		roleId := strings.TrimPrefix(role.Name, "roles/")
		if _, ok := resourceTypes[roleId]; !ok {
			resourceTypes[roleId] = make(map[string]bool)
		}
		for _, resourceType := range role.ResourceTypes {
			resourceTypes[roleId][resourceType] = true
		}
		for _, perm := range role.Permissions {
			if _, ok := permissionsMap[perm]; !ok {
				permissionsMap[perm] = make(map[string]bool)
			}
			permissionsMap[perm][role.Name] = true
		}
		if role.AllUsers {
			s.openRoles[roleId] = true
		}
	}

	for perm, roles := range permissionsMap {
		s.permissionRoles[perm] = maps.Keys(roles)
	}
	for role, sources := range resourceTypes {
		s.roleResourceTypes[role] = maps.Keys(sources)
	}

	return s
}

// WithMemberResolver registers a function to resolve whether a requester is a member of a group.
// There can be multiple different types of groups, e.g. "team:engineering" (groupType = "team",groupId="engineering")
// A group always has a type, but does not always have an id, e.g. "team:engineering" (groupType = "team",groupId="engineering") vs "all" (groupType = "all",groupId="").
// "user" and "serviceAccounts" are not allowed as group types.
// "domain" is a builtin group type that is resolved by checking if the requester's email ends with the group id.
// Results are cached per Authorizer.
func (s *ServerAuthorizer) WithMemberResolver(groupTypes []string, resolver func(ctx context.Context, groupType string, groupId string, principal *Authorizer) bool) *ServerAuthorizer {
	for _, groupType := range groupTypes {
		if groupType == "user" || groupType == "serviceAccount" || groupType == "domain" {
			alog.Fatalf(context.Background(), "cannot register builtin group type %s", groupType)
		}
		s.memberResolver[groupType] = resolver
	}
	return s
}

// Returns the roles that grant access to the given permission.
// The returned object contains the role ids and the resource types where the roles could be stored in policies.
func (s *ServerAuthorizer) GetRolesThatGrantAccess(permission string) *Roles {
	roles := s.permissionRoles[permission]
	resourceTypes := map[string]bool{}
	for _, role := range roles {
		for _, resourceType := range s.roleResourceTypes[role] {
			resourceTypes[resourceType] = true
		}
	}
	return &Roles{
		ids:           roles,
		resourceTypes: maps.Keys(resourceTypes),
	}
}

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
