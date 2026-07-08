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
  request timeout. Re-exported by `evals` as `evals.Profile`.
- `loadgen.Target` — `func(ctx context.Context) error`. Re-exported
  by `evals` as `evals.Target`.
- `loadgen.Generator` — runs a target under a profile and produces a
  `Metrics` aggregate.
- `loadgen.Metrics` — aggregate result: latency percentiles, error
  counts, actual QPS.
- `loadgen.Pacer` — fixed-rate ticker used internally.

# Files

| File | Purpose |
| ---- | ------- |
| `profile.go` | `Profile` struct, validation. |
| `generator.go` | Worker orchestration, HDR histogram integration. |
| `metrics.go` | `Metrics` aggregate and percentile computation. |
| `pacer.go` | Fixed-rate pacer used by the generator. |
| `doc.go` | Package documentation. |
| `*_test.go` | Tests. |

# Saturation

When the worker pool cannot keep up with the target rate, the observed
`actual_qps` falls below `target_qps`. The generator emits an
`alog.Warnf` at `actual_qps < 0.9 × target_qps` so the operator
notices they are measuring the generator, not the SUT.

# Related

* [Load suite](/suites/load-suite.md)
* [Load profile fields](/api/load-profile.md)
* [Load mode presets](/operations/load-mode-presets.md)

# Citations

[1] [evals/loadgen tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/loadgen)
