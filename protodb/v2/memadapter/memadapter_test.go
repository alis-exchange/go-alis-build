package memadapter_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"go.alis.build/protodb/v2"
	"go.alis.build/protodb/v2/memadapter"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/timestamppb"
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

// newBackups returns a filterable Backup table holding rows.
func newBackups(t *testing.T, cfg memadapter.Config, rows ...*databasepb.Backup) *memadapter.Table[*databasepb.Backup] {
	t.Helper()
	tbl := memadapter.New[*databasepb.Backup](cfg)
	for _, b := range rows {
		if err := tbl.Create(context.Background(), &protodb.Row[*databasepb.Backup]{Key: strKey(b.GetName()), Resource: b}); err != nil {
			t.Fatal(err)
		}
	}
	return tbl
}

// listNames lists every row matching filter and returns their names.
func listNames(t *testing.T, tbl protodb.ResourceTable[*databasepb.Backup], filter string) ([]string, error) {
	t.Helper()
	rows, _, err := tbl.List(context.Background(), protodb.ListOptions{Filter: filter})
	if err != nil {
		return nil, err
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Resource.GetName()
	}
	return names, nil
}

func TestMemFilterUnknownPathIsInvalidArgument(t *testing.T) {
	tbl := newBackups(t, backupConfig(), &databasepb.Backup{Name: "a"})
	if _, err := listNames(t, tbl, "Backup.nope = 1"); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got %v, want InvalidArgument", err)
	}
	if _, err := listNames(t, tbl, "nope = 1"); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got %v, want InvalidArgument", err)
	}
}

func TestMemFilterOnNonProtoResourceIsUnimplemented(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}, ResourceColumn: "R"})
	if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey("a"), Resource: "1"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tbl.List(ctx, protodb.ListOptions{Filter: "R.x = 1"}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("got %v, want Unimplemented", err)
	}
	// The key still works without a proto resource.
	rows, _, err := tbl.List(ctx, protodb.ListOptions{Filter: "key = 'a'"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("got %v, %v", rows, err)
	}
}

func TestMemFilterUnsupportedConstructIsUnimplemented(t *testing.T) {
	tbl := newBackups(t, backupConfig(), &databasepb.Backup{Name: "a"})
	if _, err := listNames(t, tbl, "{ 'a': 1 } == Backup.name"); status.Code(err) != codes.Unimplemented {
		t.Fatalf("got %v, want Unimplemented", err)
	}
}

func TestMemFilterInvalidFilterIsInvalidArgument(t *testing.T) {
	tbl := newBackups(t, backupConfig(), &databasepb.Backup{Name: "a"})
	if _, err := listNames(t, tbl, "Backup.name = "); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got %v, want InvalidArgument", err)
	}
}

func TestMemFilterOnColumn(t *testing.T) {
	cfg := backupConfig()
	cfg.Columns = map[string]func(protodb.Key, any) any{
		"create_time": func(_ protodb.Key, r any) any {
			b := r.(*databasepb.Backup)
			if b.GetCreateTime() == nil {
				return nil
			}
			return b.GetCreateTime().AsTime()
		},
	}
	tbl := newBackups(t, cfg,
		&databasepb.Backup{Name: "old", CreateTime: timestamppb.New(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))},
		&databasepb.Backup{Name: "new", CreateTime: timestamppb.New(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))},
		&databasepb.Backup{Name: "none"},
	)
	got, err := listNames(t, tbl, "create_time > timestamp('2024-01-01T00:00:00Z')")
	if err != nil || !reflect.DeepEqual(got, []string{"new"}) {
		t.Fatalf("got %v, %v; want [new]", got, err)
	}
	got, err = listNames(t, tbl, "create_time = NULL")
	if err != nil || !reflect.DeepEqual(got, []string{"none"}) {
		t.Fatalf("got %v, %v; want [none]", got, err)
	}
}

