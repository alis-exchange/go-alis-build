package bqschema

import (
	"cloud.google.com/go/bigquery"
	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Well-known-type names that receive bespoke BigQuery mappings.
const (
	wktTimestamp = "google.protobuf.Timestamp"
	wktDuration  = "google.protobuf.Duration"
	wktRpcStatus = "google.rpc.Status"
)

// Schema returns the BigQuery schema for evalspb.Run rows produced by both
// evals/report/pubsub (JSON via protojson) and evals/report/bigquery
// (streaming inserts via protobq with a Duration-string override).
//
// The schema is derived from the alis.evals.v1.Run descriptor:
//
//   - google.protobuf.Timestamp → TIMESTAMP
//   - google.protobuf.Duration  → STRING (protojson-native "1.500000000s")
//   - google.rpc.Status         → RECORD{code INT64, message STRING} (details omitted)
//   - Enums                     → STRING (protojson emits the enum name)
//   - Strings                   → STRING; Bytes → BYTES; Bool → BOOL
//   - All integer variants      → INT64
//   - Float / Double            → FLOAT64
//   - Regular messages          → RECORD (recurse)
//   - Repeated fields           → mode REPEATED
//   - Oneof arms                → sibling NULLABLE at parent level
//   - Map fields                → REPEATED RECORD{key, value}
//
// All fields are NULLABLE (BigQuery does not enforce proto3 required semantics).
// Column names use the proto field name (already snake_case in evalspb).
//
// Schema is deterministic and safe to call concurrently.
func Schema() bigquery.Schema {
	return fieldsSchema((&evalspb.Run{}).ProtoReflect().Descriptor().Fields())
}

// SchemaJSON returns Schema() serialised in the JSON file format accepted by
// `bq mk --schema` and Terraform's google_bigquery_table.schema resource.
//
// The output is a JSON array of field descriptors:
//
//	[
//	  {"name": "run_id", "type": "STRING", "mode": "NULLABLE"},
//	  {"name": "cases",  "type": "RECORD", "mode": "REPEATED", "fields": [...]},
//	  ...
//	]
//
// It is deterministic and safe to call concurrently.
func SchemaJSON() ([]byte, error) {
	return Schema().ToJSONFields()
}

func fieldsSchema(fds protoreflect.FieldDescriptors) bigquery.Schema {
	out := make(bigquery.Schema, 0, fds.Len())
	for i := 0; i < fds.Len(); i++ {
		out = append(out, fieldSchema(fds.Get(i)))
	}
	return out
}

func fieldSchema(fd protoreflect.FieldDescriptor) *bigquery.FieldSchema {
	fs := &bigquery.FieldSchema{Name: string(fd.Name())}
	if fd.Cardinality() == protoreflect.Repeated {
		fs.Repeated = true
	}
	switch fd.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		msg := fd.Message()
		switch msg.FullName() {
		case wktTimestamp:
			fs.Type = bigquery.TimestampFieldType
		case wktDuration:
			fs.Type = bigquery.StringFieldType
		case wktRpcStatus:
			fs.Type = bigquery.RecordFieldType
			fs.Schema = bigquery.Schema{
				{Name: "code", Type: bigquery.IntegerFieldType},
				{Name: "message", Type: bigquery.StringFieldType},
			}
		default:
			fs.Type = bigquery.RecordFieldType
			fs.Schema = fieldsSchema(msg.Fields())
		}
	case protoreflect.EnumKind:
		fs.Type = bigquery.StringFieldType
	case protoreflect.StringKind:
		fs.Type = bigquery.StringFieldType
	case protoreflect.BytesKind:
		fs.Type = bigquery.BytesFieldType
	case protoreflect.BoolKind:
		fs.Type = bigquery.BooleanFieldType
	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		fs.Type = bigquery.IntegerFieldType
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		fs.Type = bigquery.FloatFieldType
	}
	return fs
}
