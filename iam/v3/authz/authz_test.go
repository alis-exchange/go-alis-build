package authz

import (
	"encoding/base64"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	auth "go.alis.build/iam/v3"
	"google.golang.org/protobuf/proto"
)

var testIdentity = &auth.Identity{
	Type:     auth.User,
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

func TestMemberResolvers(t *testing.T) {
	AddMemberResolver([]string{"account"}, func(identity *auth.Identity, member *Member) bool {
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
