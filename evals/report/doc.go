// Package report defines the [Reporter] interface for emitting completed
// `evalspb.Run` values to external sinks (Pub/Sub topics, BigQuery,
// Spanner, dashboards) and ships two implementations for common cases.
//
// # Wiring
//
// TestServiceServer holds a single [Reporter]. It is called once per
// completed Run — one call per suite executed during a RunTest, RunEval,
// or RunLoad LRO. The default is [LogReporter], which writes a one-line
// summary via alog (Info for passing runs, Warn for failing ones). Set
// `TestServiceServer.Reporter` to nil to silence emission entirely, or
// wrap several sinks with [MultiReporter] to fan out.
//
// Concrete sinks — Pub/Sub, BigQuery, Spanner, custom dashboards — live
// outside this package. Product neurons implement [Reporter] and assign
// it once during setup:
//
//	services.TestServiceServer.Reporter = report.MultiReporter{
//	    report.LogReporter{},
//	    myPubSubReporter{topic: "eval-runs"},
//	    myBigQueryReporter{table: "runs"},
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
// LogReporter satisfies all three; use it as a reference when writing
// your own.
package report
