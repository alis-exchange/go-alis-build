package authz

import (
	"context"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/grpc/metadata"
)

func TestAuthz_Authorize(t *testing.T) {
	playIcBearer := "Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6ImFkZjVlNzEwZWRmZWJlY2JlZmE5YTYxNDk1NjU0ZDAzYzBiOGVkZjgiLCJ0eXAiOiJKV1QifQ.eyJhdWQiOiJodHRwczovL3Jlc291cmNlcy1tYXBzLXYxLWRtZXFsYngzcmEtZXcuYS5ydW4uYXBwIiwiYXpwIjoiYWxpcy1idWlsZEBwbGF5LWljLWRldi1sZ3AuaWFtLmdzZXJ2aWNlYWNjb3VudC5jb20iLCJlbWFpbCI6ImFsaXMtYnVpbGRAcGxheS1pYy1kZXYtbGdwLmlhbS5nc2VydmljZWFjY291bnQuY29tIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsImV4cCI6MTcxMTYxNDgwMCwiaWF0IjoxNzExNjExMjAwLCJpc3MiOiJodHRwczovL2FjY291bnRzLmdvb2dsZS5jb20iLCJzdWIiOiIxMDM3MjA4Mjg4ODEyOTg4NzIyODgifQ.SIGNATURE_REMOVED_FOR_TESTING"
	playMcBearer := "bearer eyJhbGciOiJSUzI1NiIsImtpZCI6ImFkZjVlNzEwZWRmZWJlY2JlZmE5YTYxNDk1NjU0ZDAzYzBiOGVkZjgiLCJ0eXAiOiJKV1QifQ.eyJhdWQiOiIzMjU1NTk0MDU1OS5hcHBzLmdvb2dsZXVzZXJjb250ZW50LmNvbSIsImF6cCI6ImFsaXMtYnVpbGRAcGxheS1tYy1kZXYtNHBlLmlhbS5nc2VydmljZWFjY291bnQuY29tIiwiZW1haWwiOiJhbGlzLWJ1aWxkQHBsYXktbWMtZGV2LTRwZS5pYW0uZ3NlcnZpY2VhY2NvdW50LmNvbSIsImVtYWlsX3ZlcmlmaWVkIjp0cnVlLCJleHAiOjE3MTE2MTYzNDcsImlhdCI6MTcxMTYxMjc0NywiaXNzIjoiaHR0cHM6Ly9hY2NvdW50cy5nb29nbGUuY29tIiwic3ViIjoiMTA5NzY0Njc5NzYyMjIxOTIwMzk0In0.SIGNATURE_REMOVED_FOR_TESTING"

	espv2ForwardedAuthCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", playIcBearer, "x-forwarded-authorization", playMcBearer))

	viewerRole := &Role{
		Name: "testorg.aa.testservices.v1/Viewer",
		Permissions: []string{
			"/testorg.aa.testservices.v1/GetTest",
			"/testorg.aa.testservices.v1/ListTests",
		},
		Extends: []string{},
	}
	editorRole := &Role{
		Name: "testorg.aa.testservices.v1/Editor",
		Permissions: []string{
			"/testorg.aa.testservices.v1/CreateTest",
			"/testorg.aa.testservices.v1/UpdateTest",
			"/testorg.aa.testservices.v1/DeleteTest",
		},
		Extends: []string{"testorg.aa.testservices.v1/Viewer"},
	}
	adminRole := &Role{
		Name: "testorg.aa.testservices.v1/Admin",
		Permissions: []string{
			"/testorg.aa.testservices.v1/SetIamPolicy",
			"/testorg.aa.testservices.v1/GetIamPolicy",
		},
		Extends: []string{"testorg.aa.testservices.v1/Editor"},
	}
	roles := []*Role{viewerRole, editorRole, adminRole}
	authz := New(roles).WithSuperAdmins([]string{"serviceAccount:103720828881298872288"})
	type args struct {
		ctx        context.Context
		permission string
		policies   []*iampb.Policy
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Can get",
			args: args{
				ctx:        espv2ForwardedAuthCtx,
				permission: "/testorg.aa.testservices.v1/GetTest",
				policies: []*iampb.Policy{
					{
						Bindings: []*iampb.Binding{
							{
								Role:    "testorg.aa.testservices.v1/Viewer",
								Members: []string{"serviceAccount:109764679762221920394"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Cannot create",
			args: args{
				ctx:        espv2ForwardedAuthCtx,
				permission: "/testorg.aa.testservices.v1/CreateTest",
				policies: []*iampb.Policy{
					{
						Bindings: []*iampb.Binding{
							{
								Role:    "testorg.aa.testservices.v1/Viewer",
								Members: []string{"serviceAccount:109764679762221920394", "user:921374742194912812341"},
							},
							{
								Role:    "testorg.aa.testservices.v1/Editor",
								Members: []string{"user:1234567890", "serviceaccount:239238091230492134"},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authInfo, err := authz.Authorize(tt.args.ctx, tt.args.permission, tt.args.policies, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Authz.Authorize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Logf("authInfo: %+v", authInfo)
		})
	}
}

func TestAuthz_isMember(t *testing.T) {
	type Cache struct {
		someString string
		someInt    int
	}
	az := New([]*Role{}).WithMemberResolver("builders", func(ctx context.Context, groupType string, groupId string, authInfo *AuthInfo, cache interface{}) (bool, error) {
		// edit resolver data
		if rd, ok := cache.(*Cache); ok {
			println("Editing resolver data")
			rd.someString = "edited"
			rd.someInt = 456
		}
		if groupId == "" {
			return true, nil
		} else if groupId == "danielGroup" && authInfo.PolicyMember == "user:123" {
			return true, nil
		} else if groupId == "janGroup" && authInfo.PolicyMember == "user:456" {
			return true, nil
		}

		return false, nil
	})
	az.WithPolicyReader(func(ctx context.Context, policyName string, cache interface{}) (*iampb.Policy, error) {
		return nil, nil
	})
	memberCache := map[string]bool{}

	cache := &Cache{}
	isMember, err := az.IsMember(context.Background(), &AuthInfo{PolicyMember: "user:123"}, "builders:danielGroup", memberCache, cache)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !isMember {
		t.Errorf("Expected true, got false")
	}

	isMember, err = az.IsMember(context.Background(), &AuthInfo{PolicyMember: "user:123"}, "builders:janGroup", memberCache, cache)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if isMember {
		t.Errorf("Expected false, got true")
	}

	isMember, err = az.IsMember(context.Background(), &AuthInfo{PolicyMember: "user:123"}, "builders", memberCache, cache)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !isMember {
		t.Errorf("Expected true, got false")
	}

	// do danielGroup again to check caching
	isMember, err = az.IsMember(context.Background(), &AuthInfo{PolicyMember: "user:123"}, "builders:danielGroup", memberCache, cache)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !isMember {
		t.Errorf("Expected true, got false")
	}

	// do builders again to check caching
	isMember, err = az.IsMember(context.Background(), &AuthInfo{PolicyMember: "user:123"}, "builders", memberCache, cache)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !isMember {
		t.Errorf("Expected true, got false")
	}

	// now do user:456
	isMember, err = az.IsMember(context.Background(), &AuthInfo{PolicyMember: "user:456"}, "builders:janGroup", memberCache, cache)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !isMember {
		t.Errorf("Expected true, got false")
	}

	isMember, err = az.IsMember(context.Background(), &AuthInfo{PolicyMember: "user:456"}, "builders:danielGroup", memberCache, cache)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if isMember {
		t.Errorf("Expected false, got true")
	}
}
