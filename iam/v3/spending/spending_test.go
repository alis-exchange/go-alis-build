package spending

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	iam "go.alis.build/iam/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func tokenContext(t *testing.T, allowed any, expiry time.Time, verifyError error) context.Context {
	t.Helper()
	reason := ""
	if allowed == false {
		reason = "MEMBER_LIMIT_REACHED"
	}
	data, err := json.Marshal(map[string]any{"sub": "u", "exp": expiry.Unix(), "active_account": map[string]any{"account_id": "a", "valid": allowed, "reason": reason}})
	if err != nil {
		t.Fatal(err)
	}
	id := &iam.Identity{Type: iam.User, ID: "u", Exp: expiry.Unix(), ActiveAccount: &iam.ActiveAccount{AccountID: "a"}}
	return WithToken(t.Context(), "header."+base64.RawURLEncoding.EncodeToString(data)+".signature", id, func(string, time.Time) error { return verifyError })
}

func TestPayerMustBeExplicitAndUnambiguous(t *testing.T) {
	g := New(func(context.Context, Payer) (Decision, error) {
		t.Fatal("invalid payer reached source")
		return Decision{}, nil
	})
	for _, p := range []Payer{{}, {Account: "accounts/a"}, {Kind: Kind(99), Account: "accounts/a"}, MemberPayer("accounts/a", ""), AccountPayer("accounts/a/members/u"), {Kind: Account, Account: "accounts/a", User: "users/u"}} {
		if err := g.Check(t.Context(), p, false); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("accepted %+v: %v", p, err)
		}
	}
}

func TestMemberTokenNeverAuthorizesAccountOwnedWork(t *testing.T) {
	now := time.Now()
	reads := 0
	g := New(func(_ context.Context, p Payer) (Decision, error) {
		reads++
		return Decision{Payer: p, Reason: "BILLING_HOLD"}, nil
	})
	g.Now = func() time.Time { return now }
	member, account := MemberPayer("accounts/a", "users/u"), AccountPayer("accounts/a")
	allowed := tokenContext(t, true, now.Add(time.Minute), nil)
	blocked := tokenContext(t, false, now.Add(time.Minute), nil)
	if err := g.Check(allowed, member, false); err != nil || reads != 0 {
		t.Fatalf("member JWT: %v reads=%d", err, reads)
	}
	if err := g.Check(blocked, member, false); status.Code(err) != codes.PermissionDenied || reads != 0 {
		t.Fatalf("member block: %v", err)
	}
	if err := g.Check(allowed, account, false); status.Code(err) != codes.PermissionDenied || reads != 1 {
		t.Fatalf("account reused member claim: %v", err)
	}
	if err := g.Check(allowed, member, true); status.Code(err) != codes.PermissionDenied || reads != 2 {
		t.Fatalf("background reused token/cache: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if err := g.Check(allowed, member, false); status.Code(err) != codes.PermissionDenied || reads != 3 {
		t.Fatalf("expired token: %v", err)
	}
	// A blocked member does not prevent independently funded account work.
	g = New(func(_ context.Context, p Payer) (Decision, error) { return Decision{Payer: p, Allowed: true}, nil })
	if err := g.Check(blocked, account, false); err != nil {
		t.Fatal(err)
	}
}

func TestUntrustedAndMismatchedTokensFallBack(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ctx   context.Context
		payer Payer
	}{
		{"missing", t.Context(), MemberPayer("accounts/a", "users/u")},
		{"undecided", tokenContext(t, nil, time.Now().Add(time.Minute), nil), MemberPayer("accounts/a", "users/u")},
		{"unverified", tokenContext(t, true, time.Now().Add(time.Minute), errors.New("signature")), MemberPayer("accounts/a", "users/u")},
		{"account", tokenContext(t, true, time.Now().Add(time.Minute), nil), MemberPayer("accounts/b", "users/u")},
		{"user", tokenContext(t, true, time.Now().Add(time.Minute), nil), MemberPayer("accounts/a", "users/v")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := 0
			g := New(func(_ context.Context, p Payer) (Decision, error) {
				reads++
				if p != tc.payer {
					t.Fatal("wrong payer")
				}
				return Decision{}, errors.New("offline")
			})
			if err := g.Check(tc.ctx, tc.payer, false); status.Code(err) != codes.Unavailable || reads != 1 {
				t.Fatalf("%v reads=%d", err, reads)
			}
		})
	}
	id := &iam.Identity{Type: iam.User, ID: "other", Exp: time.Now().Add(time.Minute).Unix(), ActiveAccount: &iam.ActiveAccount{AccountID: "a"}}
	data, _ := json.Marshal(map[string]any{"sub": "u", "exp": id.Exp, "active_account": map[string]any{"account_id": "a", "valid": true}})
	ctx := WithToken(t.Context(), "h."+base64.RawURLEncoding.EncodeToString(data)+".s", id, func(string, time.Time) error { return nil })
	if ctx.Value(claimKey{}) != nil {
		t.Fatal("accepted another authenticated identity")
	}
}

