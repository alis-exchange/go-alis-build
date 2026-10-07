package spanneradapter_test

// Env-gated conformance test that runs the protodbtest suite — plus a
// multi-column tied-order pagination check — against a real Spanner, using
// the reference table in spanner_reftable_test.go. The emulator and the
// per-test databases come from spannertest.
//
// Gating (see the README's "Running the Spanner conformance test"):
//
//   - SPANNER_EMULATOR_HOST set → use that emulator, start no container.
//   - SPANNERTEST_EMULATOR, or this repo's older PROTODB_SPANNER_CONFORMANCE,
//     non-empty → start one Cloud Spanner emulator for the test binary.
//   - neither → skip.

import (
	"context"
	"fmt"
	"os"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"go.alis.build/protodb/v2"
	"go.alis.build/protodb/v2/filtering"
	"go.alis.build/protodb/v2/protobundle"
	"go.alis.build/protodb/v2/protodbtest"
	"go.alis.build/protodb/v2/spanneradapter"
	"go.alis.build/protodb/v2/spannertest"
)

const (
	// conformanceEnv, set to any value, starts an emulator in Docker. It
	// predates spannertest and maps onto SPANNERTEST_EMULATOR.
	conformanceEnv = "PROTODB_SPANNER_CONFORMANCE"

	// booksTable has a single string key.
	booksTable = "Books"
	// shelvesTable has a two-column tagged key.
	shelvesTable = "Shelves"
	// backupsTable holds Backup protos for the shared query cases.
	backupsTable = "Backups"
	// setsTable stores Backup protos in a resource column named Set, a
	// GoogleSQL keyword, to prove filters quote the column.
	setsTable = "Sets"
)

// newSpannerDatabase creates a fresh emulator database carrying the four
// test tables and the proto bundle for Policy and Backup, and returns a
// client bound to it. The database is dropped when t ends.
func newSpannerDatabase(t *testing.T) *spanner.Client {
	t.Helper()
	if os.Getenv(conformanceEnv) != "" && os.Getenv("SPANNERTEST_EMULATOR") == "" {
		t.Setenv("SPANNERTEST_EMULATOR", "1")
	}
	bundle, err := protobundle.New(&iampb.Policy{}, &databasepb.Backup{})
	if err != nil {
		t.Fatalf("building the proto bundle: %v", err)
	}
	return spannertest.NewDatabase(t, bundle,
		"CREATE TABLE "+booksTable+" ("+
			"`key` STRING(MAX) NOT NULL,"+
			"Res STRING(MAX),"+
			"Policy `google.iam.v1.Policy`,"+
			") PRIMARY KEY (`key`)",
		"CREATE TABLE "+shelvesTable+" ("+
			"a STRING(MAX) NOT NULL,"+
			"b STRING(MAX) NOT NULL,"+
			"Res STRING(MAX),"+
			"Policy `google.iam.v1.Policy`,"+
			") PRIMARY KEY (a, b)",
		// The shared query cases (internal/querytest) run against
		// Backup rows; create_time mirrors the generated timestamp
		// columns real tables order by.
		"CREATE TABLE "+backupsTable+" ("+
			"`key` STRING(MAX) NOT NULL,"+
			"Backup `google.spanner.admin.database.v1.Backup`,"+
			"Policy `google.iam.v1.Policy`,"+
			// Seconds only: the emulator cannot read the INT32 nanos field
			// (and the fixtures have no sub-second times).
			"create_time TIMESTAMP AS (TIMESTAMP_SECONDS(Backup.create_time.seconds)) STORED,"+
			") PRIMARY KEY (`key`)",
		"CREATE TABLE "+setsTable+" ("+
			"`key` STRING(MAX) NOT NULL,"+
			"`Set` `google.spanner.admin.database.v1.Backup`,"+
			"Policy `google.iam.v1.Policy`,"+
			") PRIMARY KEY (`key`)",
	)
}

// newBooksTable builds the single-string-key reference table.
func newBooksTable(client *spanner.Client, parser *filtering.Parser) *spannerTable[string] {
	return newSpannerTable(tableConfig[string]{
		Client:         client,
		TableName:      booksTable,
		Spec:           spanneradapter.StringKeySpec("key"),
		Codec:          spanneradapter.StringCodec(),
		Encode:         func(v string) any { return v },
		ResourceColumn: "Res",
		PolicyColumn:   "Policy",
		Parser:         parser,
	})
}

// shelfKey is the two-column tagged key behind the tied-order test.
type shelfKey struct {
	A string `pdb:"a"`
	B string `pdb:"b"`
}

// KeyValues implements protodb.Key.
func (k shelfKey) KeyValues() []any { return spanneradapter.KeyValuesOf(k) }

// newShelvesTable builds the two-column tagged-key reference table.
func newShelvesTable(t *testing.T, client *spanner.Client, parser *filtering.Parser) *spannerTable[string] {
	t.Helper()
	spec, err := spanneradapter.KeySpecFor[shelfKey]()
	if err != nil {
		t.Fatalf("KeySpecFor[shelfKey]: %v", err)
	}
	return newSpannerTable(tableConfig[string]{
		Client:         client,
		TableName:      shelvesTable,
		Spec:           spec,
		Codec:          spanneradapter.StringCodec(),
		Encode:         func(v string) any { return v },
		ResourceColumn: "Res",
		PolicyColumn:   "Policy",
		Parser:         parser,
	})
}

