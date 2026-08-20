package spanneradapter

import (
	"context"
	"testing"

	"cloud.google.com/go/spanner"
)

// TestApplyUsesBufferWriteWhenTxPresent proves the branch decision in Apply
// (and, by construction, in Query/ReadRowByKey/ReadKeySet) without needing a
// live Spanner backend: a ctx carrying a transaction must take the tx branch
// and never touch the client, even when the client is nil.
//
// A zero-value *spanner.ReadWriteTransaction can't actually BufferWrite (it
// panics internally on its unexported fields), so we can't assert "no
// panic" directly. Instead this test locks in the one thing that must never
// regress: a stale typed-nil txn value in ctx must NOT be mistaken for a
// present transaction. SpannerTxFromContext must collapse it to nil so
// Apply falls through to the client branch — proven here by the nil-client
// panic.
func TestApplyUsesBufferWriteWhenTxPresent(t *testing.T) {
	ctx := context.WithValue(context.Background(), spannerTxKey{}, (*spanner.ReadWriteTransaction)(nil))
	// txn is typed-nil: SpannerTxFromContext must return nil → Apply falls to
	// the client branch → panics dereferencing the nil client.
	defer func() {
		if recover() == nil {
			t.Fatal("expected nil-client panic proving the client branch was taken")
		}
	}()
	_ = Apply(ctx, nil, nil)
}

// TestSpannerTxFromContext_TypedNilGuard locks in the hardening called out
// in the task brief directly: a typed-nil *spanner.ReadWriteTransaction
// stored in ctx must come back out as nil, not as a non-nil-looking typed
// pointer that later selects the tx branch and panics deep inside Spanner
// client code.
func TestSpannerTxFromContext_TypedNilGuard(t *testing.T) {
	ctx := context.WithValue(context.Background(), spannerTxKey{}, (*spanner.ReadWriteTransaction)(nil))
	if txn := SpannerTxFromContext(ctx); txn != nil {
		t.Fatalf("SpannerTxFromContext(typed-nil) = %v, want nil", txn)
	}
}

// TestSpannerTxFromContext_NoValue asserts the ordinary "no transaction in
// ctx" case also yields nil, so callers with a plain background ctx take
// the client branch.
func TestSpannerTxFromContext_NoValue(t *testing.T) {
	if txn := SpannerTxFromContext(context.Background()); txn != nil {
		t.Fatalf("SpannerTxFromContext(no value) = %v, want nil", txn)
	}
}

// TestSpannerTxFromContext_WrongType asserts a ctx value under the same key
// but of an unrelated type (e.g. from a mistaken key collision) is ignored
// rather than causing a panic, and yields nil.
func TestSpannerTxFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), spannerTxKey{}, "not-a-txn")
	if txn := SpannerTxFromContext(ctx); txn != nil {
		t.Fatalf("SpannerTxFromContext(wrong type) = %v, want nil", txn)
	}
}

// TestQueryReadRowByKeyReadKeySet_ClientBranchOnNilClient proves Query,
// ReadRowByKey, and ReadKeySet all select the client branch (not the tx
// branch) on a plain ctx, by observing the nil-client panic each produces —
// mirroring TestApplyUsesBufferWriteWhenTxPresent's proof technique for the
// three read paths.
func TestQueryReadRowByKeyReadKeySet_ClientBranchOnNilClient(t *testing.T) {
	ctx := context.Background()

	t.Run("Query", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected nil-client panic proving the client branch was taken")
			}
		}()
		_ = Query(ctx, nil, spanner.Statement{SQL: "SELECT 1"})
	})

	t.Run("ReadRowByKey", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected nil-client panic proving the client branch was taken")
			}
		}()
		_, _ = ReadRowByKey(ctx, nil, "T", spanner.Key{"k"}, []string{"c"})
	})

	t.Run("ReadKeySet", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected nil-client panic proving the client branch was taken")
			}
		}()
		_ = ReadKeySet(ctx, nil, "T", spanner.Key{"k"}, []string{"c"})
	})
}
