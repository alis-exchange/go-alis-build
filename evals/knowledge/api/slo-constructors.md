---
type: API Reference
title: SLO constructors
description: Constructors that produce one `SLO` value each. Every SLO produces one `SloCheck` per case run.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/slo.go
tags: [api, slo, load]
timestamp: 2026-07-08T00:00:00Z
---

# Constructors

Each constructor produces one `SLO` value; each SLO produces one
`SloCheck` per case run (passed or failed).

| Constructor | Check id | Unit | Passes when | Notes |
| ----------- | -------- | ---- | ----------- | ----- |
| `evals.SLOLatencyP50(max time.Duration)` | `latency.p50_ms` | `ms` | `observed <= limit` | Median latency. |
| `evals.SLOLatencyP95(max time.Duration)` | `latency.p95_ms` | `ms` | `observed <= limit` | 95th percentile. |
| `evals.SLOLatencyP99(max time.Duration)` | `latency.p99_ms` | `ms` | `observed <= limit` | 99th percentile — the usual tail-latency guardrail. |
| `evals.SLOErrorRate(maxFraction float64)` | `error_rate` | `%` | `observed <= limit` | Constructor accepts a fraction (`0.01` for 1%). Observed and limit are recorded as percent so wire values match human intuition. |
| `evals.SLOMinQPS(min float64)` | `actual_qps` | `rps` | `observed >= limit` | Throughput floor — useful for detecting silent capacity regressions. |

# `SLO` type

`SLO` is an opaque struct constructed only by the exported constructors
above. Custom SLOs are not currently supported through the public
API; extend by adding a constructor to the framework or by evaluating
the raw `LoadTestResults.Summary` downstream.

# Wire mapping

Every SLO surfaces one `SloCheck`:

```protobuf
message SloCheck {
  string id       = 1;
  Status status   = 2;
  string message  = 3;
  double observed = 4;
  double limit    = 5;
  string unit     = 6;
}
```

Note that:

- `latency.p*_ms` records **milliseconds**, not the raw `time.Duration`.
- `error_rate` records **percent** — a constructor argument of `0.01`
  produces `limit = 1.0` on the wire.
- `actual_qps` records **requests per second**.

# Related

* [Load suite](/suites/load-suite.md)
* [Load results wire type](/wire-types/load-results.md)
* [Load profile fields](/api/load-profile.md)

# Citations

[1] [slo.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/slo.go)
[2] [slo_test.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/slo_test.go)
