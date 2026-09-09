package iam

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/metadata"
)

func expect[T comparable](t *testing.T, got, expected T) {
	if got != expected {
		t.Fatalf("got %v, expected %v", got, expected)
	}
}

func expectSlice[T comparable](t *testing.T, got, expected []T) {
	if len(got) != len(expected) {
		t.Fatalf("got %v, expected %v", got, expected)
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("got %v, expected %v", got, expected)
		}
	}
}

// expectNil guards the nil versus empty contract for AuthzRoles. expectSlice
// cannot tell the two apart, and the difference is the whole safety property:
// nil means unrestricted, empty means restricted to nothing.
func expectNil(t *testing.T, got []string, wantNil bool) {
	t.Helper()
	if (got == nil) != wantNil {
		t.Fatalf("got %#v, expected nil=%v", got, wantNil)
	}
}

// testJWT builds an unsigned {hdr}.{payload}.{sig} token so a test can express
// an absent claim, a null claim and an empty claim exactly.
func testJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".###"
}

var testIdentity = &Identity{
	Type:     User,
	ID:       "1934872948",
	Email:    "john@example.com",
	GroupIDs: []string{"df913r888"},
}

func TestContext(t *testing.T) {
	ctx := testIdentity.Context(t.Context())
	identity := MustFromContext(ctx)
	expect(t, identity.Type, testIdentity.Type)
	expect(t, identity.ID, testIdentity.ID)
	expect(t, identity.Email, testIdentity.Email)
	expect(t, identity.Policy, testIdentity.Policy)
	expectSlice(t, identity.GroupIDs, testIdentity.GroupIDs)
}

func TestMarshal(t *testing.T) {
	data := testIdentity.Marshal()
	identity := MustUnmarshal(data)
	expect(t, identity.Type, testIdentity.Type)
	expect(t, identity.ID, testIdentity.ID)
	expect(t, identity.Email, testIdentity.Email)
	expect(t, identity.Policy, testIdentity.Policy)
	expectSlice(t, identity.GroupIDs, testIdentity.GroupIDs)
}

func TestUser(t *testing.T) {
	expect(t, testIdentity.User(), "users/1934872948")

	serviceAccount := &Identity{
		Type:  ServiceAccount,
		ID:    "110273892410436981091",
		Email: "alis-build@alis-ge-prod-qg6.iam.gserviceaccount.com",
	}
	expect(t, serviceAccount.User(), "")
}

func TestAdminEmail(t *testing.T) {
	admin := &Identity{
		Type:  User,
		ID:    "admin-user-id",
		Email: "admin@example.com",
	}
	expect(t, admin.IsAdmin(), false)
	expect(t, admin.IsPrivileged(), false)

	AddAdminEmail(admin.Email)

	expect(t, admin.IsAdmin(), true)
	expect(t, admin.IsPrivileged(), true)
	expect(t, admin.IsSystem(), false)
	expect(t, admin.Type, User)
	expect(t, admin.PolicyMember(), "user:admin-user-id")
	expect(t, admin.User(), "users/admin-user-id")
}

func TestMetadata(t *testing.T) {
	ctx := testIdentity.OutgoingMetadata(t.Context())
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("outgoing metadata not found")
	}
	ctx = metadata.NewIncomingContext(t.Context(), md)
	identity := MustFromIncomingMetadata(ctx)
	expect(t, identity.Type, testIdentity.Type)
	expect(t, identity.ID, testIdentity.ID)
	expect(t, identity.Email, testIdentity.Email)
	expect(t, identity.Policy, testIdentity.Policy)
	expectSlice(t, identity.GroupIDs, testIdentity.GroupIDs)
}