// TestMemFilterFieldPresence pins how unset fields read, matching Spanner:
// an unset message field (or a path through one) is NULL, a proto3 scalar
// without presence reads its default, and repeated fields are not supported.
func TestMemFilterFieldPresence(t *testing.T) {
	tbl := newBackups(t, backupConfig(),
		&databasepb.Backup{Name: "bare"},
		&databasepb.Backup{Name: "sized", SizeBytes: 10, EncryptionInfo: &databasepb.EncryptionInfo{
			EncryptionType: databasepb.EncryptionInfo_GOOGLE_DEFAULT_ENCRYPTION,
		}},
	)
	cases := []struct {
		filter string
		want   []string
	}{
		{"Backup.size_bytes = 0", []string{"bare"}},
		{"Backup.size_bytes = NULL", nil},
		{"Backup.encryption_info = NULL", []string{"bare"}},
		{"Backup.encryption_info.encryption_type = NULL", []string{"bare"}},
		{"Backup.encryption_info.encryption_type = 'GOOGLE_DEFAULT_ENCRYPTION'", []string{"sized"}},
	}
	for _, tc := range cases {
		got, err := listNames(t, tbl, tc.filter)
		if err != nil {
			t.Errorf("%s: %v", tc.filter, err)
			continue
		}
		if len(got) == 0 {
			got = nil
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.filter, got, tc.want)
		}
	}
	if _, err := listNames(t, tbl, "Backup.referencing_databases = 'x'"); status.Code(err) != codes.Unimplemented {
		t.Errorf("repeated field: got %v, want Unimplemented", err)
	}
}

// TestMemFilterExplicitPresence checks that an unset proto3 `optional`
// scalar is NULL, unlike a scalar without presence.
func TestMemFilterExplicitPresence(t *testing.T) {
	desc := optionalMessage(t)
	set := dynamicpb.NewMessage(desc)
	set.Set(desc.Fields().ByName("name"), protoreflect.ValueOfString("set"))
	set.Set(desc.Fields().ByName("count"), protoreflect.ValueOfInt64(0))
	unset := dynamicpb.NewMessage(desc)
	unset.Set(desc.Fields().ByName("name"), protoreflect.ValueOfString("unset"))

	ctx := context.Background()
	tbl := memadapter.New[proto.Message](memadapter.Config{KeyColumns: []string{"key"}, ResourceColumn: "M"})
	for _, m := range []proto.Message{set, unset} {
		name := m.ProtoReflect().Get(desc.Fields().ByName("name")).String()
		if err := tbl.Create(ctx, &protodb.Row[proto.Message]{Key: strKey(name), Resource: m}); err != nil {
			t.Fatal(err)
		}
	}
	rows, _, err := tbl.List(ctx, protodb.ListOptions{Filter: "M.count = NULL"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Key.KeyValues()[0] != "unset" {
		t.Fatalf("got %v, want only the unset row", rows)
	}
}

// optionalMessage builds a proto3 message `M { string name = 1; optional
// int64 count = 2; }` at runtime.
func optionalMessage(t *testing.T) protoreflect.MessageDescriptor {
	t.Helper()
	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:    proto.String("memadapter_optional_test.proto"),
		Package: proto.String("memadaptertest"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("M"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name: proto.String("name"), Number: proto.Int32(1), JsonName: proto.String("name"),
					Type:  descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				},
				{
					Name: proto.String("count"), Number: proto.Int32(2), JsonName: proto.String("count"),
					Type:           descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
					Label:          descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					OneofIndex:     proto.Int32(0),
					Proto3Optional: proto.Bool(true),
				},
			},
			OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("_count")}},
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return fd.Messages().ByName("M")
}

// pairKey is a two-column key whose second column is named "key".
type pairKey struct{ parent, key string }

func (k pairKey) KeyValues() []any { return []any{k.parent, k.key} }

func TestMemFilterKeyColumnNamedKeyWins(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[*databasepb.Backup](memadapter.Config{
		KeyColumns:     []string{"parent", "key"},
		ResourceColumn: "Backup",
	})
	row := &protodb.Row[*databasepb.Backup]{Key: pairKey{"p", "x"}, Resource: &databasepb.Backup{Name: "x"}}
	if err := tbl.Create(ctx, row); err != nil {
		t.Fatal(err)
	}
	rows, _, err := tbl.List(ctx, protodb.ListOptions{Filter: "key = 'x'"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("got %v, %v; want the row whose key column is x", rows, err)
	}
}

func TestMemFilterUnsignedFieldsReadAsInt64(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[*wrapperspb.UInt64Value](memadapter.Config{KeyColumns: []string{"key"}, ResourceColumn: "W"})
	if err := tbl.Create(ctx, &protodb.Row[*wrapperspb.UInt64Value]{Key: strKey("a"), Resource: wrapperspb.UInt64(5)}); err != nil {
		t.Fatal(err)
	}
	rows, _, err := tbl.List(ctx, protodb.ListOptions{Filter: "W.value = 5"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("got %v, %v; want one row", rows, err)
	}
}

func TestMemFilterNilResourceIsNull(t *testing.T) {
	ctx := context.Background()
	tbl := memadapter.New[*databasepb.Backup](backupConfig())
	if err := tbl.Create(ctx, &protodb.Row[*databasepb.Backup]{Key: strKey("nil")}); err != nil {
		t.Fatal(err)
	}
	for filter, want := range map[string]int{"Backup.size_bytes = 0": 0, "Backup.size_bytes = NULL": 1, "Backup = NULL": 1} {
		rows, _, err := tbl.List(ctx, protodb.ListOptions{Filter: filter})
		if err != nil || len(rows) != want {
			t.Errorf("%s: got %d rows, %v; want %d", filter, len(rows), err, want)
		}
	}
}

func TestMemFilterWithTail(t *testing.T) {
	tbl := newBackups(t, backupConfig(),
		&databasepb.Backup{Name: "a", SizeBytes: 1},
		&databasepb.Backup{Name: "b", SizeBytes: 2},
		&databasepb.Backup{Name: "c", SizeBytes: 1},
		&databasepb.Backup{Name: "d", SizeBytes: 2},
		&databasepb.Backup{Name: "e", SizeBytes: 1},
	)
	rows, _, err := tbl.List(context.Background(), protodb.ListOptions{Filter: "Backup.size_bytes = 1", PageSize: 2, Tail: true})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Resource.GetName())
	}
	if !reflect.DeepEqual(got, []string{"c", "e"}) {
		t.Fatalf("got %v, want the last two matches [c e]", got)
	}
}

