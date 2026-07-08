---
type: Go Package
title: package evals/report/bigquery
description: BigQuery streaming reporter for completed Run protos.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/report/bigquery
tags: [package, report, bigquery, reporter]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/report/bigquery` streams each completed `evalspb.Run` to a
pre-existing BigQuery table using
[go.einride.tech/protobuf-bigquery](https://github.com/einride/protobuf-bigquery-go).

# Usage

```go
import bqreport "go.alis.build/evals/report/bigquery"

r, err := bqreport.New(ctx, projectID, datasetID, tableID)
if err != nil { ... }
defer r.Close()
```

# Schema provisioning

Two options:

**External (default).** Provision the table with Terraform / `bq mk` at deploy time:

```go
schemaJSON, err := bqreport.InferSchema().ToJSONFields()
// bq mk --table PROJECT:dataset.table schema.json
```

**Framework-managed.** Let the reporter create and additively update the table on construction:

```go
r, err := bqreport.New(ctx, projectID, "evals", "runs",
    bqreport.WithAutoCreateTable(bigquery.TableMetadata{
        TimePartitioning: &bigquery.TimePartitioning{
            Field: "start_time",
            Type:  bigquery.DayPartitioningType,
        },
    }),
)
```

The dataset must exist either way. BigQuery enforces additive-only schema updates server-side, so renames, drops, and type changes on an existing table fail loudly at construction.

# Related

* [`report` package](/packages/report.md)
* [Reporters API](/api/reporters.md)