func TestUserJWT(t *testing.T) {
	testAccessToken := "eyJhbGciOiJSUzI1NiIsImtpZCI6IjIwMjYtMDQtMTQiLCJ0eXAiOiJKV1QifQ.eyJhY2NvdW50cyI6eyIyaXdwZ2giOnsic2VhdHMiOnsiMiI6eyJwbGFuIjoyLCJzZWF0IjoxfX19LCIzaXF3NnoiOnsic2VhdHMiOnsiMiI6eyJwbGFuIjo1LCJzZWF0IjoxfX19LCI0aHQ4MnMiOnsic2VhdHMiOnsiMSI6eyJwbGFuIjoxLCJzZWF0IjoxfX19LCI1eGx6ajcxIjp7InNlYXRzIjp7IjEiOnsicGxhbiI6Miwic2VhdCI6MX0sIjIiOnsicGxhbiI6Miwic2VhdCI6MX19fSwiNmdzdnZmIjp7InNlYXRzIjp7IjIiOnsicGxhbiI6NSwic2VhdCI6MX19fSwiN3J2ZmZxIjp7InNlYXRzIjp7IjEiOnsicGxhbiI6MSwic2VhdCI6M319fSwiZ2EyYmMyMSI6eyJzZWF0cyI6eyIxIjp7InBsYW4iOjQsInNlYXQiOjF9LCIyIjp7InBsYW4iOjQsInNlYXQiOjF9fX0sImlmN3cxODEiOnsic2VhdHMiOnsiMSI6eyJwbGFuIjoyLCJzZWF0IjozfX19fSwiYXBwIjoiIiwiYXVkIjoidG9kbyIsImVtYWlsIjoiZGFuaWVsLnZhbi5uaWVrZXJrQGFsaXN4LmNvbSIsImV4cCI6MTc3NjE2NzczNywiZ3JvdXBzIjpbIjI5YzZmODVmLTY1YmQtNDBlZi1iMjY0LWFmOTc0NDc3M2IzZiIsIjA1YTZiMTM2LTNiZDMtNDI0Ni1iYTQyLTBlOGRjODdiNWFlOSJdLCJpc3MiOiJodHRwczovL2lkZW50aXR5LmFsaXN4LmNvbSIsInBvbGljeSI6IkdpUTJNR000WldVeE9TMWhOalUyTFRSbE5UTXRZakUxTnkwd1ltWXlPRGxoTnprNU16TWlNQW9TY205c1pYTXZhV1JsWVM1amNtVmhkRzl5RWhwMWMyVnlPakV3T0RNd01UWXpNRGszTURVd016azVNakEwTVE9PSIsInNjb3BlcyI6bnVsbCwic3ViIjoiMTA4MzAxNjMwOTcwNTAzOTkyMDQxIn0.####"
	identity, err := FromJWT(testAccessToken)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, identity.Type, User)
	expect(t, identity.ID, "108301630970503992041")
	expect(t, identity.Email, "daniel.van.niekerk@alisx.com")
	expectSlice(t, identity.GroupIDs, []string{
		"29c6f85f-65bd-40ef-b264-af9744773b3f",
		"05a6b136-3bd3-4246-ba42-0e8dc87b5ae9",
	})
	expect(t, identity.PolicyMember(), "user:108301630970503992041")

	// A token issued before the scoped credential claims existed is unrestricted.
	expectNil(t, identity.AuthzRoles, true)
	expect(t, identity.Restricted, false)
}

func TestServiceAccountJWT(t *testing.T) {
	serviceAccountIDToken := "eyJhbGciOiJSUzI1NiIsImtpZCI6ImIzZDk1Yjk1ZmE0OGQxODBiODVmZmU4MDgyZmNmYTIxNzRiMDQ2NjciLCJ0eXAiOiJKV1QifQ.eyJhdWQiOiIzMjU1NTk0MDU1OS5hcHBzLmdvb2dsZXVzZXJjb250ZW50LmNvbSIsImF6cCI6ImFsaXMtYnVpbGRAYWxpcy1nZS1wcm9kLXFnNi5pYW0uZ3NlcnZpY2VhY2NvdW50LmNvbSIsImVtYWlsIjoiYWxpcy1idWlsZEBhbGlzLWdlLXByb2QtcWc2LmlhbS5nc2VydmljZWFjY291bnQuY29tIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsImV4cCI6MTc3NjE3OTU2NiwiaWF0IjoxNzc2MTc1OTY2LCJpc3MiOiJodHRwczovL2FjY291bnRzLmdvb2dsZS5jb20iLCJzdWIiOiIxMTAyNzM4OTI0MTA0MzY5ODEwOTEifQ.####"
	identity, err := FromJWT(serviceAccountIDToken)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, identity.Type, ServiceAccount)
	expect(t, identity.PolicyMember(), "serviceAccount:alis-build@alis-ge-prod-qg6.iam.gserviceaccount.com")
}

// TestAuthzRolesJSONEncoding pins the encoding of the two scoped credential
// claims. It is the tripwire for anyone adding omitempty, and for a future move
// to encoding/json/v2, which marshals a nil slice as [] and would turn every
// unrestricted identity into one restricted to nothing.
func TestAuthzRolesJSONEncoding(t *testing.T) {
	for _, tt := range []struct {
		name     string
		identity *Identity
		want     string
	}{
		{"nil roles marshal as null", &Identity{}, `"authz_roles":null`},
		{"unrestricted marshals as false", &Identity{}, `"restricted":false`},
		{"empty roles marshal as an empty array", &Identity{AuthzRoles: []string{}}, `"authz_roles":[]`},
		{"named roles marshal as an array", &Identity{AuthzRoles: []string{"roles/x"}}, `"authz_roles":["roles/x"]`},
		{"restricted marshals as true", &Identity{Restricted: true}, `"restricted":true`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.identity)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tt.want) {
				t.Fatalf("got %s, expected it to contain %s", data, tt.want)
			}
		})
	}
}

