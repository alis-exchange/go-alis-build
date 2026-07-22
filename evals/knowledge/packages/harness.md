---
type: Go Package
title: package evals/harness
description: Execute → map → report orchestration for TestService resume handlers.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/harness
tags: [package, harness, orchestration]
timestamp: 2026-07-22T00:00:00Z
---

# Role

`evals/harness` is the tested orchestration layer for the per-suite sequence
**execute → map → report** that product neurons (`alis/build/os/evals`,
`alis/build/ge/evals`) currently re-implement in `resume.go` handlers.

It sits above [runner](/packages/runner.md) (execution), [mapper](/packages/mapper.md)
(wire translation), and [report](/packages/report.md) (sink I/O).

# Public surface

| Symbol | Purpose |
| ------ | ------- |
| [RunMeta](https://github.com/alis-exchange/go-alis-build/tree/main/evals/harness) | Operation, optional BatchID, optional fixed RunID |
| [RunSuite](https://github.com/alis-exchange/go-alis-build/tree/main/evals/harness) | Generic map-and-report over any executor |
| [RunIntegrationBatch](https://github.com/alis-exchange/go-alis-build/tree/main/evals/harness) | `RunTestSuites` + `mapper.IntegrationRun` |
| [RunEvalBatch](https://github.com/alis-exchange/go-alis-build/tree/main/evals/harness) | `RunEvalSuites` + `mapper.AgentEvalRun` |
| [RunLoadBatch](https://github.com/alis-exchange/go-alis-build/tree/main/evals/harness) | `RunLoadSuites` + `mapper.LoadRun` |
| [RunInfraObserveBatch](https://github.com/alis-exchange/go-alis-build/tree/main/evals/harness) | `RunInfraObserveSuites` + `mapper.InfraObserveRun` |

# Migration from product resume.go

Before (hand-rolled `onSuiteComplete`):

```go
_, err := runner.RunTestSuites(ctx, runs, progress, func(ctx context.Context, sr execution.SuiteResult) error {
    runID := uuid.NewString()
    wire := mapper.IntegrationRun(sr, operation, runID, batchID)
    if reporter != nil {
        if err := reporter.ReportRun(ctx, wire); err != nil {
            alog.Errorf(ctx, "report run: %v", err)
        }
    }
    runNames = append(runNames, wire.GetName())
    return nil
})
```

After:

```go
names, err := harness.RunIntegrationBatch(ctx, runner, runs, harness.RunMeta{
    Operation: operation,
    BatchID:   batchID,
}, reporter, harness.BatchOptions{Progress: progress})
```

Products keep LRO private state, Cloud Tasks resume registration, and
`InitResumeHandlers` wiring — harness only replaces the map-and-report loop.

# Behaviour

- **Reporter errors** — logged via `alog`; batch continues (best-effort).
- **Nil reporter** — map only, no I/O.
- **Run IDs** — a fresh UUID is generated per suite unless [RunMeta.RunID] is
  set and exactly one suite result is returned from [RunSuite].
- **Progress** — optional via [BatchOptions.Progress]; forwarded to runner.

# Related

- [Decision 0001 — execution result boundary](/../../.cursor/context/decisions/0001-execution-result-boundary.md) — `R` stays `execution.*` types
- [runner package](/packages/runner.md) — ARCH-1 generic [Execute] loop
- [End-to-end lifecycle](/operations/lifecycle.md)

# Citations

[1] [evals/harness tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/harness)
