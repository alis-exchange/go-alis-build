---
type: Package Reference
title: loadinfra
description: Cloud Monitoring fetch boundary for load-integrated and standalone infra observation.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/loadinfra/doc.go
tags: [package, loadinfra, monitoring, infra]
timestamp: 2026-07-16T00:00:00Z
---

# Overview

`go.alis.build/evals/loadinfra` fetches Cloud Run and Spanner server-side
metrics from Cloud Monitoring. Both load suites and infra-observe suites
share the same `Observe` entry point.

# Key types

| Type | Role |
| ---- | ---- |
| `CloudRunTarget` / `SpannerTarget` | Declared on suites; validated at construction |
| `ObservationWindow` | Inclusive-start, exclusive-end interval attached to snapshots |
| `MetricClient` | Query boundary; production uses `NewMetricClient`, tests use `FakeMetricClient` |
| `ObserveResult` | Per-target Cloud Run and Spanner snapshots for one window |

# Windowing

- **Load-integrated:** `WindowFromMetrics` derives the window from loadgen
  measurement timestamps (warmup excluded). Query intervals extend forward
  via `CloudRunQueryWindow` / `SpannerQueryWindow`.
- **Standalone:** `WindowLookback` settles the window (`window_end = now - settle`)
  before fetching so recently ingested data is visible.

# Fetch semantics (v1)

- Per-target failures are recorded on `FetchStatus` / `FetchMessage`; they do
  not fail the parent case.
- Partial metric gaps within a target still yield OK with a partial-failure
  message.
- Configuration failures (unset lookback, missing client) produce a synthetic
  `_evals` snapshot via `ConfigFailureSnapshot`.

# Related

* [Infra observe suite](/suites/infra-observe-suite.md)
* [Load suite options — infra targets](/api/load-suite-options.md)
* [`runner` package](/packages/runner.md)

# Citations

[1] [loadinfra/doc.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/loadinfra/doc.go)
