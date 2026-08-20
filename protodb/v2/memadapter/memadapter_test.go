package memadapter_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"go.alis.build/protodb/v2"
	"go.alis.build/protodb/v2/memadapter"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// strKey is a local, single-string-column protodb.Key used by these smoke
// tests. memadapter tests must not import spanneradapter — memadapter is
// the second, independent adapter proving the ResourceTable seam, and a
// test-only dependency on the first adapter would undercut that.
type strKey string

func (k strKey) KeyValues() []any { return []any{string(k)} }

// TestMemCRUD is the brief's canonical smoke test: Create, duplicate
// Create, Read (hit and miss), and idempotent Delete of a missing key.
func TestMemCRUD(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey("a"), Resource: "1"}); err != nil {
		t.Fatal(err)
	}
	if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey("a"), Resource: "1"}); !protodb.IsAlreadyExists(err) {
		t.Fatal(err)
	}
	r, err := tbl.Read(ctx, strKey("a"))
	if err != nil || r.Resource != "1" {
		t.Fatal(r, err)
	}
	if _, err := tbl.Read(ctx, strKey("zz")); !protodb.IsNotFound(err) {
		t.Fatal(err)
	}
	if err := tbl.Delete(ctx, strKey("zz")); err != nil {
		t.Fatal("delete missing must be nil")
	}
}

// TestMemZeroRowWritesAreNoops checks the variadic write ops accept a
// zero-length call and return nil rather than erroring or panicking.
func TestMemZeroRowWritesAreNoops(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	if err := tbl.Create(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tbl.Write(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tbl.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tbl.WritePolicies(ctx); err != nil {
		t.Fatal(err)
	}
	if rows, err := tbl.BatchRead(ctx); err != nil || rows != nil {
		t.Fatalf("got %v, %v", rows, err)
	}
}

// TestMemListPagingAndTail exercises the paginated read path: PageSize 0
// defaulting, a two-page walk with no phantom last page, and Tail
// returning the last N rows in ascending (not reversed) order.
func TestMemListPagingAndTail(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	for _, k := range []string{"a", "b", "c", "d"} {
		if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey(k), Resource: k}); err != nil {
			t.Fatal(err)
		}
	}

	p1, tok, err := tbl.List(ctx, protodb.ListOptions{PageSize: 2})
	if err != nil || len(p1) != 2 || tok == "" {
		t.Fatalf("page 1: %v %q %v", p1, tok, err)
	}
	if p1[0].Resource != "a" || p1[1].Resource != "b" {
		t.Fatalf("page 1 order: %v", p1)
	}
	p2, tok2, err := tbl.List(ctx, protodb.ListOptions{PageSize: 2, PageToken: tok})
	if err != nil || len(p2) != 2 {
		t.Fatalf("page 2: %v %q %v", p2, tok2, err)
	}
	if p2[0].Resource != "c" || p2[1].Resource != "d" {
		t.Fatalf("page 2 order: %v", p2)
	}
	if tok2 != "" {
		t.Fatalf("exactly-full last page must not report a next page token, got %q", tok2)
	}

	tail, _, err := tbl.List(ctx, protodb.ListOptions{PageSize: 2, Tail: true})
	if err != nil || len(tail) != 2 {
		t.Fatalf("tail: %v %v", tail, err)
	}
	if tail[0].Resource != "c" || tail[1].Resource != "d" {
		t.Fatalf("tail order: %v", tail)
	}
}

// TestMemListNegativePageSize checks List rejects a negative PageSize with
// InvalidArgument.
func TestMemListNegativePageSize(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	if _, _, err := tbl.List(ctx, protodb.ListOptions{PageSize: -1}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got %v", err)
	}
}

