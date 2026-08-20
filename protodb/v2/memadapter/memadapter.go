// Package memadapter provides an in-memory implementation of
// protodb.ResourceTable, used as a test double throughout the protodb
// ecosystem and as the second adapter (alongside spanneradapter) proving
// the ResourceTable seam.
//
// # Storage model
//
// A Table[R] holds its rows in a plain Go map, keyed by the canonical JSON
// encoding of the row's Key.KeyValues() (mirroring
// spanneradapter's KeySpec.Encode — meaningful only for equality
// comparison within one process, never decoded back). Each stored entry is
// created fresh and never mutated in place — Write and WritePolicies
// replace the map slot with a new *entry rather than editing fields of an
// existing one — so once a *entry is handed to a reader it is safe to keep
// using after the lock is released.
//
// R is deep-copied on the way in (Create/Write) and out (Read/BatchRead/
// List/Stream) whenever it implements proto.Message, via proto.Clone
// behind a type switch: this protects the store from a caller mutating the
// Row they passed to Create/Write after the call returns, and protects the
// caller from mutating memadapter's internal state through a Row a read
// returned. R's IAM Policy (*iampb.Policy) is cloned the same way.
//
// Caveat: for R that is NOT a proto.Message — including plain structs,
// non-proto pointer types, maps, and slices — memadapter stores and
// returns the value as-is (plain assignment). If such an R holds pointers,
// the caller and memadapter's internal state alias each other; mutating
// one mutates the other. Use a proto.Message resource type (the common
// case throughout protodb) to get memadapter's copy-on-write/copy-on-read
// isolation.
//
// # Locking and transactions
//
// See transaction.go: every Table[R] and every runner returned by
// NewTransactionRunner shares one package-global sync.Mutex.
package memadapter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"iter"
	"slices"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/iam/apiv1/iampb"
	"go.alis.build/protodb/v2"
	"go.alis.build/protodb/v2/ordering"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Config configures a Table.
type Config struct {
	// KeyColumns names the table's key columns, in the same order as the
	// values returned by every row's Key.KeyValues() — position i here
	// must correspond to KeyValues()[i]. This is how memadapter resolves
	// an OrderBy column name (or a Parent match, for the default
	// ParentMatch) back to a value: it has no schema for R itself, only
	// for the key.
	KeyColumns []string
	// ParentMatch decides whether key belongs under parent, for List/
	// Stream's Parent scoping. A nil ParentMatch defaults to a prefix
	// match ("parent" is a string prefix) against the first key value —
	// the natural scoping for hierarchical resource-name-shaped keys,
	// matching spanneradapter.StringKeySpec's ParentFilter.
	ParentMatch func(parent string, key protodb.Key) bool
}

// entry is one stored row. Immutable once published into Table.entries:
// every write replaces the map slot with a new *entry rather than editing
// an existing one's fields, so a reader that copied a pointer out from
// under the lock never observes a partial or concurrent mutation.
type entry[R any] struct {
	key      protodb.Key
	resource R
	policy   *iampb.Policy
}

// Table is an in-memory protodb.ResourceTable[R]. Create it with New.
type Table[R any] struct {
	cfg     Config
	entries map[string]*entry[R]
}

// Table implements protodb.ResourceTable — verified at compile time so a
// signature drift in either package fails the build here rather than
// surfacing as a mystifying error at every call site.
var _ protodb.ResourceTable[any] = (*Table[any])(nil)

// New returns an empty in-memory Table[R] configured by cfg.
func New[R any](cfg Config) *Table[R] {
	if cfg.ParentMatch == nil {
		cfg.ParentMatch = defaultParentMatch
	}
	return &Table[R]{cfg: cfg, entries: make(map[string]*entry[R])}
}

// defaultParentMatch is Config.ParentMatch's default: a prefix match of
// parent against the first key value (which must be a string).
func defaultParentMatch(parent string, key protodb.Key) bool {
	values := key.KeyValues()
	if len(values) == 0 {
		return false
	}
	s, ok := values[0].(string)
	if !ok {
		return false
	}
	return strings.HasPrefix(s, parent)
}

