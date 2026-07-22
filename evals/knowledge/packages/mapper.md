---
type: Go Package
title: package evals/mapper
description: Translates internal `execution` results into wire `evalspb.Run` messages.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/mapper
tags: [package, mapper, wire]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/mapper` sits at the boundary between in-process [`execution`](/packages/execution.md)
types and the wire types in `evalspb`. It:

- Chooses the right result shape (`IntegrationTestResults`,
  `LoadTestResults`, `AgentEvalResults`, `InfraObservationResults`) based on suite kind.
- Copies mirrored fields from `execution` into branch messages (load summaries are the
  largest copy — ~20 fields in `mapLoadSummary`).
- Passes infra snapshot proto pointers through unchanged when already present on
  `execution.LoadCaseResult` / `InfraObserveCaseResult`.
- Fills in resource names (`runs/{run_id}`, `operations/{op_id}`).
- Stamps `google_project_id` from [`mapper.SetConfig`](../../mapper/config.go).
- Converts load-test maps (`tags`, `errors_by_code`) into repeated
  `{key, value}` wire entries via `entries.go`.

# Why the boundary exists

`execution` holds timing, provenance, and synthetic failure assembly; `mapper` is the
**single place** that knows how those shapes land on `evalspb.Run`. That concentrates schema
changes — at the cost of field-by-field copies for mirrored payloads. See
**Decision 0001** (`.cursor/context/decisions/0001-execution-result-boundary.md`).

Agent eval metrics also flow through [`internal/result.MetricsProto`](/packages/internal-result.md)
so ADK and mapper paths stay aligned.

# Files

| File | Purpose |
| ---- | ------- |
| `mapper.go` | Main translation logic. |
| `config.go` | `SetConfig` for `google_project_id`. |
| `entries.go` | Map → repeated `{key, value}` wire entry conversion for load results. |
| `doc.go` | Package documentation. |
| `mapper_test.go`, `load_test.go`, `entries_test.go` | Tests. |

# Related

* [`execution` package](/packages/execution.md)
* [Wire types](/wire-types/index.md)

# Citations

[1] [evals/mapper tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/mapper)
