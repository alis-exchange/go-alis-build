// Package bqschema is the single source of truth for the BigQuery schema of
// evalspb.Run rows and for table provisioning.
//
// Both evals/report/pubsub (JSON via protojson for Pub/Sub → BigQuery
// table-schema subscriptions) and evals/report/bigquery (streaming inserts
// via protobq with a Duration-string override) produce rows that match
// [Schema]. Use this package when you need the schema definition, a
// Terraform/bq-mk-ready JSON file, or in-process table auto-provisioning.
//
// # Schema derivation
//
// [Schema] walks the alis.evals.v1.Run protoreflect descriptor and maps:
//
//   - google.protobuf.Timestamp → TIMESTAMP
//   - google.protobuf.Duration  → STRING (protojson-native "1.500000000s")
//   - google.rpc.Status         → RECORD{code INT64, message STRING}
//   - Enums                     → STRING
//   - Regular messages          → RECORD (recurse)
//   - Repeated fields           → mode REPEATED
//   - Repeated entry lists      → REPEATED RECORD{key, value} (tags, errors_by_code)
//   - Oneof arms                → sibling NULLABLE RECORD columns at parent level
//
// google.rpc.Status.details (repeated google.protobuf.Any) is deliberately
// omitted from the schema. protojson emits Any values with @type keys, which
// are not valid BigQuery column names. Pub/Sub → BigQuery drops unmapped keys
// silently; populated details are lost at ingest but do not cause row errors.
//
// # Terraform / bq mk path
//
// Export the schema to a JSON file and provision the table at deploy time:
//
//	schemaJSON, err := bqschema.SchemaJSON()
//	if err != nil {
//	    return err
//	}
//	// write schemaJSON to schema.json, then:
//	// bq mk --table PROJECT:dataset.runs schema.json
//
// # In-process provisioning
//
// Call [EnsureTable] at bootstrap before constructing reporters:
//
//	import (
//	    "context"
//
//	    "cloud.google.com/go/bigquery"
//	    bqschema "go.alis.build/evals/report/bqschema"
//	)
//
//	func bootstrap(ctx context.Context, client *bigquery.Client) error {
//	    return bqschema.EnsureTable(ctx, client, "evals", "runs",
//	        bigquery.TableMetadata{
//	            TimePartitioning: &bigquery.TimePartitioning{
//	                Field: "start_time",
//	                Type:  bigquery.DayPartitioningType,
//	            },
//	        },
//	    )
//	}
//
// The dataset must already exist. [EnsureTable] creates the table if missing
// (with any supplied [bigquery.TableMetadata] for partitioning, clustering,
// etc.) or applies an additive schema update if the table already exists.
//
// evals/report/bigquery's [WithAutoCreateTable] option delegates to
// [EnsureTable] internally.
package bqschema
