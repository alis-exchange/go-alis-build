---
type: Concept
title: Reporter
description: The sink that receives each completed `Run` proto. Bundled implementations plus a fan-out.
tags: [reporter, sink, pubsub, bigquery]
timestamp: 2026-07-08T00:00:00Z
---

# Definition

A **reporter** receives one `evalspb.Run` per completed suite. It is
wired onto `TestServiceServer.Reporter` at process start; setting it
to `nil` disables reporting entirely.

# Interface

```go
package report

type Reporter interface {
    ReportRun(ctx context.Context, run *evalspb.Run) error
}
```

# Bundled implementations

| Type | Package | Purpose |
| ---- | ------- | ------- |
| `log.Reporter{}` | `go.alis.build/evals/report/log` | Default. Emits a one-line summary via `alog` — `Info` for `PASSED`, `Warn` otherwise. Nil-safe on nil runs. |
| `bigquery.Reporter` | `go.alis.build/evals/report/bigquery` | Streams each Run to a pre-existing BigQuery table. |
| `report.NoOpReporter{}` | `go.alis.build/evals/report` | Discard. Useful in local tests. |
| `report.MultiReporter{...}` | `go.alis.build/evals/report` | Fan-out to multiple reporters, in order. First error aborts. |

# Wiring

```go
import (
    "go.alis.build/evals/report"
    logreport "go.alis.build/evals/report/log"
    bqreport "go.alis.build/evals/report/bigquery"
)

bq, err := bqreport.New(ctx, projectID, "evals", "runs")
if err != nil { ... }
defer bq.Close()

services.TestServiceServer.Reporter = report.MultiReporter{
    logreport.Reporter{},
    bq,
    myPubSubReporter{topic: "eval-runs"},
}
```

# Contract for custom reporters

1. **Handle `run == nil`** as a no-op.
2. **Do not block the LRO goroutine for long** — persist async or with
   a short timeout.
3. **Errors are best-effort.** Returning one is logged by the caller;
   subsequent reporters in a `MultiReporter` are skipped for that run.

# Minimal Pub/Sub example

```go
type PubSubReporter struct {
    Topic *pubsub.Topic
}

func (r *PubSubReporter) ReportRun(ctx context.Context, run *evalspb.Run) error {
    if run == nil {
        return nil
    }
    b, err := proto.Marshal(run)
    if err != nil {
        return err
    }
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    _, err = r.Topic.Publish(ctx, &pubsub.Message{Data: b}).Get(ctx)
    return err
}
```

# Related

* [`report` package](/packages/report.md)
* [Reporters API](/api/reporters.md)
* [Run wire type](/wire-types/run.md)

# Citations

[1] [report/report.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/report/report.go)
[2] [report/log/log.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/report/log/log.go)
[3] [report/bigquery/bigquery.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/report/bigquery/bigquery.go)
