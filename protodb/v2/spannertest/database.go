package spannertest

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

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

// createCalls counts CreateDatabase requests; the guard test asserts a
// rejected schema sends none.
var createCalls atomic.Int32

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
	if err := checkDDL(ddl, bundle); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	opts := clientOptions(host)
	if err := ensureInstance(ctx, opts); err != nil {
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

	client, err := spanner.NewClient(ctx, name, opts...)
	if err != nil {
		t.Fatalf("spannertest: client: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

// ensureInstance creates the shared instance, tolerating one that already
// exists from an earlier test against the same emulator.
func ensureInstance(ctx context.Context, opts []option.ClientOption) error {
	admin, err := instadmin.NewInstanceAdminClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("spannertest: instance admin client: %w", err)
	}
	defer admin.Close()
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

// clientOptions connects a Spanner client to the emulator at host without
// credentials and without touching the process environment, so tests that
// use them may run in parallel.
func clientOptions(host string) []option.ClientOption {
	return []option.ClientOption{
		option.WithEndpoint(host),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}
}
