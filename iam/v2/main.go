package iam

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"go.alis.build/alog"
	"google.golang.org/grpc"
)

// Role represents a role in the IAM system.
type Role struct {
	// The name of the role
	// Format: roles/([a-z][a-zA-Z0-9.]{3,50})
	Name string
	// The permissions that this role grants
	// Almost always use the format of the grpc method name
	// E.g. /alis.open.iam.v1.RolesService/ListRoles and/or /someorg.ab.library.v1.BookService/GetBook
	Permissions []string
	// Whether the role is given to all users by default.
	// This is used to group RPCs that are available to any user that has access to the deployment.
	// If this is true, resource_types must be empty.
	// The alternative would be to create a role that gets assigned to any new users.
	AllUsers bool
}

type UsersClient interface {
	GetIamPolicy(ctx context.Context, req *iampb.GetIamPolicyRequest, opts ...grpc.CallOption) (*iampb.Policy, error)
}
type UsersServer interface {
	GetIamPolicy(ctx context.Context, req *iampb.GetIamPolicyRequest) (*iampb.Policy, error)
}

// IAM is the main IAM object that contains all the roles and permissions.
// A server authoriser is set up once per grpc server and contains static information about the roles, permissions
// and functions to resolve group memberships.
type IAM struct {
	// the email of the deployment service account
	deploymentServiceAccountEmail string
	// principals that have all permissions
	// by default, only the deployment service account is a super admin
	superAdmins map[string]bool
	// the roles
	roles []*Role
	// the function per group type that resolves whether a requester is a member of a group
	memberResolver map[string]func(ctx context.Context, groupType string, groupId string, az *Authorizer) bool
	// Globally disable auth
	disabled bool

	// a map that RoleHasPermission method can use to check if a role has a permission
	rolePermissionMap map[string]map[string]bool

	// the users client to use for fetching user policies in case they are not provided in the JWT token
	UsersClient UsersClient
	// the users server to use for fetching user policies in case they are not provided in the JWT token
	UsersServer UsersServer

	// open permissions are always allowed
	openPermissions map[string]bool
}

// IamOptions are the options for creating a new IAM object.
type IamOptions struct {
	WithoutDefaultUsersClient bool
	UserServer                UsersServer
	UserClient                UsersClient
	SuperAdmins               []string
}

// IamOption is a functional option for the New method.
type IamOption func(*IamOptions)

// WithUserServer sets the user server to use for fetching user policies in case they are not provided in the JWT token.
// If set, the default users client is not used.
//
// Should only be used by the alis managed users service.
func WithUserServer(userServer UsersServer) IamOption {
	return func(opts *IamOptions) {
		opts.UserServer = userServer
		opts.WithoutDefaultUsersClient = true
	}
}

// WithoutDefaultUsersClient disables the default users client which is normally called at "iam-users-{hash}.run.app:443"
// to fetch user policies in case they are not provided in the JWT token.
func WithoutDefaultUsersClient() IamOption {
	return func(opts *IamOptions) {
		opts.WithoutDefaultUsersClient = true
	}
}

func WithUsersClient(usersClient UsersClient) IamOption {
	return func(opts *IamOptions) {
		opts.UserClient = usersClient
		opts.WithoutDefaultUsersClient = true
	}
}

// WithAdditionalSuperAdmins sets the additional super admins. By default, only the deployment service account is a super admin.
// Arguments:
//   - superAdmins: the additional super admins e.g. 'user:<userId>', 'serviceAccount:<email>'
func WithAdditionalSuperAdmins(superAdmins ...string) IamOption {
	return func(opts *IamOptions) {
		opts.SuperAdmins = append(opts.SuperAdmins, superAdmins...)
	}
}

