package spanneradapter

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/spanner"
)

// TestRunTransactionJoinsOuterTransaction needs no emulator: a nested call
// must run fn in the transaction already on ctx and never reach the client,
// so a zero Client is enough to tell joining from starting a new one.
func TestRunTransactionJoinsOuterTransaction(t *testing.T) {
	outer := &spanner.ReadWriteTransaction{}
	ctx := context.WithValue(context.Background(), spannerTxKey{}, outer)
	r := &SpannerTransactionRunner{Client: &spanner.Client{}}
	wantErr := errors.New("inner")
	var got *spanner.ReadWriteTransaction
	var err error
	func() {
		// Before the join, the zero Client panics inside
		// ReadWriteTransaction; report that as a failure rather than
		// aborting the test binary.
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("RunTransaction started a new transaction instead of joining: panic %v", p)
			}
		}()
		err = r.RunTransaction(ctx, func(ctx context.Context) error {
			got = SpannerTxFromContext(ctx)
			return wantErr
		})
	}()
	if !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want %v", err, wantErr)
	}
	if got != outer {
		t.Fatal("fn did not run in the outer transaction")
	}
}
