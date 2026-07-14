---
type: Go Package
title: package evals/loadgen
description: The embedded load generator — Profile, Pacer, Generator, Metrics.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/loadgen
tags: [package, loadgen, load, generator]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/loadgen` implements the load generator that `NewLoadSuite`
cases run under. It owns the pacer, worker pool, latency histogram,
and error tallies.

# Public surface

- `loadgen.Profile` — target QPS, concurrency, duration, warmup, per-
  request timeout, optional QPS/concurrency stages, graceful ramp-down.
  Re-exported by `evals` as `evals.Profile`.
- `loadgen.Stage` — one step in a staged profile (`Duration`, `Target`).
- `loadgen.ResultTarget` — `func(ctx, CallData) TargetResult` with
  separate transport and check errors.
- `loadgen.TransportTarget` — adapts `func(ctx) error` to `ResultTarget`.
- `loadgen.Metrics.Stream` — aggregate stream histograms when targets
  return `StreamSample` data.
- `Profile.AbortCheck` / `AbortOnSLOFailure` context — mid-run cancel
  when partial metrics breach SLOs.
- `loadgen.CallData` — per-request context (`RequestNumber`, `WorkerID`, `Data`).
- `loadgen.Generator` — runs a target under a profile and produces a
  `Metrics` aggregate.
- `loadgen.Metrics` — aggregate result: latency percentiles, transport
  errors, check pass/fail counts, dropped ticks, actual QPS.
- `loadgen.Pacer` — constant, step-stage, or linear-stage pacing.

# Files

| File | Purpose |
| ---- | ------- |
| `profile.go` | `Profile`, `Stage`, validation. |
| `abort_context.go` | Context marker for abort-on-SLO-failure. |
| `target.go` | `CallData`, `TargetResult`, `ResultTarget`, `TransportTarget`. |
| `generator.go` | Worker orchestration, HDR histogram integration. |
| `metrics.go` | `Metrics` aggregate and percentile computation. |
| `pacer.go` | Constant, step-stage, and linear-stage pacers. |
| `doc.go` | Package documentation. |
| `*_test.go` | Tests. |

# Saturation

When the worker pool cannot keep up with the target rate, the observed
`actual_qps` falls below `target_qps` and `dropped_count` increments for
ticks that could not be dispatched. The generator emits an
`alog.Warnf` at `actual_qps < 0.9 × target_qps` so the operator
notices they are measuring the generator, not the SUT.

# Related

* [Load suite](/suites/load-suite.md)
* [Load profile fields](/api/load-profile.md)
* [Load mode presets](/operations/load-mode-presets.md)

# Citations

[1] [evals/loadgen tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/loadgen)
