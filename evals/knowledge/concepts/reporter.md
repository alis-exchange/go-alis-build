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
| `pubsub.Reporter` | `go.alis.build/evals/report/pubsub` | Publishes bare `Run` JSON via `protojson` on top of `cloud.google.com/go/pubsub/v2` (default topic `alis.evals.v1.Run`; resolves project from `ALIS_OS_PRODUCT_PROJECT`). |
| `report.NoOpReporter{}` | `go.alis.build/evals/report` | Discard. Useful in local tests. |
| `report.MultiReporter{...}` | `go.alis.build/evals/report` | Fan-out to multiple reporters, in order. First error aborts. |

# Wiring

```go
import (
    "context"
    "os"

    "go.alis.build/evals/report"
    logreport "go.alis.build/evals/report/log"
    bqreport "go.alis.build/evals/report/bigquery"
    pubsubreport "go.alis.build/evals/report/pubsub"
)

func setupReporters(ctx context.Context) error {
    productProject := os.Getenv("ALIS_OS_PRODUCT_PROJECT")
    bq, err := bqreport.New(ctx, productProject, "evals", "runs")
    if err != nil {
        return err
    }
    ps, err := pubsubreport.New(ctx)
    if err != nil {
        _ = bq.Close()
        return err
    }
    services.TestServiceServer.Reporter = report.MultiReporter{
        logreport.Reporter{},
        bq,
        ps,
    }
    return nil // Close bq and ps at server drain
}
```

# Contract for custom reporters

1. **Handle `run == nil`** as a no-op.
2. **Do not block the LRO goroutine for long** — persist async or with
   a short timeout.
3. **Errors are best-effort.** Returning one is logged by the caller;
   subsequent reporters in a `MultiReporter` are skipped for that run.

# Minimal custom-reporter example

For Pub/Sub, use the bundled
[`evals/report/pubsub`](/packages/report-pubsub.md); for BigQuery,
use [`evals/report/bigquery`](/packages/report-bigquery.md). This
webhook sketch is here as a template for wiring up any other sink:

```go
type WebhookReporter struct {
    URL    string
    Client *http.Client
}

func (r *WebhookReporter) ReportRun(ctx context.Context, run *evalspb.Run) error {
    if run == nil {
        return nil
    }
    body, err := protojson.Marshal(run)
    if err != nil {
        return err
    }
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, r.URL, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := r.Client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        return fmt.Errorf("webhook: %s", resp.Status)
    }
    return nil
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
[4] [report/pubsub/pubsub.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/report/pubsub/pubsub.go)
