package iam

import (
	"context"
	"fmt"
	"strings"

	"go.alis.build/alog"
	openIam "open.alis.services/protobuf/alis/open/iam/v1"
)

// A server authorizer is setup once per grpc server and contains static information about the roles, permissions
// and functions to resolve group memberships.
type IAM struct {
	// the email of the deployment service account
	deploymentServiceAccountEmail string
	// the roles
	roles []*openIam.Role
	// the function per group type that resolves whether a requester is a member of a group
	memberResolver map[string](func(ctx context.Context, groupType string, groupId string, az *Authorizer) bool)
	// Globally disable auth
	disabled bool
}

// New creates a new IAM object which keeps track of the given roles and deployment service account email.
func New(roles []*openIam.Role, deploymentServiceAccountEmail string) (*IAM, error) {
	// check if deployment service account email is valid
	if !strings.HasSuffix(deploymentServiceAccountEmail, ".gserviceaccount.com") {
		return nil, fmt.Errorf("invalid deployment service account email: %s", deploymentServiceAccountEmail)
	}
	i := &IAM{
		deploymentServiceAccountEmail: deploymentServiceAccountEmail,
		roles:                         roles,
		memberResolver:                make(map[string](func(ctx context.Context, groupType string, groupId string, rpcAuthz *Authorizer) bool)),
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
	for _, r := range s.roles {
		if r.Name == role {
			for _, p := range r.Permissions {
				if p == permission {
					return true
				}
			}
		}
	}
	return false
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
