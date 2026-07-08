// Package loadgen is a small, embedded load generator for the evals runner.
//
// The design is deliberately narrow: one [Profile] describes a fixed-rate
// load window, one [Pacer] schedules sends, a fixed pool of workers
// executes a caller-supplied [Target] function, and a single aggregator
// goroutine folds results into aggregate [Metrics]. Nothing in this
// package knows about proto types, suites, or SLOs — the parent [evals]
// package composes those on top.
//
// # Usage
//
// Case adapters obtain a generator from [New] and invoke [Generator.Run]
// with a resolved profile and a target closure:
//
//	g := loadgen.New()
//	m, err := g.Run(ctx, loadgen.Profile{
//	    QPS:            100,
//	    Concurrency:    25,
//	    Duration:       60*time.Second,
//	    Warmup:         10*time.Second,
//	    RequestTimeout: 5*time.Second,
//	}, func(ctx context.Context) error {
//	    _, err := clients.Files.ListFiles(ctx, req)
//	    return err
//	})
//
// # Concepts borrowed
//
// The pacer math and constant scheduling model come from vegeta / ghz —
// schedule request N at elapsed = N / rate, catch up immediately if
// behind, sleep to the absolute offset if ahead. This avoids scheduling
// error accumulating across a long window.
//
// Warmup as sample skipping (rather than a distinct pre-window) is taken
// from k6: the pacer runs at target rate for `Warmup + Duration`, and
// the aggregator drops any sample whose send timestamp is before
// `start + Warmup`. This gives autoscalers and JITs time to settle.
//
// # Coordinated omission
//
// The pacer's absolute-offset scheduling makes this an open-loop
// generator: when a request runs long the pacer does not wait, it just
// dispatches to the next available worker. Under saturation this yields
// realistic (worse) latency numbers instead of the classic
// coordinated-omission bias where slow responses artificially thin the
// arrival stream.
//
// When the worker pool cannot keep up, [Metrics.ActualQPS] falls below
// the target rate. The generator emits an alog warning when
// `ActualQPS < 0.9 × target QPS` so users notice they are measuring the
// generator rather than the SUT.
//
// # Error accounting
//
// Target errors are data, not control flow. A non-nil return increments
// [Metrics.ErrorCount] and [Metrics.ErrorsByCode] under the canonical
// gRPC status code name (`UNAVAILABLE`, `DEADLINE_EXCEEDED`, ...);
// non-gRPC errors fall into `UNKNOWN`. Panics are recovered per request
// and count as `UNKNOWN` errors.
//
// # Cancellation
//
// The generator honours `ctx`. On cancellation it stops the pacer,
// drains in-flight workers, closes the sample channel, and returns
// whatever metrics were collected together with `ctx.Err()` — partial
// but not corrupt.
package loadgen