// canonicalKey returns key's map slot: the canonical JSON encoding of its
// KeyValues(), unique for a given combination of values. A JSON array of
// typed scalars is injective for the field types protodb keys use (string,
// int64, bool, float64, time.Time) because JSON encodes each with its
// type-specific syntax, so values can never bleed across a field boundary.
func canonicalKey(key protodb.Key) (string, error) {
	b, err := json.Marshal(key.KeyValues())
	if err != nil {
		// A Key returning values json.Marshal cannot encode (e.g. a
		// channel or func) is a caller/Key-implementation bug, not a
		// malformed request — Internal, not InvalidArgument.
		return "", status.Errorf(codes.Internal, "memadapter: encode key: %v", err)
	}
	return string(b), nil
}

// cloneResource deep-copies r when it implements proto.Message (via
// proto.Clone), and returns it unchanged otherwise. See the package doc
// for the aliasing caveat this implies for non-proto R.
func cloneResource[R any](r R) R {
	switch m := any(r).(type) {
	case proto.Message:
		if cloned, ok := any(proto.Clone(m)).(R); ok {
			return cloned
		}
	}
	return r
}

// clonePolicy deep-copies p, or returns nil for a nil p.
func clonePolicy(p *iampb.Policy) *iampb.Policy {
	if p == nil {
		return nil
	}
	return proto.Clone(p).(*iampb.Policy)
}

// toRow converts a stored entry to a Row the caller owns: both the
// resource and the policy are cloned out (see cloneResource/clonePolicy),
// so mutating the returned Row can never reach back into the table.
func toRow[R any](e *entry[R]) *protodb.Row[R] {
	return &protodb.Row[R]{Key: e.key, Resource: cloneResource(e.resource), Policy: clonePolicy(e.policy)}
}

// Create implements protodb.ResourceTable.
func (t *Table[R]) Create(ctx context.Context, rows ...*protodb.Row[R]) error {
	if len(rows) == 0 {
		return nil
	}
	defer lock(ctx)()

	canon := make([]string, len(rows))
	seen := make(map[string]bool, len(rows))
	for i, r := range rows {
		c, err := canonicalKey(r.Key)
		if err != nil {
			return err
		}
		if _, exists := t.entries[c]; exists {
			return status.Errorf(codes.AlreadyExists, "memadapter: row %v already exists", r.Key.KeyValues())
		}
		if seen[c] {
			return status.Errorf(codes.AlreadyExists, "memadapter: row %v already exists", r.Key.KeyValues())
		}
		seen[c] = true
		canon[i] = c
	}
	for i, r := range rows {
		t.entries[canon[i]] = &entry[R]{key: r.Key, resource: cloneResource(r.Resource), policy: clonePolicy(r.Policy)}
	}
	return nil
}

// Write implements protodb.ResourceTable.
func (t *Table[R]) Write(ctx context.Context, rows ...*protodb.Row[R]) error {
	if len(rows) == 0 {
		return nil
	}
	defer lock(ctx)()

	for _, r := range rows {
		c, err := canonicalKey(r.Key)
		if err != nil {
			return err
		}
		t.entries[c] = &entry[R]{key: r.Key, resource: cloneResource(r.Resource), policy: clonePolicy(r.Policy)}
	}
	return nil
}

// Read implements protodb.ResourceTable.
func (t *Table[R]) Read(ctx context.Context, key protodb.Key) (*protodb.Row[R], error) {
	defer lock(ctx)()

	c, err := canonicalKey(key)
	if err != nil {
		return nil, err
	}
	e, ok := t.entries[c]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "memadapter: row %v not found", key.KeyValues())
	}
	return toRow(e), nil
}

// BatchRead implements protodb.ResourceTable.
func (t *Table[R]) BatchRead(ctx context.Context, keys ...protodb.Key) ([]*protodb.Row[R], error) {
	if len(keys) == 0 {
		return nil, nil
	}
	defer lock(ctx)()

	rows := make([]*protodb.Row[R], len(keys))
	for i, k := range keys {
		c, err := canonicalKey(k)
		if err != nil {
			return nil, err
		}
		if e, ok := t.entries[c]; ok {
			rows[i] = toRow(e)
		}
	}
	return rows, nil
}

// Delete implements protodb.ResourceTable.
func (t *Table[R]) Delete(ctx context.Context, keys ...protodb.Key) error {
	if len(keys) == 0 {
		return nil
	}
	defer lock(ctx)()

	for _, k := range keys {
		c, err := canonicalKey(k)
		if err != nil {
			return err
		}
		delete(t.entries, c)
	}
	return nil
}

