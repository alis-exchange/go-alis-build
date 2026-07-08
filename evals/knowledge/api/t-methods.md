---
type: API Reference
title: T methods
description: The full method vocabulary on `*T` — the per-case recorder for test and eval cases.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/t.go
tags: [api, t, recorder, assertion]
timestamp: 2026-07-08T00:00:00Z
---

# Vocabulary

Every method records one leaf and returns whether it passed.

| Method | Semantics | Integration wire | Eval wire |
| ------ | --------- | ---------------- | --------- |
| `t.Check(id string, pass bool) bool` | Records `id` with `PASSED`/`FAILED`. Message empty. | `Check{id, status, ""}` | `Metric{id, status, "", 0, nil, nil}` |
| `t.Checkf(id string, pass bool, format string, args ...any) bool` | Same as `Check` but formats a failure message. | `Check{id, status, msg}` | `Metric{id, status, msg, 0, nil, nil}` |
| `t.NoErr(id string, err error) bool` | Records `PASSED` when `err == nil`, otherwise `FAILED` with `err.Error()`. | `Check{id, status, err.Error()}` | `Metric{id, status, err.Error(), …}` |
| `t.Max(id string, got, limit time.Duration) bool` | Records `PASSED` when `got <= limit`. Failure message includes both values. | `Check{id, …, "id 350ms exceeds limit 300ms"}` | `Metric{id, …, message}` |
| `t.Score(id string, score, threshold float64, rationale string) bool` | Records `PASSED` when `score >= threshold`. `rationale` becomes the metric message; a default is generated when `pass == false && rationale == ""`. | Not used (score dropped on integration). | `Metric{id, status, message, threshold, score, nil}` |

# Concurrency

`T` is not safe for concurrent use inside one case. If your case fans
out to goroutines, gather their results back on the case goroutine
and record from there.

# Duplicate ids

Recording the same id twice inside one case produces a single
sentinel leaf with id `duplicate-check-id` (exported as
`evals.DuplicateCheckIDName`) and `FAILED` status. All further
attempts to reuse the id are ignored so results remain parseable.

# Return convention

The pass boolean enables the guarded-chain pattern:

```go
if !t.NoErr("grpc", r.Err) { return }
if !t.Max("latency", r.Latency, budget) { return }
t.Check("shape", r.Resp.GetName() != "")
```

No panics, no `runtime.Goexit`, no framework-specific control flow.

# Why `Score` is eval-only

Integration wire uses `Check` messages that carry only
`{id, status, message}` — no score field. If you call `Score` from an
integration case, the leaf is recorded, but the score value is
dropped by the mapper on the integration wire path. Prefer `Check`
for integration suites.

# Related

* [T recorder](/concepts/t-recorder.md)
* [Case](/concepts/case.md)
* [DuplicateCheckIDName](https://github.com/alis-exchange/go-alis-build/blob/main/evals/t.go)

# Citations

[1] [t.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/t.go)
[2] [t_test.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/t_test.go)
