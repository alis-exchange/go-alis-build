// Package authz is used to authorize whether an identity has the required role.
package authz

import (
	"encoding/base64"
	"fmt"
	"slices"

	"cloud.google.com/go/iam/apiv1/iampb"
	auth "go.alis.build/iam/v3"
	"google.golang.org/protobuf/proto"
)

type Authorizer struct {
	identity *auth.Identity
	roles    []string
}

func New(identity *auth.Identity) (*Authorizer, error) {
	var roles []string

	// extract roles from iam policy if any
	if identity.Policy != "" {
		policyBytes, err := base64.StdEncoding.DecodeString(identity.Policy)
		if err != nil {
			return nil, fmt.Errorf("decoding identity iam policy: %w", err)
		}
		if len(policyBytes) > 0 {
			policy := &iampb.Policy{}
			err = proto.Unmarshal(policyBytes, policy)
			if err != nil {
				return nil, fmt.Errorf("unmarshalling identity iam policy: %w", err)
			}
			roles = rolesFromPolicies(identity, policy)
		}
	}

	// return authorizer
	return &Authorizer{
		identity: identity,
		roles:    roles,
	}, nil
}

func MustNew(identity *auth.Identity) *Authorizer {
	authorizer, err := New(identity)
	if err != nil {
		panic(err)
	}
	return authorizer
}

// HasRole returns true if the identity has one of the specified roles (or is a sytem identity),
// considering both previously added roles and those from the provided policies.
//
// Note: Policies provided here are evaluated once and not persisted. To persist
// roles for subsequent checks (e.g., applying parent policies across multiple
// items in a List method), use AddRolesFromPolicies instead.
func (a *Authorizer) HasRole(roles []string, policies ...*iampb.Policy) bool {
	if a.identity.IsSystem() {
		return true
	}
	allRoles := append(a.roles, rolesFromPolicies(a.identity, policies...)...)
	for _, role := range roles {
		if slices.Contains(openRoles, role) {
			return true
		}
		if slices.Contains(allRoles, role) {
			return true
		}
	}
	return false
}

// AddRolesFromPolicies extracts and persists roles for the identity from the given policies.
//
// Warning: These roles will apply to all subsequent authorizer checks. For context-specific
// checks (e.g., checking individual row policies in a loop), pass policies directly
// to HasRole instead to avoid leaking permissions.
func (a *Authorizer) AddRolesFromPolicies(policies ...*iampb.Policy) {
	a.AddRoles(rolesFromPolicies(a.identity, policies...)...)
}

// AddRoles adds roles that the identity has.
func (a *Authorizer) AddRoles(roles ...string) {
	a.roles = append(a.roles, roles...)
}

func rolesFromPolicies(identity *auth.Identity, policies ...*iampb.Policy) []string {
	var roles []string
	for _, policy := range policies {
		if policy == nil {
			continue
		}
		for _, binding := range policy.Bindings {
			if isMember(identity, binding.GetMembers()) {
				roles = append(roles, binding.Role)
			}
		}
	}
	return roles
}
