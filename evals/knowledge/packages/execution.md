---
type: Go Package
title: package evals/execution
description: In-process result types between case execution and wire mapping — not a proto-free layer.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/execution
tags: [package, execution, internal]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/execution` defines the plain Go types the runner assembles before
[`mapper`](/packages/mapper.md) builds `evalspb.Run`. The layer is **not**
proto-free: it imports `evalspb` for `Status`, load `Mode`, and infra
snapshots, and passes Cloud Run / Spanner snapshot protos through unchanged
on load and infra-observe cases.

What it **does** isolate:

- Wall-clock suite timing (`StartTime`, `EndTime`) before timestamp conversion
- Case `time.Duration` before `durationpb`
- Judge provenance roll-ups (`JudgeInfo`, `JudgeCallCount`)
- Synthetic skipped/failure cases built in [`internal/result`](/packages/internal-result.md)
- Registration-only metadata not yet on the wire (for example infra case `Tags`)

See **Decision 0001** (`.cursor/context/decisions/0001-execution-result-boundary.md`)
for the full classification and rationale.

# Public surface

- `SuiteResult` / `CaseResult` — integration and agent eval suites
- `LoadSuiteResult` / `LoadCaseResult` — load suites (+ optional infra snapshots)
- `InfraObserveSuiteResult` / `InfraObserveCaseResult` — standalone infra observation
- Leaf types: `Check`, `Metric`, `SloCheckResult`, `LoadCaseSummary`, …

# Files

| File | Purpose |
| ---- | ------- |
| `types.go` | Result structs. |
| `doc.go` | Package documentation (package godoc may lag knowledge — prefer this page). |

# Related

* [`runner` package](/packages/runner.md)
* [`mapper` package](/packages/mapper.md)

# Citations

[1] [evals/execution tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/execution)
