package spannertest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"go.alis.build/protodb/v2/protobundle"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/proto"
)

// keyOnlyDDL is a one-table schema without proto columns.
const keyOnlyDDL = "CREATE TABLE T (`key` STRING(MAX) NOT NULL) PRIMARY KEY (`key`)"

// TestNewDatabaseSkipsWithoutEmulator pins AC5 for NewDatabase: with
// neither variable set it skips, naming both, before doing anything else.
// Runs without Docker.
func TestNewDatabaseSkipsWithoutEmulator(t *testing.T) {
	t.Setenv("SPANNER_EMULATOR_HOST", "")
	t.Setenv("SPANNERTEST_EMULATOR", "")
	msg := recordSkip(t, func(tb testing.TB) { NewDatabase(tb, nil) })
	if !strings.Contains(msg, "SPANNER_EMULATOR_HOST") || !strings.Contains(msg, "SPANNERTEST_EMULATOR") {
		t.Errorf("NewDatabase skip message %q must name both variables", msg)
	}
}

// TestNewDatabaseIDsAreValid pins that generated database IDs satisfy
// Spanner's rules (a lowercase letter, then lowercase letters and digits,
// 2-30 long) and do not repeat.
func TestNewDatabaseIDsAreValid(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		id := newDatabaseID()
		if len(id) < 2 || len(id) > 30 || id[0] < 'a' || id[0] > 'z' {
			t.Fatalf("newDatabaseID() = %q, want 2-30 chars starting with a lowercase letter", id)
		}
		for _, c := range id {
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
				t.Fatalf("newDatabaseID() = %q has %q, want only lowercase letters and digits", id, c)
			}
		}
		if seen[id] {
			t.Fatalf("newDatabaseID() repeated %q", id)
		}
		seen[id] = true
	}
}

