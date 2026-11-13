package spending

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	iam "go.alis.build/iam/v3"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type claim struct {
	Subject string `json:"sub"`
	Expiry  int64  `json:"exp"`
	Active  *struct {
		AccountID string `json:"account_id"`
		Valid     *bool  `json:"valid"`
		Reason    string `json:"reason"`
		Comments  string `json:"comments"`
	} `json:"active_account"`
}

type claimKey struct{}

// WithToken accepts only a signature-validated token matching the authenticated
// identity. Supply the token refreshed by authentication middleware, if any.
// The raw token is not retained; account-owned work never uses its member claim.
func WithToken(ctx context.Context, token string, identity *iam.Identity, validate func(string, time.Time) error) context.Context {
	if identity == nil || identity.User() == "" || validate == nil || validate(token, time.Now()) != nil {
		return ctx
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ctx
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ctx
	}
	c := &claim{}
	if json.Unmarshal(data, c) != nil || c.Subject != identity.ID || c.Expiry != identity.Exp || c.Active == nil || c.Active.AccountID != identity.ActiveAccountID() {
		return ctx
	}
	return context.WithValue(ctx, claimKey{}, c)
}

type Guard struct {
	Read  Reader
	Now   func() time.Time
	mu    sync.Mutex
	cache map[Payer]Decision
	loads singleflight.Group
}

func New(read Reader) *Guard {
	return &Guard{Read: read, Now: time.Now, cache: map[Payer]Decision{}}
}

func unavailable() error {
	return status.Error(codes.Unavailable, "STATUS_UNAVAILABLE: Account status could not be verified. Please try again.")
}

// Check is required before admission and each provider attempt. current forces
// a source read for scheduled, queued and recovered execution. Subsequent model
// calls may reuse a remote decision for at most a minute, shortened to Until.
// Resource authorization is separate and remains the producer's responsibility.
func (g *Guard) Check(ctx context.Context, payer Payer, current bool) error {
	if err := payer.Validate(); err != nil {
		return Refused("PAYER_REQUIRED", payer.Account, err.Error())
	}
	now := g.Now()
	if c, ok := ctx.Value(claimKey{}).(*claim); payer.Kind == Member && !current && ok && c.Active != nil && c.Active.Valid != nil && "users/"+c.Subject == payer.User && "accounts/"+c.Active.AccountID == payer.Account && now.Before(time.Unix(c.Expiry, 0)) {
		if !*c.Active.Valid {
			if c.Active.Reason == "" || c.Active.Reason == "REASON_UNSPECIFIED" {
				return unavailable()
			}
			return Refused(c.Active.Reason, payer.Account, c.Active.Comments)
		}
		if c.Active.Reason != "" && c.Active.Reason != "REASON_UNSPECIFIED" {
			return unavailable()
		}
		return nil
	}
	g.mu.Lock()
	d, ok := g.cache[payer]
	g.mu.Unlock()
	if current || !ok || !now.Before(d.Until) {
		key := fmt.Sprintf("%d/%s/%s", payer.Kind, payer.Account, payer.User)
		loaded, err, _ := g.loads.Do(key, func() (any, error) {
			if g.Read == nil {
				return nil, fmt.Errorf("standing reader unavailable")
			}
			check, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			started := g.Now()
			d, err := g.Read(check, payer)
			if err != nil {
				return nil, err
			}
			if err = validateDecision(d, payer); err != nil {
				return nil, err
			}
			until := started.Add(time.Minute)
			if !d.Until.IsZero() && d.Until.Before(until) {
				until = d.Until
			}
			if !g.Now().Before(until) {
				return nil, fmt.Errorf("standing is past its boundary")
			}
			d.Until = until
			g.mu.Lock()
			if len(g.cache) >= 4096 {
				for key, old := range g.cache {
					if !started.Before(old.Until) {
						delete(g.cache, key)
					}
				}
			}
			if len(g.cache) < 4096 {
				g.cache[payer] = d
			}
			g.mu.Unlock()
			return d, nil
		})
		if err != nil {
			g.mu.Lock()
			delete(g.cache, payer)
			g.mu.Unlock()
			return unavailable()
		}
		var valid bool
		d, valid = loaded.(Decision)
		if !valid {
			return unavailable()
		}
	}
	if !d.Allowed {
		return Refused(d.Reason, payer.Account, d.Comments)
	}
	return nil
}
