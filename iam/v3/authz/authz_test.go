package authz

import (
	"encoding/base64"
	"slices"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"go.alis.build/iam/v3"
	"google.golang.org/protobuf/proto"
)

var testIdentity = &iam.Identity{
	Type:     iam.User,
	ID:       "1934872948",
	Email:    "john@example.com",
	GroupIDs: []string{"df913r888"},
}

func init() {
	identityPolicy := &iampb.Policy{
		Bindings: []*iampb.Binding{
			{
				Role: "roles/viewer",
				Members: []string{
					"user:" + testIdentity.ID,
				},
			},
		},
	}
	marshaledPolicy, err := proto.Marshal(identityPolicy)
	if err != nil {
		panic(err)
	}
	testIdentity.Policy = base64.StdEncoding.EncodeToString(marshaledPolicy)
}

func TestHasRoleFromIdentity(t *testing.T) {
	testAZ := MustNew(testIdentity)
	if !testAZ.HasRole([]string{"roles/admin", "roles/editor", "roles/viewer"}) {
		t.Fatal("expected to have role 'roles/viewer'")
	}
	if testAZ.HasRole([]string{"roles/admin", "roles/editor"}) {
		t.Fatal("expected not to have role 'roles/admin' or 'roles/editor'")
	}
}

func TestHasRoleFromPolicy(t *testing.T) {
	testAZ := MustNew(testIdentity)
	testAZ.AddRolesFromPolicies(&iampb.Policy{
		Bindings: []*iampb.Binding{
			{
				Role: "roles/admin",
				Members: []string{
					"user:12345678",
					"serviceAccount:alis-build@my-project.iam.gserviceaccount.com",
				},
			},
			{
				Role: "roles/editor",
				Members: []string{
					"serviceAccount:alis-build@my-project.iam.gserviceaccount.com",
					"user:" + testIdentity.ID,
				},
			},
		},
	})
	if testAZ.HasRole([]string{"roles/admin"}) {
		t.Fatal("expected not to have role 'roles/admin'")
	}
	if !testAZ.HasRole([]string{"roles/editor"}) {
		t.Fatal("expected to have role 'roles/editor'")
	}
}

func TestHasRoleFromOnceOffPolicy(t *testing.T) {
	testAZ := MustNew(testIdentity)
	onceOffPolicy := &iampb.Policy{
		Bindings: []*iampb.Binding{
			{
				Role: "roles/admin",
				Members: []string{
					"user:12345678",
					"serviceAccount:alis-build@my-project.iam.gserviceaccount.com",
				},
			},
			{
				Role: "roles/editor",
				Members: []string{
					"serviceAccount:alis-build@my-project.iam.gserviceaccount.com",
					"user:" + testIdentity.ID,
				},
			},
		},
	}
	if !testAZ.HasRole([]string{"roles/editor"}, onceOffPolicy) {
		t.Fatal("expected to have role 'roles/editor'")
	}
	if testAZ.HasRole([]string{"roles/editor"}) {
		t.Fatal("expected not to have role 'roles/editor'")
	}
}

func TestHasRoleForAdmin(t *testing.T) {
	admin := &iam.Identity{
		Type:  iam.User,
		ID:    "admin-user-id",
		Email: "role-admin@example.com",
	}
	iam.AddAdminEmail(admin.Email)

	testAZ := MustNew(admin)
	if !testAZ.HasRole([]string{"roles/admin"}) {
		t.Fatal("expected admin identity to have role 'roles/admin'")
	}
	if admin.Type != iam.User {
		t.Fatalf("expected admin identity to remain user, got %v", admin.Type)
	}
}

func TestMemberResolvers(t *testing.T) {
	AddMemberResolver([]string{"account"}, func(identity *iam.Identity, member *Member) bool {
		switch member.ID {
		case "abc":
			return true
		}
		return false
	})
	testAZ := MustNew(testIdentity)
	if !testAZ.HasRole([]string{"roles/admin"}, &iampb.Policy{
		Bindings: []*iampb.Binding{
			{
				Role: "roles/admin",
				Members: []string{
					"account:abc",
				},
			},
		},
	}) {
		t.Fatal("expected to have role 'roles/admin'")
	}
	if testAZ.HasRole([]string{"roles/admin"}, &iampb.Policy{
		Bindings: []*iampb.Binding{
			{
				Role: "roles/admin",
				Members: []string{
					"account:def",
				},
			},
		},
	}) {
		t.Fatal("expected not to have role 'roles/admin'")
	}
}

// restrictedCopy returns a copy of testIdentity marked restricted. The shared
// testIdentity must not be mutated: registration state in this package is
// process global and tests run against the same value.
func restrictedCopy() *iam.Identity {
	identity := *testIdentity
	identity.Restricted = true
	return &identity
}

