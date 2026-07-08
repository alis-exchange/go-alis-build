---
type: Suite Kind
title: Load suite
description: Traffic generation at declared intensities with SLO evaluation on the aggregate metrics.
tags: [load, slo, traffic, benchmark]
timestamp: 2026-07-08T00:00:00Z
---

# Purpose

A **load suite** generates traffic against a target function at a
chosen intensity and evaluates SLOs on the aggregate metrics. It is
shaped differently from test/eval suites — there is no `T`; assertions
are SLOs declared alongside the target.

# Anatomy

```go
package v1

import (
    "context"
    "time"

    "go.alis.build/evals"
    evalspb "example.com/pb/alis/ge/evals/v1"
    examplepb "example.com/pb/example/v1"

    "example.com/internal/clients"
)

func Register() {
    s := evals.NewLoadSuite("example-v1-load",
        evals.WithLoadEnv(exampleEnv),
        // Override MODERATE for this suite: heavier than the framework default.
        evals.WithLoadProfile(evalspb.RunLoadTestRequest_MODERATE, evals.Profile{
            QPS:            250,
            Concurrency:    40,
            Duration:       90 * time.Second,
            Warmup:         15 * time.Second,
            RequestTimeout: 5 * time.Second,
        }),
    )

    s.LoadCase("list-items",
        func(ctx context.Context) error {
            _, err := clients.Example.ListItems(ctx, &examplepb.ListItemsRequest{PageSize: 5})
            return err
        },
        evals.SLOLatencyP99(500*time.Millisecond),
        evals.SLOLatencyP50(50*time.Millisecond),
        evals.SLOErrorRate(0.01),
        evals.SLOMinQPS(20),
    )

    s.LoadCase("get-item",
        func(ctx context.Context) error {
            _, err := clients.Example.GetItem(ctx, &examplepb.GetItemRequest{Name: rootItem})
            return err
        },
        evals.SLOLatencyP99(300*time.Millisecond),
        evals.SLOErrorRate(0.005),
    )

    evals.RegisterLoad(s)
}
```

# Execution model

For each case the runner:

1. **Resolves the effective `Profile`** — framework default for the
   requested `Mode`, overridden by any suite-level `WithLoadProfile`
   for that mode.
2. **Spawns `Concurrency` worker goroutines** and a fixed-rate pacer.
3. **Runs `Warmup + Duration` of traffic**; samples produced during
   `Warmup` are discarded so autoscalers and JITs have time to settle.
4. **Aggregates latency into an HDR histogram**, counts errors, groups
   them by canonical gRPC status code.
5. **Evaluates every declared SLO** against the aggregate metrics;
   each produces one `SloCheck` (passed or failed) with the observed
   value, the limit, and the unit.
6. **Rolls up the case** — `PASSED` only when every SLO passed.

Load cases run **sequentially within a suite** by design — concurrent
load windows against different targets would contaminate each other's
measurements.

# Mode presets

`RunLoadTest.mode` picks a framework preset. See
[Load mode presets](/operations/load-mode-presets.md).

# Overrides replace, they don't merge

`WithLoadProfile(mode, profile)` fully replaces the framework default
for that mode. There is no field-level merging — a partial override
at high intensity is easy to get wrong.

# Saturation warning

When the worker pool cannot keep up with the target rate, the observed
`Summary.actual_qps` falls below `target_qps`. The generator emits an
`alog.Warnf` when `actual_qps < 0.9 × target_qps` so you notice you
are measuring the generator, not the SUT — bump `Concurrency` and
rerun.

# What you cannot do

- No `T` recorder. Load assertions are SLOs, not per-request checks.
- No `StopOnFailure`. Load cases run sequentially and a failed case
  does not invalidate the next case's measurement.
- No per-case identity. Load suites always run under system identity —
  the goal is to measure the SUT under the same identity production
  traffic uses.

# Wire shape

Results appear as [`LoadTestResults`](/wire-types/load-results.md):
one `Case` per registered case, each carrying a `Summary` (QPS,
latency percentiles, error counts) and a list of `SloCheck` leaves.

# Related

* [SLO constructors](/api/slo-constructors.md)
* [Load profile fields](/api/load-profile.md)
* [Load-suite options](/api/load-suite-options.md)
* [Load mode presets](/operations/load-mode-presets.md)
* [`loadgen` package](/packages/loadgen.md)

# Citations

[1] [README — Load tests](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#load-tests)
[2] [load.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/load.go)
[3] [slo.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/slo.go)