// orderedNames lists every row of tbl in orderBy order and returns their keys.
func orderedNames(t *testing.T, tbl *memadapter.Table[string], orderBy string) ([]string, error) {
	t.Helper()
	rows, _, err := tbl.List(context.Background(), protodb.ListOptions{OrderBy: orderBy})
	if err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Key.KeyValues()[0].(string)
	}
	return out, nil
}

func TestMemOrderByColumnBadTypeIsFailedPrecondition(t *testing.T) {
	tbl := memadapter.New[string](memadapter.Config{
		KeyColumns: []string{"key"},
		Columns:    map[string]func(protodb.Key, any) any{"bad": func(protodb.Key, any) any { return struct{}{} }},
	})
	if err := tbl.Create(context.Background(), &protodb.Row[string]{Key: strKey("a"), Resource: "1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := orderedNames(t, tbl, "bad"); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("got %v, want FailedPrecondition", err)
	}
}

// TestMemOrderByColumnWidensNumbers checks that int32 and float32 column
// values order numerically after widening to int64 and float64.
func TestMemOrderByColumnWidensNumbers(t *testing.T) {
	ints := map[string]int32{"a": 3, "b": 1, "c": 2}
	floats := map[string]float32{"a": 3.5, "b": 1.5, "c": 2.5}
	tbl := memadapter.New[string](memadapter.Config{
		KeyColumns: []string{"key"},
		Columns: map[string]func(protodb.Key, any) any{
			"i": func(k protodb.Key, _ any) any { return ints[k.KeyValues()[0].(string)] },
			"f": func(k protodb.Key, _ any) any { return floats[k.KeyValues()[0].(string)] },
		},
	})
	for _, k := range []string{"a", "b", "c"} {
		if err := tbl.Create(context.Background(), &protodb.Row[string]{Key: strKey(k), Resource: k}); err != nil {
			t.Fatal(err)
		}
	}
	for _, col := range []string{"i", "f"} {
		got, err := orderedNames(t, tbl, col)
		if err != nil || !reflect.DeepEqual(got, []string{"b", "c", "a"}) {
			t.Errorf("OrderBy %s: got %v, %v; want [b c a]", col, got, err)
		}
	}
}

// TestMemOrderByColumnWidensIntegerKinds covers named integer types such as
// proto enums, and unsigned integers.
func TestMemOrderByColumnWidensIntegerKinds(t *testing.T) {
	states := map[string]databasepb.Backup_State{
		"a": databasepb.Backup_READY,
		"b": databasepb.Backup_STATE_UNSPECIFIED,
		"c": databasepb.Backup_CREATING,
	}
	sizes := map[string]uint64{"a": 30, "b": 10, "c": 20}
	tbl := memadapter.New[string](memadapter.Config{
		KeyColumns: []string{"key"},
		Columns: map[string]func(protodb.Key, any) any{
			"state": func(k protodb.Key, _ any) any { return states[k.KeyValues()[0].(string)] },
			"size":  func(k protodb.Key, _ any) any { return sizes[k.KeyValues()[0].(string)] },
		},
	})
	for _, k := range []string{"a", "b", "c"} {
		if err := tbl.Create(context.Background(), &protodb.Row[string]{Key: strKey(k), Resource: k}); err != nil {
			t.Fatal(err)
		}
	}
	for _, col := range []string{"state", "size"} {
		got, err := orderedNames(t, tbl, col)
		if err != nil || !reflect.DeepEqual(got, []string{"b", "c", "a"}) {
			t.Errorf("OrderBy %s: got %v, %v; want [b c a]", col, got, err)
		}
	}
}

func TestMemOrderByColumnMixedTypesIsFailedPrecondition(t *testing.T) {
	tbl := memadapter.New[string](memadapter.Config{
		KeyColumns: []string{"key"},
		Columns: map[string]func(protodb.Key, any) any{
			"mixed": func(k protodb.Key, _ any) any {
				if k.KeyValues()[0] == "a" {
					return "x"
				}
				return int64(1)
			},
		},
	})
	for _, k := range []string{"a", "b"} {
		if err := tbl.Create(context.Background(), &protodb.Row[string]{Key: strKey(k), Resource: k}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := orderedNames(t, tbl, "mixed"); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("got %v, want FailedPrecondition", err)
	}
}

// TestMemOrderByTimestampMessageColumn checks that a Columns function may
// return the resource's *timestamppb.Timestamp directly, as filters accept.
func TestMemOrderByTimestampMessageColumn(t *testing.T) {
	created := map[string]*timestamppb.Timestamp{
		"a": timestamppb.New(time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)),
		"b": nil,
		"c": timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	tbl := memadapter.New[string](memadapter.Config{
		KeyColumns: []string{"key"},
		Columns: map[string]func(protodb.Key, any) any{
			"ct": func(k protodb.Key, _ any) any { return created[k.KeyValues()[0].(string)] },
		},
	})
	for _, k := range []string{"a", "b", "c"} {
		if err := tbl.Create(context.Background(), &protodb.Row[string]{Key: strKey(k), Resource: k}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := orderedNames(t, tbl, "ct")
	if err != nil || !reflect.DeepEqual(got, []string{"b", "c", "a"}) {
		t.Fatalf("got %v, %v; want [b c a] (nil first)", got, err)
	}
}

// TestMemOrderByOtherMessageColumnIsFailedPrecondition checks that a column
// returning a message other than Timestamp or Duration is rejected, not read.
func TestMemOrderByOtherMessageColumnIsFailedPrecondition(t *testing.T) {
	tbl := memadapter.New[string](memadapter.Config{
		KeyColumns: []string{"key"},
		Columns:    map[string]func(protodb.Key, any) any{"m": func(protodb.Key, any) any { return &databasepb.Backup{Name: "x"} }},
	})
	if err := tbl.Create(context.Background(), &protodb.Row[string]{Key: strKey("a"), Resource: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := orderedNames(t, tbl, "m"); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("got %v, want FailedPrecondition", err)
	}
}

// runNested runs outer (which calls RunTransaction again inside) and fails
// the test if it does not return within 5 seconds, which is how a deadlock
// on the package-global lock shows up.
func runNested(t *testing.T, outer func(ctx context.Context) error) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- outer(ctx) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		t.Fatal("nested RunTransaction deadlocked")
		return nil
	}
}

func TestMemNestedTransactionJoins(t *testing.T) {
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	runner := memadapter.NewTransactionRunner()
	err := runNested(t, func(ctx context.Context) error {
		return runner.RunTransaction(ctx, func(ctx context.Context) error {
			if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey("outer"), Resource: "1"}); err != nil {
				return err
			}
			return runner.RunTransaction(ctx, func(ctx context.Context) error {
				return tbl.Create(ctx, &protodb.Row[string]{Key: strKey("inner"), Resource: "2"})
			})
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"outer", "inner"} {
		if _, err := tbl.Read(context.Background(), strKey(k)); err != nil {
			t.Errorf("%s not committed: %v", k, err)
		}
	}
}

func TestMemNestedTransactionErrorRollsBackBoth(t *testing.T) {
	tbl := memadapter.New[string](memadapter.Config{KeyColumns: []string{"key"}})
	runner := memadapter.NewTransactionRunner()
	wantErr := errors.New("inner failed")
	err := runNested(t, func(ctx context.Context) error {
		return runner.RunTransaction(ctx, func(ctx context.Context) error {
			if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey("outer"), Resource: "1"}); err != nil {
				return err
			}
			return runner.RunTransaction(ctx, func(ctx context.Context) error {
				if err := tbl.Create(ctx, &protodb.Row[string]{Key: strKey("inner"), Resource: "2"}); err != nil {
					return err
				}
				return wantErr
			})
		})
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want %v", err, wantErr)
	}
	for _, k := range []string{"outer", "inner"} {
		if _, err := tbl.Read(context.Background(), strKey(k)); !protodb.IsNotFound(err) {
			t.Errorf("%s survived the rolled-back transaction: %v", k, err)
		}
	}
}
