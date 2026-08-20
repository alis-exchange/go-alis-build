package protodb

import "context"

// ReadModifyWrite reads the row for key inside a transaction, applies fn to
// it, and writes it back — an atomic compare-and-swap under Spanner's
// serializable isolation.
//
// fn MAY RUN MORE THAN ONCE (contended transactions are retried by the
// runner): it must be re-runnable — mutate only the row it is given.
func ReadModifyWrite[R any](ctx context.Context, runner TransactionRunner, table ResourceTable[R], key Key, fn func(*Row[R]) error) error {
	return runner.RunTransaction(ctx, func(ctx context.Context) error {
		row, err := table.Read(ctx, key)
		if err != nil {
			return err
		}
		if err := fn(row); err != nil {
			return err
		}
		return table.Write(ctx, row)
	})
}
