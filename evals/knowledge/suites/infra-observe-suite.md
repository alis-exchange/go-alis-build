---
type: Suite Kind
title: Infra observe suite
description: Standalone Cloud Monitoring snapshots over a configurable lookback window without load generation.
tags: [infra, monitoring, observation]
timestamp: 2026-07-16T00:00:00Z
---

# Purpose

An **infra observe suite** fetches Cloud Run and Spanner server-side
metrics over a settled lookback window. It does not invoke loadgen or
client SLOs. Results map to `Run.Type = INFRA_OBSERVATION`.

# Anatomy

```go
s := evals.MustNewInfraObserveSuite("peak-hours",
    evals.WithLookback(30*time.Minute),
    evals.WithCloudRunTargets(evals.CloudRunTarget{
        ID: "search", Role: evals.RoleEntry,
        ProjectID: "my-project", Region: "europe-west1", ServiceName: "search-v1",
    }),
    evals.WithSpannerTargets(evals.SpannerTarget{
        ID: "orders-db", ProjectID: "my-project",
        InstanceID: "prod", Location: "europe-west1", Database: "orders",
    }),
)
s.MustInfraObserveCase("hourly")
evals.RegisterInfraObserve(s)
```

# Lookback precedence

When `runner.RunInfraObserveSuites` runs:

1. `RunInfraObservationRequest.lookback` (request override)
2. per-case `WithObserveCaseLookback`
3. suite `WithLookback`
4. error if none set

# Targets

At least one [WithCloudRunTargets] or [WithSpannerTargets] declaration is
required; suites with no targets are rejected at construction.

# Concurrency

Cases within a suite run **concurrently** (read-only Monitoring queries).

# Related

* [Load suite infra options](/api/load-suite-options.md)
* [Runner — RunInfraObserveSuites](/packages/runner.md)
