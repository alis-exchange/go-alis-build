package spannertest

import (
	"slices"
	"strings"
	"testing"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"go.alis.build/protodb/v2/protobundle"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// TestInt32Kinds pins the kinds the guard flags: exactly the five 32-bit
// integer kinds the 2026-10-07 probe showed crash emulator writes.
func TestInt32Kinds(t *testing.T) {
	var got []string
	for k := range int32Kinds {
		got = append(got, k.String())
	}
	slices.Sort(got)
	want := []string{"fixed32", "int32", "sfixed32", "sint32", "uint32"}
	if !slices.Equal(got, want) {
		t.Errorf("int32Kinds = %q, want %q", got, want)
	}
	if int32Kinds[protoreflect.Int64Kind] || int32Kinds[protoreflect.EnumKind] {
		t.Error("int32Kinds flags int64 or enum, which the emulator reads fine")
	}
}

// TestCheckDDL pins which expressions the guard flags: 32-bit integer
// leaves under a proto column, in generated columns and CHECK constraints,
// with backticked column names and IF NOT EXISTS; and what it lets through.
func TestCheckDDL(t *testing.T) {
	bundle, err := protobundle.New(&databasepb.Backup{}, &structpb.Struct{}, &wrapperspb.UInt32Value{})
	if err != nil {
		t.Fatal(err)
	}
	table := func(cols string) string {
		return "CREATE TABLE Backups (`key` STRING(MAX) NOT NULL, " +
			"Backup `google.spanner.admin.database.v1.Backup`, " + cols + ") PRIMARY KEY (`key`)"
	}
	tests := []struct {
		name     string
		ddl      string
		wantPath string // empty: no error
	}{
		{
			name:     "nanos in generated column",
			ddl:      table("t INT64 AS (Backup.create_time.nanos) STORED"),
			wantPath: "Backup.create_time.nanos",
		},
		{
			name: "nanos inside a function",
			ddl: table("t TIMESTAMP AS (TIMESTAMP_ADD(TIMESTAMP_SECONDS(Backup.create_time.seconds), " +
				"INTERVAL Backup.create_time.nanos NANOSECOND)) STORED"),
			wantPath: "Backup.create_time.nanos",
		},
		{
			name:     "nanos in CHECK",
			ddl:      table("CONSTRAINT c CHECK (Backup.expire_time.nanos >= 0)"),
			wantPath: "Backup.expire_time.nanos",
		},
		{
			name: "backticked column",
			ddl: "CREATE TABLE Sets (`key` STRING(MAX) NOT NULL, `Set` `google.spanner.admin.database.v1.Backup`, " +
				"t INT64 AS (`Set`.create_time.nanos) STORED) PRIMARY KEY (`key`)",
			wantPath: "`Set`.create_time.nanos",
		},
		{
			name: "uint32 leaf",
			ddl: "CREATE TABLE W (`key` STRING(MAX) NOT NULL, W `google.protobuf.UInt32Value`, " +
				"t INT64 AS (W.value) STORED) PRIMARY KEY (`key`)",
			wantPath: "W.value",
		},
		{
			name: "if not exists",
			ddl: "CREATE TABLE IF NOT EXISTS Backups (`key` STRING(MAX) NOT NULL, " +
				"Backup `google.spanner.admin.database.v1.Backup`, t INT64 AS (Backup.create_time.nanos) STORED) PRIMARY KEY (`key`)",
			wantPath: "Backup.create_time.nanos",
		},
		{
			name: "seconds is fine",
			ddl:  table("t TIMESTAMP AS (TIMESTAMP_SECONDS(Backup.create_time.seconds)) STORED"),
		},
		{
			name: "enum is fine",
			ddl:  table("s INT64 AS (CAST(Backup.state AS INT64)) STORED"),
		},
		{
			name: "unresolvable path ignored",
			ddl:  table("t INT64 AS (Backup.no_such_field.nanos) STORED"),
		},
		{
			name: "dotted text in a string literal ignored",
			ddl:  table("t STRING(MAX) AS (CONCAT('Backup.create_time.nanos', `key`)) STORED"),
		},
		{
			name: "non-proto column ignored",
			ddl:  "CREATE TABLE X (`key` STRING(MAX) NOT NULL, t INT64 AS (LENGTH(`key`)) STORED) PRIMARY KEY (`key`)",
		},
		{
			name: "non-table DDL ignored",
			ddl:  "CREATE INDEX ByKey ON Backups(`key`)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDDL([]string{tt.ddl}, bundle)
			if tt.wantPath == "" {
				if err != nil {
					t.Fatalf("checkDDL: %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("checkDDL: nil, want an error naming %s", tt.wantPath)
			}
			for _, want := range []string{tt.wantPath, ".seconds"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q lacks %q", err, want)
				}
			}
		})
	}
}

// TestCheckDDLNilBundle pins that a schema without a bundle is never
// rejected: with no proto columns there is nothing to check.
func TestCheckDDLNilBundle(t *testing.T) {
	ddl := "CREATE TABLE B (`key` STRING(MAX) NOT NULL, t INT64 AS (Backup.create_time.nanos) STORED) PRIMARY KEY (`key`)"
	if err := checkDDL([]string{ddl}, nil); err != nil {
		t.Errorf("checkDDL with nil bundle: %v, want nil", err)
	}
}
