---
type: Concept
title: T recorder
description: The per-case handle passed to integration and eval cases. Records assertion leaves.
tags: [t, recorder, assertion]
timestamp: 2026-07-08T00:00:00Z
---

# Purpose

`T` is the per-case recorder passed to integration and eval cases. It
is the single point through which assertions become results.

# Rules

- Every recording method (`Check`, `NoErr`, `Max`, `Score`, `Checkf`,
  `Score`) records exactly one leaf.
- Every method **returns whether the leaf passed**, so authors control
  flow with plain `if !… { return }` — no panics, no `runtime.Goexit`.
- `T` is not safe for concurrent use inside one case. Fan out with
  goroutines only if you gather results back on the case goroutine and
  record from there.
- Duplicate check ids inside one case produce a single sentinel leaf
  with id `duplicate-check-id` (exported as `evals.DuplicateCheckIDName`)
  and `FAILED` status. Later reuses of the id are ignored so downstream
  parsers stay deterministic.

# Method vocabulary

See [T methods](/api/t-methods.md) for the full API surface. In
summary:

| Method | Records |
| ------ | ------- |
| `Check(id, pass)` | Bare pass/fail leaf. |
| `Checkf(id, pass, format, args...)` | Pass/fail leaf with a formatted failure message. |
| `NoErr(id, err)` | `err == nil` ? PASS : FAIL with `err.Error()`. |
| `Max(id, got, limit)` | `got <= limit` ? PASS : FAIL with both values. |
| `Score(id, score, threshold, rationale)` | Eval-only. `score >= threshold` ? PASS : FAIL. |

# Wire mapping

On the wire the recorded leaves become either:

- `IntegrationTestResults.Case.Check` (integration suites), or
- `AgentEvalResults.Case.Metric` (eval suites).

The same `T` produces both — the mapper picks the shape based on the
enclosing suite's kind. On integration wire, scores are dropped
(there is no `Metric` shape); on eval wire, thresholds and observed
scores are preserved.

# Guarded chain pattern

```go
if !t.NoErr("grpc", r.Err) {
    return
}
if !t.Max("latency", r.Latency, 300*time.Millisecond) {
    return
}
t.Check("has-name", r.Resp.GetName() != "")
t.Checkf("size-positive", r.Resp.GetSize() > 0, "got size=%d, want > 0", r.Resp.GetSize())
```

Reads top-to-bottom, bails on the first meaningful failure, and still
records every leaf up to that point on the wire.

# Related

* [T methods (full API)](/api/t-methods.md)
* [Case](/concepts/case.md)
* [Status](/concepts/status.md)

# Citations

[1] [t.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/t.go)
