// Package report defines the [Reporter] interface for emitting completed
// evalspb.Run values to external sinks, plus generic combinators for wiring
// multiple sinks together.
//
// Concrete reporters live in subpackages so heavyweight dependencies (alog,
// BigQuery SDK, einride, Pub/Sub client, etc.) are not pulled in by callers
// that only need the interface:
//
//   - [go.alis.build/evals/report/log] — default one-line alog summary; the
//     reference implementation of the Reporter contract below
//   - [go.alis.build/evals/report/bqschema] — companion package holding the
//     canonical BigQuery schema for evalspb.Run and a table-provisioning
//     helper. Not itself a Reporter — used by the other reporters.
//   - [go.alis.build/evals/report/bigquery] — streaming insert to BigQuery
//     using the shared bqschema layout
//   - [go.alis.build/evals/report/pubsub] — publish bare Run JSON to Pub/Sub
//     via protojson (compatible with Pub/Sub → BigQuery "Use table schema"
//     subscriptions against a table provisioned from bqschema)
//
// # Wiring
//
// TestServiceServer holds a single [Reporter]. Wire [Reporter.ReportRun]
// inside the optional `onSuiteComplete` callback passed to
// [go.alis.build/evals/runner.Runner] Run*Suites methods so each
// `evalspb.Run` is emitted as soon as its suite finishes — the runner
// provides per-suite timing; the service owns mapping and I/O.
//
// Set TestServiceServer.Reporter to nil to silence emission entirely, or
// wrap several sinks with [All] or [FailFast] (or the [MultiReporter] alias
// for fail-fast) to fan out.
//
// Bootstrap the BigQuery table once, then fan out to log, BigQuery, and
// Pub/Sub:
//
//	import (
//	    "context"
//	    "io"
//
//	    "cloud.google.com/go/bigquery"
//	    "go.alis.build/evals/report"
//	    bqschema "go.alis.build/evals/report/bqschema"
//	    bqreport "go.alis.build/evals/report/bigquery"
//	    logreport "go.alis.build/evals/report/log"
//	    pubsubreport "go.alis.build/evals/report/pubsub"
//	)
//
//	type multiCloser struct{ closers []io.Closer }
//
//	func (m multiCloser) Close() error {
//	    var err error
//	    for _, c := range m.closers {
//	        if c == nil {
//	            continue
//	        }
//	        err = errors.Join(err, c.Close())
//	    }
//	    return err
//	}
//
//	func setupReporters(ctx context.Context, bqClient *bigquery.Client, datasetID, tableID string) (io.Closer, error) {
//	    // bqClient must target ALIS_OS_PRODUCT_PROJECT (not ALIS_OS_PROJECT).
//	    if err := bqschema.EnsureTable(ctx, bqClient, datasetID, tableID); err != nil {
//	        return nil, err
//	    }
//	    bq, err := bqreport.NewWithClient(ctx, bqClient, datasetID, tableID)
//	    if err != nil {
//	        return nil, err
//	    }
//	    ps, err := pubsubreport.New(ctx)
//	    if err != nil {
//	        _ = bq.Close()
//	        return nil, err
//	    }
//	    services.TestServiceServer.Reporter = report.All{
//	        logreport.Reporter{},
//	        bq,
//	        ps,
//	    }
//	    return multiCloser{closers: []io.Closer{bq, ps}}, nil
//	}
//
// Call the returned [io.Closer] during server drain so Pub/Sub and BigQuery
// clients flush cleanly.
//
// # Fan-out semantics
//
// [All] invokes every non-nil reporter and returns [errors.Join] of all
// failures. [FailFast] and [MultiReporter] stop at the first error. Nil
// entries in either slice are skipped.
//
// Reporters run **serially**. Bundled non-log sinks default to a 10s
// per-call timeout, so [All] worst-case latency is the **sum** of each sink's
// timeout — a log + BigQuery + Pub/Sub stack can add ~20s per suite on top of
// log I/O. Prefer [FailFast] when one durable sink is authoritative, or keep
// [All] when every sink must be attempted and accept the cumulative delay.
//
// # Contract
//
// Reporter implementations should be:
//
//   - Bounded latency. The reporter is called from the LRO goroutine, so a
//     slow sink stalls the operation. Persist asynchronously or wrap each
//     call in a short timeout — both bundled non-log reporters default to a
//     10s per-call timeout and expose knobs to tune it.
//   - Nil-safe. `run == nil` should be treated as a no-op that returns nil.
//     A nil receiver should also be a no-op — this keeps caller code free of
//     `if r != nil` guards when a reporter slot is optional.
//   - Best-effort. Returning an error is fine — the caller logs it and
//     continues. Under [All], one failing sink does not prevent later sinks
//     from running; under [FailFast], later sinks are skipped for that run.
//
// The log reporter satisfies all three; use it as a reference when writing
// your own.
package report
