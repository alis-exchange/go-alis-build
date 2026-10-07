package spannertest

import (
	"slices"
	"strings"
	"testing"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"go.alis.build/protodb/v2/protobundle"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
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

// backupCol is a key column plus a Backup proto column, the start of most
// test tables.
const backupCol = "k STRING(MAX) NOT NULL, Backup `google.spanner.admin.database.v1.Backup`"

// backupTable wraps extra column or constraint definitions in a CREATE
// TABLE B with backupCol.
func backupTable(defs string) string {
	return "CREATE TABLE B (" + backupCol + ", " + defs + ") PRIMARY KEY (k)"
}

// TestCheckDDL pins which expressions the guard flags (32-bit integer
// leaves reached through a proto column, in generated columns and CHECK
// constraints of CREATE TABLE and ALTER TABLE) and what it lets through,
// including every case the 2026-10-07 code review showed the old regex
// parser missed.
func TestCheckDDL(t *testing.T) {
	bundle, err := protobundle.New(&databasepb.Backup{}, &structpb.Struct{}, &wrapperspb.UInt32Value{})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		ddl  []string
		want string // the problem line, or empty for no error
	}{
		{
			name: "nanos in generated column",
			ddl:  []string{backupTable("t INT64 AS (Backup.create_time.nanos) STORED")},
			want: "table B, column t: Backup.create_time.nanos reads int32 field google.protobuf.Timestamp.nanos",
		},
		{
			name: "nanos inside a function",
			ddl: []string{backupTable("t TIMESTAMP AS (TIMESTAMP_ADD(TIMESTAMP_SECONDS(Backup.create_time.seconds), " +
				"INTERVAL Backup.create_time.nanos NANOSECOND)) STORED")},
			want: "table B, column t: Backup.create_time.nanos",
		},
		{
			name: "named CHECK",
			ddl:  []string{backupTable("CONSTRAINT c CHECK (Backup.expire_time.nanos >= 0)")},
			want: "table B, constraint c: Backup.expire_time.nanos",
		},
		{
			name: "unnamed CHECK",
			ddl:  []string{backupTable("CHECK (Backup.expire_time.nanos >= 0)")},
			want: "table B, constraint CHECK: Backup.expire_time.nanos",
		},
		{
			name: "column whose name starts with check",
			ddl:  []string{backupTable("checksum INT64 AS (Backup.create_time.nanos) STORED")},
			want: "table B, column checksum: Backup.create_time.nanos",
		},
		{
			name: "uint32 leaf",
			ddl: []string{"CREATE TABLE W (k STRING(MAX) NOT NULL, W `google.protobuf.UInt32Value`, " +
				"t INT64 AS (W.value) STORED) PRIMARY KEY (k)"},
			want: "table W, column t: W.value reads uint32 field google.protobuf.UInt32Value.value",
		},
		{
			name: "if not exists",
			ddl: []string{"CREATE TABLE IF NOT EXISTS B (" + backupCol +
				", t INT64 AS (Backup.create_time.nanos) STORED) PRIMARY KEY (k)"},
			want: "table B, column t: Backup.create_time.nanos",
		},
		{
			name: "column named in another case",
			ddl:  []string{backupTable("t INT64 AS (backup.create_time.nanos) STORED")},
			want: "table B, column t: backup.create_time.nanos",
		},
		{
			name: "field named in another case",
			ddl:  []string{backupTable("t INT64 AS (Backup.Create_Time.NANOS) STORED")},
			want: "table B, column t: Backup.Create_Time.NANOS",
		},
		{
			name: "unquoted proto type",
			ddl: []string{"CREATE TABLE B (k STRING(MAX) NOT NULL, Backup google.spanner.admin.database.v1.Backup, " +
				"t INT64 AS (Backup.create_time.nanos) STORED) PRIMARY KEY (k)"},
			want: "table B, column t: Backup.create_time.nanos",
		},
		{
			name: "leading comment with an apostrophe",
			ddl:  []string{"-- the emulator can't read nanos\n" + backupTable("t INT64 AS (Backup.create_time.nanos) STORED")},
			want: "table B, column t: Backup.create_time.nanos",
		},
		{
			name: "inline comment with an apostrophe",
			ddl:  []string{backupTable("-- it's here\n t INT64 AS (Backup.create_time.nanos) STORED")},
			want: "table B, column t: Backup.create_time.nanos",
		},
		{
			name: "escaped quote in a literal",
			ddl: []string{backupTable(
				`t STRING(MAX) AS (CONCAT('it\'s', CAST(Backup.create_time.nanos AS STRING))) STORED`,
			)},
			want: "table B, column t: Backup.create_time.nanos",
		},
		{
			name: "backticked field segment",
			ddl:  []string{backupTable("t INT64 AS (Backup.`create_time`.nanos) STORED")},
			want: "table B, column t: Backup.create_time.nanos",
		},
		{
			name: "ALTER TABLE ADD COLUMN",
			ddl: []string{
				"CREATE TABLE B (" + backupCol + ") PRIMARY KEY (k)",
				"ALTER TABLE B ADD COLUMN t2 INT64 AS (Backup.create_time.nanos) STORED",
			},
			want: "table B, column t2: Backup.create_time.nanos",
		},
		{
			name: "ALTER TABLE ADD CONSTRAINT",
			ddl: []string{
				"CREATE TABLE B (" + backupCol + ") PRIMARY KEY (k)",
				"ALTER TABLE b ADD CONSTRAINT c2 CHECK (Backup.create_time.nanos > 0)",
			},
			want: "table b, constraint c2: Backup.create_time.nanos",
		},
		{
			name: "seconds is fine",
			ddl:  []string{backupTable("t TIMESTAMP AS (TIMESTAMP_SECONDS(Backup.create_time.seconds)) STORED")},
		},
		{
			name: "enum is fine",
			ddl:  []string{backupTable("s INT64 AS (CAST(Backup.state AS INT64)) STORED")},
		},
		{
			name: "unresolvable path ignored",
			ddl:  []string{backupTable("t INT64 AS (Backup.no_such_field.nanos) STORED")},
		},
		{
			name: "dotted text in a string literal ignored",
			ddl:  []string{backupTable("t STRING(MAX) AS (CONCAT('Backup.create_time.nanos', k)) STORED")},
		},
		{
			name: "proto column of another table ignored",
			ddl: []string{
				"CREATE TABLE B (" + backupCol + ") PRIMARY KEY (k)",
				"CREATE TABLE X (k STRING(MAX) NOT NULL, Backup STRING(MAX), t INT64 AS (Backup.create_time.nanos) STORED) PRIMARY KEY (k)",
			},
		},
		{
			name: "non-table DDL ignored",
			ddl:  []string{"CREATE INDEX ByKey ON B(k)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unchecked, err := checkDDL(tt.ddl, bundle)
			if len(unchecked) != 0 {
				t.Errorf("unchecked = %q, want none", unchecked)
			}
			if tt.want == "" {
				if err != nil {
					t.Fatalf("checkDDL: %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("checkDDL: nil, want an error with %q", tt.want)
			}
			for _, want := range []string{tt.want, ".seconds"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q lacks %q", err, want)
				}
			}
		})
	}
}

// TestCheckDDLReportsUnparsedStatements pins that statements spansql
// cannot parse (v1.88.0) are returned as not checked, never treated as
// safe, while the statements around them are still checked.
func TestCheckDDLReportsUnparsedStatements(t *testing.T) {
	bundle, err := protobundle.New(&databasepb.Backup{})
	if err != nil {
		t.Fatal(err)
	}
	unparsed := []string{
		"CREATE TABLE sch.B (" + backupCol + ", t INT64 AS (Backup.create_time.nanos) STORED) PRIMARY KEY (k)",
		backupTable("t INT64 AS (`Backup`.create_time.nanos) STORED"),
		backupTable("t INT64 AS ((Backup.create_time).nanos) STORED"),
		"CREATE TABLE A (k STRING(MAX) NOT NULL, Bs ARRAY<google.spanner.admin.database.v1.Backup>) PRIMARY KEY (k)",
	}
	flagged := backupTable("t INT64 AS (Backup.create_time.nanos) STORED")
	unchecked, err := checkDDL(append(slices.Clone(unparsed), flagged), bundle)
	if len(unchecked) != len(unparsed) {
		t.Fatalf("unchecked = %q, want the %d unparseable statements", unchecked, len(unparsed))
	}
	for i, u := range unchecked {
		if !strings.Contains(u, unparsed[i]) {
			t.Errorf("unchecked[%d] = %q, want it to quote %q", i, u, unparsed[i])
		}
	}
	if err == nil || !strings.Contains(err.Error(), "table B, column t: Backup.create_time.nanos") {
		t.Errorf("checkDDL error = %v, want the parseable statement still flagged", err)
	}
}

// TestCheckDDLUsesBundleDescriptors pins that the guard reads the bundle's
// own descriptors: a type built at run time, absent from the global
// registry, is still checked.
func TestCheckDDLUsesBundleDescriptors(t *testing.T) {
	f := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("q/runtime.proto"),
		Package:    proto.String("q"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/protobuf/timestamp.proto"},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("K"),
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name: proto.String("ts"), Number: proto.Int32(1), JsonName: proto.String("ts"),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: proto.String(".google.protobuf.Timestamp"),
			}},
		}},
	}
	fd, err := protodesc.NewFile(f, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := protoregistry.GlobalFiles.FindDescriptorByName("q.K"); err == nil {
		t.Fatal("q.K is in the global registry; the test needs a type that is not")
	}
	bundle, err := protobundle.New(dynamicpb.NewMessage(fd.Messages().ByName("K")))
	if err != nil {
		t.Fatal(err)
	}
	ddl := "CREATE TABLE R (k STRING(MAX) NOT NULL, K `q.K`, t INT64 AS (K.ts.nanos) STORED) PRIMARY KEY (k)"
	if _, err := checkDDL([]string{ddl}, bundle); err == nil || !strings.Contains(err.Error(), "K.ts.nanos") {
		t.Errorf("checkDDL = %v, want K.ts.nanos flagged", err)
	}
}

// TestCheckDDLNilBundle pins that a schema without a bundle is never
// rejected or reported: with no proto columns there is nothing to check.
func TestCheckDDLNilBundle(t *testing.T) {
	ddl := backupTable("t INT64 AS (Backup.create_time.nanos) STORED")
	if unchecked, err := checkDDL([]string{ddl}, nil); err != nil || len(unchecked) != 0 {
		t.Errorf("checkDDL with nil bundle = (%q, %v), want nothing", unchecked, err)
	}
}
