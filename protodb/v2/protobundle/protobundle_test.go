package protobundle_test

import (
	"slices"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"go.alis.build/protodb/v2/protobundle"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
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

// TestNewRejectsMissingRoots pins the error cases: no roots, a lone nil
// root, and a nil root after a valid one.
func TestNewRejectsMissingRoots(t *testing.T) {
	if _, err := protobundle.New(); err == nil {
		t.Error("New() returned nil error, want one")
	}
	if _, err := protobundle.New(nil); err == nil {
		t.Error("New(nil) returned nil error, want one")
	}
	if _, err := protobundle.New(&timestamppb.Timestamp{}, nil); err == nil {
		t.Error("New(ts, nil) returned nil error, want one")
	}
}

// TestNewAcceptsTypedNilRoot pins that a typed nil pointer is a valid
// root: only its descriptor is read.
func TestNewAcceptsTypedNilRoot(t *testing.T) {
	b, err := protobundle.New((*timestamppb.Timestamp)(nil))
	if err != nil {
		t.Fatalf("New(typed nil): %v", err)
	}
	if got, want := b.Types(), []string{"google.protobuf.Timestamp"}; !slices.Equal(got, want) {
		t.Errorf("Types() = %q, want %q", got, want)
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

// nestedOnlyRoot builds message p.A { p.M.State s = 1; p.M.N n = 2; }, where
// the parent p.M is never itself the type of a field.
func nestedOnlyRoot(t *testing.T) proto.Message {
	t.Helper()
	field := func(name string, num int32, typ descriptorpb.FieldDescriptorProto_Type, ref string) *descriptorpb.FieldDescriptorProto {
		return &descriptorpb.FieldDescriptorProto{
			Name: proto.String(name), Number: proto.Int32(num), JsonName: proto.String(name),
			Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:  typ.Enum(), TypeName: proto.String(ref),
		}
	}
	f := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("p/nested.proto"),
		Package: proto.String("p"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("A"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("s", 1, descriptorpb.FieldDescriptorProto_TYPE_ENUM, ".p.M.State"),
					field("n", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".p.M.N"),
				},
			},
			{
				Name:       proto.String("M"),
				NestedType: []*descriptorpb.DescriptorProto{{Name: proto.String("N")}},
				EnumType: []*descriptorpb.EnumDescriptorProto{{
					Name:  proto.String("State"),
					Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("STATE_UNSPECIFIED"), Number: proto.Int32(0)}},
				}},
			},
		},
	}
	fd, err := protodesc.NewFile(f, nil)
	if err != nil {
		t.Fatalf("building the nested-only file: %v", err)
	}
	return dynamicpb.NewMessage(fd.Messages().ByName("A"))
}

// TestTypesListsParentsOfNestedTypes pins Spanner's rule that a bundle
// naming a nested type also names every message containing it: p.M is
// listed although no field has type p.M.
func TestTypesListsParentsOfNestedTypes(t *testing.T) {
	b, err := protobundle.New(nestedOnlyRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"p.A", "p.M", "p.M.N", "p.M.State"}
	if got := b.Types(); !slices.Equal(got, want) {
		t.Errorf("Types() = %q, want %q", got, want)
	}
}

// TestNewFromDescriptorsAcceptsEnumRoot pins that a top-level enum, which
// an ENUM column can use without any message referencing it, can be a
// root on its own.
func TestNewFromDescriptorsAcceptsEnumRoot(t *testing.T) {
	b, err := protobundle.NewFromDescriptors(databasepb.DatabaseDialect(0).Descriptor())
	if err != nil {
		t.Fatalf("NewFromDescriptors: %v", err)
	}
	want := []string{"google.spanner.admin.database.v1.DatabaseDialect"}
	if got := b.Types(); !slices.Equal(got, want) {
		t.Errorf("Types() = %q, want %q", got, want)
	}
}

// TestNewFromDescriptorsRejectsOtherKinds pins the error cases: no roots,
// a nil root, and a descriptor that is neither a message nor an enum.
func TestNewFromDescriptorsRejectsOtherKinds(t *testing.T) {
	field := (&iampb.Policy{}).ProtoReflect().Descriptor().Fields().Get(0)
	for name, roots := range map[string][]protoreflect.Descriptor{
		"none":  nil,
		"nil":   {nil},
		"field": {field},
	} {
		if _, err := protobundle.NewFromDescriptors(roots...); err == nil {
			t.Errorf("%s: NewFromDescriptors returned nil error, want one", name)
		}
	}
}

// TestLookup pins that Lookup returns the bundled descriptor for a listed
// type, nested enums included, and nil for a type outside the bundle.
func TestLookup(t *testing.T) {
	b, err := protobundle.New(&databasepb.Backup{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"google.spanner.admin.database.v1.Backup",
		"google.spanner.admin.database.v1.Backup.State",
	} {
		d := b.Lookup(name)
		if d == nil || string(d.FullName()) != name {
			t.Errorf("Lookup(%q) = %v, want its descriptor", name, d)
		}
	}
	if d := b.Lookup("google.iam.v1.Policy"); d != nil {
		t.Errorf("Lookup(Policy) = %v, want nil for a type outside the bundle", d)
	}
}
