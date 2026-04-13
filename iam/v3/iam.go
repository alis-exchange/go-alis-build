package iam

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
)

// Role represents an IAM role and the permissions it grants.
type Role struct {
	Name        string
	Permissions []string
	AllUsers    bool
}

// PolicyFetcher loads an IAM policy for a resource.
type PolicyFetcher interface {
	GetPolicy(ctx context.Context, resource string) (*iampb.Policy, error)
}

// PolicyFetcherFunc adapts a function into a PolicyFetcher.
type PolicyFetcherFunc func(ctx context.Context, resource string) (*iampb.Policy, error)

func (f PolicyFetcherFunc) GetPolicy(ctx context.Context, resource string) (*iampb.Policy, error) {
	return f(ctx, resource)
}

// Option configures an IAM instance during construction.
type Option func(*options)

type options struct {
	deploymentServiceAccountEmail string
	superAdmins                   []string
	policyFetcher                 PolicyFetcher
}

type IAM struct {
	deploymentServiceAccountEmail string
	superAdmins                   map[string]bool
	roles                         []*Role
	rolePermissionMap             map[string]map[string]bool
	openPermissions               map[string]bool
	memberResolver                map[string]func(ctx context.Context, groupType string, groupID string, authz *Authorizer) bool
	policyFetcher                 PolicyFetcher
	disabled                      bool
}

// WithDeploymentServiceAccount overrides the deployment service account email
// used to identify the trusted internal caller.
func WithDeploymentServiceAccount(email string) Option {
	return func(opts *options) {
		opts.deploymentServiceAccountEmail = email
	}
}

// WithProjectID derives the deployment service account from a project ID using
// the default Alis service account naming convention.
func WithProjectID(projectID string) Option {
	return func(opts *options) {
		if projectID == "" {
			return
		}
		opts.deploymentServiceAccountEmail = fmt.Sprintf("alis-build@%s.iam.gserviceaccount.com", projectID)
	}
}

// WithPolicyFetcher configures how user or resource policies are loaded when
// they are not already embedded on the caller identity.
func WithPolicyFetcher(fetcher PolicyFetcher) Option {
	return func(opts *options) {
		opts.policyFetcher = fetcher
	}
}

// WithAdditionalSuperAdmins adds principals that should be trusted as internal
// callers with super-admin access.
func WithAdditionalSuperAdmins(superAdmins ...string) Option {
	return func(opts *options) {
		opts.superAdmins = append(opts.superAdmins, superAdmins...)
	}
}

// New constructs a new IAM instance with the supplied roles and options.
func New(roles []*Role, opts ...Option) (*IAM, error) {
	options := &options{}
	for _, opt := range opts {
		opt(options)
	}

	if options.deploymentServiceAccountEmail == "" {
		projectID := os.Getenv("ALIS_OS_PROJECT")
		if projectID == "" {
			return nil, fmt.Errorf("deployment service account not configured and ALIS_OS_PROJECT not set")
		}
		options.deploymentServiceAccountEmail = fmt.Sprintf("alis-build@%s.iam.gserviceaccount.com", projectID)
	}

	iam := &IAM{
		deploymentServiceAccountEmail: options.deploymentServiceAccountEmail,
		superAdmins:                   map[string]bool{},
		roles:                         roles,
		rolePermissionMap:             map[string]map[string]bool{},
		openPermissions:               map[string]bool{},
		memberResolver:                map[string]func(ctx context.Context, groupType string, groupID string, authz *Authorizer) bool{},
		policyFetcher:                 options.policyFetcher,
	}

	defaultSuperAdmin := "serviceAccount:" + options.deploymentServiceAccountEmail
	iam.superAdmins[defaultSuperAdmin] = true
	for _, superAdmin := range options.superAdmins {
		iam.superAdmins[superAdmin] = true
	}

	for _, role := range roles {
		if role == nil {
			continue
		}
		roleName := ensureCorrectRoleName(role.Name)
		permissions := map[string]bool{}
		for _, permission := range role.Permissions {
			permissions[permission] = true
			if role.AllUsers {
				iam.openPermissions[permission] = true
			}
		}
		iam.rolePermissionMap[roleName] = permissions
	}

	return iam, nil
}

// RoleHasPermission reports whether a role grants the supplied permission.
func (i *IAM) RoleHasPermission(role string, permission string) bool {
	role = ensureCorrectRoleName(role)
	permissions, ok := i.rolePermissionMap[role]
	if !ok {
		return false
	}
	return permissions[permission]
}

// Disable turns off authorization checks for the IAM instance.
func (i *IAM) Disable() {
	i.disabled = true
}

// WithMemberResolver registers a custom group-membership resolver for one or
// more non-builtin group types.
func (i *IAM) WithMemberResolver(groupTypes []string, resolver func(ctx context.Context, groupType string, groupID string, authz *Authorizer) bool) *IAM {
	for _, groupType := range groupTypes {
		switch groupType {
		case groupTypeUser, groupTypeServiceAccount, groupTypeDomain, groupTypeGroup:
			panic(fmt.Sprintf("cannot register builtin group type %s", groupType))
		}
		i.memberResolver[groupType] = resolver
	}
	return i
}

func ensureCorrectRoleName(role string) string {
	if strings.HasPrefix(role, "roles/") {
		return role
	}
	roleParts := strings.Split(role, "/")
	return "roles/" + roleParts[len(roleParts)-1]
}
