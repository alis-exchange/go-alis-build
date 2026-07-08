---
type: Playbook
title: Quickstart
description: Shortest path from zero to a `Run`. Register an env, author a suite, wire a reporter.
tags: [quickstart, playbook, setup]
timestamp: 2026-07-08T00:00:00Z
---

# Prerequisites

- A Go neuron that already serves gRPC.
- `go.alis.build/adk/launchers/evals` imported so the `/api/...`
  handlers are mounted.
- Access to the SUT (a real gRPC client) from inside the neuron.

# Steps

## 1. Register any shared environment in package init

```go
import "go.alis.build/evals/env"

func init() {
    env.Register("example-v1",
        env.WithSetup(seedExample),
        env.WithTeardown(cleanupExample),
    )
}
```

Environments activate **once per LRO**, so multiple suites can share
expensive setup.

## 2. Author a suite and publish it once

```go
import (
    "go.alis.build/evals"
    "go.alis.build/evals/env"
)

func Register() {
    s := evals.NewSuite("example-v1",
        evals.WithEnv("example-v1"),
    )
    s.Case("get-item", func(ctx context.Context, t *evals.T) {
        r := evals.Call(ctx, func(ctx context.Context) (*examplepb.Item, error) {
            return clients.Example.GetItem(ctx, &examplepb.GetItemRequest{Name: rootItem})
        })
        if !t.NoErr("grpc", r.Err) { return }
        t.Max("latency", r.Latency, 300*time.Millisecond)
        t.Check("has-name", r.Resp.GetName() != "")
    })
    evals.RegisterIntegration(s)
}
```

## 3. Wire the service and (optionally) fan out to reporters

```go
import "go.alis.build/evals/report"

services.TestServiceServer.Reporter = report.MultiReporter{
    report.LogReporter{},
    myPubSubReporter{topic: "eval-runs"},
}
```

## 4. Import the launcher

```go
import _ "go.alis.build/adk/launchers/evals"    // mounts /api/... handlers
```

This is required even if you are not writing ADK-backed agent evals —
the framework mounts LRO callbacks and (when applicable) agent-eval
discovery under `/api/`.

## Result

Once the binary starts, `RunIntegrationTest`, `RunAgentEval`, and
`RunLoadTest` on the `TestService` see the registered suites; every
completed suite becomes a `Run` published to the configured reporter.

# Related

* [Overview](/overview.md)
* [Integration suite](/suites/integration-suite.md)
* [End-to-end lifecycle](/operations/lifecycle.md)

# Citations

[1] [README — Quickstart](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#quickstart)
[2] [example_test.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/example_test.go)