// WritePolicies implements protodb.ResourceTable. Writing a policy for a
// key that does not exist is a NotFound error — WritePolicies attaches a
// policy to an existing row, it does not create one.
//
// Like Create, the whole batch is validated — every key resolved,
// confirmed to already exist, and confirmed not to repeat within the same
// call — before anything is mutated. A NotFound (or a repeated key)
// anywhere in the batch leaves every row's policy untouched; there is no
// partial application, matching how a real Spanner mutation group either
// commits entirely or not at all.
func (t *Table[R]) WritePolicies(ctx context.Context, entries ...protodb.PolicyEntry) error {
	if len(entries) == 0 {
		return nil
	}
	defer lock(ctx)()

	canon := make([]string, len(entries))
	seen := make(map[string]bool, len(entries))
	for i, pe := range entries {
		c, err := canonicalKey(pe.Key)
		if err != nil {
			return err
		}
		if _, ok := t.entries[c]; !ok {
			return status.Errorf(codes.NotFound, "memadapter: row %v not found", pe.Key.KeyValues())
		}
		if seen[c] {
			return status.Errorf(codes.InvalidArgument, "memadapter: row %v specified more than once in the same WritePolicies call", pe.Key.KeyValues())
		}
		seen[c] = true
		canon[i] = c
	}
	for i, pe := range entries {
		e := t.entries[canon[i]]
		// Replace, never mutate in place — see entry's doc comment.
		t.entries[canon[i]] = &entry[R]{key: e.key, resource: e.resource, policy: clonePolicy(pe.Policy)}
	}
	return nil
}

// List implements protodb.ResourceTable.
func (t *Table[R]) List(ctx context.Context, opts protodb.ListOptions) ([]*protodb.Row[R], string, error) {
	if opts.PageSize < 0 {
		return nil, "", status.Errorf(codes.InvalidArgument, "memadapter: page size must not be negative, got %d", opts.PageSize)
	}
	pageSize := int(opts.PageSize)
	if pageSize == 0 {
		pageSize = protodb.DefaultPageSize
	}
	if strings.TrimSpace(opts.Filter) != "" {
		return nil, "", errFilterUnimplemented
	}
	order, err := t.effectiveOrder(opts.OrderBy, opts.Tail)
	if err != nil {
		return nil, "", err
	}
	fp := fingerprint(opts.Parent, opts.Filter, opts.OrderBy, opts.Tail)

	rows := t.orderedSnapshot(ctx, opts.Parent, order)

	start := 0
	if opts.PageToken != "" {
		cursor, err := decodePageToken(opts.PageToken, fp)
		if err != nil {
			return nil, "", err
		}
		if len(cursor) != len(order) {
			return nil, "", status.Error(codes.InvalidArgument, "memadapter: invalid page token")
		}
		// The first row that sorts strictly after the cursor in this
		// effective order — rows is already sorted by that same order,
		// so a binary search finds it directly.
		start = sort.Search(len(rows), func(i int) bool {
			return orderCompare(order, rows[i].tuple, cursor) > 0
		})
	}

	end := start + pageSize
	var nextToken string
	if end < len(rows) {
		nextToken = encodePageToken(rows[end-1].tuple, fp)
	} else {
		end = len(rows)
	}

	page := rows[start:end]
	out := make([]*protodb.Row[R], len(page))
	for i, p := range page {
		out[i] = toRow(p.e)
	}
	if opts.Tail {
		// page was gathered in the inverted order (effectiveOrder
		// inverted OrderBy for Tail); Tail must return the last N rows
		// in the ORIGINAL order, not the inverted one.
		slices.Reverse(out)
	}
	return out, nextToken, nil
}

