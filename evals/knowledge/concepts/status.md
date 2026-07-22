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
  NOT_EVALUATED      = 3;   // selected but skipped (StopOnFailure or cancellation)
}
```

# Semantics

- `PASSED` — the case executed and every recorded leaf passed. For load
  cases, every declared SLO produced a passing `SloCheck`.
- `FAILED` — the case executed and at least one leaf failed. Includes
  panics (surfaced as an `_evals.case` leaf), setup failures
  (`_evals.setup`), transport errors captured by `NoErr`, and duplicate
  check-id sentinels.
- `NOT_EVALUATED` — the case was skipped. Reasons include:
  - the case's suite has `StopOnFailure` and an earlier case failed,
  - cancellation occurred before the selected case started.

Cases excluded by a filter do not appear in the run. Setup failures appear as
`FAILED`, not `NOT_EVALUATED`.
- `STATUS_UNSPECIFIED` — reserved. Consumers should treat unrecognised
  values conservatively.

# Rollup

Status rolls up the hierarchy via `evals/verdict`:

- **Case** — `PASSED` iff every leaf passed. `FAILED` if any leaf failed.
  Empty integration/eval cases fail with `_evals.no-checks-recorded` unless
  the author calls `t.Pass(id)`.
- **Run** — `FAILED` if any case failed. `NOT_EVALUATED` only when every
  case is `NOT_EVALUATED`. Otherwise `PASSED`, including runs where some
  cases are `NOT_EVALUATED` and the rest passed (cancellation partial results).

| Case statuses in run | Run status |
| --- | --- |
| all `PASSED` | `PASSED` |
| any `FAILED` | `FAILED` |
| mix of `PASSED` + `NOT_EVALUATED` | `PASSED` |
| all `NOT_EVALUATED` | `NOT_EVALUATED` |

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
