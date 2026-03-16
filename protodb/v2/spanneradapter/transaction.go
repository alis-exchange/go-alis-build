package spanneradapter

import (
	"context"

	"cloud.google.com/go/spanner"
)

// spannerTxKey is the context key used to store a Spanner ReadWriteTransaction.
// ResourceTable implementations check for this to decide whether to use the
// transaction (BufferWrite/ReadRow/Read/Query) or the client directly (Apply/Single).
type spannerTxKey struct{}

// SpannerTxFromContext returns the active Spanner ReadWriteTransaction from ctx,
// or nil if no transaction is active. ResourceTable implementations use this to
// participate in transactional operations when called within RunTransaction.
func SpannerTxFromContext(ctx context.Context) *spanner.ReadWriteTransaction {
	txn, _ := ctx.Value(spannerTxKey{}).(*spanner.ReadWriteTransaction)
	return txn
}

// SpannerTransactionRunner runs cross-table Spanner transactions. Create it once
// at service init with the shared *spanner.Client; pass the same runner to any
// table operations that should participate in transactions.
type SpannerTransactionRunner struct {
	Client *spanner.Client
}

// RunTransaction executes fn inside a Spanner ReadWriteTransaction. The context
// passed to fn contains the transaction; any ResourceTable operations that accept
// that context will automatically use the transaction (reads, writes, deletes).
// If fn returns an error, the transaction is rolled back; otherwise it is committed.
func (r *SpannerTransactionRunner) RunTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	_, err := r.Client.ReadWriteTransaction(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		txCtx := context.WithValue(ctx, spannerTxKey{}, txn)
		return fn(txCtx)
	})
	return err
}
