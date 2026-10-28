package iam

import (
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestEAPSurvivesIdentityForwarding(t *testing.T) {
	for _, tc := range []struct {
		name    string
		claim   any
		present bool
		want    bool
	}{
		{name: "legacy"},
		{name: "opted out", claim: false, present: true},
		{name: "opted in", claim: true, present: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := map[string]any{"sub": "123", "email": "person@example.com"}
			if tc.present {
				claims["eap"] = tc.claim
			}
			identity, err := FromJWT(testJWT(t, claims))
			if err != nil {
				t.Fatal(err)
			}
			if identity.IsEAP() != tc.want {
				t.Fatalf("IsEAP = %v, want %v", identity.IsEAP(), tc.want)
			}
			decoded, err := Unmarshal(identity.Marshal())
			if err != nil {
				t.Fatal(err)
			}
			if decoded.IsEAP() != tc.want {
				t.Fatal("marshal lost EAP")
			}
			out := identity.OutgoingMetadata(t.Context())
			md, _ := metadata.FromOutgoingContext(out)
			forwarded, err := FromIncomingMetadata(metadata.NewIncomingContext(t.Context(), md))
			if err != nil {
				t.Fatal(err)
			}
			if forwarded.IsEAP() != tc.want {
				t.Fatal("metadata forwarding lost EAP")
			}
			forwarded, err = FromJWT(identity.UnsignedJWT(t.Context()))
			if err != nil {
				t.Fatal(err)
			}
			if forwarded.IsEAP() != tc.want {
				t.Fatal("JWT forwarding lost EAP")
			}
		})
	}
	if (*Identity)(nil).IsEAP() {
		t.Fatal("nil identity must not be EAP")
	}
}

func TestEAPRejectsNonBooleanClaim(t *testing.T) {
	for _, claim := range []any{"true", 1} {
		if _, err := FromJWT(testJWT(t, map[string]any{"eap": claim})); err == nil {
			t.Fatalf("accepted non-boolean eap: %v", claim)
		}
	}
}
