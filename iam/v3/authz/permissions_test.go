package authz

import (
	"testing"

	auth "go.alis.build/iam/v3"
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
	identity := &auth.Identity{
		Type: auth.User,
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
