---
type: Go Package
title: package evals/report
description: The `Reporter` interface plus `NoOpReporter` and `MultiReporter`. Concrete sinks live in subpackages.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/report
tags: [package, report, reporter]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/report` defines the sink interface and generic combinators.
Concrete reporters live in subpackages so heavyweight dependencies are
not pulled in by callers that only need the interface:

* [`evals/report/log`](/packages/report-log.md) — default one-line alog summary
* [`evals/report/bigquery`](/packages/report-bigquery.md) — streaming insert to BigQuery

Consumers wire a reporter (or a `MultiReporter`) to
`TestServiceServer.Reporter` at process start.

# Public surface

```go
type Reporter interface {
    ReportRun(ctx context.Context, run *evalspb.Run) error
}

type NoOpReporter struct{}    // discard
type MultiReporter []Reporter // fan-out
```

# Files

| File | Purpose |
| ---- | ------- |
| `report.go` | Interface and `NoOpReporter`, `MultiReporter`. |
| `doc.go` | Package documentation. |
| `report_test.go` | Tests. |
| `log/` | Default log reporter (`log.Reporter`). |
| `bigquery/` | BigQuery reporter (`bigquery.Reporter`) + `InferSchema`. |

# Related

* [Reporter concept](/concepts/reporter.md)
* [Reporters API](/api/reporters.md)
* [`report/log`](/packages/report-log.md)
* [`report/bigquery`](/packages/report-bigquery.md)

# Citations

[1] [evals/report tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/report)
