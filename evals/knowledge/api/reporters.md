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

| Type | Purpose |
| ---- | ------- |
| `report.LogReporter{}` | Default. Emits a one-line summary via `alog` — `Info` for `PASSED` runs, `Warn` for anything else. Nil-safe on nil runs. |
| `report.NoOpReporter{}` | Discard. Useful for local tests where you want the LRO to complete without emitting anything. |
| `report.MultiReporter{…}` | Fan out to multiple reporters, in order. First error aborts the fan-out. |

# Wiring

```go
services.TestServiceServer.Reporter = report.MultiReporter{
    report.LogReporter{},
    myPubSubReporter{topic: "eval-runs"},
    myBigQueryReporter{table: "runs"},
}
```

# Contract for custom reporters

1. Handle `run == nil` as a no-op.
2. Do not block the LRO goroutine for long — persist async or with a
   short timeout.
3. Errors are best-effort. Returning one is logged by the caller;
   subsequent reporters in a `MultiReporter` are skipped.

# Minimal Pub/Sub implementation

```go
type PubSubReporter struct {
    Topic *pubsub.Topic
}

func (r *PubSubReporter) ReportRun(ctx context.Context, run *evalspb.Run) error {
    if run == nil {
        return nil
    }
    b, err := proto.Marshal(run)
    if err != nil {
        return err
    }
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    _, err = r.Topic.Publish(ctx, &pubsub.Message{Data: b}).Get(ctx)
    return err
}
```

# Related

* [Reporter concept](/concepts/reporter.md)
* [`report` package](/packages/report.md)
* [Run wire envelope](/wire-types/run.md)

# Citations

[1] [report/report.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/report/report.go)
[2] [report/log.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/report/log.go)
