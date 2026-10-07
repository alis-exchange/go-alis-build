package protobundle_test

import (
	"slices"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"go.alis.build/protodb/v2/protobundle"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestTypes pins the field-reachable closure for well-known roots: nested
// enums are listed, map entries (Struct.FieldsEntry) are not, and the list
// is sorted and de-duplicated. The expected lists were captured from the
// linked registry on 2026-10-07.
func TestTypes(t *testing.T) {
	tests := []struct {
		name  string
		roots []proto.Message
		want  []string
	}{
		{
			name:  "timestamp",
			roots: []proto.Message{&timestamppb.Timestamp{}},
			want:  []string{"google.protobuf.Timestamp"},
		},
		{
			name:  "struct skips map entries",
			roots: []proto.Message{&structpb.Struct{}},
			want: []string{
				"google.protobuf.ListValue", "google.protobuf.NullValue",
				"google.protobuf.Struct", "google.protobuf.Value",
			},
		},
		{
			name:  "policy",
			roots: []proto.Message{&iampb.Policy{}},
			want: []string{
				"google.iam.v1.AuditConfig", "google.iam.v1.AuditLogConfig",
				"google.iam.v1.AuditLogConfig.LogType", "google.iam.v1.Binding",
				"google.iam.v1.Policy", "google.type.Expr",
			},
		},
		{
			name:  "backup",
			roots: []proto.Message{&databasepb.Backup{}},
			want: []string{
				"google.protobuf.Any", "google.protobuf.Timestamp", "google.rpc.Status",
				"google.spanner.admin.database.v1.Backup",
				"google.spanner.admin.database.v1.Backup.State",
				"google.spanner.admin.database.v1.BackupInstancePartition",
				"google.spanner.admin.database.v1.DatabaseDialect",
				"google.spanner.admin.database.v1.EncryptionInfo",
				"google.spanner.admin.database.v1.EncryptionInfo.Type",
			},
		},
		{
			name:  "duplicate roots merge",
			roots: []proto.Message{&timestamppb.Timestamp{}, &timestamppb.Timestamp{}},
			want:  []string{"google.protobuf.Timestamp"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := protobundle.New(tt.roots...)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if got := b.Types(); !slices.Equal(got, tt.want) {
				t.Errorf("Types() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

// TestTypesReturnsCopy pins that callers cannot change the bundle by
// editing the slice Types returns.
func TestTypesReturnsCopy(t *testing.T) {
	b, err := protobundle.New(&timestamppb.Timestamp{})
	if err != nil {
		t.Fatal(err)
	}
	b.Types()[0] = "changed"
	if got := b.Types()[0]; got != "google.protobuf.Timestamp" {
		t.Errorf("Types()[0] = %q after editing a returned slice, want google.protobuf.Timestamp", got)
	}
}

// TestNewRejectsMissingRoots pins the two error cases: no roots, and a nil
// root.
func TestNewRejectsMissingRoots(t *testing.T) {
	if _, err := protobundle.New(); err == nil {
		t.Error("New() returned nil error, want one")
	}
	if _, err := protobundle.New(&timestamppb.Timestamp{}, nil); err == nil {
		t.Error("New(ts, nil) returned nil error, want one")
	}
}
