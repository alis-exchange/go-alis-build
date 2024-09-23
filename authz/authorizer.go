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
// There can be multiple different types of groups, e.g. "team:engineering" (groupType = "team",groupId="engineering") or "domain:example.com" (groupType = "domain",groupId="example.com").
// A group always has a type, but does not always have an id, e.g. "allAuthenticatedUsers" or "allAlisBuilders".
// Group type of "user" and "serviceAccount" are reserved and should not be used.
// Results are cached per authorizer instance, so if you need to resolve the same group multiple times, it will only be resolved once.
func (s *ServerAuthorizer) WithMemberResolver(groupTypes []string, resolver func(ctx context.Context, groupType string, groupId string, principal *Authorizer) bool) *ServerAuthorizer {
	for _, groupType := range groupTypes {
		s.memberResolver[groupType] = resolver
	}
	return s
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
	// Whether authorization is required. No auth required is requester is super admin or the auth is already claimed.
	requireAuth bool

	// The policy cache
	policyCache sync.Map
	// The member cache
	memberCache sync.Map

	// The ctx
	ctx context.Context
}

func (s *ServerAuthorizer) Authorizer(ctx context.Context) (*Authorizer, context.Context) {
	requireAuth := true

	requester := getRequester(ctx, s.deploymentServiceAccountEmail)
	// if principal is nil, assume super admin
	if requester == nil {
		requester = &Requester{
			Email:        s.deploymentServiceAccountEmail,
			IsSuperAdmin: true,
		}
	}
	if requester.IsSuperAdmin {
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

// Get the cached policy (if any) for the given resource in this authorizer.
func (r *Authorizer) GetCachedPolicy(resource string) *iampb.Policy {
	if policy, ok := r.policyCache.Load(resource); ok {
		return policy.(*iampb.Policy)
	}
	return nil
}

// Cache the policy for the given resource in this authorizer.
func (r *Authorizer) CachePolicy(resource string, policy *iampb.Policy) {
	r.policyCache.Store(resource, policy)
}

// An object that contains the role ids and the resource types where the roles could be stored in policies.
type Roles struct {
	ids           []string
	resourceTypes []string
}

// Returns the role ids that grant access to the given permission.
func (r *Roles) GetIds() []string {
	return r.ids
}

// Returns the resource types where the roles could be stored in policies.
// E.g. alis.open.iam.v1.User and/or abc.de.library.v1.Book
func (r *Roles) GetResourceTypes() []string {
	return r.resourceTypes
}

// Returns the roles that grant access to the given permission.
// The returned object contains the role ids and the resource types where the roles could be stored in policies.
func (s *Authorizer) GetRolesThatGrantAccess(permission string) *Roles {
	roles := s.authorizer.permissionRoles[permission]
	resourceTypes := map[string]bool{}
	for _, role := range roles {
		for _, resourceType := range s.authorizer.roleResourceTypes[role] {
			resourceTypes[resourceType] = true
		}
	}
	return &Roles{
		ids:           roles,
		resourceTypes: maps.Keys(resourceTypes),
	}
}
