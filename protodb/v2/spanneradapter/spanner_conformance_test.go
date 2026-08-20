package spanneradapter_test

// Env-gated conformance test that runs the protodbtest suite — plus a
// multi-column tied-order pagination check — against a real Spanner, using
// the reference table in spanner_reftable_test.go.
//
// Gating (see the README's "Running the Spanner conformance test"):
//
//   - SPANNER_EMULATOR_HOST set  → use that emulator, start no container.
//   - PROTODB_SPANNER_CONFORMANCE non-empty → start the Cloud Spanner
//     emulator with testcontainers and point the clients at it.
//   - neither → skip.

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	dbadmin "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	instadmin "cloud.google.com/go/spanner/admin/instance/apiv1"
	"cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner"
	"github.com/testcontainers/testcontainers-go"
	tcspanner "github.com/testcontainers/testcontainers-go/modules/gcloud/spanner"
	"go.alis.build/protodb/v2"
	"go.alis.build/protodb/v2/filtering"
	"go.alis.build/protodb/v2/protodbtest"
	"go.alis.build/protodb/v2/spanneradapter"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	// emulatorImage is pinned rather than :latest so a CI run and a local
	// run exercise the same Spanner build.
	emulatorImage = "gcr.io/cloud-spanner-emulator/emulator:1.5.56"

	emulatorHostEnv = "SPANNER_EMULATOR_HOST"
	conformanceEnv  = "PROTODB_SPANNER_CONFORMANCE"

	testProjectID  = "protodb-test"
	testInstanceID = "protodb-test"

	booksTable   = "Books"
	shelvesTable = "Shelves"
)

// resolveEmulator implements the two gates. It either finds an emulator
// already named by SPANNER_EMULATOR_HOST, starts one in a container, or
// skips the test.
func resolveEmulator(t *testing.T) {
	t.Helper()
	if host := os.Getenv(emulatorHostEnv); host != "" {
		t.Logf("using the Spanner emulator already at %s=%s", emulatorHostEnv, host)
		return
	}
	if os.Getenv(conformanceEnv) == "" {
		t.Skip("set SPANNER_EMULATOR_HOST or PROTODB_SPANNER_CONFORMANCE=1 to run the Spanner conformance test")
	}

	ctx := context.Background()
	ctr, err := tcspanner.Run(ctx, emulatorImage)
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Logf("terminating the Spanner emulator container: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("starting the Spanner emulator container (%s): %v", emulatorImage, err)
	}

	// The Spanner Go client and both admin clients read this variable and
	// switch to a plaintext, credential-free connection when it is set.
	t.Setenv(emulatorHostEnv, ctr.URI())
	t.Logf("started the Spanner emulator at %s", ctr.URI())
}

