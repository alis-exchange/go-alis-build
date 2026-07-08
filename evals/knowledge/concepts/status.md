---
type: Concept
title: Status enum
description: The four-value status enum used throughout the wire types.
tags: [status, enum, wire]
timestamp: 2026-07-08T00:00:00Z
---

# Definition

```protobuf
enum Status {
  STATUS_UNSPECIFIED = 0;
  PASSED             = 1;   // executed and every check passed
  FAILED             = 2;   // executed and one or more checks failed
  NOT_EVALUATED      = 3;   // skipped (StopOnFailure, setup fail, filter)
}
```

# Semantics

- `PASSED` — the case executed and every recorded leaf passed. For load
  cases, every declared SLO produced a passing `SloCheck`.
- `FAILED` — the case executed and at least one leaf failed. Includes
  panics (surfaced as a `panic` leaf), transport errors captured by
  `NoErr`, and duplicate check-id sentinels.
- `NOT_EVALUATED` — the case was skipped. Reasons include:
  - the case's suite has `StopOnFailure` and an earlier case failed,
  - the suite's setup hook returned an error,
  - the case wasn't selected by the filter (in this case the case
    typically doesn't appear in the run at all — the enum value is
    reserved for cases that were selected but skipped).
- `STATUS_UNSPECIFIED` — reserved. Consumers should treat unrecognised
  values conservatively.

# Rollup

Status rolls up the hierarchy:

- **Case** — `PASSED` iff every leaf passed. `FAILED` if any leaf
  failed. `NOT_EVALUATED` if the case was skipped.
- **Suite/Run** — `PASSED` iff every case passed. `FAILED` if any case
  failed. Otherwise `NOT_EVALUATED`.

# Where it appears

- Every `Run` (top-level and its embedded results).
- Every `Case` in `IntegrationTestResults`, `LoadTestResults`, and
  `AgentEvalResults`.
- Every leaf: `Check`, `SloCheck`, `Metric`, `RubricScore`.

# Related

* [Run wire envelope](/wire-types/run.md)
* [StopOnFailure](/operations/stop-on-failure.md)

# Citations

[1] [README — Wire types](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#wire-types)
