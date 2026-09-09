package authz

import "cloud.google.com/go/iam/apiv1/iampb"

var (
	openRoles       []string
	permissionRoles = map[string][]string{}
)

// AddOpenRolePermissions registers role as open to all identities and associates
// it with the given permissions.
//
// The id is process-local in the same way [AddRolePermissions] describes, and
// open roles are where that bites hardest: "roles/open" is the conventional
// name for the any-signed-in-user role, so nearly every service registers one
// and no two grant the same thing. Definitions in production today range from
// reading your own profile to deleting your own account to creating another
// domain's resources. Never treat an open role id from one service as though
// it named the same authority in another.
//
// It is intended for process startup configuration, typically from init
// functions, and must not be called concurrently with authorization checks.
func AddOpenRolePermissions(role string, permissions []string) {
	openRoles = append(openRoles, role)
	AddRolePermissions(role, permissions)
}

// AddRolePermissions registers the permissions granted by role.
//
// Role ids are unnamespaced and scoped to the registering process. The map
// lives in this package, so "roles/admin" here and "roles/admin" in another
// service are unrelated strings that happen to match, and the same id may grant
// entirely different permissions in each. That is fine, and needs no guard, so
// long as no id is carried across a service boundary as authorization data.
//
// Do not do that. Nothing that leaves this process, a token claim especially,
// can be read elsewhere as naming the permissions registered here. See
// [go.alis.build/iam/v3.Identity.AuthzRoles], which carries such ids and is
// meaningful only to the service that minted them.
//
// Qualifying ids by service, so that they could be reasoned about across a
// boundary, would be the fix if cross-service role comparison were ever wanted.
// It is not done because every policy binding already stored names a bare
// string, making it a data migration rather than a change here.
//
// It is intended for process startup configuration, typically from init
// functions, and must not be called concurrently with authorization checks.
func AddRolePermissions(role string, permissions []string) []string {
	for _, permission := range permissions {
		permissionRoles[permission] = append(permissionRoles[permission], role)
	}
	return permissions
}

// HasPermission returns true if the identity has the specified permission (or is
// privileged and not Restricted), considering both previously added roles and
// those from the provided policies.
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
