// Package pubsub implements a [report.Reporter] that publishes each
// completed evalspb.Run to Pub/Sub via [go.alis.build/events].
//
// Every Run is wrapped in an alis.evals.v1.RunPublishedEvent envelope and
// published to the topic corresponding to that message's proto full name
// (i.e. alis.evals.v1.RunPublishedEvent). On the Alis Build platform this
// topic is auto-provisioned by the define step, along with a proto schema
// registered against it.
//
// # BigQuery integration
//
// Pub/Sub can attach a BigQuery subscription to write messages directly
// into a table, but note two constraints of the direct-subscription proto
// path:
//
//  1. Pub/Sub → BigQuery marks proto oneof fields as unmappable. The
//     underlying Run has a oneof data { integration_test, load_test,
//     agent_eval }, so a "Use topic schema" subscription against this
//     topic will fail schema compatibility.
//  2. Even without the oneof, Pub/Sub → BigQuery does not unwrap
//     well-known types the way this repo's in-process
//     [go.alis.build/evals/report/bigquery] reporter does — e.g.
//     Timestamps land as RECORD<seconds, nanos> rather than TIMESTAMP.
//
// If you want a queryable BigQuery table fed by Pub/Sub whose shape
// matches the in-process reporter's output, run a small subscriber that
// unmarshals RunPublishedEvent and forwards evt.Run to
// [go.alis.build/evals/report/bigquery.Reporter]. If you only need a raw
// archive of published events, use a "Don't use a schema" subscription
// with a single data BYTES column.
//
// # Wiring
//
// Import with an alias when the file also uses the standard library or
// the cloud.google.com/go/pubsub client:
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
// By default every ReportRun call blocks until the Pub/Sub broker acks
// the message and returns any broker error to the caller. This is the
// safer choice for short-lived eval processes that may exit right after
// completing a run. Use [WithBackground] only when publish latency
// dominates and best-effort delivery is acceptable.
//
// # Client ownership
//
// [New] creates and owns its events.Client — call Close to release it.
// [NewWithClient] borrows a client and Close is a no-op; the caller
// retains ownership of the client.
package pubsub