// newParser returns a filtering.Parser with no typed identifiers.
func newParser(t *testing.T) *filtering.Parser {
	t.Helper()
	parser, err := filtering.NewParser()
	if err != nil {
		t.Fatalf("filtering.NewParser: %v", err)
	}
	return parser
}

// TestSpannerConformance runs the full protodbtest suite against the
// reference Spanner table — the same suite memadapter runs, now executing
// the statement builder's SQL for real.
func TestSpannerConformance(t *testing.T) {
	client := newSpannerDatabase(t)
	parser := newParser(t)
	ctx := context.Background()

	protodbtest.Conformance[string]{
		NewTable: func(t *testing.T) protodb.ResourceTable[string] {
			table := newBooksTable(client, parser)
			// Every subtest expects an empty table; the emulator has no
			// cheap "fresh database", so the rows are cleared instead.
			if err := table.truncate(ctx); err != nil {
				t.Fatalf("truncating %s: %v", booksTable, err)
			}
			return table
		},
		Runner: func(t *testing.T) protodb.TransactionRunner {
			return &spanneradapter.SpannerTransactionRunner{Client: client}
		},
		MakeRow: func(i int) *protodb.Row[string] {
			return &protodb.Row[string]{
				Key:      spanneradapter.StringKey(fmt.Sprintf("k%03d", i)),
				Resource: fmt.Sprintf("v%d", i),
			}
		},
		Equal:          func(a, b string) bool { return a == b },
		SupportsFilter: true,
	}.Run(t)
}

// TestSpannerTiedOrderPagination executes the multi-column, null-safe
// keyset cursor against real Spanner: every row ties on the caller's
// OrderBy column, so only the appended key tiebreaker keeps the scan
// moving. A cursor built from a hand-picked subset of the effective order
// (the mistake the README warns about) would skip or repeat rows here.
func TestSpannerTiedOrderPagination(t *testing.T) {
	client := newSpannerDatabase(t)
	parser := newParser(t)
	ctx := context.Background()

	table := newShelvesTable(t, client, parser)
	if err := table.truncate(ctx); err != nil {
		t.Fatalf("truncating %s: %v", shelvesTable, err)
	}

	const aisle = "aisle-1"
	want := make([]shelfKey, 0, 5)
	for i := 1; i <= 5; i++ {
		key := shelfKey{A: aisle, B: fmt.Sprintf("b%03d", i)}
		want = append(want, key)
		if err := table.Create(ctx, &protodb.Row[string]{Key: key, Resource: fmt.Sprintf("v%d", i)}); err != nil {
			t.Fatalf("Create %v: %v", key, err)
		}
	}

	// Page one row at a time through rows that all share the same "a".
	var got []shelfKey
	seen := map[shelfKey]bool{}
	token := ""
	for page := 0; ; page++ {
		if page > len(want)+1 {
			t.Fatalf("pagination did not terminate after %d pages", page)
		}
		rows, next, err := table.List(ctx, protodb.ListOptions{PageSize: 1, OrderBy: "a", PageToken: token})
		if err != nil {
			t.Fatalf("List page %d: %v", page, err)
		}
		for _, row := range rows {
			key, ok := row.Key.(shelfKey)
			if !ok {
				t.Fatalf("page %d: key is %T, want shelfKey", page, row.Key)
			}
			if seen[key] {
				t.Fatalf("page %d: row %v repeated across pages", page, key)
			}
			seen[key] = true
			got = append(got, key)
		}
		if next == "" {
			break
		}
		token = next
	}

	if len(got) != len(want) {
		t.Fatalf("paged %d rows (%v), want %d (%v) — a row was skipped", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("page order at %d: got %v want %v (full: %v)", i, got[i], want[i], got)
		}
	}

	// One Tail page over the same table: the last two rows of the order,
	// still returned in that order.
	tail, _, err := table.List(ctx, protodb.ListOptions{PageSize: 2, OrderBy: "a", Tail: true})
	if err != nil {
		t.Fatalf("Tail List: %v", err)
	}
	if len(tail) != 2 {
		t.Fatalf("Tail returned %d rows, want 2", len(tail))
	}
	if tail[0].Key != want[3] || tail[1].Key != want[4] {
		t.Fatalf("Tail window/order wrong: got %v,%v want %v,%v", tail[0].Key, tail[1].Key, want[3], want[4])
	}
}

// TestSpannerNestedTransactionJoins checks against the emulator that a
// nested RunTransaction joins the outer one: both writes land in one commit.
func TestSpannerNestedTransactionJoins(t *testing.T) {
	client := newSpannerDatabase(t)
	ctx := context.Background()
	table := newBooksTable(client, newParser(t))
	if err := table.truncate(ctx); err != nil {
		t.Fatal(err)
	}
	runner := &spanneradapter.SpannerTransactionRunner{Client: client}
	err := runner.RunTransaction(ctx, func(ctx context.Context) error {
		if err := table.Create(ctx, &protodb.Row[string]{Key: spanneradapter.StringKey("outer"), Resource: "1"}); err != nil {
			return err
		}
		return runner.RunTransaction(ctx, func(ctx context.Context) error {
			return table.Create(ctx, &protodb.Row[string]{Key: spanneradapter.StringKey("inner"), Resource: "2"})
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"outer", "inner"} {
		if _, err := table.Read(ctx, spanneradapter.StringKey(k)); err != nil {
			t.Errorf("%s not committed: %v", k, err)
		}
	}
}
