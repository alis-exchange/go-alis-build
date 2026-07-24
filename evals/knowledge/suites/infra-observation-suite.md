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
        r.SetWindow(30*time.Minute, window.Start, window.End)
        obs, err := loadinfra.Observe(ctx, client, cloud, spanner, window, false, 1)
        if err != nil {
            r.Fail(err)
            return
        }
        for _, snapshot := range obs.CloudRun {
            r.AddCloudRunSnapshot(snapshot)
        }
        for _, snapshot := range obs.Spanner {
            r.AddSpannerSnapshot(snapshot)
        }
    })
```

An unavailable target snapshot fails the case without adding an extra
validation row. `r.Fail(err)` remains available for ordinary Go errors that
the developer wants represented as case failure.

Added protobuf snapshots and SLO checks are cloned. The observation window is
a singleton: the first `SetWindow` wins; duplicate window calls and nil
protobuf values fail the case while retaining existing data. `Fail(nil)` is a
no-op.

An empty observation result is `NOT_EVALUATED`. Failed infra SLO checks, broken
validations, unavailable snapshots, or `Fail(err)` fail the case while
preserving partial results.
