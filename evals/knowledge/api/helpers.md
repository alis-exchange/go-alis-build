---
type: API Reference
title: Helpers
description: Small utilities on the public authoring surface — `Call`, streaming helpers, `Result[T]`, `Rouge1F1`, load-profile resolution.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/call.go
tags: [api, helpers, utility, streaming]
timestamp: 2026-07-14T00:00:00Z
---

# Functions

| Function | Purpose |
| -------- | ------- |
| `evals.Call[T](ctx context.Context, fn func(context.Context) (T, error)) Result[T]` | Invoke a unary RPC, capture typed response, error, and wall-clock latency. Records nothing on `T` — assertions are explicit. |
| `evals.CallServerStream[T](ctx, openFn) ServerStreamResult[T]` | Drain a bounded server stream; capture messages, TTFB (0 when empty), total duration, and inter-message intervals. |
| `evals.CallClientStream[Req, Resp](ctx, openFn, sendFn) ClientStreamResult[Resp]` | Client-streaming with split send/response timing and message count. |
| `evals.Result[T]` | `{Resp T, Err error, Latency time.Duration}`. |
| `evals.ServerStreamResult[T]` | `{Messages []T, Err, TTFB, TotalDuration, MessageIntervals}`. |
| `evals.ClientStreamResult[Resp]` | `{Resp, Err, SendDuration, ResponseLatency, TotalDuration, MessagesSent}`. |
| `evals.Rouge1F1(hypothesis, reference string) float64` | Deterministic ROUGE-1 unigram F1 scorer. Empty-empty → 1; one-empty → 0. Feed into `t.Score`. |
| `evals.DefaultLoadProfile(mode evalspb.RunLoadTestRequest_Mode) (Profile, bool)` | Look up the framework default profile for `mode`. Returns `(zero, false)` for `MODE_UNSPECIFIED`. |
| `evals.ResolveLoadProfile(mode, overrides map[Mode]Profile) (Profile, bool)` | Merge suite overrides with defaults. Override wins; other modes keep defaults. |

# `Call` and `Result[T]`

`Call` is the idiomatic wrapper for unary RPC invocation:

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

# Streaming helpers

## When to use

- **Unary `Call`** — single request/response RPCs (and client-streaming when split timing is not needed).
- **`CallServerStream`** — bounded server-streaming RPCs that terminate with EOF.
- **`CallClientStream`** — client-streaming RPCs where send-side vs response-side timing matters.

## Server-streaming example

```go
res := evals.CallServerStream(ctx, func(ctx context.Context) (grpc.ServerStreamingClient[examplepb.Event], error) {
    return clients.Example.WatchEvents(ctx, req)
})
if !t.NoErr("grpc", res.Err) { return }
if len(res.Messages) > 0 {
    t.Max("ttfb", res.TTFB, 100*time.Millisecond)
}
t.Max("total", res.TotalDuration, 2*time.Second)
if len(res.MessageIntervals) > 0 {
    t.Max("gap-0", res.MessageIntervals[0], 500*time.Millisecond)
}
```

## Client-streaming example

```go
r := evals.CallClientStream(ctx,
    func(ctx context.Context) (grpc.ClientStreamingClient[examplepb.Chunk, examplepb.UploadResult], error) {
        return clients.Example.Upload(ctx)
    },
    func(stream grpc.ClientStreamingClient[examplepb.Chunk, examplepb.UploadResult]) (examplepb.UploadResult, error) {
        for _, chunk := range chunks {
            if err := stream.Send(chunk); err != nil {
                return examplepb.UploadResult{}, err
            }
        }
        resp, err := stream.CloseAndRecv()
        if err != nil {
            return examplepb.UploadResult{}, err
        }
        return *resp, nil
    },
)
if !t.NoErr("grpc", r.Err) { return }
t.Max("send", r.SendDuration, 500*time.Millisecond)
t.Max("response", r.ResponseLatency, 200*time.Millisecond)
```

## `ServerStreamResult[T]`

| Field | Meaning |
| ----- | ------- |
| `Messages` | All successfully received messages (partial on recv error). |
| `Err` | Transport error; nil on clean EOF. Set on nil-message `Recv`. |
| `TTFB` | Start through first message; includes `openFn`. **0 when empty — guard before asserting.** |
| `TotalDuration` | Wall clock from start through recv loop exit. |
| `MessageIntervals` | `len = max(0, len(Messages)-1)`; gap between consecutive messages. |

**Warning:** Do not use on watch/subscribe RPCs that never send EOF. Bound with a context deadline.

## `ClientStreamResult[Resp]`

| Field | Meaning |
| ----- | ------- |
| `Resp` | Final response from `sendFn`. |
| `Err` | Transport or sendFn error. |
| `SendDuration` | Start through last successful Send (includes `openFn`). |
| `ResponseLatency` | `CloseAndRecv` entry through return; **0 if never reached** (e.g. send error). |
| `TotalDuration` | Wall clock through `sendFn` return; may exceed `SendDuration + ResponseLatency`. |
| `MessagesSent` | Count of successful Sends. |

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
* [Integration suite](/suites/integration-suite.md)
* [Agent-eval suite](/suites/agent-eval-suite.md)
* [Load profile fields](/api/load-profile.md)

# Citations

[1] [call.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/call.go)
[2] [stream_server.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/stream_server.go)
[3] [stream_client.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/stream_client.go)
[4] [score.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/score.go)
[5] [load_profile.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/load_profile.go)
