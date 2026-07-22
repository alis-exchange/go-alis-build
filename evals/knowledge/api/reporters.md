---
type: API Reference
title: Reporters
description: The `report.Reporter` interface and bundled implementations.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/report/report.go
tags: [api, reporter, sink]
timestamp: 2026-07-09T00:00:00Z
---

# Interface

```go
package report

type Reporter interface {
    ReportRun(ctx context.Context, run *evalspb.Run) error
}
```

# Bundled reporter implementations

| Type | Package | Purpose |
| ---- | ------- | ------- |
| `log.Reporter{}` | `go.alis.build/evals/report/log` | Default one-line `alog` summary. |
| `bigquery.Reporter` | `go.alis.build/evals/report/bigquery` | Streaming insert via protobq (Duration as STRING). |
| `pubsub.Reporter` | `go.alis.build/evals/report/pubsub` | Publishes bare `Run` JSON via `protojson` + `pubsub/v2`. |
| `report.NoOpReporter{}` | `go.alis.build/evals/report` | Discard sink. |
| `report.All{…}` | `go.alis.build/evals/report` | Fan out in order; call every sink; [errors.Join] failures. |
| `report.FailFast{…}` / `report.MultiReporter{…}` | `go.alis.build/evals/report` | Fan out in order; stop at first error. |

# Fan-out latency

Reporters run **serially**. Bundled non-log sinks default to a **10s**
per-call timeout. With `report.All{log, bigquery, pubsub}`, worst-case
reporting adds up to **~20s** per suite (BigQuery + Pub/Sub) on top of
log I/O — once per suite in a multi-suite LRO. Use `FailFast` when one
durable sink is authoritative, or accept the cumulative delay when every
sink must be attempted.

# Companion packages

Not themselves `Reporter` implementations, but used by the reporters above:

| Symbol | Package | Purpose |
| ------ | ------- | ------- |
| `bqschema.Schema()` / `bqschema.SchemaJSON()` / `bqschema.EnsureTable()` | `go.alis.build/evals/report/bqschema` | Canonical BigQuery schema for `evalspb.Run` rows and a table-provisioning helper. |

JSON (pubsub) and streaming-insert (bigquery) paths converge on `bqschema.Schema()`.

On Alis Build, evals infrastructure lives in the **product** GCP project
(`ALIS_OS_PRODUCT_PROJECT`): topic `alis.evals.v1.Run`, dataset `evals`,
table `runs`, and a Pub/Sub → BigQuery subscription. Environment Terraform
grants the env `alis-build` service account `pubsub.publisher` on the product
topic. The Pub/Sub reporter resolves its project from `ALIS_OS_PRODUCT_PROJECT`
automatically; pass the same value as `projectID` when using the BigQuery
reporter directly.

`Run.google_project_id` records where the eval **ran** — set
[`mapper.SetConfig`](../../mapper/config.go) from `ALIS_OS_PROJECT` at
bootstrap. Do not confuse that field with the product project used for sinks.

# Wiring

```go
import (
    "context"
    "errors"
    "io"

    "cloud.google.com/go/bigquery"
    "go.alis.build/evals/report"
    bqschema "go.alis.build/evals/report/bqschema"
    bqreport "go.alis.build/evals/report/bigquery"
    logreport "go.alis.build/evals/report/log"
    pubsubreport "go.alis.build/evals/report/pubsub"
)

type multiCloser struct{ closers []io.Closer }

func (m multiCloser) Close() error {
    var err error
    for _, c := range m.closers {
        if c == nil {
            continue
        }
        err = errors.Join(err, c.Close())
    }
    return err
}

func setupReporters(ctx context.Context, bqClient *bigquery.Client, datasetID, tableID string) (io.Closer, error) {
    // bqClient must target ALIS_OS_PRODUCT_PROJECT (not ALIS_OS_PROJECT).
    if err := bqschema.EnsureTable(ctx, bqClient, datasetID, tableID); err != nil {
        return nil, err
    }
    bq, err := bqreport.NewWithClient(ctx, bqClient, datasetID, tableID)
    if err != nil {
        return nil, err
    }
    ps, err := pubsubreport.New(ctx)
    if err != nil {
        _ = bq.Close()
        return nil, err
    }
    services.TestServiceServer.Reporter = report.All{
        logreport.Reporter{},
        bq,
        ps,
    }
    return multiCloser{closers: []io.Closer{bq, ps}}, nil
}
```

Call `Close()` on the returned helper during server drain.

# Related

* [`report-bqschema`](/packages/report-bqschema.md)
* [`report-pubsub`](/packages/report-pubsub.md)
* [`report-bigquery`](/packages/report-bigquery.md)
* [Troubleshooting — reporter friction](/operations/troubleshooting.md)
