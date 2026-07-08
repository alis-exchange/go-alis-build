// Package report defines the [Reporter] interface for emitting completed
// evalspb.Run values to external sinks, plus generic combinators for wiring
// multiple sinks together.
//
// Concrete reporters live in subpackages so heavyweight dependencies (alog,
// BigQuery SDK, einride, Pub/Sub clients, etc.) are not pulled in by callers
// that only need the interface:
//
//   - [go.alis.build/evals/report/log] — default one-line alog summary
//   - [go.alis.build/evals/report/bigquery] — streaming insert to BigQuery
//
// # Wiring
//
// TestServiceServer holds a single [Reporter]. It is called once per
// completed Run — one call per suite executed during a RunTest, RunEval,
// or RunLoad LRO. Set `TestServiceServer.Reporter` to nil to silence
// emission entirely, or wrap several sinks with [MultiReporter] to fan out.
//
//	import (
//	    "go.alis.build/evals/report"
//	    logreport "go.alis.build/evals/report/log"
//	    bqreport "go.alis.build/evals/report/bigquery"
//	)
//
//	bq, err := bqreport.New(ctx, projectID, datasetID, tableID)
//	if err != nil { ... }
//	defer bq.Close()
//
//	services.TestServiceServer.Reporter = report.MultiReporter{
//	    logreport.Reporter{},
//	    bq,
//	    myPubSubReporter{topic: "eval-runs"},
//	}
//
// # Contract
//
// Reporter implementations should be:
//
//   - Non-blocking on happy path. The reporter is called from the LRO
//     goroutine; a slow sink stalls the operation.
//   - Nil-safe. `run == nil` should be treated as a no-op.
//   - Best-effort. Returning an error is fine — the caller logs it and
//     continues. A failing reporter must not prevent the LRO from
//     completing.
//
// The log reporter satisfies all three; use it as a reference when writing
// your own.
package report
