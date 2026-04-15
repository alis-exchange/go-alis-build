package iam

import (
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
