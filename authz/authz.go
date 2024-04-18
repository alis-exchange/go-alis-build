package authz

import (
	"context"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	AuthForwardingHeader  = "x-forwarded-authorization" // The header used by ESPv2 gateways and other neurons to forward the JWT token
	IAPJWTAssertionHeader = "x-goog-iap-jwt-assertion"  // The header used by IAP to forward the JWT token
	AuthorizationHeader   = "Authorization"             // The header used by clients to send the JWT token directly to cloudrun (without ESPv2 or IAP)
	AuthorizationHeader2  = "authorization"             // The header used by clients to send the JWT token directly to cloudrun (without ESPv2 or IAP)
	ServerlessAuthHeader1 = "X-Serverless-Authorization"
	ServerlessAuthHeader2 = "x-serverless-authorization"
)

type AuthInfo struct {
	// The jwt token
	Jwt string
	// The principal id e.g. 123456789
	Id string
	// The principal email e.g. john@gmail.com
	Email string
	// Whether the principal is a super admin
	IsSuperAdmin bool
	// Principal is a service account; if false, principal is a user
	IsServiceAccount bool
	// Policy member in the format "user:123456789" or "serviceAccount:123456789"
	PolicyMember string

	// The principal roles
	Roles []string
}

// Authz is a struct that contains the dependencies required to validate access
// at the method level
type Authz struct {
	// permission=>set of roles
	permissionsMap map[string](map[string]bool)
	// role => set of permissions
	rolesMap                 map[string](map[string]bool)
	superAdmins              []string
	policyReader             func(ctx context.Context, resource string) (*iampb.Policy, error)
	memberResolver           map[string](func(ctx context.Context, id string, authInfo *AuthInfo) (bool, error))
	skipAuthIfAuthJwtMissing bool
}

type Role struct {
	// The role name
	Name string
	// Permissions that this role has
	Permissions []string
	// Which other roles this role extends
	Extends []string
}

// New returns a new Authz object used to authorize a permission on a resource.
func New(roles []*Role) *Authz {
	a := &Authz{
		permissionsMap: map[string](map[string]bool){},
		rolesMap:       map[string](map[string]bool){},
	}
	rolesMap := map[string]*Role{}
	for _, role := range roles {
		rolesMap[role.Name] = role
	}
	for role := range rolesMap {
		perms := getAllRolePermissions(rolesMap, role)
		a.rolesMap[role] = make(map[string]bool)
		for _, perm := range perms {
			a.rolesMap[role][perm] = true
			if _, ok := a.permissionsMap[perm]; !ok {
				a.permissionsMap[perm] = make(map[string]bool)
			}
			a.permissionsMap[perm][role] = true
		}
	}
	return a
}

// WithSuperAdmins registers the set of super administrators.
// Super admins do not need to be members of any role to have access.
// Super admins can forward authorization of other users in the x-forwarded-authorization header.
// Super admins might have special permissions in your business logic (can call List methods without providing a parent)
// Each needs to be prefixed by 'user:' or 'serviceAccount:' for example:
// ["user:10.....297", "serviceAccount:10246.....354"]
// It is recommended to only use the service accounts of your product deployment as super admins, and not individual users.
func (a *Authz) WithSuperAdmins(superAdmins []string) *Authz {
	a.superAdmins = superAdmins
	return a
}

// Use SkipAuthIfAuthJwtMissing only if your service is behind some sort of auth proxy (e.g. a non-public cloudrun service, or a service behind ESPv2 or IAP).
// This is useful if your methods need to call each other with empty contexts, and you want to bypass authorization in those cases.
// If this is used, any method that calls Authorize or AuthorizeWithResources will need to check for nil authInfo (even if err==nil) and handle it accordingly.
func (a *Authz) SkipAuthIfAuthJwtMissing() *Authz {
	a.skipAuthIfAuthJwtMissing = true
	return a
}

// WithPolicyReader registers the function to read the IAM policy for a resource. This is required if you are planning on using the AuthorizeFromResources method.
func (a *Authz) WithPolicyReader(policyReader func(ctx context.Context, resource string) (*iampb.Policy, error)) *Authz {
	a.policyReader = policyReader
	return a
}

