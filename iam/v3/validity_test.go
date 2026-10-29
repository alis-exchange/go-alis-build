package iam

import "testing"

// TestCreditValidity reads the claims as alis.os.iam.v2 mints them: valid is left
// out when there is no verdict, and false is minted as false.
func TestCreditValidity(t *testing.T) {
	for _, tt := range []struct {
		name   string
		active map[string]any
		want   Validity
		ok     bool
	}{
		{"no active account", nil, Validity{}, false},
		{"no verdict", map[string]any{"account_id": "acme"}, Validity{}, false},
		{"the old standing is not a verdict", map[string]any{"account_id": "acme", "account_standing": map[string]any{"allow": true}}, Validity{}, false},
		{"valid", map[string]any{"account_id": "acme", "valid": true}, Validity{Valid: true}, true},
		{"not valid", map[string]any{"account_id": "acme", "valid": false, "reason": "NO_BILLING_DETAILS", "comments": "This account has no billing details."},
			Validity{Reason: "NO_BILLING_DETAILS", Comments: "This account has no billing details."}, true},
	} {
		claims := map[string]any{"sub": "123", "email": "jan@example.com"}
		if tt.active != nil {
			claims["active_account"] = tt.active
		}
		identity, err := FromJWT(testJWT(t, claims))
		if err != nil {
			t.Fatal(err)
		}
		t.Run(tt.name, func(t *testing.T) {
			got, ok := identity.CreditValidity()
			if got != tt.want || ok != tt.ok {
				t.Fatalf("got %+v, %t; want %+v, %t", got, ok, tt.want, tt.ok)
			}
		})
	}
}

// TestCreditValiditySurvivesForwarding checks that the validity claims are kept
// when an identity is forwarded to the next service (Marshal, then
// Unmarshal), so a downstream gate reads the same verdict, including false.
func TestCreditValiditySurvivesForwarding(t *testing.T) {
	original, err := FromJWT(testJWT(t, map[string]any{
		"sub": "123", "email": "jan@example.com",
		"active_account": map[string]any{
			"account_id": "acme",
			"valid":      false,
			"reason":     "MEMBER_USAGE_LIMIT_REACHED",
			"comments":   "You have reached your personal usage limit.",
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	forwarded, err := Unmarshal(original.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	got, ok := forwarded.CreditValidity()
	expect(t, ok, true)
	expect(t, got, Validity{Reason: "MEMBER_USAGE_LIMIT_REACHED", Comments: "You have reached your personal usage limit."})
}