// TestMemListBadPageToken checks that a page token minted for one query
// shape is rejected against a different one (fingerprint mismatch).
func TestMemListBadPageToken(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	for _, k := range []string{"a", "b", "c"} {
		if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey(k), Resource: k}); err != nil {
			t.Fatal(err)
		}
	}
	_, tok, err := tbl.List(ctx, protodb.ListOptions{PageSize: 1})
	if err != nil || tok == "" {
		t.Fatal(err)
	}
	if _, _, err := tbl.List(ctx, protodb.ListOptions{PageSize: 1, PageToken: tok, Parent: "different"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got %v", err)
	}
	if _, _, err := tbl.List(ctx, protodb.ListOptions{PageSize: 1, PageToken: "not-a-real-token"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("garbage token: got %v", err)
	}
}

// TestMemFilterIsUnimplemented checks that a non-empty Filter is loudly
// rejected on both List and Stream — memadapter has no filter engine.
func TestMemFilterIsUnimplemented(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey("a"), Resource: "1"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tbl.List(ctx, protodb.ListOptions{Filter: "x == 1"}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("List: got %v", err)
	}
	for _, err := range tbl.Stream(ctx, protodb.StreamOptions{Filter: "x == 1"}) {
		if status.Code(err) != codes.Unimplemented {
			t.Fatalf("Stream: got %v", err)
		}
	}
}

// TestMemOrderByNonKeyColumnIsUnimplemented checks that OrderBy naming a
// column memadapter cannot resolve to a key index is rejected loudly
// rather than silently ignored.
func TestMemOrderByNonKeyColumnIsUnimplemented(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	if _, _, err := tbl.List(ctx, protodb.ListOptions{OrderBy: "not_a_key_column"}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("got %v", err)
	}
}

// TestMemStreamAllAndBreak checks Stream yields every matching row with no
// hidden cap, and that a consumer `break` unwinds cleanly.
func TestMemStreamAllAndBreak(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	for i := 0; i < 150; i++ {
		key := strKey(fmt.Sprintf("k%03d", i))
		if err := tbl.Create(ctx, &protodb.Row[string]{Key: key, Resource: "v"}); err != nil {
			t.Fatal(key, err)
		}
	}
	var n int
	for row, err := range tbl.Stream(ctx, protodb.StreamOptions{}) {
		if err != nil {
			t.Fatal(err)
		}
		_ = row
		n++
	}
	if n != 150 {
		t.Fatalf("streamed %d, want 150 (hidden cap?)", n)
	}
	n = 0
	for _, err := range tbl.Stream(ctx, protodb.StreamOptions{}) {
		if err != nil {
			t.Fatal(err)
		}
		n++
		if n == 3 {
			break
		}
	}
}

// TestMemTransactionRunner checks that RunTransaction commits fn's writes
// and that table ops used inside fn (which run under the same
// package-global mutex, via the ctx marker) don't deadlock.
func TestMemTransactionRunner(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey("a"), Resource: "1"}); err != nil {
		t.Fatal(err)
	}
	runner := memadapter.NewTransactionRunner()
	err := runner.RunTransaction(ctx, func(ctx context.Context) error {
		row, err := tbl.Read(ctx, strKey("a"))
		if err != nil {
			return err
		}
		row.Resource = "2"
		return tbl.Write(ctx, row)
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := tbl.Read(ctx, strKey("a"))
	if err != nil || got.Resource != "2" {
		t.Fatalf("got %v, %v", got, err)
	}
}

// TestMemProtoResourceIsDeepCopied checks that a proto.Message resource is
// cloned both on the way in (Create/Write) and on the way out (Read):
// mutating the caller's Row after Create, or mutating a Row returned by
// Read, must never reach memadapter's stored state.
func TestMemProtoResourceIsDeepCopied(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[*wrapperspb.StringValue](memadapter.Config{KeyColumns: []string{"key"}})

	in := &protodb.Row[*wrapperspb.StringValue]{Key: strKey("a"), Resource: wrapperspb.String("original")}
	if err := tbl.Create(ctx, in); err != nil {
		t.Fatal(err)
	}
	// Mutate the caller's own copy after Create; the store must not see it.
	in.Resource.Value = "mutated-after-create"

	got, err := tbl.Read(ctx, strKey("a"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Resource.Value != "original" {
		t.Fatalf("Create aliased the caller's resource: got %q", got.Resource.Value)
	}

	// Mutate the Row a Read returned; a second Read must not see it.
	got.Resource.Value = "mutated-after-read"
	got2, err := tbl.Read(ctx, strKey("a"))
	if err != nil {
		t.Fatal(err)
	}
	if got2.Resource.Value != "original" {
		t.Fatalf("Read aliased the stored resource: got %q", got2.Resource.Value)
	}
}

// TestMemWritePoliciesAtomicOnPartialNotFound checks that WritePolicies
// validates the whole batch before mutating anything: a batch of
// [existing, missing] must fail NotFound and must not have applied the
// policy for the existing row either — no partial application, matching
// Create's behavior for a batch containing a bad key.
func TestMemWritePoliciesAtomicOnPartialNotFound(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey("a"), Resource: "1"}); err != nil {
		t.Fatal(err)
	}

	newPolicy := &iampb.Policy{Version: 3}
	err := tbl.WritePolicies(ctx,
		protodb.PolicyEntry{Key: strKey("a"), Policy: newPolicy},
		protodb.PolicyEntry{Key: strKey("missing"), Policy: newPolicy},
	)
	if !protodb.IsNotFound(err) {
		t.Fatalf("got %v, want NotFound", err)
	}

	got, err := tbl.Read(ctx, strKey("a"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Policy != nil {
		t.Fatalf("existing row's policy changed despite a NotFound elsewhere in the batch: %v", got.Policy)
	}
}

// TestMemWritePoliciesRejectsIntraBatchDuplicate checks that a repeated
// key within one WritePolicies call is rejected (InvalidArgument) rather
// than silently letting the last entry win — mirroring Create's rejection
// of a repeated key within one Create call.
func TestMemWritePoliciesRejectsIntraBatchDuplicate(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey("a"), Resource: "1"}); err != nil {
		t.Fatal(err)
	}

	err := tbl.WritePolicies(ctx,
		protodb.PolicyEntry{Key: strKey("a"), Policy: &iampb.Policy{Version: 1}},
		protodb.PolicyEntry{Key: strKey("a"), Policy: &iampb.Policy{Version: 2}},
	)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got %v, want InvalidArgument", err)
	}

	got, err := tbl.Read(ctx, strKey("a"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Policy != nil {
		t.Fatalf("existing row's policy changed despite a rejected duplicate-key batch: %v", got.Policy)
	}
}

// twoColKey is a two-column key used only by
// TestMemListPagesThroughTiedOrderColumn, where the first column ties
// across every row.
type twoColKey struct {
	A string
	B string
}

func (k twoColKey) KeyValues() []any { return []any{k.A, k.B} }

// TestMemListPagesThroughTiedOrderColumn exercises effectiveOrder's
// implicit tiebreaker (see memadapter.go): OrderBy names only the first key
// column ("a"), which is identical on every row here, so paging correctness
// depends entirely on effectiveOrder appending the second key column ("b")
// as a tiebreaker. protodbtest's adapter-agnostic conformance suite can't
// exercise this — it has no generic way to force a tie on a non-key column
// across rows — so this lives here instead, against memadapter directly.
// PageSize 1 forces a page token after every single row, so any tiebreaker
// bug (e.g. comparing only the OrderBy columns when computing/resuming the
// cursor) would either skip or repeat a row.
func TestMemListPagesThroughTiedOrderColumn(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"a", "b"}})

	const n = 5
	for i := 0; i < n; i++ {
		key := twoColKey{A: "tied", B: fmt.Sprintf("b%02d", i)}
		if err := tbl.Create(ctx, &protodb.Row[string]{Key: key, Resource: key.B}); err != nil {
			t.Fatal(err)
		}
	}

	var order []string
	seen := make(map[string]bool, n)
	var pageToken string
	for {
		rows, next, err := tbl.List(ctx, protodb.ListOptions{PageSize: 1, OrderBy: "a", PageToken: pageToken})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if seen[r.Resource] {
				t.Fatalf("row %q repeated across pages; full order so far: %v", r.Resource, order)
			}
			seen[r.Resource] = true
			order = append(order, r.Resource)
		}
		if next == "" {
			break
		}
		pageToken = next
	}
	if len(seen) != n {
		t.Fatalf("got %d distinct rows across pages, want %d (some skipped): %v", len(seen), n, order)
	}
	// With "a" tied on every row, the implicit tiebreaker on "b" makes the
	// full traversal order deterministic: ascending by b.
	want := []string{"b00", "b01", "b02", "b03", "b04"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("got order %v want %v", order, want)
	}
}

// TestMemTransactionRollsBackOnError checks memadapter's rollback
// mechanics directly: a Create followed by a returned error inside
// RunTransaction must leave the table exactly as it was before the
// transaction started, and a Write to a pre-existing row must revert to
// the row's pre-transaction value too (not just newly created rows).
func TestMemTransactionRollsBackOnError(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey("a"), Resource: "1"}); err != nil {
		t.Fatal(err)
	}

	runner := memadapter.NewTransactionRunner()
	wantErr := errors.New("boom")
	err := runner.RunTransaction(ctx, func(ctx context.Context) error {
		if err := tbl.Write(ctx, &protodb.Row[string]{Key: strKey("a"), Resource: "2"}); err != nil {
			return err
		}
		if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey("b"), Resource: "new"}); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want %v", err, wantErr)
	}

	got, err := tbl.Read(ctx, strKey("a"))
	if err != nil || got.Resource != "1" {
		t.Fatalf("row \"a\" not rolled back to its pre-transaction value: got %v, %v", got, err)
	}
	if _, err := tbl.Read(ctx, strKey("b")); !protodb.IsNotFound(err) {
		t.Fatalf("row \"b\" created by a rolled-back transaction is still visible: got %v", err)
	}
}