// WithMemberResolver registers a function to resolve whether a principal is a member of a principal group.
// There can be multiple different types of principal groups, e.g. "team:engineering" (groupType = "team",groupId="engineering") or "domain:example.com" (groupType = "domain",groupId="example.com").
// A group always has a type, but does not always have an id, e.g. "allAuthenticatedUsers" or "allAlisBuilders".
// Group type of "user" and "serviceAccount" are reserved and should not be used.
// Note within a single Authorize call, the same groupType:groupId pair will only be resolved once and cached.
func (a *Authz) WithMemberResolver(groupType string, resolver func(ctx context.Context, groupId string, authInfo *AuthInfo) (bool, error)) *Authz {
	if a.memberResolver == nil {
		a.memberResolver = map[string](func(ctx context.Context, id string, authInfo *AuthInfo) (bool, error)){}
	}
	a.memberResolver[groupType] = resolver
	return a
}

// Authorize first extracts the principal from the incoming context, also accomodating for IAP and ESPv2 forwarded JWT tokens.
// It then determines which roles will grant the required permission, based on the roles provided in the New method.
// Lastly it checks whether the principal is part of any of the roles that grant the required permission.
func (a *Authz) Authorize(ctx context.Context, permission string, policies []*iampb.Policy) (*AuthInfo, error) {
	authInfo, err := getAuthInfoWithoutRoles(ctx, a.superAdmins)
	if err != nil {
		if a.skipAuthIfAuthJwtMissing {
			return nil, nil
		} else {
			return nil, err
		}
	}

	// If the principal is a super admin, grant access regardless of roles
	if authInfo.IsSuperAdmin {
		return authInfo, nil
	}

	// Get the roles that grant the required permission
	rolesThatGrantThisPermission := a.permissionsMap[permission]
	if rolesThatGrantThisPermission == nil {
		return nil, status.Errorf(codes.PermissionDenied, "no role has the permission %s", permission)
	}

	// Loop through the policies to see whether the principal has permission
	roles := map[string]bool{}
	cachedMemberResolveResults := map[string]bool{}
	for _, policy := range policies {
		if policy != nil {
			for _, binding := range policy.Bindings {
				// if the role contains the required permission, check membership
				_, ok := rolesThatGrantThisPermission[binding.GetRole()]
				if ok {
					for _, member := range binding.GetMembers() {
						isMember, err := a.isMember(authInfo, member, cachedMemberResolveResults)
						if err != nil {
							return nil, status.Errorf(codes.Internal, "unable to resolve group membership for member %s: %s", member, err)
						}
						if isMember {
							roles[binding.GetRole()] = true
						}
					}
				}
			}
		}
	}
	for role := range roles {
		authInfo.Roles = append(authInfo.Roles, role)
	}

	// If at least one role that the principal is part of has the required permission, then grant access
	if len(authInfo.Roles) > 0 {
		return authInfo, nil
	}

	// If the principal is not a super admin and is not part of any role that has the required permission, deny access
	return authInfo, status.Errorf(codes.PermissionDenied, "you do not have the required permission to access this resource")
}

func (a *Authz) isMember(authInfo *AuthInfo, member string, cachedMemberResolveResults map[string]bool) (bool, error) {
	if member == authInfo.PolicyMember {
		return true, nil
	} else {
		parts := strings.Split(member, ":")
		partBeforeColon := parts[0]
		for groupType, resolver := range a.memberResolver {
			if partBeforeColon == groupType {
				cachedRes, ok := cachedMemberResolveResults[member]
				if ok {
					return cachedRes, nil
				} else {
					id := ""
					if len(parts) > 1 {
						id = strings.Join(parts[1:], ":")
					}
					isMember, err := resolver(context.Background(), id, authInfo)
					if err != nil {
						return false, err
					}
					cachedMemberResolveResults[member] = isMember
					return isMember, nil
				}
			}
		}
	}
	return false, nil
}

// AuthorizeFromResources does the exact same thing as Authorize, except that it also retrieves the policies for the resources
// using the policyReader function provided in WithPolicyReader. This is useful when you have a list of resources and you want to
// authorize a principal against all of them, without having to retrieve the policies manually.
func (a *Authz) AuthorizeFromResources(ctx context.Context, permission string, resources []string) (*AuthInfo, error) {
	authInfo, err := getAuthInfoWithoutRoles(ctx, a.superAdmins)
	if err != nil {
		if a.skipAuthIfAuthJwtMissing {
			return nil, nil
		} else {
			return nil, err
		}
	}

	// If the principal is a super admin, no need to pull policies, just grant access
	if authInfo.IsSuperAdmin {
		return authInfo, nil
	}

	var policies []*iampb.Policy
	for _, resource := range resources {
		policy, err := a.policyReader(ctx, resource)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "unable to retrieve policy for resource %s: %s", resource, err)
		}
		policies = append(policies, policy)
	}
	return a.Authorize(ctx, permission, policies)
}

