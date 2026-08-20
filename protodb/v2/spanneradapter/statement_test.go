package spanneradapter

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"go.alis.build/protodb/v2"
	"go.alis.build/protodb/v2/filtering"
	"go.alis.build/protodb/v2/ordering"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newTestBuilder(t *testing.T) StatementBuilder {
	t.Helper()
	p, err := filtering.NewParser()
	if err != nil {
		t.Fatal(err)
	}
	return StatementBuilder{TableName: "T", Spec: StringKeySpec("key"), ResourceColumn: "Res", PolicyColumn: "Policy", Parser: p}
}

func TestBuildListMinimal(t *testing.T) {
	stmt, order, _, err := newTestBuilder(t).BuildList(protodb.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT `key`,`Res`,`Policy` FROM T ORDER BY `key` ASC LIMIT 101"
	if stmt.SQL != want {
		t.Fatalf("got %q", stmt.SQL)
	}
	if !reflect.DeepEqual(order, []ordering.ColumnOrder{{Column: "key", Desc: false}}) {
		t.Fatal(order)
	}
}

func TestBuildListOrderDeterministicWithTiebreaker(t *testing.T) {
	stmt, order, _, err := newTestBuilder(t).BuildList(protodb.ListOptions{OrderBy: "b desc, a"})
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []ordering.ColumnOrder{{Column: "b", Desc: true}, {Column: "a", Desc: false}, {Column: "key", Desc: false}}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatal(order)
	}
	if !strings.Contains(stmt.SQL, "ORDER BY `b` DESC, `a` ASC, `key` ASC") {
		t.Fatalf("%q", stmt.SQL)
	}
}

func TestBuildListTailInverts(t *testing.T) {
	_, order, _, _ := newTestBuilder(t).BuildList(protodb.ListOptions{OrderBy: "ts asc", Tail: true})
	want := []ordering.ColumnOrder{{Column: "ts", Desc: true}, {Column: "key", Desc: true}} // inverted incl. tiebreaker
	if !reflect.DeepEqual(order, want) {
		t.Fatal(order)
	}
}

func TestBuildListPageSizeRules(t *testing.T) {
	b := newTestBuilder(t)
	if _, _, _, err := b.BuildList(protodb.ListOptions{PageSize: -1}); status.Code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
	stmt, _, _, _ := b.BuildList(protodb.ListOptions{PageSize: 10})
	if !strings.HasSuffix(stmt.SQL, "LIMIT 11") {
		t.Fatalf("%q", stmt.SQL)
	}
}