// newSpannerDatabase resolves the emulator, creates the instance (once per
// emulator) and a fresh database carrying the PROTO BUNDLE and the two test
// tables, and returns a client bound to it.
func newSpannerDatabase(t *testing.T) *spanner.Client {
	t.Helper()
	resolveEmulator(t)

	ctx := context.Background()
	createInstance(ctx, t)

	descriptors, err := protoBundleDescriptors()
	if err != nil {
		t.Fatalf("building the proto descriptor set: %v", err)
	}

	databaseID := fmt.Sprintf("d%d", time.Now().UnixNano())
	adminClient, err := dbadmin.NewDatabaseAdminClient(ctx)
	if err != nil {
		t.Fatalf("database admin client: %v", err)
	}
	t.Cleanup(func() {
		if err := adminClient.Close(); err != nil {
			t.Logf("closing the database admin client: %v", err)
		}
	})

	op, err := adminClient.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:          fmt.Sprintf("projects/%s/instances/%s", testProjectID, testInstanceID),
		CreateStatement: fmt.Sprintf("CREATE DATABASE `%s`", databaseID),
		ExtraStatements: []string{
			// The bundle has to name every proto type the schema below
			// mentions; the descriptors travel out-of-band as bytes.
			"CREATE PROTO BUNDLE (`google.iam.v1.Policy`)",
			"CREATE TABLE " + booksTable + " (" +
				"`key` STRING(MAX) NOT NULL," +
				"Res STRING(MAX)," +
				"Policy `google.iam.v1.Policy`," +
				") PRIMARY KEY (`key`)",
			"CREATE TABLE " + shelvesTable + " (" +
				"a STRING(MAX) NOT NULL," +
				"b STRING(MAX) NOT NULL," +
				"Res STRING(MAX)," +
				"Policy `google.iam.v1.Policy`," +
				") PRIMARY KEY (a, b)",
		},
		ProtoDescriptors: descriptors,
	})
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}
	if _, err := op.Wait(ctx); err != nil {
		t.Fatalf("waiting for CreateDatabase (DDL rejected?): %v", err)
	}

	databaseName := fmt.Sprintf("projects/%s/instances/%s/databases/%s", testProjectID, testInstanceID, databaseID)
	// Drop the database the run created. On the container path this is
	// redundant (the emulator dies with the container), but under the
	// SPANNER_EMULATOR_HOST reuse gate the emulator outlives the test, and
	// without this every run would leave an orphaned database behind.
	// Registered after creation succeeded, so it never fires for a database
	// that does not exist; LIFO cleanup ordering puts it after the Spanner
	// client's Close and before the container is terminated.
	t.Cleanup(func() {
		if err := adminClient.DropDatabase(ctx, &databasepb.DropDatabaseRequest{Database: databaseName}); err != nil {
			t.Errorf("dropping test database %s: %v", databaseName, err)
		}
	})

	client, err := spanner.NewClient(ctx, databaseName)
	if err != nil {
		t.Fatalf("spanner client: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func createInstance(ctx context.Context, t *testing.T) {
	t.Helper()
	admin, err := instadmin.NewInstanceAdminClient(ctx)
	if err != nil {
		t.Fatalf("instance admin client: %v", err)
	}
	defer admin.Close()

	op, err := admin.CreateInstance(ctx, &instancepb.CreateInstanceRequest{
		Parent:     "projects/" + testProjectID,
		InstanceId: testInstanceID,
		Instance: &instancepb.Instance{
			Config:      fmt.Sprintf("projects/%s/instanceConfigs/emulator-config", testProjectID),
			DisplayName: "protodb conformance",
			NodeCount:   1,
		},
	})
	if status.Code(err) == codes.AlreadyExists {
		return
	}
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	if _, err := op.Wait(ctx); err != nil && status.Code(err) != codes.AlreadyExists {
		t.Fatalf("waiting for CreateInstance: %v", err)
	}
}

// protoBundleDescriptors serialises google.iam.v1.Policy's file descriptor
// and everything it imports, transitively, into the FileDescriptorSet the
// CREATE PROTO BUNDLE statement needs. It is built from the compiled-in
// registry, so the bytes always match the iampb the library links against —
// no .proto files or protoc at test time.
func protoBundleDescriptors() ([]byte, error) {
	set := &descriptorpb.FileDescriptorSet{}
	seen := map[string]bool{}
	var add func(protoreflect.FileDescriptor)
	add = func(fd protoreflect.FileDescriptor) {
		if seen[fd.Path()] {
			return
		}
		seen[fd.Path()] = true
		// Dependencies first: a FileDescriptorSet must list a file after
		// every file it imports.
		imports := fd.Imports()
		for i := 0; i < imports.Len(); i++ {
			add(imports.Get(i).FileDescriptor)
		}
		set.File = append(set.File, protodesc.ToFileDescriptorProto(fd))
	}
	add((&iampb.Policy{}).ProtoReflect().Descriptor().ParentFile())
	return proto.Marshal(set)
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

func (k shelfKey) KeyValues() []any { return spanneradapter.KeyValuesOf(k) }

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