// New creates a new IAM object.
// ALIS_OS_PROJECT environment variable must be set.
func New(roles []*Role, opts ...IamOption) (*IAM, error) {
	ctx := context.Background()
	// determine deployment service account email based on project id
	projectId := os.Getenv("ALIS_OS_PROJECT")
	if projectId == "" {
		alog.Fatal(ctx, "ALIS_OS_PROJECT not set")
	}
	deploymentServiceAccount := fmt.Sprintf("alis-build@%s.iam.gserviceaccount.com", projectId)

	// Configure final options from default and user overrides
	options := &IamOptions{
		SuperAdmins: []string{"serviceAccount:" + deploymentServiceAccount},
	}
	for _, opt := range opts {
		opt(options)
	}

	// Validate roles

	// create IAM object
	i := &IAM{
		deploymentServiceAccountEmail: deploymentServiceAccount,
		roles:                         roles,
		memberResolver:                make(map[string]func(ctx context.Context, groupType string, groupId string, rpcAuthz *Authorizer) bool),
		openPermissions:               make(map[string]bool),
		superAdmins:                   make(map[string]bool),
	}

	// populate rolePermissionMap
	i.rolePermissionMap = make(map[string]map[string]bool)
	for _, role := range roles {
		roleName := role.Name
		i.rolePermissionMap[roleName] = make(map[string]bool)
		for _, permission := range role.Permissions {
			i.rolePermissionMap[roleName][permission] = true
			if role.AllUsers {
				i.openPermissions[permission] = true
			}
		}
	}

	// initialise super admins
	i.superAdmins[deploymentServiceAccount] = true
	for _, superAdmin := range options.SuperAdmins {
		i.superAdmins[superAdmin] = true
	}

	// initialise users server if specified
	if options.UserServer != nil {
		i.UsersServer = options.UserServer
	}
	if options.UserClient != nil {
		i.UsersClient = options.UserClient
	}

	return i, nil
}

/*
RoleHasPermission check whether the specified role has the requested permission
Arguments:
  - role: the Role resource name, for example 'roles/admin', 'roles/report.viewer', etc.
  - permission: the Permission, for example '/alis.in.reports.v1.ReportsService/GetReport'
*/
func (s *IAM) RoleHasPermission(role string, permission string) bool {
	role = ensureCorrectRoleName(role)
	if _, ok := s.rolePermissionMap[role]; !ok {
		return false
	}
	return s.rolePermissionMap[role][permission]
}

// Constant values associated with policy member groupings
const (
	groupTypeUser           = "user"
	groupTypeServiceAccount = "serviceAccount"
	groupTypeDomain         = "domain"
	groupTypeGroup          = "group"
)

// WithMemberResolver registers a function to resolve whether a requester is a member of a group.
// There can be multiple different types of groups, e.g. "team:engineering" (groupType = "team",groupId="engineering")
// A group always has a type, but does not always have an id, e.g. "team:engineering" (groupType = "team",groupId="engineering") vs "all" (groupType = "all",groupId="").
// "user" and "serviceAccounts" are not allowed as group types.
// "domain" is a builtin group type that is resolved by checking if the requester's email ends with the group id.
// Results are cached per Authorizer.
func (s *IAM) WithMemberResolver(groupTypes []string, resolver func(ctx context.Context, groupType string, groupId string, principal *Authorizer) bool) *IAM {
	for _, groupType := range groupTypes {
		if groupType == groupTypeUser || groupType == groupTypeServiceAccount || groupType == groupTypeDomain || groupType == groupTypeGroup {
			alog.Fatalf(context.Background(), "cannot register builtin group type %s", groupType)
		}
		s.memberResolver[groupType] = resolver
	}
	return s
}

// Disable removes any authentication checks across all Authorizers.
// Use this method for testing methods without enforcing authorization.
func (i *IAM) Disable() {
	i.disabled = true
}

// Converts the role name to the correct format of 'roles/{role_id}'.
// Accomodates legacy bindings where role was either just roleId
// or alis-build role name, e.g. organisations/*/products/*/roles/*
func ensureCorrectRoleName(role string) string {
	if !strings.HasPrefix(role, "roles/") {
		roleParts := strings.Split(role, "/")
		roleId := roleParts[len(roleParts)-1]
		role = "roles/" + roleId
	}
	return role
}
