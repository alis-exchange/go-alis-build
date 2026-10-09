---
title: infra observation suites
description: Infra observation builder and loadinfra helper usage.
tags: [infra-observation, loadinfra]
---

# Infra observation suites

Infra observation cases receive `*evals.InfraObservationResult`.

Use `evals/loadinfra` to collect Cloud Run and Spanner Monitoring snapshots, or
add protobuf snapshots directly:

```go
suite := evals.NewInfraObservationSuite("checkout-runtime").
    AddCase("peak-window", func(ctx context.Context, r *evals.InfraObservationResult) {
        obs, err := loadinfra.ObserveLookback(ctx, client, loadinfra.Targets{
            CloudRun: cloud,
            Spanner:  spanner,
        }, 30*time.Minute)
        if err != nil {
            r.Fail(err)
            return
        }
        obs.RecordTo(r)
    })
```

A snapshot whose fetch failed (`UNAVAILABLE`, `PERMISSION_DENIED` or
`TIMEOUT`) fails the case without adding an extra validation row.
`r.Fail(err)` remains available for ordinary Go errors that the developer
wants represented as case failure.

Added protobuf snapshots and SLO checks are cloned. The observation window is
a singleton: the first `SetWindow` wins; duplicate window calls and nil
protobuf values fail the case while retaining existing data. `Fail(nil)` is a
no-op. `obs.RecordTo(r)` sets the window from obs.Window with lookback
`End - Start`, so do not also call `SetWindow`.

An empty observation result is `NOT_EVALUATED`. Failed infra SLO checks, broken
validations, failed-fetch snapshots, or `Fail(err)` fail the case while
preserving partial results.

For load-integrated diagnostics, use `ObserveLoad(ctx, client, targets,
metrics)`. Advanced callers can provide an explicit `loadinfra.Request` to
`Observe`; its named fields expose custom windows and target concurrency;
`ExtendQueryEnd` is deprecated and ignored.
