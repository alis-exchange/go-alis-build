// Package bigquery implements a [report.Reporter] that streams completed
// evalspb.Run values to a pre-existing BigQuery table.
//
// # Schema
//
// The target table schema must match [InferSchema] (or [Reporter.Schema]
// when the reporter is configured with [WithSchemaOptions]). Both use
// go.einride.tech/protobuf-bigquery to mirror the Run proto as nested
// BigQuery columns. Well-known types are mapped for query ergonomics:
// google.protobuf.Timestamp → TIMESTAMP, Duration → FLOAT (seconds),
// google.rpc.Status → RECORD, wrapper types → their scalar type.
//
// # Provisioning
//
// Export the inferred schema and create the table at deploy time:
//
//	schemaJSON, _ := bqreport.InferSchema().ToJSONFields()
//	// write schemaJSON to schema.json, then:
//	// bq mk --table proj:dataset.runs schema.json
//
// If the reporter is constructed with [WithSchemaOptions], provision using
// r.Schema() instead so schema and marshaling stay in sync.
//
// Alternatively, opt into framework-managed provisioning with
// [WithAutoCreateTable]. On construction the reporter will create the table
// if it is missing (with any [bigquery.TableMetadata] you supply for
// partitioning / clustering / expiration), or apply an additive schema
// update if the table already exists. The dataset must exist either way —
// missing datasets fail at construction with a clear error.
//
// The reporter never creates the dataset.
//
// # Wiring
//
// Import with an alias when the file also uses cloud.google.com/go/bigquery:
//
//	import bqreport "go.alis.build/evals/report/bigquery"
//
// Standalone:
//
//	r, err := bqreport.New(ctx, projectID, datasetID, tableID)
//	if err != nil { ... }
//	defer r.Close()
//	services.TestServiceServer.Reporter = r
//
// Fan out alongside other sinks:
//
//	import (
//	    "go.alis.build/evals/report"
//	    logreport "go.alis.build/evals/report/log"
//	    bqreport "go.alis.build/evals/report/bigquery"
//	)
//
//	r, err := bqreport.New(ctx, projectID, datasetID, tableID)
//	if err != nil { ... }
//	defer r.Close()
//	services.TestServiceServer.Reporter = report.MultiReporter{
//	    logreport.Reporter{},
//	    r,
//	}
//
// # Client ownership
//
// [New] creates and owns its BigQuery client — call Close to release it.
// [NewWithClient] borrows a client and Close is a no-op; the caller retains
// ownership of the client.
package bigquery
