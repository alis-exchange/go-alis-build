package iam

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
)

// identityFromClaims builds the identity a request carrying an unsigned token
// with these claims would produce.
func identityFromClaims(t *testing.T, claims map[string]any) *Identity {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	token := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(AuthHeader, "Bearer "+token))
	identity, err := ExtractIdentityFromCtx(ctx, "alis-build@example.iam.gserviceaccount.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

// TestCreditStanding reads the claims exactly as alis.os.iam.v2 mints them:
// a blocked standing has no "allow", and member_blocked_until is a protobuf
// Timestamp ({"seconds":…}).
func TestCreditStanding(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	nextMonth := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name    string
		active  map[string]any
		account string
		want    CreditStanding
	}{
		{"no active account", nil, "accounts/acme", CreditStanding{}},
		{"no standing received", map[string]any{"account_id": "acme"}, "accounts/acme", CreditStanding{}},
		{"another account", map[string]any{"account_id": "acme", "account_standing": map[string]any{"allow": true}}, "accounts/other", CreditStanding{}},
		{"allowed", map[string]any{"account_id": "acme", "account_standing": map[string]any{"allow": true}}, "acme", CreditStanding{Known: true, Allowed: true}},
		{"blocked arrives without allow", map[string]any{"account_id": "acme", "account_standing": map[string]any{"reason": "ACCOUNT_ARCHIVED"}}, "accounts/acme",
			CreditStanding{Known: true, Reason: "ACCOUNT_ARCHIVED", Message: "This account is archived."}},
		{"member blocked", map[string]any{"account_id": "acme", "account_standing": map[string]any{"allow": true}, "member_blocked_until": map[string]any{"seconds": nextMonth.Unix()}}, "accounts/acme",
			CreditStanding{Known: true, Reason: MemberCreditLimitReached, Message: creditMessage(MemberCreditLimitReached), BlockedUntil: nextMonth}},
		{"member block lapsed", map[string]any{"account_id": "acme", "account_standing": map[string]any{"allow": true}, "member_blocked_until": map[string]any{"seconds": now.Add(-time.Hour).Unix()}}, "accounts/acme",
			CreditStanding{Known: true, Allowed: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			claims := map[string]any{"sub": "123", "email": "jan@example.com"}
			if tt.active != nil {
				claims["active_account"] = tt.active
			}
			got := identityFromClaims(t, claims).CreditStanding(tt.account, now)
			if got.Known != tt.want.Known || got.Allowed != tt.want.Allowed || got.Reason != tt.want.Reason ||
				got.Message != tt.want.Message || !got.BlockedUntil.Equal(tt.want.BlockedUntil) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
