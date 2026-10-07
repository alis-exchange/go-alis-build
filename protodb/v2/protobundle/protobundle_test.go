package protobundle_test

import (
	"slices"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"go.alis.build/protodb/v2/protobundle"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
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

// TestDescriptors pins that the bytes are a FileDescriptorSet protodesc can
// load, so every import is present, that each file follows its imports,
// and that the roots' own files are included.
func TestDescriptors(t *testing.T) {
	b, err := protobundle.New(&iampb.Policy{}, &databasepb.Backup{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := b.Descriptors()
	if err != nil {
		t.Fatalf("Descriptors: %v", err)
	}
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(raw, set); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, err := protodesc.NewFiles(set); err != nil {
		t.Fatalf("protodesc.NewFiles: %v", err)
	}
	pos := map[string]int{}
	for i, f := range set.GetFile() {
		pos[f.GetName()] = i
	}
	for i, f := range set.GetFile() {
		for _, dep := range f.GetDependency() {
			if p, ok := pos[dep]; !ok || p >= i {
				t.Errorf("%s (index %d) imports %s at index %d (present=%v); want it earlier", f.GetName(), i, dep, p, ok)
			}
		}
	}
	for _, name := range []string{
		"google/iam/v1/policy.proto",
		"google/spanner/admin/database/v1/backup.proto",
		"google/protobuf/timestamp.proto",
	} {
		if _, ok := pos[name]; !ok {
			t.Errorf("descriptor set lacks %s", name)
		}
	}
}

// TestCreateStatement pins the exact DDL: backticked names in Types order.
func TestCreateStatement(t *testing.T) {
	b, err := protobundle.New(&structpb.Struct{})
	if err != nil {
		t.Fatal(err)
	}
	want := "CREATE PROTO BUNDLE (`google.protobuf.ListValue`, `google.protobuf.NullValue`, " +
		"`google.protobuf.Struct`, `google.protobuf.Value`)"
	if got := b.CreateStatement(); got != want {
		t.Errorf("CreateStatement() =\n%s\nwant\n%s", got, want)
	}
}

// siblingEnumRoot builds message p.A { p.M.State s = 1; p.M m = 2; }, where
// the field s reaches the nested enum M.State before the walk visits M.
func siblingEnumRoot(t *testing.T) proto.Message {
	t.Helper()
	f := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("p/sibling.proto"),
		Package: proto.String("p"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("A"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name: proto.String("s"), Number: proto.Int32(1), JsonName: proto.String("s"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
						TypeName: proto.String(".p.M.State"),
					},
					{
						Name: proto.String("m"), Number: proto.Int32(2), JsonName: proto.String("m"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".p.M"),
					},
				},
			},
			{
				Name: proto.String("M"),
				EnumType: []*descriptorpb.EnumDescriptorProto{{
					Name:  proto.String("State"),
					Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("STATE_UNSPECIFIED"), Number: proto.Int32(0)}},
				}},
			},
		},
	}
	fd, err := protodesc.NewFile(f, nil)
	if err != nil {
		t.Fatalf("building the sibling-enum file: %v", err)
	}
	return dynamicpb.NewMessage(fd.Messages().ByName("A"))
}

// TestTypesListsSiblingNestedEnumOnce pins that a nested enum reached by a
// field before its parent message is listed once, not again when the
// parent's nested enums are added.
func TestTypesListsSiblingNestedEnumOnce(t *testing.T) {
	b, err := protobundle.New(siblingEnumRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"p.A", "p.M", "p.M.State"}
	if got := b.Types(); !slices.Equal(got, want) {
		t.Errorf("Types() = %q, want %q", got, want)
	}
}
