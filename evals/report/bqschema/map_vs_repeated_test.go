package bqschema

import (
	"testing"

	"cloud.google.com/go/bigquery"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/report/bqschema/testdata/mapshape"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// TestMapVsRepeated_bqschemaIdentical asserts that bqschema derives the same
// BigQuery column layout from proto map fields and from explicit repeated
// {key, value} entry messages. A proto migration from map → repeated therefore
// does not change the Terraform google_bigquery_table schema for those columns.
func TestMapVsRepeated_bqschemaIdentical(t *testing.T) {
	t.Parallel()

	mapSchema := fieldsSchema((&mapshape.WithMap{}).ProtoReflect().Descriptor().Fields())
	repeatedSchema := fieldsSchema((&mapshape.WithRepeated{}).ProtoReflect().Descriptor().Fields())
	if !schemasEqual(mapSchema, repeatedSchema) {
		mapJSON, _ := marshalSchema(mapSchema)
		repeatedJSON, _ := marshalSchema(repeatedSchema)
		t.Fatalf("map vs repeated schema mismatch\n map:      %s\n repeated: %s", mapJSON, repeatedJSON)
	}
}

// TestMapVsRepeated_runLoadFieldsMatchSynthetic asserts the load-test map
// columns in the real Run schema (errors_by_code, tags) match the synthetic
// map/repeated equivalence above.
func TestMapVsRepeated_runLoadFieldsMatchSynthetic(t *testing.T) {
	t.Parallel()

	runSchema := Schema()
	synthetic := fieldsSchema((&mapshape.WithMap{}).ProtoReflect().Descriptor().Fields())
	byName := indexByName(synthetic)

	cases := []struct {
		path      []string
		synthetic string // field name in WithMap / WithRepeated
	}{
		{path: []string{"load_test", "cases", "summary", "errors_by_code"}, synthetic: "counts"},
		{path: []string{"load_test", "cases", "tags"}, synthetic: "tags"},
	}
	for _, tc := range cases {
		t.Run(joinPath(tc.path), func(t *testing.T) {
			t.Parallel()
			got := lookupField(runSchema, tc.path)
			if got == nil {
				t.Fatalf("field %v not found in Run schema", tc.path)
			}
			want := byName[tc.synthetic]
			if want == nil {
				t.Fatalf("synthetic field %q not found", tc.synthetic)
			}
			if !fieldStructureEqual(got, want) {
				gotJSON, _ := marshalSchema(bigquery.Schema{got})
				wantJSON, _ := marshalSchema(bigquery.Schema{want})
				t.Fatalf("Run field structure differs from map-derived synthetic\n got:  %s\n want: %s", gotJSON, wantJSON)
			}
		})
	}
}

// TestMapVsRepeated_runDescriptorUsesRepeatedEntries confirms evalspb.Run
// load-test tags and errors_by_code are repeated {key, value} entry messages
// (Pub/Sub → BigQuery compatible JSON arrays, not protojson map objects).
func TestMapVsRepeated_runDescriptorUsesRepeatedEntries(t *testing.T) {
	t.Parallel()

	errorsByCode := lookupProtoField((&evalspb.Run{}).ProtoReflect().Descriptor(), []string{"load_test", "cases", "summary", "errors_by_code"})
	if errorsByCode == nil {
		t.Fatal("errors_by_code not found in Run descriptor")
	}
	if errorsByCode.IsMap() {
		t.Fatal("errors_by_code is still a map field; want repeated Int64Entry")
	}
	if !errorsByCode.IsList() {
		t.Fatal("errors_by_code is not repeated")
	}
	assertEntryShape(t, errorsByCode, protoreflect.Int64Kind)

	tags := lookupProtoField((&evalspb.Run{}).ProtoReflect().Descriptor(), []string{"load_test", "cases", "tags"})
	if tags == nil {
		t.Fatal("tags not found in Run descriptor")
	}
	if tags.IsMap() {
		t.Fatal("tags is still a map field; want repeated StringEntry")
	}
	if !tags.IsList() {
		t.Fatal("tags is not repeated")
	}
	assertEntryShape(t, tags, protoreflect.StringKind)
}

func assertEntryShape(t *testing.T, fd protoreflect.FieldDescriptor, valueKind protoreflect.Kind) {
	t.Helper()
	entry := fd.Message()
	if entry == nil {
		t.Fatalf("%s: missing entry message", fd.Name())
	}
	key := entry.Fields().ByName("key")
	val := entry.Fields().ByName("value")
	if key == nil || key.Kind() != protoreflect.StringKind {
		t.Fatalf("%s entry key = %v, want STRING", fd.Name(), key)
	}
	if val == nil || val.Kind() != valueKind {
		t.Fatalf("%s entry value = %v, want %v", fd.Name(), val, valueKind)
	}
}

func fieldStructureEqual(a, b *bigquery.FieldSchema) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Type != b.Type || a.Repeated != b.Repeated {
		return false
	}
	if len(a.Schema) != len(b.Schema) {
		return false
	}
	for i := range a.Schema {
		if !fieldStructureEqual(a.Schema[i], b.Schema[i]) {
			return false
		}
	}
	return true
}

func lookupProtoField(d protoreflect.MessageDescriptor, path []string) protoreflect.FieldDescriptor {
	if len(path) == 0 {
		return nil
	}
	fd := d.Fields().ByName(protoreflect.Name(path[0]))
	if fd == nil {
		return nil
	}
	if len(path) == 1 {
		return fd
	}
	if fd.Kind() != protoreflect.MessageKind {
		return nil
	}
	return lookupProtoField(fd.Message(), path[1:])
}
