package bqschema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/bigquery"
)

// TestSchema_matchesGoldenFixture asserts that Schema() equals the checked-in
// bq-mk-compatible fixture. Any change to alis.evals.v1.Run's field graph that
// isn't reflected in the fixture fails CI. Bring both in sync deliberately.
func TestSchema_matchesGoldenFixture(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("testdata", "run.schema.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	fixture, err := bigquery.SchemaFromJSON(raw)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	got := Schema()
	if !schemasEqual(got, fixture) {
		gotJSON, _ := marshalSchema(got)
		wantJSON, _ := marshalSchema(fixture)
		t.Fatalf("Schema() mismatch with testdata/run.schema.json\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

// TestSchema_typeMapping asserts a small table of proto-kind → BQ-column-type
// mappings on well-chosen field paths, so a regression in one field type is
// caught without decoding the full golden fixture.
func TestSchema_typeMapping(t *testing.T) {
	t.Parallel()
	s := Schema()
	tests := []struct {
		path         []string
		wantType     bigquery.FieldType
		wantRepeated bool
	}{
		{path: []string{"name"}, wantType: bigquery.StringFieldType},
		{path: []string{"batch_id"}, wantType: bigquery.StringFieldType},
		{path: []string{"type"}, wantType: bigquery.StringFieldType},
		{path: []string{"status"}, wantType: bigquery.StringFieldType},
		{path: []string{"start_time"}, wantType: bigquery.TimestampFieldType},
		{path: []string{"end_time"}, wantType: bigquery.TimestampFieldType},
		{path: []string{"create_time"}, wantType: bigquery.TimestampFieldType},
		{path: []string{"operation"}, wantType: bigquery.StringFieldType},
		{path: []string{"google_project_id"}, wantType: bigquery.StringFieldType},
		{path: []string{"error"}, wantType: bigquery.RecordFieldType},
		{path: []string{"error", "code"}, wantType: bigquery.IntegerFieldType},
		{path: []string{"error", "message"}, wantType: bigquery.StringFieldType},
		{path: []string{"integration_test"}, wantType: bigquery.RecordFieldType},
		{path: []string{"integration_test", "cases"}, wantType: bigquery.RecordFieldType, wantRepeated: true},
		{path: []string{"integration_test", "cases", "id"}, wantType: bigquery.StringFieldType},
		{path: []string{"integration_test", "cases", "duration"}, wantType: bigquery.StringFieldType},
		{path: []string{"integration_test", "cases", "checks"}, wantType: bigquery.RecordFieldType, wantRepeated: true},
		{path: []string{"integration_test", "cases", "checks", "status"}, wantType: bigquery.StringFieldType},
		{path: []string{"load_test"}, wantType: bigquery.RecordFieldType},
		{path: []string{"load_test", "cases", "summary", "duration"}, wantType: bigquery.StringFieldType},
		{path: []string{"load_test", "cases", "summary", "target_qps"}, wantType: bigquery.FloatFieldType},
		{path: []string{"load_test", "cases", "summary", "request_count"}, wantType: bigquery.IntegerFieldType},
		{path: []string{"load_test", "cases", "summary", "mode"}, wantType: bigquery.StringFieldType},
		{path: []string{"agent_eval"}, wantType: bigquery.RecordFieldType},
		{path: []string{"agent_eval", "cases", "duration"}, wantType: bigquery.StringFieldType},
		{path: []string{"agent_eval", "cases", "metrics"}, wantType: bigquery.RecordFieldType, wantRepeated: true},
	}
	for _, tt := range tests {
		t.Run(joinPath(tt.path), func(t *testing.T) {
			t.Parallel()
			f := lookupField(s, tt.path)
			if f == nil {
				t.Fatalf("field %v not found in schema", tt.path)
			}
			if f.Type != tt.wantType {
				t.Errorf("type = %v, want %v", f.Type, tt.wantType)
			}
			if f.Repeated != tt.wantRepeated {
				t.Errorf("repeated = %v, want %v", f.Repeated, tt.wantRepeated)
			}
			if f.Required {
				t.Errorf("required = true, want false (all evalspb.Run fields are NULLABLE)")
			}
		})
	}
}

// TestSchemaJSON_roundtripsThroughSchemaFromJSON asserts that SchemaJSON()
// emits bytes accepted by bigquery.SchemaFromJSON() and that the roundtripped
// schema equals Schema().
func TestSchemaJSON_roundtripsThroughSchemaFromJSON(t *testing.T) {
	t.Parallel()
	raw, err := SchemaJSON()
	if err != nil {
		t.Fatalf("SchemaJSON: %v", err)
	}
	got, err := bigquery.SchemaFromJSON(raw)
	if err != nil {
		t.Fatalf("SchemaFromJSON: %v", err)
	}
	if !schemasEqual(got, Schema()) {
		t.Fatalf("roundtrip mismatch\n got: %s", raw)
	}
}

// TestSchemaJSON_matchesGoldenFixture asserts that SchemaJSON() and the
// checked-in testdata/run.schema.json produce the same []*FieldSchema after
// parsing. This proves the fixture is exactly what `bq mk --schema` would
// accept from SchemaJSON().
func TestSchemaJSON_matchesGoldenFixture(t *testing.T) {
	t.Parallel()
	raw, err := SchemaJSON()
	if err != nil {
		t.Fatalf("SchemaJSON: %v", err)
	}
	fromEmitter, err := bigquery.SchemaFromJSON(raw)
	if err != nil {
		t.Fatalf("SchemaFromJSON(emitter): %v", err)
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "run.schema.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	fromFixture, err := bigquery.SchemaFromJSON(fixture)
	if err != nil {
		t.Fatalf("SchemaFromJSON(fixture): %v", err)
	}
	if !schemasEqual(fromEmitter, fromFixture) {
		t.Fatal("SchemaJSON() output does not match testdata/run.schema.json")
	}
}

// TestSchemaJSON_omitsStatusDetails is a wire-format smoke test asserting
// google.rpc.Status.details is not present in the emitted JSON bytes.
func TestSchemaJSON_omitsStatusDetails(t *testing.T) {
	t.Parallel()
	raw, err := SchemaJSON()
	if err != nil {
		t.Fatalf("SchemaJSON: %v", err)
	}
	if bytesContains(raw, "details") {
		t.Errorf("SchemaJSON output contains 'details'; google.rpc.Status.details must be omitted")
	}
}

func bytesContains(hay []byte, needle string) bool {
	return indexOf(string(hay), needle) >= 0
}

func indexOf(hay, needle string) int {
	n := len(needle)
	for i := 0; i+n <= len(hay); i++ {
		if hay[i:i+n] == needle {
			return i
		}
	}
	return -1
}

// TestSchema_omitsStatusDetails asserts that google.rpc.Status.details
// (repeated Any) is omitted from the BQ schema. protojson emits Any with
// @type keys which are invalid BQ column names.
func TestSchema_omitsStatusDetails(t *testing.T) {
	t.Parallel()
	errField := lookupField(Schema(), []string{"error"})
	if errField == nil {
		t.Fatal("error field missing from schema")
	}
	for _, sub := range errField.Schema {
		if sub.Name == "details" {
			t.Errorf("google.rpc.Status.details must be omitted; found %+v", sub)
		}
	}
	// Verify the two allowed subfields are present.
	names := make(map[string]struct{}, len(errField.Schema))
	for _, sub := range errField.Schema {
		names[sub.Name] = struct{}{}
	}
	for _, want := range []string{"code", "message"} {
		if _, ok := names[want]; !ok {
			t.Errorf("error subfield %q missing", want)
		}
	}
	if len(errField.Schema) != 2 {
		t.Errorf("error subfield count = %d, want 2 (code, message)", len(errField.Schema))
	}
}

// TestSchema_mapAsRepeatedRecord asserts that proto map fields become
// REPEATED RECORD{key, value} — matching go.einride.tech/protobuf-bigquery's
// convention so the JSON path (bqschema) and streaming-insert path
// (bqreport) stay in lockstep on schema shape.
func TestSchema_mapAsRepeatedRecord(t *testing.T) {
	t.Parallel()
	f := lookupField(Schema(), []string{"load_test", "cases", "summary", "errors_by_code"})
	if f == nil {
		t.Fatal("errors_by_code map field missing from schema")
	}
	if f.Type != bigquery.RecordFieldType {
		t.Errorf("type = %v, want RECORD", f.Type)
	}
	if !f.Repeated {
		t.Errorf("repeated = false, want true")
	}
	if got, want := len(f.Schema), 2; got != want {
		t.Fatalf("subfield count = %d, want %d (key, value)", got, want)
	}
	key := f.Schema[0]
	val := f.Schema[1]
	if key.Name != "key" || key.Type != bigquery.StringFieldType {
		t.Errorf("key subfield = %+v, want name=key type=STRING", key)
	}
	if val.Name != "value" || val.Type != bigquery.IntegerFieldType {
		t.Errorf("value subfield = %+v, want name=value type=INTEGER", val)
	}
}

// TestSchema_oneofArmsAreSiblings asserts that the three arms of
// Run.data oneof surface as sibling top-level RECORD fields, not nested
// inside a "data" wrapper.
func TestSchema_oneofArmsAreSiblings(t *testing.T) {
	t.Parallel()
	s := Schema()
	for _, name := range []string{"integration_test", "load_test", "agent_eval", "infra_observation"} {
		if lookupField(s, []string{name}) == nil {
			t.Errorf("oneof arm %q missing from top-level schema", name)
		}
	}
	if lookupField(s, []string{"data"}) != nil {
		t.Errorf("unexpected top-level %q wrapper; oneof arms should be siblings", "data")
	}
}

func lookupField(s bigquery.Schema, path []string) *bigquery.FieldSchema {
	if len(path) == 0 {
		return nil
	}
	for _, f := range s {
		if f.Name == path[0] {
			if len(path) == 1 {
				return f
			}
			return lookupField(f.Schema, path[1:])
		}
	}
	return nil
}

func joinPath(p []string) string {
	out := ""
	for i, s := range p {
		if i > 0 {
			out += "."
		}
		out += s
	}
	return out
}

// schemasEqual compares two schemas semantically (order-independent by field name).
func schemasEqual(a, b bigquery.Schema) bool {
	if len(a) != len(b) {
		return false
	}
	ai := indexByName(a)
	bi := indexByName(b)
	if len(ai) != len(bi) {
		return false
	}
	for name, af := range ai {
		bf, ok := bi[name]
		if !ok {
			return false
		}
		if !fieldsEqual(af, bf) {
			return false
		}
	}
	return true
}

func fieldsEqual(a, b *bigquery.FieldSchema) bool {
	if a.Name != b.Name || a.Type != b.Type || a.Repeated != b.Repeated || a.Required != b.Required {
		return false
	}
	return schemasEqual(a.Schema, b.Schema)
}

func indexByName(s bigquery.Schema) map[string]*bigquery.FieldSchema {
	out := make(map[string]*bigquery.FieldSchema, len(s))
	for _, f := range s {
		out[f.Name] = f
	}
	return out
}

func marshalSchema(s bigquery.Schema) (string, error) {
	buf, err := json.MarshalIndent(sanitize(s), "", "  ")
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// sanitize converts a Schema into a plain []map for stable diff output.
func sanitize(s bigquery.Schema) []map[string]any {
	out := make([]map[string]any, 0, len(s))
	for _, f := range s {
		entry := map[string]any{
			"name": f.Name,
			"type": string(f.Type),
			"mode": modeOf(f),
		}
		if len(f.Schema) > 0 {
			entry["fields"] = sanitize(f.Schema)
		}
		out = append(out, entry)
	}
	return out
}

func modeOf(f *bigquery.FieldSchema) string {
	switch {
	case f.Repeated:
		return "REPEATED"
	case f.Required:
		return "REQUIRED"
	default:
		return "NULLABLE"
	}
}
