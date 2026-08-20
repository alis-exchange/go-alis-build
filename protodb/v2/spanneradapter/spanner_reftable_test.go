package spanneradapter_test

// spannerTable is the reference consumer table used by the Spanner
// conformance test: a full protodb.ResourceTable[R] composed *only* out of
// the spanneradapter building blocks, following the "Shape of a consumer
// table" sketch in the package README as closely as a complete
// implementation can.
//
// It lives in the test package on purpose — spanneradapter ships building
// blocks, not a table. This type exists so the blocks can be executed
// against a real Spanner (the emulator) rather than only asserted on as
// SQL strings.

import (
	"context"
	"iter"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner"
	"go.alis.build/protodb/v2"
	"go.alis.build/protodb/v2/filtering"
	"go.alis.build/protodb/v2/spanneradapter"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// tableConfig describes one physical Spanner table to spannerTable.
type tableConfig[R any] struct {
	Client *spanner.Client
	// TableName is the physical table.
	TableName string
	// Spec is the table's KeySpec (StringKeySpec or KeySpecFor).
	Spec spanneradapter.KeySpec
	// Codec decodes the resource column; Encode is its write-side
	// counterpart (ValueCodec is read-only by design).
	Codec  spanneradapter.ValueCodec[R]
	Encode func(R) any
	// ResourceColumn / PolicyColumn name the non-key columns.
	ResourceColumn string
	PolicyColumn   string
	// Parser is required whenever a filter may be supplied.
	Parser *filtering.Parser
}

// spannerTable is a protodb.ResourceTable[R] over one Spanner table.
type spannerTable[R any] struct {
	cfg     tableConfig[R]
	scanner spanneradapter.Scanner[R]
	stmts   spanneradapter.StatementBuilder
}

var _ protodb.ResourceTable[string] = (*spannerTable[string])(nil)

func newSpannerTable[R any](cfg tableConfig[R]) *spannerTable[R] {
	return &spannerTable[R]{
		cfg: cfg,
		scanner: spanneradapter.Scanner[R]{
			Spec:           cfg.Spec,
			Codec:          cfg.Codec,
			ResourceColumn: cfg.ResourceColumn,
			PolicyColumn:   cfg.PolicyColumn,
		},
		stmts: spanneradapter.StatementBuilder{
			TableName:      cfg.TableName,
			Spec:           cfg.Spec,
			ResourceColumn: cfg.ResourceColumn,
			PolicyColumn:   cfg.PolicyColumn,
			Parser:         cfg.Parser,
		},
	}
}

// writeColumns is the full column list a Create/Write mutation supplies:
// key columns, the resource column and — when configured — the policy
// column. The policy column is always written (as NULL when Row.Policy is
// nil) so that Write really is a full row replacement, per the
// ResourceTable contract.
func (t *spannerTable[R]) writeColumns() []string {
	cols := append([]string{}, t.cfg.Spec.Columns()...)
	cols = append(cols, t.cfg.ResourceColumn)
	if t.cfg.PolicyColumn != "" {
		cols = append(cols, t.cfg.PolicyColumn)
	}
	return cols
}

func (t *spannerTable[R]) writeValues(row *protodb.Row[R]) []any {
	vals := append([]any{}, row.Key.KeyValues()...)
	vals = append(vals, t.cfg.Encode(row.Resource))
	if t.cfg.PolicyColumn != "" {
		vals = append(vals, policyValue(row.Policy))
	}
	return vals
}

// policyValue renders an IAM policy as a Spanner PROTO column value. A nil
// policy becomes a typed-nil NullProtoMessage, which is how the Spanner
// client writes a NULL into a PROTO column (a bare nil cannot be typed).
func policyValue(p *iampb.Policy) any {
	if p == nil {
		return spanner.NullProtoMessage{ProtoMessageVal: (*iampb.Policy)(nil), Valid: true}
	}
	return p
}

func (t *spannerTable[R]) mutate(ctx context.Context, rows []*protodb.Row[R], mut func(string, []string, []any) *spanner.Mutation) error {
	if len(rows) == 0 {
		return nil
	}
	cols := t.writeColumns()
	ms := make([]*spanner.Mutation, 0, len(rows))
	for _, row := range rows {
		ms = append(ms, mut(t.cfg.TableName, cols, t.writeValues(row)))
	}
	return spanneradapter.Apply(ctx, t.cfg.Client, ms)
}

// Create inserts rows; an existing key yields AlreadyExists (Spanner's own
// Insert semantics, surfaced through ErrorToStatus by Apply).
func (t *spannerTable[R]) Create(ctx context.Context, rows ...*protodb.Row[R]) error {
	return t.mutate(ctx, rows, spanner.Insert)
}

// Write upserts rows, replacing every column of an existing row.
func (t *spannerTable[R]) Write(ctx context.Context, rows ...*protodb.Row[R]) error {
	return t.mutate(ctx, rows, spanner.InsertOrUpdate)
}

// Delete removes rows by key; a missing key is not an error.
func (t *spannerTable[R]) Delete(ctx context.Context, keys ...protodb.Key) error {
	if len(keys) == 0 {
		return nil
	}
	ms := make([]*spanner.Mutation, 0, len(keys))
	for _, k := range keys {
		ms = append(ms, spanner.Delete(t.cfg.TableName, spanneradapter.ToKey(k)))
	}
	return spanneradapter.Apply(ctx, t.cfg.Client, ms)
}

// Read is the README sketch verbatim: ReadRowByKey + Scanner.ScanRow.
func (t *spannerTable[R]) Read(ctx context.Context, key protodb.Key) (*protodb.Row[R], error) {
	raw, err := spanneradapter.ReadRowByKey(ctx, t.cfg.Client, t.cfg.TableName, spanneradapter.ToKey(key), t.scanner.Columns())
	if err != nil {
		return nil, err
	}
	return t.scanner.ScanRow(raw)
}

// BatchRead reads keys in one round trip and re-joins the results to the
// requested positions through KeySpec.Encode — the in-process canonical
// key string, never decoded back into a key.
func (t *spannerTable[R]) BatchRead(ctx context.Context, keys ...protodb.Key) ([]*protodb.Row[R], error) {
	if len(keys) == 0 {
		return nil, nil
	}
	it := spanneradapter.ReadKeySet(ctx, t.cfg.Client, t.cfg.TableName, spanneradapter.ToKeySets(keys), t.scanner.Columns())
	defer it.Stop()

	found := make(map[string]*protodb.Row[R], len(keys))
	for {
		raw, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, spanneradapter.ErrorToStatus(err)
		}
		row, err := t.scanner.ScanRow(raw)
		if err != nil {
			return nil, err
		}
		enc, err := t.cfg.Spec.Encode(row.Key)
		if err != nil {
			return nil, spanneradapter.ErrorToStatus(err)
		}
		found[enc] = row
	}

	out := make([]*protodb.Row[R], len(keys))
	for i, k := range keys {
		enc, err := t.cfg.Spec.Encode(k)
		if err != nil {
			return nil, spanneradapter.ErrorToStatus(err)
		}
		out[i] = found[enc] // nil when absent — not an error
	}
	return out, nil
}

