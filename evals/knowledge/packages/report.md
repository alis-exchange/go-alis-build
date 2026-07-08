---
type: Go Package
title: package evals/report
description: The `Reporter` interface plus `LogReporter`, `NoOpReporter`, and `MultiReporter`.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/report
tags: [package, report, reporter]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/report` defines the sink interface and bundles a small set of
implementations. Consumers wire a reporter (or a `MultiReporter`) to
`TestServiceServer.Reporter` at process start.

# Public surface

```go
type Reporter interface {
    ReportRun(ctx context.Context, run *evalspb.Run) error
}

type LogReporter struct{}     // default
type NoOpReporter struct{}    // discard
type MultiReporter []Reporter // fan-out
```

# Files

| File | Purpose |
| ---- | ------- |
| `report.go` | Interface and `NoOpReporter`, `MultiReporter`. |
| `log.go` | `LogReporter` implementation using `alog`. |
| `doc.go` | Package documentation. |
| `log_test.go`, `report_test.go` | Tests. |

# Related

* [Reporter concept](/concepts/reporter.md)
* [Reporters API](/api/reporters.md)

# Citations

[1] [evals/report tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/report)
