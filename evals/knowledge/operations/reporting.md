---
title: reporting
description: Reporter replacement and publication guidance.
tags: [reporters, pubsub, bigquery]
---

# Reporting

`Run` never reports. `RunAndPublish` reports once after execution.

Use `WithReporter` to replace the reporter for one run:

```go
run, err := suite.RunAndPublish(ctx, evals.WithReporter(myReporter))
```

To publish to multiple sinks, pass a reporter combinator explicitly:

```go
run, err := suite.RunAndPublish(ctx, evals.WithReporter(report.All{
    logreport.Reporter{},
    pubsubReporter,
    bigqueryReporter,
}))
```

The standard Pub/Sub topic is `alis.evals.v1.Run`. Pub/Sub and BigQuery sinks
usually target the product project (`ALIS_OS_PRODUCT_PROJECT`). The
`Run.google_project_id` field defaults from `ALIS_OS_PROJECT` and can be
overridden with `WithGoogleProjectID`.
