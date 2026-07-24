---
title: load suites
description: Load suite builder and helper usage.
tags: [load, loadgen]
---

# Load suites

Load cases receive `*evals.LoadResult`.

The result builder accepts protobuf-native summaries, SLO checks, tags, and
diagnostic snapshots:

```go
suite := evals.NewLoadSuite("checkout-capacity").
    AddCase("steady-traffic", func(ctx context.Context, r *evals.LoadResult) {
        profile := loadgen.Profile{QPS: 100, Concurrency: 25, Duration: time.Minute}
        metrics, err := loadgen.New().Run(ctx, profile, target)
        if err != nil {
            r.Fail(err)
            return
        }
        r.SetSummary(loadgen.Summary(mode, profile, metrics))
        r.AddSLOCheck(checkProto)
    })
```

Default concurrency is one active case. `WithMaxConcurrency` also applies to
load suites, but parallel load cases combine traffic and can distort
measurements. Use it only when combined traffic is intentional.

The builder also accepts tags, Cloud Run/Spanner snapshots, infra SLO checks,
and general validation rules. Added protobuf messages are cloned. `SetSummary`
is a singleton: the first value wins; duplicate or nil values fail the case
without discarding existing data. `Fail(nil)` is a no-op.

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
