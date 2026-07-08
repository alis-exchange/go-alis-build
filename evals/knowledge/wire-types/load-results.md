---
type: Wire Type
title: LoadTestResults
description: Per-case `Summary` (QPS, latency percentiles, error counts) plus one `SloCheck` per declared SLO.
tags: [wire, proto, load]
timestamp: 2026-07-08T00:00:00Z
---

# Definition

```protobuf
message LoadTestResults {
  repeated Case cases = 1;

  message Case {
    string  id      = 1;
    Status  status  = 2;
    Summary summary = 3;
    repeated SloCheck checks = 4;
  }

  message Summary {
    RunLoadTestRequest.Mode mode            = 1;
    double                  target_qps      = 2;
    int32                   concurrency     = 3;
    Duration                duration        = 4;
    int64                   request_count   = 5;
    int64                   error_count     = 6;
    double                  actual_qps      = 7;
    LatencyPercentiles      latency         = 8;
    map<string,int64>       errors_by_code  = 9;   // "UNAVAILABLE" → n
  }

  message LatencyPercentiles {
    double p50_ms  = 1;
    double p95_ms  = 2;
    double p99_ms  = 3;
    double min_ms  = 4;
    double mean_ms = 5;
    double max_ms  = 6;
  }

  message SloCheck {
    string id       = 1;   // "latency.p99_ms", "error_rate", …
    Status status   = 2;
    string message  = 3;
    double observed = 4;
    double limit    = 5;
    string unit     = 6;   // "ms", "%", "rps"
  }
}
```

# Field notes

## `Summary`

- `target_qps`, `concurrency`, `duration` — the resolved `Profile`
  values.
- `request_count`, `error_count` — over `Duration` (warmup samples are
  discarded).
- `actual_qps` — observed rate. When `actual_qps < 0.9 × target_qps`
  the framework emits an `alog.Warnf` — you are measuring the
  generator, not the SUT.
- `errors_by_code` — canonical gRPC status code names (`UNAVAILABLE`,
  `DEADLINE_EXCEEDED`, …). Non-gRPC errors are grouped under
  `UNKNOWN`.

## `LatencyPercentiles`

All values in **milliseconds** (not the raw `time.Duration`).
Latencies are computed from an HDR histogram, so p99 is stable
across measurement windows.

## `SloCheck`

Every declared SLO produces one `SloCheck`. See
[SLO constructors](/api/slo-constructors.md) for id / unit / passes
mapping.

# Related

* [Load suite](/suites/load-suite.md)
* [SLO constructors](/api/slo-constructors.md)
* [Load profile](/api/load-profile.md)
* [Load mode presets](/operations/load-mode-presets.md)

# Citations

[1] [README — Load wire](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#load)
[2] [mapper/mapper.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/mapper/mapper.go)
