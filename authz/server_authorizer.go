package authz

import (
	"context"
	"strings"

	"go.alis.build/alog"
	"golang.org/x/exp/maps"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	openIam "open.alis.services/protobuf/alis/open/iam/v1"
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

// Returns a grpc error for the specified permission with the PermissionDenied code and an appropriate message.
func PermissionDeniedError(permission string, roles []string, resources ...string) error {
	if len(roles) == 0 {
		return status.Errorf(codes.PermissionDenied, "%s is an internal method", permission)
	}
	if len(resources) == 0 {
		return status.Errorf(codes.PermissionDenied, "missing one of the following roles to call %s: %v", permission, strings.Join(roles, ", "))
	} else if len(resources) == 1 {
		return status.Errorf(codes.PermissionDenied, "missing one of the following roles to call %s on %s: %v", permission, resources[0], strings.Join(roles, ", "))
	} else {
		return status.Errorf(codes.PermissionDenied, "missing one of the following roles to call %s: %v", permission, strings.Join(roles, ", "))
	}
}
