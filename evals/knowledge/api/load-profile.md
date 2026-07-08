---
type: API Reference
title: Load profile fields
description: The `Profile` struct — target QPS, concurrency, duration, warmup, request timeout.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/loadgen/profile.go
tags: [api, load, profile]
timestamp: 2026-07-08T00:00:00Z
---

# `evals.Profile`

`evals.Profile` re-exports `loadgen.Profile`. Fields:

| Field | Type | Meaning |
| ----- | ---- | ------- |
| `QPS` | `float64` | Target requests per second. Must be > 0. |
| `Concurrency` | `int` | Number of worker goroutines. Must be ≥ 1. Sized to keep enough requests in flight for the target rate at the target's expected latency (Little's law). |
| `Duration` | `time.Duration` | The measurement window. Must be > 0. |
| `Warmup` | `time.Duration` | Traffic runs at target rate for this leading period but the samples are dropped. Zero disables warmup. |
| `RequestTimeout` | `time.Duration` | Per-request `context.WithTimeout` cap. Zero → 30 s default. Always further capped by the remaining window so a straggler cannot pollute the next case. |

# Sizing `Concurrency`

Little's law: `in-flight = qps × latency`. For 100 QPS at an expected
200 ms median, keep at least `100 × 0.2 = 20` workers, with headroom
for tail latency. Under-sizing produces `actual_qps < target_qps`
and a saturation warning; over-sizing wastes goroutines but does not
distort the measurement.

# Warmup

Warmup samples are **discarded**. Latency-sensitive services with
cold caches, JIT-compiled runtimes, or autoscalers should use a
non-zero warmup so the measurement window reflects steady state.

Framework defaults use 2 – 20 s depending on mode; see
[Load mode presets](/operations/load-mode-presets.md).

# `RequestTimeout`

The framework wraps every request in `context.WithTimeout`. The
effective timeout is `min(RequestTimeout, remaining window)` — a
straggler that runs past the case's end time is cancelled so the next
case's measurement is not contaminated.

# Related

* [Load suite](/suites/load-suite.md)
* [SLO constructors](/api/slo-constructors.md)
* [Load mode presets](/operations/load-mode-presets.md)
* [`loadgen` package](/packages/loadgen.md)

# Citations

[1] [loadgen/profile.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/loadgen/profile.go)
[2] [load_profile.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/load_profile.go)
