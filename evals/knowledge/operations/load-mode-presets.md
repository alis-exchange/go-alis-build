---
type: Reference
title: Load mode presets
description: The five built-in `Mode` presets exposed on `RunLoadTest.mode`, and when to pick each.
tags: [load, mode, preset, profile]
timestamp: 2026-07-08T00:00:00Z
---

# Presets

`RunLoadTest.mode` picks a framework preset. Suite-level overrides via
[`WithLoadProfile`](/api/load-suite-options.md) replace the default for
that mode only.

| Mode | QPS | Concurrency | Duration | Warmup |
| ---- | --: | ----------: | -------: | -----: |
| `MINIMAL` | 5 | 2 | 15 s | 2 s |
| `CONSERVATIVE` | 25 | 10 | 30 s | 5 s |
| `MODERATE` | 100 | 25 | 60 s | 10 s |
| `HIGH` | 400 | 100 | 120 s | 15 s |
| `LUDICROUS` | 1000 | 250 | 180 s | 20 s |

# When to use each

- **`MINIMAL`** — smoke test the load path in CI. Won't detect
  meaningful regressions but confirms wiring works.
- **`CONSERVATIVE`** — safe default on newly deployed neurons. Enough
  volume to surface obvious latency issues without stressing the
  autoscaler.
- **`MODERATE`** — the usual production check. Detects the majority
  of latency and error-rate regressions.
- **`HIGH`** — capacity check ahead of a launch. Runs long enough to
  hit autoscaler behavior.
- **`LUDICROUS`** — soak testing and headroom validation. Use with
  care against production surfaces.

# Overrides fully replace

`WithLoadProfile(mode, profile)` replaces the framework default for
the specific mode. Other modes keep their defaults. Field-level
merging is not supported — you must supply a complete profile.

# `MODE_UNSPECIFIED`

Passing `MODE_UNSPECIFIED` to the RPC is rejected with
`InvalidArgument`. `DefaultLoadProfile(MODE_UNSPECIFIED)` returns
`(zero, false)`.

# Related

* [Load profile fields](/api/load-profile.md)
* [Load suite](/suites/load-suite.md)
* [Helpers — profile resolution](/api/helpers.md)

# Citations

[1] [README — Mode presets](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#mode-presets)
[2] [load_profile.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/load_profile.go)
