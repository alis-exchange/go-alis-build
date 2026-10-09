---
title: load suites
description: Load suite builder and helper usage.
tags: [load, loadgen]
---

# Load suites

Load cases receive `*evals.LoadResult`.

The result builder accepts protobuf-native summaries, SLO checks, and
diagnostic snapshots. Tags use ordinary key/value strings:

```go
suite := evals.NewLoadSuite("checkout-capacity").
    AddCase("steady-traffic", func(ctx context.Context, r *evals.LoadResult) {
        profile := loadgen.Profile{QPS: 100, Concurrency: 25, Duration: time.Minute}
        metrics, err := loadgen.Run(ctx, profile, target)
        if err != nil {
            r.Fail(err)
            return
        }
        r.SetSummary(loadgen.Summary(loadgen.Moderate, profile, metrics))
        r.AddSLOCheck(checkProto)
        r.AddTag("rpc", "CreateOrder")
    })
```

Default concurrency is one active case. `WithMaxConcurrency` also applies to
load suites, but parallel load cases combine traffic and can distort
measurements. Use it only when combined traffic is intentional.

For load-integrated infrastructure diagnostics, wait until Monitoring has
settled the measurement window, then observe it and record the snapshots on the case:

```go
settle := loadinfra.SpannerSettlePadding // CloudRunSettlePadding if only Cloud Run targets
if wait := time.Until(metrics.MeasurementEnd.Add(settle)); wait > 0 {
    timer := time.NewTimer(wait)
    select {
    case <-ctx.Done():
        timer.Stop()
        r.Fail(ctx.Err())
        return
    case <-timer.C:
    }
}
observed, err := loadinfra.ObserveLoad(ctx, metricClient, targets, metrics)
if err != nil {
    r.Fail(err)
    return
}
observed.RecordTo(r)
```

`RecordTo` adds every observed snapshot to the builder. Both
`*evals.LoadResult` and `*evals.InfraObservationResult` satisfy
`loadinfra.SnapshotRecorder`.

`ObserveLoad` requires non-nil metrics and queries the measurement window
rounded out to whole minutes. It does not wait; a call before the settle
time returns at once and each snapshot's `FetchMessage` says the data may be
incomplete. Use `ObserveLookback` for standalone settled windows or
`Observe` with a named `loadinfra.Request` for a custom window and target
concurrency.

`AddTagProto` is available when a caller already has the generated tag message.
The builder also accepts Cloud Run/Spanner snapshots, infra SLO checks, and
general validation rules. Added protobuf messages are cloned. `SetSummary` is a
singleton: the first value wins; duplicate or nil values fail the case without
discarding existing data. `Fail(nil)` is a no-op.

There are no framework `SLO*` constructors. Evaluate thresholds in case code
and add protobuf-native `LoadTestResults.SloCheck` or `InfraSloCheck` values.

For client-streaming load targets, copy `evals.CallClientStream` timing into
`loadgen.TargetResult.Stream`:

```go
got := evals.CallClientStream(ctx, openStream, sendRequests)
return loadgen.TargetResult{
    TransportErr: got.Err,
    Stream: &loadgen.StreamSample{
        SendDuration:    got.SendDuration,
        ResponseLatency: got.ResponseLatency,
        TotalDuration:   got.TotalDuration,
        MessagesSent:    got.MessagesSent,
    },
}
```

An empty load result is `NOT_EVALUATED`. Failed SLO/infra checks, broken
validations, or `Fail(err)` fail the case while preserving partial results.
