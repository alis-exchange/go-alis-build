// Package spannertest runs tests against the Cloud Spanner emulator: it
// finds or starts an emulator, creates a fresh database per test with a
// proto bundle and DDL, and drops it when the test ends.
//
//	func TestShelves(t *testing.T) {
//		bundle, err := protobundle.New(&iampb.Policy{})
//		if err != nil {
//			t.Fatal(err)
//		}
//		client := spannertest.NewDatabase(t, bundle,
//			"CREATE TABLE Shelves (`key` STRING(MAX) NOT NULL, Policy `google.iam.v1.Policy`) PRIMARY KEY (`key`)")
//		runShelfTests(t, client) // your tests against the table
//	}
//
// # Choosing the emulator
//
// The tests are opt-in, so a plain go test needs no Docker:
//
//   - SPANNER_EMULATOR_HOST set: use the emulator already running there.
//   - else SPANNERTEST_EMULATOR set to any value: start DefaultImage (or
//     SPANNERTEST_EMULATOR_IMAGE) with testcontainers-go. The first test to
//     ask starts it; every later test in the same test binary shares it. A
//     container that cannot start fails the test, since the run opted in.
//   - else: the test is skipped.
//
// The testcontainers reaper (Ryuk) removes the container when the test
// binary exits. With TESTCONTAINERS_RYUK_DISABLED=true it outlives the run
// and has to be removed by hand.
//
// # Clients
//
// spannertest never sets SPANNER_EMULATOR_HOST. The clients it builds
// connect through explicit endpoint options, so tests may call t.Parallel.
// Code under test that builds its own client from the environment needs
// the variable set by the test, which then must not be parallel:
//
//	client := spannertest.NewDatabase(t, bundle, ddl...)
//	t.Setenv("SPANNER_EMULATOR_HOST", spannertest.Host(t))
//	svc := newService(ctx, client.DatabaseName())
//
// # The emulator's 32-bit gap
//
// The emulator cannot read 32-bit integer proto fields (int32, uint32,
// sint32, fixed32, sfixed32), such as google.protobuf.Timestamp.nanos.
// A STORED generated column that reads one makes every write fail with an
// opaque "Unexpected error in RPC handling", and a CHECK constraint may do
// the same, so NewDatabase rejects both before creating anything and names
// the table, the column or constraint, and the path; read .seconds instead.
//
// The check parses each statement with spansql and covers generated
// columns and CHECK constraints in CREATE TABLE and in ALTER TABLE ADD
// COLUMN / ADD CONSTRAINT, for paths that start at one of the table's
// proto columns (quoted or not, matched case-insensitively). A statement
// spansql cannot parse, such as one with an ARRAY<proto> column, a
// schema-qualified table name or a backticked path head, is not checked:
// NewDatabase logs it and carries on.
//
// Queries that read such a field fail with "Type not found: INT32" or
// "UINT32"; wrap errors with Explain to get a hint. Explain also hints on
// "Unexpected error in RPC handling", which the emulator raises for other
// internal errors too, so its hint says the error may come from such a
// read.
package spannertest