func TestBuildListFilterParamsAndParent(t *testing.T) {
	stmt, _, _, err := newTestBuilder(t).BuildList(protodb.ListOptions{
		Parent: "res/", Filter: "a == param('x')", FilterParams: map[string]any{"x": "v"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stmt.SQL, "WHERE STARTS_WITH(`key`, @parent) AND (") &&
		!strings.Contains(stmt.SQL, "WHERE STARTS_WITH(`key`, @parent) AND a") {
		t.Fatalf("%q", stmt.SQL)
	}
	if stmt.Params["parent"] != "res/" {
		t.Fatal(stmt.Params)
	}
	if stmt.Params["p0"] != "v" {
		t.Fatalf("filter param not merged: %v", stmt.Params)
	}
}

func TestBuildListCursorPredicate(t *testing.T) {
	b := newTestBuilder(t)
	fp := Fingerprint("", "", "", false)
	tok := EncodePageToken(PageToken{OrderValues: []any{"k5"}, KeyValues: []any{"k5"}}, fp)
	stmt, _, _, err := b.BuildList(protodb.ListOptions{PageToken: tok})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stmt.SQL, "(`key` > @c0)") {
		t.Fatalf("cursor predicate missing: %q", stmt.SQL)
	}
	if stmt.Params["c0"] != "k5" {
		t.Fatal(stmt.Params)
	}
}

func TestBuildListTokenFingerprintMismatch(t *testing.T) {
	b := newTestBuilder(t)
	tok := EncodePageToken(PageToken{}, Fingerprint("other", "", "", false))
	if _, _, _, err := b.BuildList(protodb.ListOptions{PageToken: tok}); status.Code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
}

// --- beyond the brief -------------------------------------------------

func TestBuildListSelectsNonKeyOrderColumns(t *testing.T) {
	stmt, _, _, err := newTestBuilder(t).BuildList(protodb.ListOptions{OrderBy: "ts desc"})
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT `key`,`Res`,`Policy`,`ts` FROM T ORDER BY `ts` DESC, `key` ASC LIMIT 101"
	if stmt.SQL != want {
		t.Fatalf("got %q want %q", stmt.SQL, want)
	}
}

func TestBuildListWithoutPolicyColumn(t *testing.T) {
	b := newTestBuilder(t)
	b.PolicyColumn = ""
	stmt, _, _, err := b.BuildList(protodb.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT `key`,`Res` FROM T ORDER BY `key` ASC LIMIT 101"
	if stmt.SQL != want {
		t.Fatalf("got %q", stmt.SQL)
	}
}

func TestBuildListInvalidOrderBy(t *testing.T) {
	if _, _, _, err := newTestBuilder(t).BuildList(protodb.ListOptions{OrderBy: "1bad ascending"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

func TestBuildListInvalidFilter(t *testing.T) {
	if _, _, _, err := newTestBuilder(t).BuildList(protodb.ListOptions{Filter: "a =="}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

func TestBuildListFingerprintIsOverOriginalOptions(t *testing.T) {
	opts := protodb.ListOptions{Parent: "res/", Filter: "a == 'b'", OrderBy: "ts asc", Tail: true}
	_, _, fp, err := newTestBuilder(t).BuildList(opts)
	if err != nil {
		t.Fatal(err)
	}
	if want := Fingerprint(opts.Parent, opts.Filter, opts.OrderBy, opts.Tail); fp != want {
		t.Fatalf("fingerprint %d, want %d (over the ORIGINAL, non-inverted options)", fp, want)
	}
}

func TestBuildListTailCursorFollowsInvertedDirection(t *testing.T) {
	b := newTestBuilder(t)
	opts := protodb.ListOptions{Tail: true}
	fp := Fingerprint(opts.Parent, opts.Filter, opts.OrderBy, opts.Tail)
	opts.PageToken = EncodePageToken(PageToken{OrderValues: []any{"k5"}}, fp)
	stmt, _, _, err := b.BuildList(opts)
	if err != nil {
		t.Fatal(err)
	}
	// Tail inverts the effective order to DESC, so "follows" is the DESC form.
	if !strings.Contains(stmt.SQL, "(`key` IS NULL OR `key` < @c0)") {
		t.Fatalf("cursor predicate not inverted: %q", stmt.SQL)
	}
}

func TestBuildListTokenTooShortForOrder(t *testing.T) {
	b := newTestBuilder(t)
	opts := protodb.ListOptions{OrderBy: "a, b"}
	fp := Fingerprint(opts.Parent, opts.Filter, opts.OrderBy, opts.Tail)
	// Effective order is a, b, key — three columns — but the token carries one value.
	opts.PageToken = EncodePageToken(PageToken{OrderValues: []any{"x"}}, fp)
	if _, _, _, err := b.BuildList(opts); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

func TestBuildListParamNamespacesDoNotCollide(t *testing.T) {
	b := newTestBuilder(t)
	opts := protodb.ListOptions{Parent: "res/", Filter: "a == param('x')", FilterParams: map[string]any{"x": "v"}}
	fp := Fingerprint(opts.Parent, opts.Filter, opts.OrderBy, opts.Tail)
	opts.PageToken = EncodePageToken(PageToken{OrderValues: []any{"k5"}}, fp)
	stmt, _, _, err := b.BuildList(opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"parent", "p0", "c0"} {
		if _, ok := stmt.Params[name]; !ok {
			t.Fatalf("param %q missing from %v", name, stmt.Params)
		}
	}
	if len(stmt.Params) != 3 {
		t.Fatalf("unexpected params: %v", stmt.Params)
	}
}

func TestBuildStream(t *testing.T) {
	stmt, err := newTestBuilder(t).BuildStream(protodb.StreamOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT `key`,`Res`,`Policy` FROM T ORDER BY `key` ASC"
	if stmt.SQL != want {
		t.Fatalf("got %q", stmt.SQL)
	}
}

func TestBuildStreamParentFilterAndOrder(t *testing.T) {
	stmt, err := newTestBuilder(t).BuildStream(protodb.StreamOptions{
		Parent: "res/", Filter: "a == param('x')", FilterParams: map[string]any{"x": "v"}, OrderBy: "ts desc",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT `key`,`Res`,`Policy` FROM T WHERE STARTS_WITH(`key`, @parent) AND (a = @p0) ORDER BY `ts` DESC, `key` ASC"
	if stmt.SQL != want {
		t.Fatalf("got %q want %q", stmt.SQL, want)
	}
	if stmt.Params["parent"] != "res/" || stmt.Params["p0"] != "v" {
		t.Fatal(stmt.Params)
	}
	if strings.Contains(stmt.SQL, "LIMIT") {
		t.Fatalf("stream must not be limited: %q", stmt.SQL)
	}
}

func TestBuildStreamInvalidOrderBy(t *testing.T) {
	if _, err := newTestBuilder(t).BuildStream(protodb.StreamOptions{OrderBy: "1bad"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

func TestCursorPredicateExpansion(t *testing.T) {
	order := []ordering.ColumnOrder{{Column: "a", Desc: false}, {Column: "b", Desc: true}, {Column: "key", Desc: false}}
	params := map[string]any{}
	got := cursorPredicate(order, []any{"x", int64(3), "k"}, params)
	want := "((`a` > @c0) OR " +
		"((`a` = @c0) AND (`b` IS NULL OR `b` < @c1)) OR " +
		"((`a` = @c0) AND (`b` = @c1) AND (`key` > @c2)))"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if !reflect.DeepEqual(params, map[string]any{"c0": "x", "c1": int64(3), "c2": "k"}) {
		t.Fatal(params)
	}
}

func TestCursorPredicateNullValuesBindNoParams(t *testing.T) {
	order := []ordering.ColumnOrder{{Column: "a", Desc: false}, {Column: "b", Desc: true}, {Column: "key", Desc: false}}
	params := map[string]any{}
	got := cursorPredicate(order, []any{nil, nil, "k"}, params)
	want := "((`a` IS NOT NULL) OR " +
		"((`a` IS NULL) AND (FALSE)) OR " +
		"((`a` IS NULL) AND (`b` IS NULL) AND (`key` > @c2)))"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if !reflect.DeepEqual(params, map[string]any{"c2": "k"}) {
		t.Fatalf("nil values must not be bound: %v", params)
	}
}

func TestOrderValuesFromRow(t *testing.T) {
	ts := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	row, err := spanner.NewRow(
		[]string{"s", "i", "f", "b", "t", "n"},
		[]any{"x", int64(7), 1.5, true, ts, spanner.NullString{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	order := []ordering.ColumnOrder{
		{Column: "s"}, {Column: "i"}, {Column: "f"}, {Column: "b"}, {Column: "t"}, {Column: "n"},
	}
	got, err := OrderValuesFromRow(row, order)
	if err != nil {
		t.Fatal(err)
	}
	want := []any{"x", int64(7), 1.5, true, ts, nil}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestOrderValuesFromRowRoundTripsThroughPageToken(t *testing.T) {
	ts := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	row, err := spanner.NewRow([]string{"ts", "key"}, []any{ts, "k5"})
	if err != nil {
		t.Fatal(err)
	}
	order := []ordering.ColumnOrder{{Column: "ts", Desc: true}, {Column: "key", Desc: true}}
	vals, err := OrderValuesFromRow(row, order)
	if err != nil {
		t.Fatal(err)
	}
	fp := Fingerprint("", "", "ts asc", true)
	tok, err := DecodePageToken(EncodePageToken(PageToken{OrderValues: vals}, fp), fp)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tok.OrderValues, vals) {
		t.Fatalf("round trip changed values: %#v vs %#v", tok.OrderValues, vals)
	}
}

func TestOrderValuesFromRowMissingColumn(t *testing.T) {
	row, err := spanner.NewRow([]string{"a"}, []any{"x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OrderValuesFromRow(row, []ordering.ColumnOrder{{Column: "missing"}}); err == nil {
		t.Fatal("want error for a column absent from the row")
	}
}

func TestOrderValuesFromRowUnsupportedType(t *testing.T) {
	row, err := spanner.NewRow([]string{"a"}, []any{[]string{"x", "y"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OrderValuesFromRow(row, []ordering.ColumnOrder{{Column: "a"}}); err == nil {
		t.Fatal("want error for an unsupported (non-scalar) column type")
	}
}
