// Package pubsub implements a [report.Reporter] that publishes each
// completed evalspb.Run as JSON to Pub/Sub via
// google.golang.org/protobuf/encoding/protojson, on top of
// cloud.google.com/go/pubsub/v2.
//
// # JSON payload contract
//
// Marshaling is locked to protojson with UseProtoNames=true and
// EmitUnpopulated=true. Timestamps are RFC 3339 strings, Durations are
// protojson-native strings (e.g. "1.500s"), enums are proto names, and only
// the set oneof arm is emitted. The payload is a bare evalspb.Run — no
// RunPublishedEvent envelope.
//
// # Default topic
//
// The default topic is "alis.evals.v1.Run", the proto full name of the
// payload. Unlike "*Event"-suffixed messages, this topic is NOT
// auto-provisioned by the Alis Build platform's define step. Callers must
// provision the topic (and any Pub/Sub → BigQuery subscription) via
// Terraform. Override with [WithTopic] when your platform provisions under
// a different name.
//
// # BigQuery integration
//
// Pub/Sub can attach a "Use table schema" BigQuery subscription that lands
// each published message directly into a table, using the message payload
// as JSON and matching keys against the table's column names. Because this
// package emits protojson-formatted JSON (not the raw protobuf-wire path
// that Pub/Sub → BigQuery marks as unmappable for oneof and well-known
// types), the resulting rows fit a table provisioned from
// [go.alis.build/evals/report/bqschema] with no additional glue.
//
// If you want the streaming-insert path instead (lower latency, no
// subscription plumbing), pair this reporter with
// [go.alis.build/evals/report/bigquery] behind a [report.MultiReporter];
// both produce rows that match bqschema's canonical layout. If you only
// need a raw archive of published events, attach a "Don't use a schema"
// subscription with a single BYTES data column.
//
// # Wiring
//
// Import with an alias when the file also uses cloud.google.com/go/pubsub:
//
//	import pubsubreport "go.alis.build/evals/report/pubsub"
//
// Standalone:
//
//	func setupReporter(ctx context.Context) (*pubsubreport.Reporter, error) {
//	    r, err := pubsubreport.New(ctx)
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
//	    pubsubreport "go.alis.build/evals/report/pubsub"
//	)
//
//	func setupReporters(ctx context.Context, projectID, datasetID, tableID string) error {
//	    bq, err := bqreport.New(ctx, projectID, datasetID, tableID)
//	    if err != nil {
//	        return err
//	    }
//	    ps, err := pubsubreport.New(ctx)
//	    if err != nil {
//	        _ = bq.Close()
//	        return err
//	    }
//	    services.TestServiceServer.Reporter = report.MultiReporter{
//	        logreport.Reporter{},
//	        bq,
//	        ps,
//	    }
//	    return nil // Close bq and ps at server drain
//	}
//
// # Delivery semantics
//
// By default every ReportRun call blocks until the Pub/Sub broker acks the
// message and any broker error is returned to the caller (default 10s
// timeout via [WithPublishTimeout]). This is the safer choice for
// short-lived eval processes that may exit right after completing a run.
// Use [WithBackground] only when publish latency dominates and best-effort
// delivery is acceptable; call [Reporter.Close] before process exit to
// flush any pending messages. [WithOrderingKey] enables message ordering
// on the underlying publisher.
//
// # Client ownership
//
// [New] creates and owns its *pubsub.Client — call [Reporter.Close] to
// release it. [NewWithClient] borrows an existing client and Close is a
// no-op with respect to the client (only the *pubsub.Publisher is
// stopped); the caller retains ownership of the client.
package pubsub
