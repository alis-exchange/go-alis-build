package iam

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	iamv3 "go.alis.build/iam/v3"
	"google.golang.org/grpc/metadata"
)

func TestEAPAndV3Forwarding(t *testing.T) {
	for _, tc := range []struct {
		name             string
		present, enabled bool
	}{
		{name: "legacy"}, {name: "opted out", present: true}, {name: "opted in", present: true, enabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := map[string]any{"sub": "123", "email": "person@example.com"}
			if tc.present {
				claims["eap"] = tc.enabled
			}
			body, err := json.Marshal(claims)
			if err != nil {
				t.Fatal(err)
			}
			token := "e30." + base64.RawURLEncoding.EncodeToString(body) + ".test"
			ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+token))
			identity, err := ExtractIdentityFromCtx(ctx, "deployment@project.iam.gserviceaccount.com", nil)
			if err != nil {
				t.Fatal(err)
			}
			if identity.IsEAP() != tc.enabled {
				t.Fatalf("IsEAP = %v, want %v", identity.IsEAP(), tc.enabled)
			}
			out, err := identity.ForwardV3Identity(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			md, _ := metadata.FromOutgoingContext(out)
			forwarded, err := iamv3.FromIncomingMetadata(metadata.NewIncomingContext(t.Context(), md))
			if err != nil {
				t.Fatal(err)
			}
			if forwarded.IsEAP() != tc.enabled {
				t.Fatal("v2 to v3 forwarding lost EAP")
			}
		})
	}
	if (*Identity)(nil).IsEAP() {
		t.Fatal("nil identity must not be EAP")
	}
}