func TestCacheBoundaryFailureAndPayerIsolation(t *testing.T) {
	now := time.Now()
	reads := 0
	fail := false
	g := New(func(_ context.Context, p Payer) (Decision, error) {
		reads++
		if fail {
			return Decision{}, errors.New("offline")
		}
		return Decision{Payer: p, Allowed: true, Until: now.Add(10 * time.Second)}, nil
	})
	g.Now = func() time.Time { return now }
	a, m := AccountPayer("accounts/a"), MemberPayer("accounts/a", "users/u")
	for _, p := range []Payer{a, a, m, m} {
		if err := g.Check(t.Context(), p, false); err != nil {
			t.Fatal(err)
		}
	}
	if reads != 2 {
		t.Fatalf("payer cache isolation: %d", reads)
	}
	fail = true
	if err := g.Check(t.Context(), a, true); status.Code(err) != codes.Unavailable {
		t.Fatal(err)
	}
	if err := g.Check(t.Context(), a, false); status.Code(err) != codes.Unavailable || reads != 4 {
		t.Fatalf("cached allowance survived failed current read: %v reads=%d", err, reads)
	}
	now = now.Add(10 * time.Second)
	if err := g.Check(t.Context(), m, false); status.Code(err) != codes.Unavailable || reads != 5 {
		t.Fatalf("boundary: %v reads=%d", err, reads)
	}
}

func TestRemoteAllowanceIsBoundedAndMalformedResponsesFailClosed(t *testing.T) {
	p := AccountPayer("accounts/a")
	now := time.Now()
	for _, d := range []Decision{{Allowed: true}, {Payer: p, Allowed: true, Reason: "BILLING_HOLD"}, {Payer: p}, {Payer: p, Allowed: true, Until: now.Add(-time.Second)}} {
		g := New(func(context.Context, Payer) (Decision, error) { return d, nil })
		g.Now = func() time.Time { return now }
		if err := g.Check(t.Context(), p, false); status.Code(err) != codes.Unavailable {
			t.Fatalf("malformed allowed: %v", err)
		}
	}
	reads := 0
	g := New(func(_ context.Context, p Payer) (Decision, error) {
		reads++
		return Decision{Payer: p, Allowed: true, Until: now.Add(time.Hour)}, nil
	})
	g.Now = func() time.Time { return now }
	if err := g.Check(t.Context(), p, false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := g.Check(t.Context(), p, false); err != nil || reads != 2 {
		t.Fatalf("TTL: %v reads=%d", err, reads)
	}
}

func TestConcurrentChecks(t *testing.T) {
	var reads atomic.Int64
	g := New(func(_ context.Context, p Payer) (Decision, error) {
		reads.Add(1)
		return Decision{Payer: p, Allowed: true}, nil
	})
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			if err := g.Check(t.Context(), AccountPayer("accounts/a"), false); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if reads.Load() == 0 {
		t.Fatal("source never checked")
	}
	if len(g.cache) != 1 {
		t.Fatalf("cache: %d", len(g.cache))
	}
}

func TestAuthorityNeverFallsBackOnFailureOrCutover(t *testing.T) {
	p := AccountPayer("accounts/a")
	for _, phase := range []Phase{Unknown, Legacy, Shadow, Draining, Active, Phase(99)} {
		v1, v2 := 0, 0
		s := Sources{Authority: func(context.Context, string) (Ownership, error) {
			return Ownership{Account: p.Account, Phase: phase}, nil
		},
			V1: func(_ context.Context, p Payer) (Decision, error) {
				v1++
				return Decision{Payer: p, Allowed: true}, nil
			},
			V2: func(_ context.Context, p Payer) (Decision, error) {
				v2++
				return Decision{Payer: p, Allowed: true, Revision: 1, EvaluatedAt: time.Now()}, nil
			}}
		_, err := s.Read(t.Context(), p)
		switch phase {
		case Legacy, Shadow:
			if err != nil || v1 != 1 || v2 != 0 {
				t.Fatalf("legacy route: %v", err)
			}
		case Active:
			if err != nil || v1 != 0 || v2 != 1 {
				t.Fatalf("active route: %v", err)
			}
		default:
			if err == nil || v1+v2 != 0 {
				t.Fatalf("cutover fell back: %v", err)
			}
		}
	}
	for _, owner := range []Ownership{{Account: "accounts/b", Phase: Active}, {Account: p.Account, Phase: Active}} {
		s := Sources{Authority: func(context.Context, string) (Ownership, error) { return owner, nil }, V2: func(context.Context, Payer) (Decision, error) { return Decision{}, errors.New("offline") }, V1: func(context.Context, Payer) (Decision, error) { t.Fatal("fallback"); return Decision{}, nil }}
		if _, err := s.Read(t.Context(), p); err == nil {
			t.Fatal("invalid authority/reader accepted")
		}
	}
	s := Sources{Authority: func(context.Context, string) (Ownership, error) { return Ownership{}, errors.New("offline") }}
	if _, err := s.Read(t.Context(), p); err == nil {
		t.Fatal("failed authority allowed")
	}
}

func TestActiveAllowanceRequiresVersionedExactDecision(t *testing.T) {
	p := MemberPayer("accounts/a", "users/u")
	for _, d := range []Decision{{Payer: p, Allowed: true}, {Payer: p, Allowed: true, Revision: 1}, {Payer: AccountPayer("accounts/a"), Allowed: true, Revision: 1, EvaluatedAt: time.Now()}} {
		s := Sources{Authority: func(context.Context, string) (Ownership, error) {
			return Ownership{Account: p.Account, Phase: Active}, nil
		}, V2: func(context.Context, Payer) (Decision, error) { return d, nil }}
		if _, err := s.Read(t.Context(), p); err == nil {
			t.Fatal("incomplete/cross-payer allowance")
		}
	}
}
