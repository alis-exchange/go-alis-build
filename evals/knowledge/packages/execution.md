---
type: Go Package
title: package evals/execution
description: Proto-free in-process result types — the boundary between case-facing and wire-facing.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/execution
tags: [package, execution, internal]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/execution` defines the plain Go types that the runner produces
and the mapper consumes. These are proto-free so that the runner
does not link the wire proto module — that's the mapper's job.

The two sides of the boundary:

- **Case-facing.** The `T` recorder writes `execution.CaseResult`
  entries.
- **Wire-facing.** The [`mapper`](/packages/mapper.md) converts
  `execution.RunResult` into `evalspb.Run`.

# Public surface

- `execution.RunResult` — per-suite result.
- `execution.CaseResult` — per-case result, including a rolled-up
  `Status` and a list of leaves.
- `execution.CheckResult` — one recorded leaf (check, metric, or
  SLO check).

# Files

| File | Purpose |
| ---- | ------- |
| `types.go` | The result types. |
| `doc.go` | Package documentation. |

# Related

* [`runner` package](/packages/runner.md)
* [`mapper` package](/packages/mapper.md)

# Citations

[1] [evals/execution tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/execution)