func TestHasRoleOpenRoleRestricted(t *testing.T) {
	identity := &iam.Identity{Type: iam.User, ID: "restricted-zero-role", Restricted: true}
	if MustNew(identity).HasRole([]string{"roles/open"}) {
		t.Fatal("expected restricted identity not to have open role 'roles/open'")
	}

	// The same identity without the claim still gets the open role.
	identity.Restricted = false
	if !MustNew(identity).HasRole([]string{"roles/open"}) {
		t.Fatal("expected unrestricted identity to have open role 'roles/open'")
	}
}

func TestHasRoleForRestrictedAdmin(t *testing.T) {
	admin := &iam.Identity{
		Type:       iam.User,
		ID:         "restricted-role-admin-id",
		Email:      "restricted-role-admin@example.com",
		Restricted: true,
	}
	iam.AddAdminEmail(admin.Email)

	testAZ := MustNew(admin)
	if testAZ.HasRole([]string{"roles/admin"}) {
		t.Fatal("expected restricted admin not to have role 'roles/admin'")
	}
	if testAZ.HasRole([]string{"roles/open"}) {
		t.Fatal("expected restricted admin not to have open role 'roles/open'")
	}

	// Roles the credential explicitly carries still count.
	if !testAZ.HasRole([]string{"roles/editor"}, &iampb.Policy{
		Bindings: []*iampb.Binding{
			{Role: "roles/editor", Members: []string{"user:" + admin.ID}},
		},
	}) {
		t.Fatal("expected restricted admin to have role 'roles/editor' from the supplied policy")
	}
	testAZ.AddRoles("roles/viewer")
	if !testAZ.HasRole([]string{"roles/viewer"}) {
		t.Fatal("expected restricted admin to have added role 'roles/viewer'")
	}
	if admin.Type != iam.User {
		t.Fatalf("expected restricted admin to remain user, got %v", admin.Type)
	}

	// Without the claim the admin bypass is unchanged.
	admin.Restricted = false
	if !MustNew(admin).HasRole([]string{"roles/admin"}) {
		t.Fatal("expected unrestricted admin to have role 'roles/admin'")
	}
}

func TestHasRoleForRestrictedSystem(t *testing.T) {
	system := &iam.Identity{Type: iam.System, ID: "system", Restricted: true}
	testAZ := MustNew(system)
	if testAZ.HasRole([]string{"roles/admin"}) {
		t.Fatal("expected restricted system identity not to have role 'roles/admin'")
	}
	testAZ.AddRoles("roles/admin")
	if !testAZ.HasRole([]string{"roles/admin"}) {
		t.Fatal("expected restricted system identity to have added role 'roles/admin'")
	}
	if !MustNew(iam.SystemIdentity).HasRole([]string{"roles/admin"}) {
		t.Fatal("expected the system identity to have role 'roles/admin'")
	}
}

func TestHasRoleRestrictedWithPolicyRole(t *testing.T) {
	// Restriction removes the bypasses, not the roles the credential carries.
	testAZ := MustNew(restrictedCopy())
	if !testAZ.HasRole([]string{"roles/viewer"}) {
		t.Fatal("expected restricted identity to keep role 'roles/viewer' from its policy claim")
	}
	if testAZ.HasRole([]string{"roles/admin"}) {
		t.Fatal("expected restricted identity not to have role 'roles/admin'")
	}
}

// TestAuthzRolesAreCarriedNotEnforced pins the non-goal: the library carries the
// allowlist, the consuming service enforces it. Enforcing it here would need to
// be a deliberate change to this test.
func TestAuthzRolesAreCarriedNotEnforced(t *testing.T) {
	identity := *testIdentity
	identity.AuthzRoles = []string{}
	if !MustNew(&identity).HasRole([]string{"roles/viewer"}) {
		t.Fatal("expected authz to ignore AuthzRoles and keep role 'roles/viewer'")
	}
}

// TestHasRoleDoesNotRetainOnceOffRoles checks that a once-off policy role does
// not land in the authorizer's own role slice. Appending into its spare
// capacity would let concurrent checks that share an Authorizer, such as
// per-row policy checks in a List method, overwrite each other's roles.
func TestHasRoleDoesNotRetainOnceOffRoles(t *testing.T) {
	testAZ := MustNew(testIdentity)
	testAZ.AddRoles("roles/viewer", "roles/editor", "roles/owner")
	testAZ.roles = testAZ.roles[:1] // leave spare capacity, as a filtered slice would

	onceOff := &iampb.Policy{
		Bindings: []*iampb.Binding{
			{Role: "roles/admin", Members: []string{"user:" + testIdentity.ID}},
		},
	}
	if !testAZ.HasRole([]string{"roles/admin"}, onceOff) {
		t.Fatal("expected the once-off policy to grant role 'roles/admin'")
	}
	if len(testAZ.roles) != 1 || slices.Contains(testAZ.roles, "roles/admin") {
		t.Fatalf("expected once-off role not to be retained, got %v", testAZ.roles)
	}
	if cap(testAZ.roles) > 1 && testAZ.roles[:cap(testAZ.roles)][1] == "roles/admin" {
		t.Fatal("once-off role was written into the authorizer's spare capacity")
	}
}
