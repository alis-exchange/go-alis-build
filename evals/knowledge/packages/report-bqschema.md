---
type: Go Package
title: package evals/report/bqschema
description: Shared BigQuery schema and table provisioning for evalspb.Run rows.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/report/bqschema
tags: [package, report, bigquery, schema, bqschema]
timestamp: 2026-07-09T00:00:00Z
---

# Role

`evals/report/bqschema` is the single source of truth for the BigQuery
schema of `evalspb.Run` rows and for table provisioning. Both
`evals/report/pubsub` (JSON via protojson) and `evals/report/bigquery`
(streaming inserts) produce rows that match `bqschema.Schema()`.

Use this package when you need:

* the schema definition for Terraform or `bq mk`
* a `bq mk --schema`-compatible JSON file (`SchemaJSON()`)
* in-process table auto-provisioning (`EnsureTable()`)

# Usage

## Terraform / bq mk path

Export the schema and provision the table at deploy time:

```go
schemaJSON, err := bqschema.SchemaJSON()
if err != nil {
    return err
}
// write schemaJSON to schema.json, then:
// bq mk --table PROJECT:evals.runs schema.json
```

Wire a Pub/Sub → BigQuery subscription in **table-schema mode** against
this table. See [`report-pubsub`](/packages/report-pubsub.md) for the
JSON payload contract.

## In-process provisioning

Call `EnsureTable` at bootstrap before constructing reporters:

```go
import (
    "context"

    "cloud.google.com/go/bigquery"
    bqschema "go.alis.build/evals/report/bqschema"
)

func bootstrap(ctx context.Context, client *bigquery.Client) error {
    return bqschema.EnsureTable(ctx, client, "evals", "runs",
        bigquery.TableMetadata{
            TimePartitioning: &bigquery.TimePartitioning{
                Field: "start_time",
                Type:  bigquery.DayPartitioningType,
            },
        },
    )
}
```

The dataset must already exist. `EnsureTable` creates the table if
missing or applies an additive schema update if it already exists.

`evals/report/bigquery`'s `WithAutoCreateTable` option delegates to
`EnsureTable` internally.

# API

| Function | Purpose |
| --- | --- |
| `Schema()` | Returns `bigquery.Schema` for `evalspb.Run` |
| `SchemaJSON()` | Same schema as JSON for `bq mk --schema` / Terraform |
| `EnsureTable(ctx, client, dataset, table, md...)` | Create or additively update the target table |

# Schema notes

* `google.protobuf.Duration` → `STRING` (protojson form `"1.500000000s"`)
* `google.protobuf.Timestamp` → `TIMESTAMP`
* `google.rpc.Status` → `RECORD{code, message}` — `details` omitted
* Oneof arms (`integration_test`, `load_test`, `agent_eval`) are sibling
  nullable RECORD columns at the Run level

# Related

* [`report` package](/packages/report.md)
* [`report-pubsub`](/packages/report-pubsub.md) — JSON reporter
* [`report-bigquery`](/packages/report-bigquery.md) — streaming insert reporter
* [Reporters API](/api/reporters.md)
