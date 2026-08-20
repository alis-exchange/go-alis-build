package spanneradapter

import (
	"context"

	"cloud.google.com/go/spanner"
)

// Apply writes ms. If ctx carries a transaction (see SpannerTxFromContext),
// the mutations are buffered onto it via BufferWrite — they take effect at
// that transaction's commit, alongside any other table's writes sharing the
// ctx. Otherwise ms are applied immediately as their own single-use
// transaction via client.Apply.
//
// Errors are converted with ErrorToStatus.
func Apply(ctx context.Context, client *spanner.Client, ms []*spanner.Mutation) error {
	if txn := SpannerTxFromContext(ctx); txn != nil {
		if err := txn.BufferWrite(ms); err != nil {
			return ErrorToStatus(err)
		}
		return nil
	}
	if _, err := client.Apply(ctx, ms); err != nil {
		return ErrorToStatus(err)
	}
	return nil
}

// Query runs stmt. If ctx carries a transaction, the query runs against it
// (so it observes that transaction's read timestamp, not any of its own
// buffered writes — see RunTransaction's contract). Otherwise the query
// runs against a single-use snapshot via client.Single().
//
// The returned iterator is handed back as-is; callers are responsible for
// calling Stop() and for mapping iteration errors (e.g. via ErrorToStatus).
func Query(ctx context.Context, client *spanner.Client, stmt spanner.Statement) *spanner.RowIterator {
	if txn := SpannerTxFromContext(ctx); txn != nil {
		return txn.Query(ctx, stmt)
	}
	return client.Single().Query(ctx, stmt)
}

// ReadRowByKey reads the row at key from table, projecting columns. If ctx
// carries a transaction, the read participates in it; otherwise it runs
// against a single-use snapshot via client.Single().
//
// Errors (including "not found") are converted with ErrorToStatus.
func ReadRowByKey(ctx context.Context, client *spanner.Client, table string, key spanner.Key, columns []string) (*spanner.Row, error) {
	if txn := SpannerTxFromContext(ctx); txn != nil {
		row, err := txn.ReadRow(ctx, table, key, columns)
		if err != nil {
			return nil, ErrorToStatus(err)
		}
		return row, nil
	}
	row, err := client.Single().ReadRow(ctx, table, key, columns)
	if err != nil {
		return nil, ErrorToStatus(err)
	}
	return row, nil
}

// ReadKeySet reads the rows in keys from table, projecting columns. If ctx
// carries a transaction, the read participates in it; otherwise it runs
// against a single-use snapshot via client.Single().
//
// The returned iterator is handed back as-is; callers are responsible for
// calling Stop() and for mapping iteration errors (e.g. via ErrorToStatus).
func ReadKeySet(ctx context.Context, client *spanner.Client, table string, keys spanner.KeySet, columns []string) *spanner.RowIterator {
	if txn := SpannerTxFromContext(ctx); txn != nil {
		return txn.Read(ctx, table, keys, columns)
	}
	return client.Single().Read(ctx, table, keys, columns)
}
