// Package authz authorizes whether an identity has the required roles or
// permissions to perform an action.
//
// Create an [Authorizer] from an [auth.Identity], then check the roles that
// identity has either from its embedded IAM policy, from policies passed to a
// specific check, or from roles you add directly with [(*Authorizer).AddRoles]
// or [(*Authorizer).AddRolesFromPolicies].
//
// Role checks:
//
//	identity := auth.MustFromContext(ctx)
//	authorizer := authz.MustNew(identity)
//
//	if !authorizer.HasRole([]string{"roles/viewer", "roles/editor"}, resourcePolicy) {
//		return status.Error(codes.PermissionDenied, "unauthorized")
//	}
//
// Permissions are mapped to roles up front, usually once during service
// startup. [AddRolePermissions] returns the same permission slice so roles can
// build on each other without repeating shared permissions.
//
// Example:
//
//	authz.AddOpenRolePermissions("roles/open", []string{
//		"/example.v1.Examples/Create",
//	})
//	viewerPermissions := authz.AddRolePermissions("roles/viewer", []string{
//		"/example.v1.Examples/List",
//		"/example.v1.Examples/Get",
//	})
//	authz.AddRolePermissions("roles/admin", append(viewerPermissions,
//		"/example.v1.Examples/Delete",
//		"/example.v1.Examples/Update"),
//	)
//
// With the permission map registered, use [(*Authorizer).HasPermission] in
// handlers or interceptors to enforce RPC-level access checks.
//
// Example:
//
//	identity := auth.MustFromContext(ctx)
//	authorizer := authz.MustNew(identity)
//	authorizer.AddRolesFromPolicies(parentPolicy)
//
//	if !authorizer.HasPermission("/example.v1.Examples/Get", resourcePolicy) {
//		return nil, status.Error(codes.PermissionDenied, "unauthorized")
//	}
//
// [AddOpenRolePermissions] marks a role as open. Any permission attached to an
// open role is granted without the caller needing that role explicitly. This is
// useful for methods that should be reachable by any authenticated caller while
// still being described in the same permission map.
//
// Policies passed directly to [(*Authorizer).HasRole] or
// [(*Authorizer).HasPermission] are evaluated for that check only. Use
// [(*Authorizer).AddRolesFromPolicies] when parent or inherited policies should
// persist across multiple checks in the same request.
//
// System identities automatically pass all role and permission checks.
package authz
