---
type: Go Package
title: package evals/report
description: The `Reporter` interface plus `NoOpReporter`, `All`, `FailFast`, and `MultiReporter`. Concrete sinks live in subpackages.
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
* [`evals/report/pubsub`](/packages/report-pubsub.md) — JSON publish to Pub/Sub

Consumers wire a reporter (or `All` / `FailFast`) to
`TestServiceServer.Reporter` at process start.

# Public surface

```go
type Reporter interface {
    ReportRun(ctx context.Context, run *evalspb.Run) error
}

type NoOpReporter struct{}       // discard
type All []Reporter              // fan-out; call every sink
type FailFast []Reporter        // fan-out; stop at first error
type MultiReporter = FailFast    // alias
```

# Files

| File | Purpose |
| ---- | ------- |
| `report.go` | Interface, `NoOpReporter`, `All`, `FailFast`, `MultiReporter`. |
| `doc.go` | Package documentation and wiring examples. |
| `report_test.go` | Fan-out contract tests. |
| `log/` | Default log reporter (`log.Reporter`). |
| `bigquery/` | BigQuery reporter (`bigquery.Reporter`). |
| `pubsub/` | Pub/Sub JSON reporter. |
| `bqschema/` | Shared BigQuery schema (not a Reporter). |

# Related

* [Reporter concept](/concepts/reporter.md)
* [Reporters API](/api/reporters.md)
* [`report/log`](/packages/report-log.md)
* [`report/bigquery`](/packages/report-bigquery.md)

# Citations

[1] [evals/report tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/report)
