---
type: Go Package
title: package evals/report/pubsub
description: Pub/Sub reporter — publishes RunPublishedEvent envelopes via go.alis.build/events.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/report/pubsub
tags: [package, report, pubsub, events, reporter]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/report/pubsub` implements a `report.Reporter` that wraps each
completed `evalspb.Run` in an `alis.evals.v1.RunPublishedEvent`
envelope and publishes it to Pub/Sub via
[`go.alis.build/events`](https://pkg.go.dev/go.alis.build/events).

The default topic is the event message's proto full name
(`alis.evals.v1.RunPublishedEvent`) — the same string the Alis Build
platform provisions when defining the message, so a bare `New(ctx)`
Just Works in a standard neuron.

# Usage

```go
import (
    "context"

    pubsubreport "go.alis.build/evals/report/pubsub"
)

func setupReporter(ctx context.Context) (*pubsubreport.Reporter, error) {
    ps, err := pubsubreport.New(ctx)
    if err != nil {
        return nil, err
    }
    services.TestServiceServer.Reporter = ps
    return ps, nil // Close() at server drain
}
```

Fan out alongside other reporters:

```go
services.TestServiceServer.Reporter = report.MultiReporter{
    logreport.Reporter{},
    bq,
    ps,
}
```

# Options

| Option | Effect |
| ------ | ------ |
| `WithProject(id)` | Override the Google Cloud project (defaults to `ALIS_OS_PROJECT`). Only valid with `New`. |
| `WithTopic(name)` | Override the default topic. Accepts a bare topic ID or a fully-qualified `projects/<p>/topics/<t>` resource string. |
| `WithOrderingKey(k)` | Apply the Pub/Sub ordering key on every message. |
| `WithBackground()` | Fire-and-forget publishing — do not wait for the broker to ack. Best-effort delivery only. |
| `WithPublishTimeout(d)` | Bound each publish via `context.WithTimeout`. Defaults to 10s. |

# Delivery semantics

By default `ReportRun` blocks until the Pub/Sub broker acks the
message. This is the safer choice for short-lived eval processes
that may exit right after completing a run. `WithBackground()`
switches to fire-and-forget for latency-sensitive callers that can
absorb occasional loss.

The reporter is nil-safe on nil runs and on a nil receiver.

# Client ownership

- `New(ctx, opts...)` constructs a `*events.Client` internally and
  closes it on `Close`.
- `NewWithClient(client, opts...)` borrows an existing
  `*events.Client`; `Close` is a no-op. Passing `WithProject` here
  returns an error since the borrowed client already has a project
  bound.

# Pub/Sub → BigQuery

Pub/Sub can attach a BigQuery subscription that writes messages
directly into a table, but the direct-subscription proto path has
two constraints that make it awkward for `Run`:

1. Pub/Sub → BigQuery marks proto `oneof` fields as **unmappable**.
   `Run` has `oneof data { integration_test, load_test, agent_eval }`,
   so a "Use topic schema" subscription against this topic fails
   schema compatibility.
2. Pub/Sub → BigQuery does not unwrap well-known types the way the
   in-process BigQuery reporter does — Timestamps land as
   `RECORD<seconds, nanos>` rather than `TIMESTAMP`, etc.

If you want a queryable BigQuery table whose shape matches the
in-process reporter's output, run a small subscriber that
unmarshals `RunPublishedEvent` and forwards `evt.Run` to
`bqreport.Reporter`. If you only need a raw archive of published
events, use a "Don't use a schema" subscription with a single
`data BYTES` column.

# Related

* [`report` package](/packages/report.md)
* [`report/bigquery` package](/packages/report-bigquery.md)
* [Reporters API](/api/reporters.md)
* [Reporter concept](/concepts/reporter.md)