// List follows the README sketch exactly, with the one addition the sketch
// omits for brevity: Tail pages come back in the inverted (query) order and
// are reversed before returning, after the cursor values have been read off
// the last row of the query order.
func (t *spannerTable[R]) List(ctx context.Context, opts protodb.ListOptions) ([]*protodb.Row[R], string, error) {
	stmt, order, fingerprint, err := t.stmts.BuildList(opts)
	if err != nil {
		return nil, "", err
	}
	pageSize := opts.PageSize
	if pageSize == 0 {
		pageSize = protodb.DefaultPageSize
	}

	it := spanneradapter.Query(ctx, t.cfg.Client, stmt)
	defer it.Stop()

	// raws is kept parallel to rows so OrderValuesFromRow can be called on
	// the raw *spanner.Row once the LIMIT+1 sentinel is dropped below — a
	// protodb.Row alone doesn't carry non-key order-column values.
	var rows []*protodb.Row[R]
	var raws []*spanner.Row
	for {
		raw, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, "", spanneradapter.ErrorToStatus(err)
		}
		row, err := t.scanner.ScanRow(raw)
		if err != nil {
			return nil, "", err
		}
		rows = append(rows, row)
		raws = append(raws, raw)
	}

	var nextPageToken string
	if int32(len(rows)) > pageSize {
		// The sentinel row proves a next page exists; drop it before
		// returning to the caller.
		rows, raws = rows[:pageSize], raws[:pageSize]

		// CRITICAL: OrderValues must come from OrderValuesFromRow over the
		// FULL effective order BuildList returned — never a hand-picked
		// subset of it — or pagination silently corrupts on a multi-column
		// key/order the moment two rows tie on the caller's OrderBy.
		// KeyValues come from the last kept row's own key.
		orderValues, err := spanneradapter.OrderValuesFromRow(raws[len(raws)-1], order)
		if err != nil {
			return nil, "", err
		}
		nextPageToken = spanneradapter.EncodePageToken(spanneradapter.PageToken{
			OrderValues: orderValues,
			KeyValues:   rows[len(rows)-1].Key.KeyValues(),
		}, fingerprint)
	}

	if opts.Tail {
		// BuildList inverted the order to serve the last page as a head
		// page; hand the rows back in the caller's stated order.
		reverse(rows)
	}
	return rows, nextPageToken, nil
}

func reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// Stream traverses every matching row. Errors are yielded as the final
// element; a break by the consumer returns from the closure, which runs the
// deferred it.Stop().
func (t *spannerTable[R]) Stream(ctx context.Context, opts protodb.StreamOptions) iter.Seq2[*protodb.Row[R], error] {
	return func(yield func(*protodb.Row[R], error) bool) {
		stmt, err := t.stmts.BuildStream(opts)
		if err != nil {
			yield(nil, err)
			return
		}
		it := spanneradapter.Query(ctx, t.cfg.Client, stmt)
		defer it.Stop()
		for {
			raw, err := it.Next()
			if err == iterator.Done {
				return
			}
			if err != nil {
				yield(nil, spanneradapter.ErrorToStatus(err))
				return
			}
			row, err := t.scanner.ScanRow(raw)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(row, nil) {
				return
			}
		}
	}
}

// WritePolicies honours the documented contract: every key is confirmed to
// exist before any policy is written, so a NotFound anywhere in the batch
// leaves every row's policy unchanged. On Spanner that check and the writes
// have to share one transaction, otherwise "validated then written" is only
// true until someone else deletes a row in between — so when the caller is
// not already inside a transaction, one is opened here.
func (t *spannerTable[R]) WritePolicies(ctx context.Context, entries ...protodb.PolicyEntry) error {
	if len(entries) == 0 {
		return nil
	}
	if t.cfg.PolicyColumn == "" {
		return status.Error(codes.Unimplemented, "table has no policy column")
	}
	if spanneradapter.SpannerTxFromContext(ctx) != nil {
		return t.writePolicies(ctx, entries)
	}
	runner := &spanneradapter.SpannerTransactionRunner{Client: t.cfg.Client}
	return runner.RunTransaction(ctx, func(ctx context.Context) error {
		return t.writePolicies(ctx, entries)
	})
}

func (t *spannerTable[R]) writePolicies(ctx context.Context, entries []protodb.PolicyEntry) error {
	keys := make([]protodb.Key, len(entries))
	for i, e := range entries {
		keys[i] = e.Key
	}

	it := spanneradapter.ReadKeySet(ctx, t.cfg.Client, t.cfg.TableName, spanneradapter.ToKeySets(keys), t.cfg.Spec.Columns())
	defer it.Stop()
	exists := make(map[string]bool, len(keys))
	for {
		raw, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return spanneradapter.ErrorToStatus(err)
		}
		key, err := t.cfg.Spec.Decode(raw)
		if err != nil {
			return spanneradapter.ErrorToStatus(err)
		}
		enc, err := t.cfg.Spec.Encode(key)
		if err != nil {
			return spanneradapter.ErrorToStatus(err)
		}
		exists[enc] = true
	}

	cols := append(t.cfg.Spec.Columns(), t.cfg.PolicyColumn)
	ms := make([]*spanner.Mutation, 0, len(entries))
	for _, e := range entries {
		enc, err := t.cfg.Spec.Encode(e.Key)
		if err != nil {
			return spanneradapter.ErrorToStatus(err)
		}
		if !exists[enc] {
			return status.Errorf(codes.NotFound, "row %v not found", e.Key.KeyValues())
		}
		vals := append([]any{}, e.Key.KeyValues()...)
		vals = append(vals, policyValue(e.Policy))
		ms = append(ms, spanner.Update(t.cfg.TableName, cols, vals))
	}
	return spanneradapter.Apply(ctx, t.cfg.Client, ms)
}

// truncate removes every row, so each conformance subtest starts from an
// empty table.
func (t *spannerTable[R]) truncate(ctx context.Context) error {
	_, err := t.cfg.Client.Apply(ctx, []*spanner.Mutation{spanner.Delete(t.cfg.TableName, spanner.AllKeys())})
	return spanneradapter.ErrorToStatus(err)
}
