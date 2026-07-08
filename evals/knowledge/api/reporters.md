---
type: API Reference
title: Reporters
description: The `report.Reporter` interface and bundled implementations.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/report/report.go
tags: [api, reporter, sink]
timestamp: 2026-07-08T00:00:00Z
---

# Interface

```go
package report

type Reporter interface {
    ReportRun(ctx context.Context, run *evalspb.Run) error
}
```

Reporters receive one `evalspb.Run` per completed suite. Wire your
sink onto `TestServiceServer.Reporter` at process start. `nil`
disables reporting entirely.

# Bundled implementations

| Type | Package | Purpose |
| ---- | ------- | ------- |
| `log.Reporter{}` | `go.alis.build/evals/report/log` | Default. Emits a one-line summary via `alog` — `Info` for `PASSED` runs, `Warn` for anything else. Nil-safe on nil runs. |
| `bigquery.Reporter` | `go.alis.build/evals/report/bigquery` | Streams each Run to a pre-existing BigQuery table via protobq. |
| `pubsub.Reporter` | `go.alis.build/evals/report/pubsub` | Wraps each Run in a `RunPublishedEvent` envelope and publishes it to Pub/Sub via [go.alis.build/events](https://pkg.go.dev/go.alis.build/events). |
| `report.NoOpReporter{}` | `go.alis.build/evals/report` | Discard. Useful for local tests where you want the LRO to complete without emitting anything. |
| `report.MultiReporter{…}` | `go.alis.build/evals/report` | Fan out to multiple reporters, in order. First error aborts the fan-out. |

# Wiring

```go
import (
    "context"

    "go.alis.build/evals/report"
    logreport "go.alis.build/evals/report/log"
    bqreport "go.alis.build/evals/report/bigquery"
    pubsubreport "go.alis.build/evals/report/pubsub"
)

func setupReporters(ctx context.Context, projectID string) error {
    bq, err := bqreport.New(ctx, projectID, "evals", "runs")
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

# BigQuery reporter

The BigQuery reporter writes each completed `Run` to a pre-existing table whose schema matches `bqreport.InferSchema()`. Provision the table at deploy time:

```go
schemaJSON, err := bqreport.InferSchema().ToJSONFields()
// write schemaJSON to schema.json, then: bq mk --table PROJECT:evals.runs schema.json
```

Or opt into framework-managed provisioning with `bqreport.WithAutoCreateTable(...)`: on construction the reporter creates the table if it is missing (with any `bigquery.TableMetadata` you supply for partitioning / clustering / expiration), or applies an additive schema update if the table already exists. The dataset must exist in either case.

Each row uses `run.name` as the BigQuery insert ID for best-effort deduplication. Inserts are bounded by a 10s timeout (`bqreport.WithInsertTimeout`).

# Pub/Sub reporter

The Pub/Sub reporter wraps each completed `Run` in an
`alis.evals.v1.RunPublishedEvent` envelope and publishes it via
[`go.alis.build/events`](https://pkg.go.dev/go.alis.build/events).
The default topic is the event message's proto full name
(`alis.evals.v1.RunPublishedEvent`) — the same string the Alis
Build platform provisions when defining the message, so `New(ctx)`
is enough in a standard neuron:

```go
func setupPubsub(ctx context.Context) (*pubsubreport.Reporter, error) {
    ps, err := pubsubreport.New(ctx)
    if err != nil {
        return nil, err
    }
    services.TestServiceServer.Reporter = ps
    return ps, nil // Close() at server drain
}
```

Options: `WithProject`, `WithTopic`, `WithOrderingKey`,
`WithBackground` (fire-and-forget), `WithPublishTimeout`
(defaults to 10s). By default `ReportRun` blocks on the broker
ack — the safer choice for short-lived eval processes.
`pubsubreport.New` owns its underlying `*events.Client` and closes
it on `Close`; `pubsubreport.NewWithClient` borrows one.

**Pub/Sub → BigQuery.** Pub/Sub can attach a BigQuery
subscription that writes messages directly into a table, but note
two constraints of the direct-subscription proto path:

1. Pub/Sub → BigQuery marks proto `oneof` fields as **unmappable**.
   `Run` has `oneof data { integration_test, load_test, agent_eval }`,
   so a "Use topic schema" subscription fails schema compatibility.
2. Pub/Sub → BigQuery does not unwrap well-known types the way the
   in-process BigQuery reporter does — Timestamps land as
   `RECORD<seconds, nanos>` rather than `TIMESTAMP`, etc.

For a queryable BigQuery table matching the in-process reporter's
shape, run a small subscriber that unmarshals `RunPublishedEvent`
and forwards `evt.Run` to `bqreport.Reporter`. For a raw archive
only, use a "Don't use a schema" subscription with a single
`data BYTES` column.

# Contract for custom reporters

1. Handle `run == nil` as a no-op.
2. Do not block the LRO goroutine for long — persist async or with a
   short timeout.
3. Errors are best-effort. Returning one is logged by the caller;
   subsequent reporters in a `MultiReporter` are skipped.

# Minimal custom-reporter example (webhook)

Use the bundled `pubsubreport` / `bqreport` for Pub/Sub and BigQuery.
This webhook sketch is here as a template for wiring up any other
sink:

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

* [Reporter concept](/concepts/reporter.md)
* [`report` package](/packages/report.md)
* [Run wire envelope](/wire-types/run.md)

# Citations

[1] [report/report.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/report/report.go)
[2] [report/log/log.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/report/log/log.go)
[3] [report/bigquery/bigquery.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/report/bigquery/bigquery.go)
[4] [report/pubsub/pubsub.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/report/pubsub/pubsub.go)
