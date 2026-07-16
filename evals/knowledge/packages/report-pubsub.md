---
type: Go Package
title: package evals/report/pubsub
description: JSON Pub/Sub reporter for completed evalspb.Run rows.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/report/pubsub
tags: [package, report, pubsub, reporter, json]
timestamp: 2026-07-09T00:00:00Z
---

# Role

`evals/report/pubsub` publishes each completed `evalspb.Run` as JSON to
Pub/Sub using `protojson` (`UseProtoNames=true`, `EmitUnpopulated=true`).
The payload is a bare `Run` — no `RunPublishedEvent` envelope — targeting
Pub/Sub → BigQuery **table-schema** subscriptions.

Default topic: `alis.evals.v1.Run`. On Alis Build products this topic
(and its Pub/Sub → BigQuery subscription) is provisioned in the **product
GCP project** via Terraform — unlike `*Event`-suffixed messages it is not
created by the define step. Outside Alis Build, provision the topic and
subscription yourself.

# Usage

```go
import pubsubreport "go.alis.build/evals/report/pubsub"

// Resolves project from ALIS_OS_PRODUCT_PROJECT (the product project where
// the topic lives, not ALIS_OS_PROJECT where the neuron runs).
ps, err := pubsubreport.New(ctx)
```

Fan out with other reporters:

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
| `WithProject(id)` | Override project (defaults to `ALIS_OS_PRODUCT_PROJECT`). Only with `New`. |
| `WithTopic(name)` | Override default topic `alis.evals.v1.Run`. |
| `WithOrderingKey(k)` | Set Pub/Sub ordering key; enables message ordering. |
| `WithBackground()` | Fire-and-forget — do not wait for broker ack. |
| `WithPublishTimeout(d)` | Bound sync-mode publish (default 10s). |

# BigQuery path

Provision the target table with [`report/bqschema`](/packages/report-bqschema.md)
(`SchemaJSON` + Terraform, or `EnsureTable` at bootstrap). Attach a Pub/Sub →
BigQuery subscription in **table-schema** mode. On Alis Build, product
Terraform provisions `evals.runs` and the subscription; environment Terraform
grants the env `alis-build` service account `pubsub.publisher` on the product
topic.

# Related

* [`report-bqschema`](/packages/report-bqschema.md)
* [`report-bigquery`](/packages/report-bigquery.md)
* [Reporters API](/api/reporters.md)
