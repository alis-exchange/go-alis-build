package spending

import (
	"context"
	"fmt"
	"time"
)

// Decision carries a complete response for the exact payer. Until is the next
// evaluation boundary, not a reservation or a guarantee that work will finish.
type Decision struct {
	Payer            Payer
	Allowed          bool
	Reason, Comments string
	Revision         int64
	EvaluatedAt      time.Time
	Until            time.Time
}

type Reader func(context.Context, Payer) (Decision, error)

type Phase uint8

const (
	Unknown Phase = iota
	Legacy
	Shadow
	Draining
	Active
)

type Ownership struct {
	Account string
	Phase   Phase
}

// Sources binds typed service clients to shared authority semantics. Keep RPC
// transport, credentials and protobuf conversion in the caller's adapter; this
// package has no dependency on private generated service packages. Both readers
// must distinguish member and account mode, verify response identity, and use
// the service's trusted-reader permissions. No nil source grants permission.
type Sources struct {
	Authority func(context.Context, string) (Ownership, error)
	V2        Reader
	V1        Reader
}

func (s Sources) Read(ctx context.Context, payer Payer) (Decision, error) {
	if err := payer.Validate(); err != nil {
		return Decision{}, err
	}
	if s.Authority == nil {
		return Decision{}, fmt.Errorf("authority reader unavailable")
	}
	owner, err := s.Authority(ctx, payer.Account)
	if err != nil {
		return Decision{}, err
	}
	if owner.Account != payer.Account {
		return Decision{}, fmt.Errorf("authority response belongs to another account")
	}
	var read Reader
	switch owner.Phase {
	case Active:
		read = s.V2
	case Legacy, Shadow:
		read = s.V1
	default:
		return Decision{}, fmt.Errorf("account authority is unresolved")
	}
	if read == nil {
		return Decision{}, fmt.Errorf("standing reader unavailable")
	}
	d, err := read(ctx, payer)
	if err != nil {
		return Decision{}, err
	}
	if err = validateDecision(d, payer); err != nil {
		return Decision{}, err
	}
	// A definitive missing-member response can deny without inventing a
	// revision. Allowances always require a complete authoritative snapshot.
	if owner.Phase == Active && d.Allowed && (d.Revision <= 0 || d.EvaluatedAt.IsZero()) {
		return Decision{}, fmt.Errorf("versioned standing is incomplete")
	}
	return d, nil
}

func validateDecision(d Decision, payer Payer) error {
	if d.Payer != payer {
		return fmt.Errorf("standing belongs to another payer")
	}
	unspecified := d.Reason == "" || d.Reason == "REASON_UNSPECIFIED"
	if d.Allowed != unspecified {
		return fmt.Errorf("standing reason contradicts its decision")
	}
	return nil
}