// Stream implements protodb.ResourceTable.
func (t *Table[R]) Stream(ctx context.Context, opts protodb.StreamOptions) iter.Seq2[*protodb.Row[R], error] {
	return func(yield func(*protodb.Row[R], error) bool) {
		if strings.TrimSpace(opts.Filter) != "" {
			yield(nil, errFilterUnimplemented)
			return
		}
		order, err := t.effectiveOrder(opts.OrderBy, false)
		if err != nil {
			yield(nil, err)
			return
		}
		// Snapshot-then-iterate: the lock is held only long enough to
		// copy entry pointers and sort them (see orderedSnapshot), never
		// while calling back into yield. Holding a lock across a yield
		// would risk deadlock if the consumer's loop body calls back
		// into any memadapter table outside a transaction, and would
		// block writers for the whole traversal even when it wouldn't
		// deadlock.
		rows := t.orderedSnapshot(ctx, opts.Parent, order)
		for _, p := range rows {
			if err := ctx.Err(); err != nil {
				yield(nil, err)
				return
			}
			if !yield(toRow(p.e), nil) {
				return
			}
		}
	}
}

// errFilterUnimplemented is returned by List and Stream for any non-empty
// Filter. memadapter has no filter engine — this is a loud, deliberate
// limitation of the in-memory test double, not an oversight: consumers
// that need AIP-160 filtering exercised end-to-end should test against
// spanneradapter (or another adapter with a real filter implementation),
// not memadapter.
var errFilterUnimplemented = status.Error(codes.Unimplemented,
	"memadapter: List/Stream Filter is not implemented; only Filter == \"\" is supported")

// orderCol is one column of a resolved effective order: an index into a
// key's KeyValues() (per Config.KeyColumns) plus a sort direction.
type orderCol struct {
	idx  int
	desc bool
}

// effectiveOrder parses orderBy, inverts it when tail is set (Tail is
// served by inverting the order and paging forward — see List), and
// appends every key column it doesn't already mention as a tiebreaker
// (following the inversion, so the whole clause reads in one direction).
// This mirrors spanneradapter.StatementBuilder.effectiveOrder, with one
// difference forced by memadapter having no schema beyond the key: every
// column named in orderBy MUST be a key column (resolved via
// Config.KeyColumns), or effectiveOrder fails loudly with Unimplemented —
// memadapter cannot order by a column it doesn't know how to read a value
// for.
func (t *Table[R]) effectiveOrder(orderBy string, tail bool) ([]orderCol, error) {
	parsed, err := ordering.NewOrder(orderBy)
	if err != nil {
		// ordering.ErrInvalidOrder implements GRPCStatus() (InvalidArgument).
		return nil, err
	}
	if tail {
		parsed = parsed.Invert()
	}
	cols := parsed.Columns()

	order := make([]orderCol, 0, len(cols)+len(t.cfg.KeyColumns))
	seen := make(map[string]bool, len(cols))
	for _, c := range cols {
		idx := slices.Index(t.cfg.KeyColumns, c.Column)
		if idx < 0 {
			return nil, status.Errorf(codes.Unimplemented,
				"memadapter: OrderBy column %q is not a key column; memadapter can only order by key columns %v (ordering by non-key columns is not implemented)",
				c.Column, t.cfg.KeyColumns)
		}
		order = append(order, orderCol{idx: idx, desc: c.Desc})
		seen[c.Column] = true
	}
	for i, kc := range t.cfg.KeyColumns {
		if !seen[kc] {
			order = append(order, orderCol{idx: i, desc: tail})
		}
	}
	return order, nil
}

// snapshotRow pairs a stored entry with its precomputed sort tuple (its
// key values, projected and reordered per an effective order).
type snapshotRow[R any] struct {
	e     *entry[R]
	tuple []any
}

// orderedSnapshot copies out every entry matching parent (under lock),
// releases the lock, and returns them sorted by order. Because entries are
// never mutated in place (see entry's doc comment), it's safe to keep
// using the copied *entry pointers after the lock is released.
func (t *Table[R]) orderedSnapshot(ctx context.Context, parent string, order []orderCol) []snapshotRow[R] {
	unlock := lock(ctx)
	rows := make([]snapshotRow[R], 0, len(t.entries))
	for _, e := range t.entries {
		if parent != "" && !t.cfg.ParentMatch(parent, e.key) {
			continue
		}
		rows = append(rows, snapshotRow[R]{e: e, tuple: tupleFor(order, e.key.KeyValues())})
	}
	unlock()

	sort.Slice(rows, func(i, j int) bool {
		return orderCompare(order, rows[i].tuple, rows[j].tuple) < 0
	})
	return rows
}

