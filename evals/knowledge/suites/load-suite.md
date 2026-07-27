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

For load-integrated Monitoring diagnostics, use the metrics returned by the
generator:

```go
observed, err := loadinfra.ObserveLoad(ctx, metricClient, targets, metrics)
if err != nil {
    r.Fail(err)
    return
}
for _, snapshot := range observed.CloudRun {
    r.AddCloudRunSnapshot(snapshot)
}
for _, snapshot := range observed.Spanner {
    r.AddSpannerSnapshot(snapshot)
}
```

Nil metrics are rejected before any Monitoring query. `ObserveLookback` owns
standalone settle timing; an explicit `loadinfra.Request` supports custom
windows and target concurrency.

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
