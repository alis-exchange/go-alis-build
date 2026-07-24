---
title: reporting
description: Reporter replacement and publication guidance.
tags: [reporters, pubsub, bigquery]
---

# Reporting

`Run` never creates or invokes a reporter. `RunAndPublish` reports once after
execution. Without an override it lazily creates the standard Pub/Sub reporter,
then closes that owned reporter after publication.

Use `WithReporter` to replace the reporter for one run:

```go
run, err := suite.RunAndPublish(ctx, evals.WithReporter(myReporter))
```

A custom reporter is borrowed and is never closed by the suite. Repeated
`WithReporter` options replace one another; the last reporter wins.

To publish to multiple sinks, pass a reporter combinator explicitly:

```go
run, err := suite.RunAndPublish(ctx, evals.WithReporter(report.All{
    logreport.Reporter{},
    pubsubReporter,
    bigqueryReporter,
}))
```

`report.All` calls every reporter serially and joins their errors.
`report.MultiReporter` is an alias for fail-fast `report.FailFast`, which stops
at the first reporting error.

Reporter errors do not discard the run: `RunAndPublish` returns both the
materialized run and the operational error.

Partial cancelled runs are still published. Publication uses a fresh
10-second context derived from `context.Background()`, so cancellation of the
execution context does not immediately cancel delivery.

The standard Pub/Sub topic is `alis.evals.v1.Run`. Pub/Sub and BigQuery sinks
usually target the product project (`ALIS_OS_PRODUCT_PROJECT`). The
`Run.google_project_id` field defaults from `ALIS_OS_PROJECT` and can be
overridden with `WithGoogleProjectID`.
