package spanneradapter

import (
	"context"

	"cloud.google.com/go/spanner"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// spannerTxKey is the context key used to store a Spanner ReadWriteTransaction.
// ResourceTable implementations check for this to decide whether to use the
// transaction (BufferWrite/ReadRow/Read/Query) or the client directly (Apply/Single).
type spannerTxKey struct{}

// SpannerTxFromContext returns the active Spanner ReadWriteTransaction from ctx,
// or nil if no transaction is active. ResourceTable implementations use this to
// participate in transactional operations when called within RunTransaction.
//
// A typed-nil value stored under this key — e.g. a stale or manually
// constructed ctx carrying (*spanner.ReadWriteTransaction)(nil) — is treated
// the same as no value at all and yields nil, so callers never mistake it
// for a present transaction.
func SpannerTxFromContext(ctx context.Context) *spanner.ReadWriteTransaction {
	txn, ok := ctx.Value(spannerTxKey{}).(*spanner.ReadWriteTransaction)
	if !ok || txn == nil {
		return nil
	}
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
//
// Contract:
//   - fn MAY RUN MORE THAN ONCE: Spanner aborts and retries contended
//     transactions. fn must be re-runnable — mutate only via the ctx it is
//     given; no external side effects.
//   - NO READ-YOUR-WRITES: writes are buffered client-side until commit.
//     Reads inside fn observe the database as of transaction start and DO
//     NOT see fn's own buffered writes.
//   - Multiple tables may participate: any table operation receiving fn's
//     ctx joins this transaction.
//
// For example:
//
//	runner.RunTransaction(ctx, func(ctx context.Context) error {
//		if err := resources.Write(ctx, row); err != nil { return err }
//		return revisions.Create(ctx, revRow)
//	})
func (r *SpannerTransactionRunner) RunTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if r == nil {
		return status.Error(codes.InvalidArgument, "Spanner transaction runner is nil")
	}

	if r.Client == nil {
		return status.Error(codes.InvalidArgument, "Spanner client is nil")
	}

	_, err := r.Client.ReadWriteTransaction(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		txCtx := context.WithValue(ctx, spannerTxKey{}, txn)
		return fn(txCtx)
	})
	return err
}
