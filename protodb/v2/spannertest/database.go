package spannertest

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	dbadmin "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	instadmin "cloud.google.com/go/spanner/admin/instance/apiv1"
	"cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"
	"go.alis.build/protodb/v2/protobundle"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	// projectID is the emulator project every test database lives in.
	projectID = "spannertest"
	// instanceID is the emulator instance every test database lives in.
	instanceID = "spannertest"
	// instancePath is the full name of that instance.
	instancePath = "projects/" + projectID + "/instances/" + instanceID
)

var (
	// createCalls counts CreateDatabase requests; the guard test asserts a
	// rejected schema sends none.
	createCalls atomic.Int32
	// instanceCreates counts CreateInstance requests; tests assert one per
	// emulator host.
	instanceCreates atomic.Int32
	// instancesMu guards instancesReady and serialises instance creation.
	instancesMu sync.Mutex
	// instancesReady records the emulator hosts whose shared instance
	// exists. Only success is recorded, so a failed attempt is retried.
	instancesReady = map[string]bool{}
	// createInstance creates the shared instance; tests replace it.
	createInstance = ensureInstance
)

// instanceTimeout bounds one attempt to create the shared instance.
const instanceTimeout = 30 * time.Second

// NewDatabase creates a fresh database on the emulator from Host and
// returns a client for it. The database holds bundle's CREATE PROTO BUNDLE
// statement and descriptors (when bundle is not nil) followed by ddl. When
// t ends the client is closed and the database dropped.
//
// NewDatabase skips t when no emulator is configured (see Host), and fails
// t before creating anything when a generated column or CHECK constraint
// in one of ddl's CREATE TABLE statements reads a 32-bit integer proto
// field the emulator cannot handle. Tests calling it may run in parallel: each
// gets its own database, and SPANNER_EMULATOR_HOST is never set.
func NewDatabase(t testing.TB, bundle *protobundle.Bundle, ddl ...string) *spanner.Client {
	t.Helper()
	host := Host(t)
	unchecked, err := checkDDL(ddl, bundle)
	for _, u := range unchecked {
		t.Logf("spannertest: could not parse %s, so it was not checked for 32-bit proto field reads", u)
	}
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	opts := clientOptions(host)
	if err := instanceFor(host, opts); err != nil {
		t.Fatal(err)
	}

	stmts := ddl
	var descriptors []byte
	if bundle != nil {
		var err error
		if descriptors, err = bundle.Descriptors(); err != nil {
			t.Fatalf("spannertest: proto descriptors: %v", err)
		}
		stmts = append([]string{bundle.CreateStatement()}, ddl...)
	}

	admin, err := dbadmin.NewDatabaseAdminClient(ctx, opts...)
	if err != nil {
		t.Fatalf("spannertest: database admin client: %v", err)
	}
	t.Cleanup(func() {
		if err := admin.Close(); err != nil {
			t.Logf("spannertest: closing the database admin client: %v", err)
		}
	})

	id := newDatabaseID()
	createCalls.Add(1)
	op, err := admin.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:           instancePath,
		CreateStatement:  "CREATE DATABASE `" + id + "`",
		ExtraStatements:  stmts,
		ProtoDescriptors: descriptors,
	})
	if err == nil {
		_, err = op.Wait(ctx)
	}
	if err != nil {
		t.Fatalf("spannertest: creating database: %v", Explain(err))
	}
	name := instancePath + "/databases/" + id
	// Registered after creation succeeded, so it never drops a database
	// that does not exist. Cleanups run last-in first-out: the client
	// closes, then the database is dropped, then the admin client closes.
	t.Cleanup(func() {
		err := admin.DropDatabase(context.Background(), &databasepb.DropDatabaseRequest{Database: name})
		if err != nil {
			t.Errorf("spannertest: dropping %s: %v", name, err)
		}
	})

	client, err := spanner.NewClientWithConfig(ctx, name, clientConfig(), opts...)
	if err != nil {
		t.Fatalf("spannertest: client: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

// instanceFor makes sure host's shared instance exists, creating it on
// the first call that succeeds. Callers wait for an attempt in progress;
// after a failure the next call tries again.
func instanceFor(host string, opts []option.ClientOption) error {
	instancesMu.Lock()
	defer instancesMu.Unlock()
	if instancesReady[host] {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), instanceTimeout)
	defer cancel()
	if err := createInstance(ctx, opts); err != nil {
		return err
	}
	instancesReady[host] = true
	return nil
}

// ensureInstance creates the shared instance, tolerating one that already
// exists from an earlier test binary against the same emulator.
// NewDatabase reaches it through instanceFor.
func ensureInstance(ctx context.Context, opts []option.ClientOption) error {
	admin, err := instadmin.NewInstanceAdminClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("spannertest: instance admin client: %w", err)
	}
	defer admin.Close()
	instanceCreates.Add(1)
	op, err := admin.CreateInstance(ctx, &instancepb.CreateInstanceRequest{
		Parent:     "projects/" + projectID,
		InstanceId: instanceID,
		Instance: &instancepb.Instance{
			Config:      "projects/" + projectID + "/instanceConfigs/emulator-config",
			DisplayName: "spannertest",
			NodeCount:   1,
		},
	})
	if err == nil {
		_, err = op.Wait(ctx)
	}
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return fmt.Errorf("spannertest: creating instance: %w", err)
	}
	return nil
}

// newDatabaseID returns a random ID that satisfies Spanner's database ID
// rules: a lowercase letter followed by lowercase letters and digits, 21
// characters in all. It uses crypto/rand.Text.
func newDatabaseID() string {
	return "t" + strings.ToLower(rand.Text()[:20])
}

// clientConfig turns off built-in metrics. Explicit options take the
// client off its SPANNER_EMULATOR_HOST path, which would leave them on, and
// every Close would then try to export to Cloud Monitoring.
func clientConfig() spanner.ClientConfig {
	return spanner.ClientConfig{DisableNativeMetrics: true}
}

// clientOptions connects a Spanner client to the emulator at host (as
// host:port, see normalizeHost) without
// credentials and without touching the process environment, so tests that
// use them may run in parallel.
func clientOptions(host string) []option.ClientOption {
	return []option.ClientOption{
		// passthrough:/// skips gRPC name resolution, as the official
		// client does for SPANNER_EMULATOR_HOST.
		option.WithEndpoint("passthrough:///" + host),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}
}
