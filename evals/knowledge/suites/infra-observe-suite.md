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
if err := evals.RegisterInfraObserve(s); err != nil {
    panic(err)
}
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
`WithInfraObserveStopOnFailure` switches the suite to sequential execution and
marks cases after the first failure `NOT_EVALUATED`. Use
`WithInfraObserveContext` to decorate setup, teardown, and Monitoring requests.

# Case status rollup (standalone)

Standalone infra-observe cases (this suite kind) **fail** when any declared
target snapshot has non-OK `FetchStatus`. Load-integrated infra snapshots
attached to load cases remain diagnostic-only in v1 — fetch failures are
recorded on the snapshot but do not fail the load case.

| Context | Bad infra fetch | Case status |
| --- | --- | --- |
| Standalone infra observe | any target not OK | `FAILED` |
| Load case with infra targets | any target not OK | unchanged (diagnostic) |

# Teardown diagnostics

Infra results have no generic check collection. When suite teardown fails, the
runner marks every case `FAILED` and appends a synthetic
`CloudRunTargetSnapshot` with `id = "_evals.teardown"`, unavailable fetch
status, and the teardown error in `fetch_message`. Consumers should recognize
that reserved ID as framework provenance rather than a declared Cloud Run
target.

# Related

* [Load suite infra options](/api/load-suite-options.md)
* [Runner — RunInfraObserveSuites](/packages/runner.md)