// tupleFor projects keyValues down to order's columns, in order.
func tupleFor(order []orderCol, keyValues []any) []any {
	out := make([]any, len(order))
	for i, c := range order {
		out[i] = keyValues[c.idx]
	}
	return out
}

// orderCompare compares two tuples produced by tupleFor for the same
// order, column by column, honoring each column's direction. Returns -1 if
// a sorts before b under order, 1 if after, 0 if equal in every column
// order covers (which — because order always ends with every key column
// as a tiebreaker — only happens for two tuples belonging to the same
// row).
func orderCompare(order []orderCol, a, b []any) int {
	for i, c := range order {
		cmp := compareValue(a[i], b[i])
		if cmp == 0 {
			continue
		}
		if c.desc {
			cmp = -cmp
		}
		return cmp
	}
	return 0
}

// compareValue compares two key values of the same column. Supported
// types are the five protodb key field types (mirroring
// spanneradapter.KeySpecFor): string, int64, bool, float64, and
// time.Time. Any other type falls back to comparing fmt.Sprint(a) against
// fmt.Sprint(b) — deterministic but not a meaningful ordering — so a
// stray unsupported type degrades sort quality rather than panicking.
func compareValue(a, b any) int {
	switch av := a.(type) {
	case string:
		bv, _ := b.(string)
		return strings.Compare(av, bv)
	case int64:
		bv, _ := b.(int64)
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		default:
			return 0
		}
	case bool:
		bv, _ := b.(bool)
		switch {
		case av == bv:
			return 0
		case bv:
			return -1
		default:
			return 1
		}
	case float64:
		bv, _ := b.(float64)
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		default:
			return 0
		}
	case time.Time:
		bv, _ := b.(time.Time)
		switch {
		case av.Before(bv):
			return -1
		case av.After(bv):
			return 1
		default:
			return 0
		}
	default:
		return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
	}
}

// pageToken is the gob-encoded wire form of a memadapter page-token
// cursor. gob (not JSON) is used deliberately, mirroring
// spanneradapter/pagetoken.go: JSON decodes numeric values into float64,
// which would silently corrupt an int64 key value that exceeds float64's
// exact integer range.
type pageToken struct {
	Fingerprint uint64
	Values      []any // the last row's tuple (see tupleFor), in effective-order column sequence
}

func init() {
	// string, int64, bool, and float64 are gob's built-in basic types,
	// auto-registered by the gob package itself. time.Time is the one
	// non-primitive protodb key field type, so it needs explicit
	// registration to cross the []any (Values) interface boundary.
	gob.Register(time.Time{})
}

// encodePageToken serializes values together with fp into an opaque,
// base64url page-token string.
func encodePageToken(values []any, fp uint64) string {
	var buf bytes.Buffer
	// Encode errors here only arise from a key value type gob can't
	// carry, which — given canonicalKey already required json.Marshal to
	// succeed on the same values — would be a memadapter bug, not a
	// runtime condition callers can act on; encoding proceeds
	// best-effort, matching spanneradapter's reference implementation.
	_ = gob.NewEncoder(&buf).Encode(pageToken{Fingerprint: fp, Values: values})
	return base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

// decodePageToken decodes a page-token string produced by encodePageToken
// and validates it against fp. It returns an InvalidArgument status error
// if the token is malformed or was minted for a different
// parent/filter/orderBy/tail combination than fp represents.
func decodePageToken(s string, fp uint64) ([]any, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "memadapter: invalid page token")
	}
	var p pageToken
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return nil, status.Error(codes.InvalidArgument, "memadapter: invalid page token")
	}
	if p.Fingerprint != fp {
		return nil, status.Error(codes.InvalidArgument,
			"memadapter: page token does not match the request; tokens are only valid for the exact parent, filter, orderBy and tail that produced them")
	}
	return p.Values, nil
}

// fingerprint computes a stable fnv64a hash over the canonical tuple a
// page token must have been minted for: parent, filter, orderBy, and tail.
// Mirrors spanneradapter.Fingerprint's formula (reimplemented locally —
// memadapter must not import spanneradapter).
func fingerprint(parent, filter, orderBy string, tail bool) uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%q|%q|%q|%v", parent, filter, orderBy, tail)
	return h.Sum64()
}
