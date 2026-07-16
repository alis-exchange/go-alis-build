---
type: Go Package
title: package evals/report/bigquery
description: BigQuery streaming reporter for completed Run protos.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/report/bigquery
tags: [package, report, bigquery, reporter]
timestamp: 2026-07-09T00:00:00Z
---

# Role

`evals/report/bigquery` streams each completed `evalspb.Run` to BigQuery
using [go.einride.tech/protobuf-bigquery](https://github.com/einride/protobuf-bigquery-go)
with Duration values written as protojson-native strings.

Schema inference and table provisioning are delegated to
[`report/bqschema`](/packages/report-bqschema.md).

# Usage

Pass the **product** GCP project ID — on Alis Build use
`ALIS_OS_PRODUCT_PROJECT`, not `ALIS_OS_PROJECT` (the environment where the
neuron runs). Default dataset/table on Alis Build products: `evals` / `runs`.

```go
import (
    "os"

    bqreport "go.alis.build/evals/report/bigquery"
)

r, err := bqreport.New(ctx, os.Getenv("ALIS_OS_PRODUCT_PROJECT"), "evals", "runs")
```

On Alis Build, the platform-provisioned write path from environment neurons
is Pub/Sub → BigQuery (see [`report/pubsub`](/packages/report-pubsub.md)).
Direct streaming inserts require `roles/bigquery.dataEditor` on the product
dataset, which is not granted to environment service accounts by default.

`WithAutoCreateTable` delegates to `bqschema.EnsureTable`.

`WithSchemaOptions` still exists but controls **row marshaling only**, not
schema inference — the schema is always sourced from `bqschema.Schema()` so
`bqreport` and Pub/Sub → BigQuery subscriptions stay aligned.

# Related

* [`report-bqschema`](/packages/report-bqschema.md)
* [`report-pubsub`](/packages/report-pubsub.md)
* [Reporters API](/api/reporters.md)
