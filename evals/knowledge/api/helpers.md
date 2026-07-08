---
type: API Reference
title: Helpers
description: Small utilities on the public authoring surface — `Call`, `Result[T]`, `Rouge1F1`, load-profile resolution.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/call.go
tags: [api, helpers, utility]
timestamp: 2026-07-08T00:00:00Z
---

# Functions

| Function | Purpose |
| -------- | ------- |
| `evals.Call[T](ctx context.Context, fn func(context.Context) (T, error)) Result[T]` | Invoke an RPC, capture typed response, error, and wall-clock latency. Records nothing on `T` — assertions are explicit. |
| `evals.Result[T]` | `{Resp T, Err error, Latency time.Duration}`. |
| `evals.Rouge1F1(hypothesis, reference string) float64` | Deterministic ROUGE-1 unigram F1 scorer. Empty-empty → 1; one-empty → 0. Feed into `t.Score`. |
| `evals.DefaultLoadProfile(mode evalspb.RunLoadTestRequest_Mode) (Profile, bool)` | Look up the framework default profile for `mode`. Returns `(zero, false)` for `MODE_UNSPECIFIED`. |
| `evals.ResolveLoadProfile(mode, overrides map[Mode]Profile) (Profile, bool)` | Merge suite overrides with defaults. Override wins; other modes keep defaults. |

# `Call` and `Result[T]`

`Call` is the idiomatic wrapper for RPC invocation:

```go
r := evals.Call(ctx, func(ctx context.Context) (*examplepb.Item, error) {
    return client.GetItem(ctx, &examplepb.GetItemRequest{Name: rootItem})
})
if !t.NoErr("grpc", r.Err) { return }
t.Max("latency", r.Latency, 300*time.Millisecond)
```

`Result[T]` is a plain struct:

```go
type Result[T any] struct {
    Resp    T
    Err     error
    Latency time.Duration
}
```

`Latency` is wall-clock only — the framework does not attempt to
subtract client-side serialisation.

# `Rouge1F1`

`Rouge1F1` computes unigram ROUGE F1 between two whitespace-tokenised
strings. Behaviour at edges:

- Both empty → `1.0`.
- One empty, other non-empty → `0.0`.
- Otherwise the standard harmonic mean of precision and recall.

Feed the result to `t.Score` with a threshold appropriate for the
domain (`0.5` is a reasonable starting point for summarisation).

# Profile resolution

- `DefaultLoadProfile(mode)` returns the framework default profile
  and `ok=false` when `mode == MODE_UNSPECIFIED`.
- `ResolveLoadProfile(mode, overrides)` merges suite-level overrides
  with defaults. Only the exact mode requested is affected; other
  modes keep their defaults. The framework does not do field-level
  merging.

# Related

* [T recorder](/concepts/t-recorder.md)
* [Agent-eval suite](/suites/agent-eval-suite.md)
* [Load profile fields](/api/load-profile.md)

# Citations

[1] [call.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/call.go)
[2] [score.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/score.go)
[3] [load_profile.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/load_profile.go)
