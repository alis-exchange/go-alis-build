package protodb

import (
	"context"
	"iter"
)

// TransactionRunner runs multi-operation transactions. Implementations (e.g.
// spanneradapter.SpannerTransactionRunner) inject the transaction into the
// context passed to fn; ResourceTable operations that receive that context
// will use the transaction instead of standalone reads/writes.
type TransactionRunner interface {
	// RunTransaction executes fn with a transactional context. If fn returns
	// nil, the transaction is committed; otherwise it is rolled back.
	RunTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// ResourceTable is the single generic interface for a table storing
// resources of type R. There is no longer a proto-constrained variant —
// R may be any type; adapters that need protobuf-specific behavior
// (merge, read masks, etc.) implement it above this interface, not within
// it.
//
// ResourceTable implementations support both non-transactional and
// transactional usage. When used outside a transaction, operations apply
// immediately. When used inside TransactionRunner.RunTransaction, all
// operations sharing that context participate in the same transaction and
// commit or roll back atomically.
//
// Usage example (non-transactional):
//
//	row, err := table.Read(ctx, key)
//	if err != nil {
//		return err
//	}
//	_ = row.Resource
//
// Usage example (transactional — cross-table operations in a single
// transaction):
//
//	txRunner := &spanneradapter.SpannerTransactionRunner{Client: spannerClient}
//	err := txRunner.RunTransaction(ctx, func(ctx context.Context) error {
//		row, err := tableA.Read(ctx, key1)
//		if err != nil {
//			return err
//		}
//		if err := tableB.Write(ctx, &protodb.Row[B]{Key: key2, Resource: resource}); err != nil {
//			return err
//		}
//		row.Resource = updatedResource
//		return tableA.Write(ctx, row)
//	})
type ResourceTable[R any] interface {
	// Create inserts new rows. It fails with an AlreadyExists status error
	// if any key already exists. A zero-length call is a no-op that
	// returns nil.
	Create(ctx context.Context, rows ...*Row[R]) error
	// Write creates or updates rows. Existing rows are fully replaced —
	// there is no partial update. A zero-length call is a no-op that
	// returns nil.
	Write(ctx context.Context, rows ...*Row[R]) error
	// Read retrieves one row. Returns a NotFound status error when the key
	// does not exist.
	Read(ctx context.Context, key Key) (*Row[R], error)
	// BatchRead retrieves multiple rows by key. The returned slice has the
	// same length as keys, position for position: result[i] is the row for
	// keys[i], or nil if keys[i] does not exist. A missing key is not an
	// error — BatchRead does not return NotFound. A zero-length call is a
	// no-op that returns nil, nil.
	BatchRead(ctx context.Context, keys ...Key) ([]*Row[R], error)
	// List retrieves one bounded page of rows. PageSize 0 means
	// DefaultPageSize; negative returns InvalidArgument. Never unbounded —
	// use Stream to traverse everything.
	List(ctx context.Context, opts ListOptions) (rows []*Row[R], nextPageToken string, err error)
	// Stream traverses every matching row; no paging. Errors are yielded
	// as the final element; `break` cancels cleanly.
	Stream(ctx context.Context, opts StreamOptions) iter.Seq2[*Row[R], error]
	// Delete removes rows. Deleting a key that does not exist is not an
	// error (idempotent, matching Spanner mutation semantics). A
	// zero-length call is a no-op that returns nil.
	Delete(ctx context.Context, keys ...Key) error
	// WritePolicies writes the IAM policies for existing rows. A key that
	// does not exist is a NotFound status error — WritePolicies attaches a
	// policy to an existing row, it never creates one. The batch is
	// validated in full — every key confirmed to exist — before any policy
	// is written: a NotFound anywhere in the batch leaves every row's
	// policy unchanged, matching the atomicity of Create and Write. A
	// zero-length call is a no-op that returns nil.
	WritePolicies(ctx context.Context, entries ...PolicyEntry) error
}
