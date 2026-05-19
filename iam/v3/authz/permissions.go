package authz

import "cloud.google.com/go/iam/apiv1/iampb"

var (
	openRoles       []string
	permissionRoles = map[string][]string{}
)

// AddOpenRolePermissions registers role as open to all identities and associates
// it with the given permissions.
//
// It is intended for process startup configuration, typically from init
// functions, and must not be called concurrently with authorization checks.
func AddOpenRolePermissions(role string, permissions []string) {
	openRoles = append(openRoles, role)
	AddRolePermissions(role, permissions)
}

// AddRolePermissions registers the permissions granted by role.
//
// It is intended for process startup configuration, typically from init
// functions, and must not be called concurrently with authorization checks.
func AddRolePermissions(role string, permissions []string) []string {
	for _, permission := range permissions {
		permissionRoles[permission] = append(permissionRoles[permission], role)
	}
	return permissions
}

// HasPermission returns true if the identity has the specified permission (or is privileged),
// considering both previously added roles and those from the provided policies.
//
// Note: Policies provided here are evaluated once and not persisted. To persist
// roles for subsequent checks (e.g., applying parent policies across multiple
// items in a List method), use AddRolesFromPolicies instead.
func (a *Authorizer) HasPermission(permission string, policies ...*iampb.Policy) bool {
	if a.identity.IsPrivileged() {
		return true
	}
	roles := permissionRoles[permission]
	return a.HasRole(roles, policies...)
}
