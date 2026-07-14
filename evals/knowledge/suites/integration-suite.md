---
type: Suite Kind
title: Integration suite
description: Behavioural contract testing against a live gRPC surface. Uses `NewIntegrationSuite` and `RegisterIntegration`.
tags: [integration, test, grpc]
timestamp: 2026-07-08T00:00:00Z
---

# Purpose

An **integration suite** asserts behavioural contracts on a live gRPC
surface after deploy. Its cases exercise the SUT via real RPCs and
record boolean or latency-bound assertions.

# Anatomy

```go
package v1

import (
    "context"
    "time"

    "go.alis.build/evals"
    "go.alis.build/evals/env"

    "example.com/internal/clients"
    examplepb "example.com/pb/example/v1"
)

const exampleEnv = "example-v1"

func Register() {
    env.MustRegister(exampleEnv,
        env.WithSetup(seedExample),
        env.WithTeardown(cleanupExample),
    )

    s := evals.MustNewIntegrationSuite("example-v1",
        evals.WithEnv(exampleEnv),
        evals.WithSetup(sanityCheck),
    )

    s.MustCase("get-item", func(ctx context.Context, t *evals.T) {
        r := evals.Call(ctx, func(ctx context.Context) (*examplepb.Item, error) {
            return clients.Example.GetItem(ctx, &examplepb.GetItemRequest{Name: rootItem})
        })
        if !t.NoErr("grpc", r.Err) {
            return
        }
        if !t.Max("latency", r.Latency, 300*time.Millisecond) {
            return
        }
        t.Check("has-name", r.Resp.GetName() != "")
        t.Checkf("size-positive", r.Resp.GetSize() > 0, "got size=%d, want > 0", r.Resp.GetSize())
    }).MustCase("list-empty-parent", func(ctx context.Context, t *evals.T) {
        r := evals.Call(ctx, func(ctx context.Context) (*examplepb.ListItemsResponse, error) {
            return clients.Example.ListItems(ctx, &examplepb.ListItemsRequest{Parent: emptyParent})
        })
        if !t.NoErr("grpc", r.Err) {
            return
        }
        t.Check("empty", len(r.Resp.GetItems()) == 0)
    })

    if err := evals.RegisterIntegration(s); err != nil {
        panic(err)
    }
}
```

# What each case can assert

Every method on `*T` records one `IntegrationTestResults.Case.Check`
leaf on the wire and returns whether that leaf passed. See
[T methods](/api/t-methods.md).

# Patterns

## Guarded chain

```go
if !t.NoErr("grpc", err)                { return }
if !t.Max("latency", r.Latency, budget) { return }
t.Check("shape", r.Resp.GetName() != "")
```

## Stateful flow

For a create → get → update → delete flow where later steps have no
meaning after an earlier failure, add `evals.StopOnFailure()` to the
suite. Subsequent cases are recorded `NOT_EVALUATED` with a
"preceding case … failed" reason.

## Context decoration

`evals.WithContext(fn)` installs a `func(ctx) ctx` applied to the
suite's setup, teardown, and every case body. This is the framework's
only auth-adjacent surface — callers stamp caller identity, auth
headers, tokens, tracing, or any request-scoped values here. The
framework itself never attaches auth. Different suites can install
different decorators; see [Context decoration in the README](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#context-and-authentication).

## Streaming RPCs

For server-streaming and client-streaming RPCs, use `CallServerStream`
and `CallClientStream` instead of unary `Call`. See
[Helpers — Streaming](/api/helpers.md#streaming-helpers) and the
[README Streaming RPCs section](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#streaming-rpcs).

# Wire shape

Results appear as [`IntegrationTestResults`](/wire-types/integration-results.md):
one `Case` per registered case, each with a list of `Check` leaves
and a rolled-up `Status`.

# Related

* [T recorder](/concepts/t-recorder.md)
* [Suite constructors](/api/suite-constructors.md)
* [Shared suite options](/api/suite-options.md)
* [StopOnFailure](/operations/stop-on-failure.md)

# Citations

[1] [README — Integration tests](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#integration-tests)
[2] [doc.go — Integration tests](https://github.com/alis-exchange/go-alis-build/blob/main/evals/doc.go)
