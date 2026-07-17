---
type: Wire Type
title: LoadTestResults
description: Per-case `Summary` (QPS, latency percentiles, error counts, stream metrics) plus one `SloCheck` per declared SLO.
tags: [wire, proto, load]
timestamp: 2026-07-08T00:00:00Z
---

# Definition

```protobuf
message LoadTestResults {
  repeated Case cases = 1;

  message StringEntry {
    string key = 1;
    string value = 2;
  }

  message Int64Entry {
    string key = 1;
    int64 value = 2;
  }

  message Case {
    string  id      = 1;
    Status  status  = 2;
    Summary summary = 3;
    repeated SloCheck checks = 4;
    repeated StringEntry tags = 5;
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
    repeated Int64Entry     errors_by_code  = 9;
    int64                   dropped_count   = 10;
    int64                   check_passed_count = 11;
    int64                   check_failed_count = 12;
    StreamSummary           stream          = 13;
    repeated LoadStage      qps_stages      = 14;
    repeated LoadStage      concurrency_stages = 15;
  }

  message LoadStage {
    Duration duration = 1;
    double   target   = 2;
  }

  message StreamSummary {
    int64 stream_count        = 1;
    int64 messages_sent_total = 2;
    LatencyPercentiles ttfb           = 3;
    LatencyPercentiles response_latency = 4;
    LatencyPercentiles total_duration   = 5;
  }

  message LatencyPercentiles { /* p50–max ms */ }

  message SloCheck { /* id, status, message, observed, limit, unit */ }
}
```

# Field notes

## `Summary`

- `target_qps`, `concurrency`, `duration` — the resolved `Profile`
  values.
- `request_count`, `error_count` — over `Duration` (warmup samples are
  discarded). `error_count` is transport failures only.
- `check_passed_count`, `check_failed_count` — semantic assertions
  separate from transport errors.
- `dropped_count` — ticks not executed (pacer backpressure and
  worker-side skips after the window ended).
- `actual_qps` — observed rate. When `actual_qps < 0.9 × target_qps`
  the framework emits an `alog.Warnf` — you are measuring the
  generator, not the SUT.
- `errors_by_code` — canonical gRPC status code names (`UNAVAILABLE`,
  `DEADLINE_EXCEEDED`, …) as repeated `{key, value}` entries (BigQuery-
  compatible JSON arrays on the Pub/Sub path). Non-gRPC errors are
  grouped under `UNKNOWN`.
- `qps_stages`, `concurrency_stages` — resolved staged profile config
  (empty when constant rate/concurrency).
- `stream` — populated when the case exercised streaming RPCs.

## `StreamSummary`

- `ttfb` aggregates client-stream send duration (`SendDuration`).
- `response_latency` and `total_duration` mirror the load generator
  stream histograms.

## `SloCheck`

Every declared SLO produces one `SloCheck`. Semantic check failures
without an explicit SLO produce a synthetic check with id `checks`.
See [SLO constructors](/api/slo-constructors.md).

# Related

* [Load suite](/suites/load-suite.md)
* [SLO constructors](/api/slo-constructors.md)
* [Load profile](/api/load-profile.md)
* [Load mode presets](/operations/load-mode-presets.md)

# Citations

[1] [README — Load wire](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#load)
[2] [mapper/mapper.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/mapper/mapper.go)
