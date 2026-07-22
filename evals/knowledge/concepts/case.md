---
type: Concept
title: Case
description: The unit of execution — a `func(ctx, *T)` for test/eval, or a `ResultTarget` + `SLO`s for load.
tags: [case, core]
timestamp: 2026-07-08T00:00:00Z
---

# Definition

A **case** is the smallest unit the runner executes. It always belongs
to one [suite](/concepts/suite.md) and appears on the wire as one
`Case` message inside the suite's `Run`.

# Case types

## Test / eval

Signature:

```go
type CaseFunc func(ctx context.Context, t *evals.T)
```

The function receives the LRO context and a per-case
[`*T` recorder](/concepts/t-recorder.md). It calls the SUT via
`evals.Call` and records assertion leaves on `t`.

## Load

A load case is defined by a `ResultTarget` and a set of SLOs:

```go
s.LoadCase("list-items",
    evals.TransportTarget(func(ctx context.Context) error {
        _, err := client.ListItems(ctx, &examplepb.ListItemsRequest{PageSize: 5})
        return err
    }),
    []evals.SLO{
        evals.SLOLatencyP99(500 * time.Millisecond),
        evals.SLOErrorRate(0.01),
    },
)
```

The framework invokes the target many times under a resolved
[`Profile`](/api/load-profile.md), aggregates metrics, and evaluates
every declared SLO.

# Case identity

Every case is qualified as `{suite}.{case}` at registration. The
combination is what `case_ids` filters match. Case names must:

- be non-empty,
- not contain `.` (that character is the suite/case separator),
- be unique within the suite.

# Rollup

- `PASSED` — the case executed and every recorded leaf passed.
- `FAILED` — the case executed and at least one leaf failed.
- `NOT_EVALUATED` — the case was skipped. Reasons include
  `StopOnFailure` or cancellation before the selected case started. Cases
  excluded by a filter do not appear; setup failures are `FAILED`.

# Related

* [T recorder](/concepts/t-recorder.md)
* [Case registration](/api/case-registration.md)
* [Filter grammar](/operations/filter-grammar.md)
* [Status](/concepts/status.md)

# Citations

[1] [suite.go — Case / LoadCase methods](https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite.go)
