package authz

import (
	"testing"

	"go.alis.build/iam/v3"
)

func init() {
	AddOpenRolePermissions("roles/open", []string{
		"/example.v1.Examples/Create",
	})
	viewerPermissions := AddRolePermissions("roles/viewer", []string{
		"/example.v1.Examples/List",
		"/example.v1.Examples/Get",
	})
	AddRolePermissions("roles/admin", append(viewerPermissions,
		"/example.v1.Examples/Delete",
		"/example.v1.Examples/Update"),
	)
}

func TestHasPermission(t *testing.T) {
	identity := &iam.Identity{
		Type: iam.User,
		ID:   "1234",
	}
	authorizer := MustNew(identity)

	// permission with open role
	if !authorizer.HasPermission("/example.v1.Examples/Create") {
		t.Errorf("expected to have permission /example.v1.Examples/Create")
	}

	// no roles yet
	if authorizer.HasPermission("/example.v1.Examples/Get") {
		t.Errorf("expected not to have permission /example.v1.Examples/Get")
	}

	// add viewer role
	authorizer.AddRoles("roles/viewer")
	if !authorizer.HasPermission("/example.v1.Examples/Get") {
		t.Errorf("expected to have permission /example.v1.Examples/Get")
	}
	if authorizer.HasPermission("/example.v1.Examples/Delete") {
		t.Errorf("expected not to have permission /example.v1.Examples/Delete")
	}

	// add admin role
	authorizer.AddRoles("roles/admin")
	if !authorizer.HasPermission("/example.v1.Examples/Delete") {
		t.Errorf("expected to have permission /example.v1.Examples/Delete")
	}
}

func TestHasPermissionForAdmin(t *testing.T) {
	admin := &iam.Identity{
		Type:  iam.User,
		ID:    "admin-user-id",
		Email: "permission-admin@example.com",
	}
	iam.AddAdminEmail(admin.Email)

	authorizer := MustNew(admin)
	if !authorizer.HasPermission("/example.v1.Examples/Delete") {
		t.Errorf("expected admin identity to have permission /example.v1.Examples/Delete")
	}
	if admin.Type != iam.User {
		t.Fatalf("expected admin identity to remain user, got %v", admin.Type)
	}
}

func TestHasPermissionForRestrictedIdentity(t *testing.T) {
	identity := &iam.Identity{Type: iam.User, ID: "restricted-1234", Restricted: true}
	authorizer := MustNew(identity)

	// The open role no longer grants its permissions.
	if authorizer.HasPermission("/example.v1.Examples/Create") {
		t.Errorf("expected restricted identity not to have permission /example.v1.Examples/Create")
	}

	// Roles the credential explicitly carries still grant theirs.
	authorizer.AddRoles("roles/viewer")
	if !authorizer.HasPermission("/example.v1.Examples/Get") {
		t.Errorf("expected restricted identity to have permission /example.v1.Examples/Get")
	}
	if authorizer.HasPermission("/example.v1.Examples/Create") {
		t.Errorf("expected restricted identity not to have permission /example.v1.Examples/Create")
	}
	if authorizer.HasPermission("/example.v1.Examples/Delete") {
		t.Errorf("expected restricted identity not to have permission /example.v1.Examples/Delete")
	}
}

func TestHasPermissionForRestrictedAdmin(t *testing.T) {
	admin := &iam.Identity{
		Type:       iam.User,
		ID:         "restricted-permission-admin-id",
		Email:      "restricted-permission-admin@example.com",
		Restricted: true,
	}
	iam.AddAdminEmail(admin.Email)

	authorizer := MustNew(admin)
	if authorizer.HasPermission("/example.v1.Examples/Delete") {
		t.Errorf("expected restricted admin not to have permission /example.v1.Examples/Delete")
	}
	if authorizer.HasPermission("/example.v1.Examples/Create") {
		t.Errorf("expected restricted admin not to have open permission /example.v1.Examples/Create")
	}
	authorizer.AddRoles("roles/admin")
	if !authorizer.HasPermission("/example.v1.Examples/Delete") {
		t.Errorf("expected restricted admin to have permission /example.v1.Examples/Delete via an added role")
	}
	if admin.Type != iam.User {
		t.Fatalf("expected restricted admin to remain user, got %v", admin.Type)
	}
}
