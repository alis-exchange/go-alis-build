// Package protodbtest provides a conformance suite that any
// protodb.ResourceTable implementation can run against itself. The suite is
// the behavioral contract: it is written out in full as ordinary Go test
// code (not a checklist), so a table implementation that passes
// Conformance[R].Run(t) is, by definition, conformant.
//
// This package intentionally depends only on protodb and testing (plus
// gRPC's codes/status for error-code assertions) — never on a specific
// adapter (memadapter, spanneradapter, ...). Adapters import protodbtest to
// prove themselves against it; protodbtest must never import an adapter,
// or the suite would no longer be adapter-agnostic.
package protodbtest

import (
	"context"
	"testing"

	"go.alis.build/protodb/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Conformance is a table-driven conformance suite for a
// protodb.ResourceTable[R] implementation. Construct one with every field
// set appropriately for the implementation under test and call Run.
type Conformance[R any] struct {
	// NewTable returns a fresh, empty table for a single subtest to use.
	NewTable func(t *testing.T) protodb.ResourceTable[R]
	// Runner returns a protodb.TransactionRunner for the implementation
	// under test. A nil Runner skips the transactional subtests.
	Runner func(t *testing.T) protodb.TransactionRunner
	// MakeRow deterministically builds the i-th row. Keys must be strictly
	// ordered by i under the table's default order.
	MakeRow func(i int) *protodb.Row[R]
	// Equal reports whether two resources are equal.
	Equal func(a, b R) bool
	// SupportsFilter reports whether the implementation under test
	// implements AIP-160 List/Stream filtering. When false, the suite
	// asserts that a non-empty Filter returns Unimplemented.
	SupportsFilter bool
}

// Run executes every conformance subtest against the table implementation
// described by c.
func (c Conformance[R]) Run(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateThenReadRoundtrips", func(t *testing.T) {
		tbl := c.NewTable(t)
		want := c.MakeRow(1)
		if err := tbl.Create(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, err := tbl.Read(ctx, want.Key)
		if err != nil {
			t.Fatal(err)
		}
		if !c.Equal(got.Resource, want.Resource) {
			t.Fatalf("got %v want %v", got.Resource, want.Resource)
		}
	})

	t.Run("CreateExistingIsAlreadyExists", func(t *testing.T) {
		tbl := c.NewTable(t)
		r := c.MakeRow(1)
		must(t, tbl.Create(ctx, r))
		if err := tbl.Create(ctx, c.MakeRow(1)); !protodb.IsAlreadyExists(err) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("ReadMissingIsNotFound", func(t *testing.T) {
		tbl := c.NewTable(t)
		if _, err := tbl.Read(ctx, c.MakeRow(99).Key); !protodb.IsNotFound(err) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("WriteUpserts", func(t *testing.T) {
		tbl := c.NewTable(t)
		must(t, tbl.Write(ctx, c.MakeRow(1))) // insert path
		must(t, tbl.Write(ctx, c.MakeRow(1))) // update path
	})

	t.Run("DeleteMissingIsNil", func(t *testing.T) {
		tbl := c.NewTable(t)
		if err := tbl.Delete(ctx, c.MakeRow(99).Key); err != nil {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("BatchReadNilSlots", func(t *testing.T) {
		tbl := c.NewTable(t)
		must(t, tbl.Create(ctx, c.MakeRow(1), c.MakeRow(3)))
		rows, err := tbl.BatchRead(ctx, c.MakeRow(1).Key, c.MakeRow(2).Key, c.MakeRow(3).Key)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 3 || rows[0] == nil || rows[1] != nil || rows[2] == nil {
			t.Fatalf("%v", rows)
		}
	})

	t.Run("ZeroRowWritesAreNoops", func(t *testing.T) {
		tbl := c.NewTable(t)
		must(t, tbl.Create(ctx))
		must(t, tbl.Write(ctx))
		must(t, tbl.Delete(ctx))
	})

	t.Run("ListPaginatesWithoutPhantomPage", func(t *testing.T) {
		tbl := c.NewTable(t)
		for i := 1; i <= 4; i++ {
			must(t, tbl.Create(ctx, c.MakeRow(i)))
		}
		p1, tok, err := tbl.List(ctx, protodb.ListOptions{PageSize: 2})
		if err != nil || len(p1) != 2 || tok == "" {
			t.Fatalf("%v %q %v", p1, tok, err)
		}
		p2, tok2, err := tbl.List(ctx, protodb.ListOptions{PageSize: 2, PageToken: tok})
		if err != nil || len(p2) != 2 {
			t.Fatal(err)
		}
		if tok2 != "" { // exactly-full last page must NOT report another page
			p3, _, err := tbl.List(ctx, protodb.ListOptions{PageSize: 2, PageToken: tok2})
			if err != nil {
				t.Fatal(err)
			}
			if len(p3) != 0 {
				t.Fatal("phantom page returned rows")
			}
			t.Fatal("phantom next-page token on exactly-full last page")
		}
	})

	t.Run("ListNegativePageSizeIsInvalidArgument", func(t *testing.T) {
		tbl := c.NewTable(t)
		_, _, err := tbl.List(ctx, protodb.ListOptions{PageSize: -1})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("ListTokenFingerprintEnforced", func(t *testing.T) {
		tbl := c.NewTable(t)
		for i := 1; i <= 3; i++ {
			must(t, tbl.Create(ctx, c.MakeRow(i)))
		}
		_, tok, err := tbl.List(ctx, protodb.ListOptions{PageSize: 1})
		if err != nil || tok == "" {
			t.Fatal(err)
		}
		_, _, err = tbl.List(ctx, protodb.ListOptions{PageSize: 1, PageToken: tok, Parent: "different"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("ListTailReturnsLastNInOrder", func(t *testing.T) {
		tbl := c.NewTable(t)
		for i := 1; i <= 5; i++ {
			must(t, tbl.Create(ctx, c.MakeRow(i)))
		}
		rows, _, err := tbl.List(ctx, protodb.ListOptions{PageSize: 2, Tail: true})
		if err != nil || len(rows) != 2 {
			t.Fatal(err)
		}
		// last two in default order: rows for i=4 then i=5
		if !c.Equal(rows[0].Resource, c.MakeRow(4).Resource) || !c.Equal(rows[1].Resource, c.MakeRow(5).Resource) {
			t.Fatalf("tail returned wrong window/order")
		}
	})

	t.Run("StreamYieldsAllAndStopsOnBreak", func(t *testing.T) {
		tbl := c.NewTable(t)
		for i := 1; i <= 250; i++ {
			must(t, tbl.Create(ctx, c.MakeRow(i)))
		} // >100: no hidden cap
		var n int
		for row, err := range tbl.Stream(ctx, protodb.StreamOptions{}) {
			if err != nil {
				t.Fatal(err)
			}
			_ = row
			n++
		}
		if n != 250 {
			t.Fatalf("streamed %d, want 250 (hidden cap?)", n)
		}
		n = 0
		for _, err := range tbl.Stream(ctx, protodb.StreamOptions{}) {
			if err != nil {
				t.Fatal(err)
			}
			n++
			if n == 3 {
				break
			} // must not leak or panic
		}
	})

	t.Run("FilterContract", func(t *testing.T) {
		tbl := c.NewTable(t)
		must(t, tbl.Create(ctx, c.MakeRow(1)))
		_, _, err := tbl.List(ctx, protodb.ListOptions{Filter: "x == 1"})
		if c.SupportsFilter {
			if err != nil && status.Code(err) == codes.Unimplemented {
				t.Fatal("declared filter support but Unimplemented")
			}
		} else if status.Code(err) != codes.Unimplemented {
			t.Fatalf("want Unimplemented, got %v", err)
		}
	})

	t.Run("ReadModifyWrite", func(t *testing.T) {
		if c.Runner == nil {
			t.Skip("no runner provided")
		}
		tbl := c.NewTable(t)
		must(t, tbl.Create(ctx, c.MakeRow(1)))
		mutated := c.MakeRow(2).Resource
		err := protodb.ReadModifyWrite(ctx, c.Runner(t), tbl, c.MakeRow(1).Key, func(r *protodb.Row[R]) error {
			r.Resource = mutated
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		got, _ := tbl.Read(ctx, c.MakeRow(1).Key)
		if !c.Equal(got.Resource, mutated) {
			t.Fatal("rmw not applied")
		}
	})
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