// TestFromJWTAuthzRoles checks that an absent claim and an empty claim stay
// distinguishable after parsing.
func TestFromJWTAuthzRoles(t *testing.T) {
	for _, tt := range []struct {
		name    string
		claims  map[string]any
		wantNil bool
		want    []string
	}{
		{"absent claim is unrestricted", map[string]any{}, true, nil},
		{"null claim is unrestricted", map[string]any{"authz_roles": nil}, true, nil},
		{"empty claim is restricted to nothing", map[string]any{"authz_roles": []string{}}, false, []string{}},
		{"populated claim is restricted to those roles", map[string]any{"authz_roles": []string{"roles/app.developer"}}, false, []string{"roles/app.developer"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.claims["sub"] = "1934872948"
			tt.claims["email"] = "john@example.com"
			identity, err := FromJWT(testJWT(t, tt.claims))
			if err != nil {
				t.Fatal(err)
			}
			expectNil(t, identity.AuthzRoles, tt.wantNil)
			expectSlice(t, identity.AuthzRoles, tt.want)
			expect(t, identity.Restricted, false)
			expect(t, identity.Type, User)
		})
	}
}

func TestFromJWTRestricted(t *testing.T) {
	for _, tt := range []struct {
		name   string
		claims map[string]any
		want   bool
	}{
		{"absent claim is unrestricted", map[string]any{}, false},
		{"true claim is restricted", map[string]any{"restricted": true}, true},
		{"false claim is unrestricted", map[string]any{"restricted": false}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.claims["sub"] = "1934872948"
			tt.claims["email"] = "john@example.com"
			identity, err := FromJWT(testJWT(t, tt.claims))
			if err != nil {
				t.Fatal(err)
			}
			expect(t, identity.Restricted, tt.want)
		})
	}
}

func TestRestrictedAdminIsNotPrivileged(t *testing.T) {
	admin := &Identity{
		Type:       User,
		ID:         "restricted-admin-user-id",
		Email:      "restricted-admin@example.com",
		Restricted: true,
	}
	AddAdminEmail(admin.Email)

	// The principal is still an admin; only the credential is restricted.
	expect(t, admin.IsAdmin(), true)
	expect(t, admin.IsSystem(), false)
	expect(t, admin.IsPrivileged(), false)
	expect(t, admin.Type, User)
	expect(t, admin.PolicyMember(), "user:restricted-admin-user-id")

	// The same admin without the claim keeps the bypass it has always had.
	admin.Restricted = false
	expect(t, admin.IsPrivileged(), true)
}

func TestRestrictedSystemIsNotPrivileged(t *testing.T) {
	system := &Identity{Type: System, ID: "system", Restricted: true}
	expect(t, system.IsSystem(), true)
	expect(t, system.IsPrivileged(), false)
	expect(t, SystemIdentity.IsPrivileged(), true)

	// A restricted token from a system email keeps its system type, so callers
	// that classify trust still see a system caller, but it no longer bypasses.
	email := "restricted-system@example.iam.gserviceaccount.com"
	AddSystemEmail(email)
	identity, err := FromJWT(testJWT(t, map[string]any{
		"sub":        "110273892410436981091",
		"email":      email,
		"restricted": true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	expect(t, identity.Type, System)
	expect(t, identity.IsSystem(), true)
	expect(t, identity.IsPrivileged(), false)
}

// TestScopedClaimsRoundTrip is the safety property test: the nil versus empty
// distinction has to survive every hop an identity takes between services, not
// just the initial parse.
func TestScopedClaimsRoundTrip(t *testing.T) {
	transports := map[string]func(*testing.T, *Identity) *Identity{
		"marshal": func(t *testing.T, in *Identity) *Identity {
			return MustUnmarshal(in.Marshal())
		},
		"metadata": func(t *testing.T, in *Identity) *Identity {
			ctx := in.OutgoingMetadata(t.Context())
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatal("outgoing metadata not found")
			}
			return MustFromIncomingMetadata(metadata.NewIncomingContext(t.Context(), md))
		},
		"header": func(t *testing.T, in *Identity) *Identity {
			r := httptest.NewRequest("GET", "http://example.com/", nil)
			in.AddHeader(r)
			return MustFromHeader(r)
		},
		"unsignedJWT": func(t *testing.T, in *Identity) *Identity {
			return MustFromJWT(in.UnsignedJWT(t.Context()))
		},
	}

	fixtures := []struct {
		name       string
		authzRoles []string
		restricted bool
	}{
		{"unrestricted", nil, false},
		{"restricted to nothing", []string{}, true},
		{"restricted to roles", []string{"roles/app.developer", "roles/viewer"}, true},
		{"restricted with no allowlist", nil, true},
	}

	for transportName, transport := range transports {
		for _, fixture := range fixtures {
			t.Run(transportName+"/"+fixture.name, func(t *testing.T) {
				in := &Identity{
					Type:       User,
					ID:         "1934872948",
					Email:      "john@example.com",
					AuthzRoles: fixture.authzRoles,
					Restricted: fixture.restricted,
				}
				out := transport(t, in)
				expectNil(t, out.AuthzRoles, fixture.authzRoles == nil)
				expectSlice(t, out.AuthzRoles, fixture.authzRoles)
				expect(t, out.Restricted, fixture.restricted)
			})
		}
	}
}
