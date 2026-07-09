// Package bigquery implements a [report.Reporter] that streams completed
// evalspb.Run values to a pre-existing BigQuery table.
//
// # Schema
//
// Schema inference is delegated to
// [go.alis.build/evals/report/bqschema]. Both [InferSchema] and
// [Reporter.Schema] return bqschema.Schema(). [WithSchemaOptions] affects
// row marshaling only, not schema inference — provision the target table
// with bqschema.Schema() (or bqschema.SchemaJSON() for Terraform / `bq mk`)
// to guarantee the written rows always fit.
//
// Well-known types are mapped for query ergonomics:
// google.protobuf.Timestamp → TIMESTAMP; google.protobuf.Duration → STRING
// (protojson-native form like "1.500s", matching the JSON path from
// [go.alis.build/evals/report/pubsub]); google.rpc.Status → RECORD.
//
// # Provisioning
//
// Prefer [go.alis.build/evals/report/bqschema] for table provisioning:
//
//	bqschema.EnsureTable(ctx, client, datasetID, tableID)
//
// Or export the schema for Terraform / `bq mk`:
//
//	schemaJSON, _ := bqschema.SchemaJSON()
//	// write schemaJSON to schema.json, then:
//	// bq mk --table proj:dataset.runs schema.json
//
// [WithAutoCreateTable] opts into framework-managed provisioning: on
// construction the reporter delegates to bqschema.EnsureTable, which
// creates the table if it is missing (with any [bigquery.TableMetadata]
// you supply for partitioning / clustering / expiration), or applies an
// additive schema update if the table already exists. The dataset must
// exist either way — missing datasets fail at construction with a clear
// error. The reporter never creates the dataset.
//
// # Wiring
//
// Import with an alias when the file also uses cloud.google.com/go/bigquery:
//
//	import bqreport "go.alis.build/evals/report/bigquery"
//
// Standalone:
//
//	func setupReporter(ctx context.Context, projectID, datasetID, tableID string) (*bqreport.Reporter, error) {
//	    r, err := bqreport.New(ctx, projectID, datasetID, tableID)
//	    if err != nil {
//	        return nil, err
//	    }
//	    services.TestServiceServer.Reporter = r
//	    return r, nil // Close() at server drain
//	}
//
// Fan out alongside other sinks:
//
//	import (
//	    "context"
//
//	    "go.alis.build/evals/report"
//	    logreport "go.alis.build/evals/report/log"
//	    bqreport "go.alis.build/evals/report/bigquery"
//	)
//
//	func setupReporters(ctx context.Context, projectID, datasetID, tableID string) (*bqreport.Reporter, error) {
//	    r, err := bqreport.New(ctx, projectID, datasetID, tableID)
//	    if err != nil {
//	        return nil, err
//	    }
//	    services.TestServiceServer.Reporter = report.MultiReporter{
//	        logreport.Reporter{},
//	        r,
//	    }
//	    return r, nil // Close() at server drain
//	}
//
// # Client ownership
//
// [New] creates and owns its *bigquery.Client — call [Reporter.Close] to
// release it. [NewWithClient] borrows an existing client; Close is a no-op
// and the caller retains ownership of the client.
package bigquery
