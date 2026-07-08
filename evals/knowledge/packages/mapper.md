---
type: Go Package
title: package evals/mapper
description: Translates internal `execution` results into wire `evalspb.Run` messages.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/mapper
tags: [package, mapper, wire]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/mapper` sits at the boundary between the in-process `execution`
types and the wire types in `evalspb`. It:

- Chooses the right result shape (`IntegrationTestResults`,
  `LoadTestResults`, `AgentEvalResults`) based on the suite kind.
- Converts `execution.CheckResult` leaves into `Check`, `Metric`, or
  `SloCheck` wire messages.
- Fills in resource names (`runs/{run_id}`, `operations/{op_id}`).
- Populates `start_time`, `end_time`, `duration`, and any
  `google_project_id` from context.

# Why the boundary exists

Keeping `execution` proto-free means the runner and the case authors
never import the wire proto module directly. Only the mapper does.
This makes the wire types swappable without touching authoring code.

# Files

| File | Purpose |
| ---- | ------- |
| `mapper.go` | Main translation logic. |
| `doc.go` | Package documentation. |
| `mapper_test.go`, `load_test.go` | Tests. |

# Related

* [`execution` package](/packages/execution.md)
* [Wire types](/wire-types/index.md)

# Citations

[1] [evals/mapper tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/mapper)