// TestNewDatabaseRoundTripsProtoColumn pins that the bundle reaches the
// database: a proto column value written is read back equal. Needs an
// emulator.
func TestNewDatabaseRoundTripsProtoColumn(t *testing.T) {
	bundle, err := protobundle.New(&iampb.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	client := NewDatabase(t, bundle,
		"CREATE TABLE Policies (`key` STRING(MAX) NOT NULL, Policy `google.iam.v1.Policy`) PRIMARY KEY (`key`)")
	ctx := context.Background()
	want := &iampb.Policy{
		Version:  3,
		Etag:     []byte("e1"),
		Bindings: []*iampb.Binding{{Role: "roles/viewer", Members: []string{"user:a@example.com"}}},
	}
	m := spanner.Insert("Policies", []string{"key", "Policy"}, []any{"k1", want})
	if _, err := client.Apply(ctx, []*spanner.Mutation{m}); err != nil {
		t.Fatalf("Apply: %v", Explain(err))
	}
	row, err := client.Single().ReadRow(ctx, "Policies", spanner.Key{"k1"}, []string{"Policy"})
	if err != nil {
		t.Fatalf("ReadRow: %v", err)
	}
	got := &iampb.Policy{}
	if err := row.Columns(got); err != nil {
		t.Fatalf("Columns: %v", err)
	}
	if !proto.Equal(got, want) {
		t.Errorf("read %v, want %v", got, want)
	}
}

// TestNewDatabaseParallelShareOneEmulator pins that parallel tests get
// separate databases on one emulator, starting at most one container.
// Needs an emulator.
func TestNewDatabaseParallelShareOneEmulator(t *testing.T) {
	Host(t) // skip the group, not each subtest, when no emulator is configured
	names := make(chan string, 2)
	t.Run("group", func(t *testing.T) {
		for _, n := range []string{"a", "b"} {
			t.Run(n, func(t *testing.T) {
				t.Parallel()
				names <- NewDatabase(t, nil, keyOnlyDDL).DatabaseName()
			})
		}
	})
	close(names)
	var got []string
	for n := range names {
		got = append(got, n)
	}
	if len(got) != 2 || got[0] == got[1] {
		t.Errorf("databases = %q, want two distinct names", got)
	}
	// Reusing SPANNER_EMULATOR_HOST starts nothing; otherwise exactly one
	// container serves the whole binary.
	want := int32(1)
	if os.Getenv("SPANNER_EMULATOR_HOST") != "" {
		want = 0
	}
	if c := startCount.Load(); c != want {
		t.Errorf("started %d containers, want %d", c, want)
	}
}

// TestNewDatabaseReachableThroughEnv pins the documented pattern for code
// that builds its own client from SPANNER_EMULATOR_HOST: after t.Setenv
// with Host(t), a plain spanner.NewClient on DatabaseName() reads the same
// database. Needs an emulator.
func TestNewDatabaseReachableThroughEnv(t *testing.T) {
	client := NewDatabase(t, nil, keyOnlyDDL)
	t.Setenv("SPANNER_EMULATOR_HOST", Host(t))
	ctx := context.Background()
	m := spanner.Insert("T", []string{"key"}, []any{"k1"})
	if _, err := client.Apply(ctx, []*spanner.Mutation{m}); err != nil {
		t.Fatal(err)
	}
	own, err := spanner.NewClient(ctx, client.DatabaseName())
	if err != nil {
		t.Fatalf("env-configured client: %v", err)
	}
	defer own.Close()
	if _, err := own.Single().ReadRow(ctx, "T", spanner.Key{"k1"}, []string{"key"}); err != nil {
		t.Errorf("env-configured client cannot read the row: %v", err)
	}
}

// fatalRecorder captures the first Fatal and stops the goroutine, so a
// test can assert NewDatabase failed without failing itself.
type fatalRecorder struct {
	testing.TB
	msg string
}

// Fatal records args and exits the calling goroutine.
func (r *fatalRecorder) Fatal(args ...any) { r.msg = fmt.Sprint(args...); runtime.Goexit() }

// Fatalf records the message and exits the calling goroutine.
func (r *fatalRecorder) Fatalf(format string, args ...any) {
	r.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// TestNewDatabaseGuardFailsBeforeCreating pins that a nanos generated
// column fails NewDatabase with the path named and sends no CreateDatabase
// request. It counts requests from this process, not databases on the
// emulator, so other packages sharing the emulator cannot disturb it.
// Needs an emulator.
func TestNewDatabaseGuardFailsBeforeCreating(t *testing.T) {
	bundle, err := protobundle.New(&databasepb.Backup{})
	if err != nil {
		t.Fatal(err)
	}
	Host(t) // skip here, not inside the recorder, when no emulator is configured
	before := createCalls.Load()
	rec := &fatalRecorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		NewDatabase(rec, bundle, "CREATE TABLE B (`key` STRING(MAX) NOT NULL, "+
			"Backup `google.spanner.admin.database.v1.Backup`, t INT64 AS (Backup.create_time.nanos) STORED) PRIMARY KEY (`key`)")
	}()
	<-done
	if !strings.Contains(rec.msg, "Backup.create_time.nanos") {
		t.Errorf("NewDatabase failure %q does not name the path", rec.msg)
	}
	if after := createCalls.Load(); after != before {
		t.Errorf("CreateDatabase calls went from %d to %d, want none", before, after)
	}
}

// TestNewDatabaseAcceptsSchemedHost pins that an emulator address with an
// http:// prefix, which the official client accepts, works here too: Host
// returns it bare and NewDatabase connects. Needs an emulator.
func TestNewDatabaseAcceptsSchemedHost(t *testing.T) {
	host := Host(t)
	t.Setenv("SPANNER_EMULATOR_HOST", "http://"+host)
	if got := Host(t); got != host {
		t.Errorf("Host() = %q, want %q", got, host)
	}
	NewDatabase(t, nil, keyOnlyDDL)
}

// TestInstanceCreatedOncePerHost pins that however many databases a binary
// creates, the shared instance is created once per emulator host. Needs an
// emulator.
func TestInstanceCreatedOncePerHost(t *testing.T) {
	NewDatabase(t, nil, keyOnlyDDL)
	NewDatabase(t, nil, keyOnlyDDL)
	if c := instanceCreates.Load(); c != 1 {
		t.Errorf("instance created %d times, want 1", c)
	}
}

// TestClientConfigDisablesNativeMetrics pins that clients skip built-in
// metrics: with explicit options instead of SPANNER_EMULATOR_HOST they
// would otherwise try to export to Cloud Monitoring on every Close.
func TestClientConfigDisablesNativeMetrics(t *testing.T) {
	if !clientConfig().DisableNativeMetrics {
		t.Error("clientConfig().DisableNativeMetrics = false, want true")
	}
}

// TestInstanceForRetriesAfterFailure pins that a failed instance creation
// is not cached: the next call tries again, and once it succeeds later
// calls make no request. Runs without Docker.
func TestInstanceForRetriesAfterFailure(t *testing.T) {
	calls := 0
	orig := createInstance
	t.Cleanup(func() { createInstance = orig })
	createInstance = func(context.Context, []option.ClientOption) error {
		calls++
		if calls == 1 {
			return errors.New("emulator hiccup")
		}
		return nil
	}
	const host = "retry.invalid:1"
	if err := instanceFor(host, nil); err == nil {
		t.Fatal("first call: nil error, want the hiccup")
	}
	for range 2 {
		if err := instanceFor(host, nil); err != nil {
			t.Fatalf("retry: %v, want success", err)
		}
	}
	if calls != 2 {
		t.Errorf("createInstance called %d times, want 2 (fail, then succeed once)", calls)
	}
}