func (a *Authz) GetRoles(ctx context.Context, policies []*iampb.Policy) ([]string, error) {
	authInfo, err := getAuthInfoWithoutRoles(ctx, a.superAdmins)
	if err != nil {
		if a.skipAuthIfAuthJwtMissing {
			return []string{}, nil
		} else {
			return nil, err
		}
	}

	// If the principal is a super admin, it has no roles
	if authInfo.IsSuperAdmin {
		return []string{}, nil
	}

	roles := map[string]bool{}
	for _, policy := range policies {
		if policy != nil {
			for _, binding := range policy.Bindings {
				for _, member := range binding.GetMembers() {
					isMember, err := a.isMember(authInfo, member, map[string]bool{})
					if err != nil {
						return nil, status.Errorf(codes.Internal, "unable to resolve group membership for member %s: %s", member, err)
					}
					if isMember {
						roles[binding.GetRole()] = true
					}
				}
			}
		}
	}
	rolesList := []string{}
	for role := range roles {
		rolesList = append(rolesList, role)
	}
	return rolesList, nil
}

func (a *Authz) GetRolesFromResources(ctx context.Context, resources []string) ([]string, error) {
	authInfo, err := getAuthInfoWithoutRoles(ctx, a.superAdmins)
	if err != nil {
		if a.skipAuthIfAuthJwtMissing {
			return []string{}, nil
		} else {
			return nil, err
		}
	}

	// If the principal is a super admin, it has no roles
	if authInfo.IsSuperAdmin {
		return []string{}, nil
	}

	var policies []*iampb.Policy
	for _, resource := range resources {
		policy, err := a.policyReader(ctx, resource)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "unable to retrieve policy for resource %s: %s", resource, err)
		}
		policies = append(policies, policy)
	}
	return a.GetRoles(ctx, policies)
}

// This method is used to add the JWT token to the outgoing context in the x-forwarded-authorization header. This might be useful
// if one service needs wants to make a grpc hit in the same product deployment as the requester, in stead of as itself.
func (a *Authz) AddRequesterJwtToOutgoingCtx(ctx context.Context) (context.Context, error) {
	// add jwt to outgoing context in forwarded authorization header
	authInfo, err := getAuthInfoWithoutRoles(ctx, a.superAdmins)
	if err != nil {
		return ctx, err
	}

	// first remove any existing forwarded authorization header
	currentOutgoingMd, _ := metadata.FromOutgoingContext(ctx)
	if currentOutgoingMd == nil {
		currentOutgoingMd = metadata.New(nil)
	}
	currentOutgoingMd.Delete(AuthForwardingHeader)
	currentOutgoingMd.Set(AuthForwardingHeader, "Bearer "+authInfo.Jwt)
	ctx = metadata.NewOutgoingContext(ctx, currentOutgoingMd)

	return ctx, nil
}

// GetPermissions extracts the principal from the incoming context, also accomodating for IAP and ESPv2 forwarded JWT tokens.
// It then determines which permissions the principal has, based on the roles provided in the New method.
// Use this for implementing TestIamPermissions in your grpc service.
func (a *Authz) GetPermissions(ctx context.Context, policies []*iampb.Policy, permissions []string) []string {
	authInfo, _ := getAuthInfoWithoutRoles(ctx, a.superAdmins)
	permSet := map[string]bool{}
	for _, policy := range policies {
		if policy != nil {
			for _, binding := range policy.Bindings {
				if sliceContains(binding.GetMembers(), authInfo.PolicyMember) {
					for permission := range a.rolesMap[binding.GetRole()] {
						permSet[permission] = true
					}
				}
			}
		}
	}
	perms := []string{}
	for perm := range permSet {
		if len(permissions) == 0 || sliceContains(permissions, perm) {
			perms = append(perms, perm)
		}
	}
	return perms
}

// GetPermissionsFromResources does the exact same thing as GetPermissions, except that it also retrieves the policies for the resources
func (a *Authz) GetPermissionsFromResources(ctx context.Context, resources []string, permissions []string) []string {
	var policies []*iampb.Policy
	for _, resource := range resources {
		policy, err := a.policyReader(ctx, resource)
		if err != nil {
			continue
		}
		policies = append(policies, policy)
	}
	return a.GetPermissions(ctx, policies, permissions)
}
