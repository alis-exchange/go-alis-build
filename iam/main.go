package iam

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	parametermanager "cloud.google.com/go/parametermanager/apiv1"
	"cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
	"go.alis.build/alog"
	"go.alis.build/client"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	openConfig "open.alis.services/protobuf/alis/open/config/v1"
	openIam "open.alis.services/protobuf/alis/open/iam/v1"
)

// A server authorizer is setup once per grpc server and contains static information about the roles, permissions
// and functions to resolve group memberships.
type IAM struct {
	// the email of the deployment service account
	deploymentServiceAccountEmail string
	// principals that have all permissions
	// by default, only the deployment service account is a super admin
	superAdmins map[string]bool
	// the roles
	roles []*openIam.Role
	// the function per group type that resolves whether a requester is a member of a group
	memberResolver map[string](func(ctx context.Context, groupType string, groupId string, az *Authorizer) bool)
	// Globally disable auth
	disabled bool

	// a map that RoleHasPermission method can use to check if a role has a permission
	rolePermissionMap map[string]map[string]bool

	// the users client to use for fetching user policies in case they are not provided in the JWT token
	UsersClient openIam.UsersServiceClient

	// the users server to use for fetching user policies in case they are not provided in the JWT token
	UsersServer openIam.UsersServiceServer

	// open permissions are always allowed
	openPermissions map[string]bool
}

// IamOptions are the options for creating a new IAM object.
type IamOptions struct {
	WithoutDefaultUsersClient bool
	UserServer                openIam.UsersServiceServer
	SuperAdmins               []string
}

// IamOption is a functional option for the New method.
type IamOption func(*IamOptions)

// Should only be used by the alis managed users service.
// WithUserServer sets the user server to use for fetching user policies in case they are not provided in the JWT token.
// If set, the default users client is not used.
func WithUserServer(userServer openIam.UsersServiceServer) IamOption {
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

// Sets the additional super admins. By default, only the deployment service account is a super admin.
// Arguments:
//   - superAdmins: the additional super admins e.g. 'user:<userId>', 'serviceAccount:<email>'
func WithAdditionalSuperAdmins(superAdmins ...string) IamOption {
	return func(opts *IamOptions) {
		opts.SuperAdmins = append(opts.SuperAdmins, superAdmins...)
	}
}

// New creates a new IAM object.
// ALIS_OS_PROJECT environment variable must be set.
// ALIS_PRODUCT_CONFIG environment variable must be set if the product_config parameter is not set in Parameter Manager(https://console.cloud.google.com/security/parametermanager/locations/global/parameters/product_config).
func New(opts ...IamOption) (*IAM, error) {
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

	// extract roles from product config
	var productConfigStr string
	if os.Getenv("ALIS_PRODUCT_CONFIG") != "" {
		productConfigStr = os.Getenv("ALIS_PRODUCT_CONFIG")
	} else {
		// Initiate parameter manager client
		parameterManagerClient, err := parametermanager.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("error creating parameter manager client: %v", err)
		}
		defer parameterManagerClient.Close()

		// List available versions
		// Only get the latest version
		versionsIt := parameterManagerClient.ListParameterVersions(ctx, &parametermanagerpb.ListParameterVersionsRequest{
			Parent:  fmt.Sprintf("projects/%s/locations/global/parameters/product_config", projectId),
			OrderBy: "name desc",
		})
		for {
			parameterVersion, err := versionsIt.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("error getting parameter version: %v", err)
			}

			// Get the latest version
			// We have to explicitly get the version because the payload is not returned by ListParameterVersions
			// TODO: Change after this is fixed
			version, err := parameterManagerClient.GetParameterVersion(ctx, &parametermanagerpb.GetParameterVersionRequest{
				Name: parameterVersion.GetName(),
			})
			if err != nil {
				return nil, fmt.Errorf("error getting parameter version: %v", err)
			}

			productConfigBytes := version.GetPayload().GetData()
			if len(productConfigBytes) > 0 {
				productConfigStr = string(productConfigBytes)
				break
			}
		}
	}

	// If product config is still empty, return error
	if productConfigStr == "" {
		alog.Fatal(ctx, "ALIS_PRODUCT_CONFIG environment variable or product_config parameter is required")
	}
	productConfigBytes, err := base64.StdEncoding.DecodeString(productConfigStr)
	if err != nil {
		alog.Fatalf(ctx, "error base64 decoding product config: %v", err)
	}
	productConfig := &openConfig.ProductConfig{}
	err = proto.Unmarshal(productConfigBytes, productConfig)
	if err != nil {
		alog.Fatalf(ctx, "error proto unmarshalling product config: %v", err)
	}
	roles := productConfig.Roles

	// create IAM object
	i := &IAM{
		deploymentServiceAccountEmail: deploymentServiceAccount,
		roles:                         roles,
		memberResolver:                make(map[string](func(ctx context.Context, groupType string, groupId string, rpcAuthz *Authorizer) bool)),
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

	// create users client if not disabled
	if !options.WithoutDefaultUsersClient {
		maxSizeOptions := grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(2000000000), grpc.MaxCallRecvMsgSize(2000000000))
		conn, err := client.NewConnWithRetry(ctx, "iam-users-"+os.Getenv("ALIS_RUN_HASH")+".run.app:443", false, maxSizeOptions)
		if err != nil {
			return nil, fmt.Errorf("error creating users client: %v", err)
		}
		i.UsersClient = openIam.NewUsersServiceClient(conn)
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

// WithMemberResolver registers a function to resolve whether a requester is a member of a group.
// There can be multiple different types of groups, e.g. "team:engineering" (groupType = "team",groupId="engineering")
// A group always has a type, but does not always have an id, e.g. "team:engineering" (groupType = "team",groupId="engineering") vs "all" (groupType = "all",groupId="").
// "user" and "serviceAccounts" are not allowed as group types.
// "domain" is a builtin group type that is resolved by checking if the requester's email ends with the group id.
// Results are cached per Authorizer.
func (s *IAM) WithMemberResolver(groupTypes []string, resolver func(ctx context.Context, groupType string, groupId string, principal *Authorizer) bool) *IAM {
	for _, groupType := range groupTypes {
		if groupType == "user" || groupType == "serviceAccount" || groupType == "domain" {
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
