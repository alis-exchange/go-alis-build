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
    env.MustRegister("example-v1",
        env.WithSetup(seedExample),
        env.WithTeardown(cleanupExample),
    )
}
```

Environments activate **once per LRO**, so multiple suites can share
expensive setup. `MustRegister` panics on duplicate names; use
`env.Register` when you want to propagate the error.

## 2. Author a suite and publish it once

```go
import (
    "go.alis.build/evals"
    "go.alis.build/evals/env"
)

func Register() {
    s := evals.MustNewIntegrationSuite("example-v1",
        evals.WithEnv("example-v1"),
    )
    s.MustCase("get-item", func(ctx context.Context, t *evals.T) {
        r := evals.Call(ctx, func(ctx context.Context) (*examplepb.Item, error) {
            return clients.Example.GetItem(ctx, &examplepb.GetItemRequest{Name: rootItem})
        })
        if !t.NoErr("grpc", r.Err) { return }
        t.Max("latency", r.Latency, 300*time.Millisecond)
        t.Check("has-name", r.Resp.GetName() != "")
    })
    if err := evals.RegisterIntegration(s); err != nil {
        panic(err)
    }
}
```

The `Must*` constructors and `MustCase` panic on config errors so init-time
misuse fails loudly. Use the error-returning variants (`NewIntegrationSuite`,
`Case`, etc.) if you'd rather propagate.

## 3. Wire the service and (optionally) fan out to reporters

```go
import (
    "context"

    "go.alis.build/evals/report"
    logreport "go.alis.build/evals/report/log"
    pubsubreport "go.alis.build/evals/report/pubsub"
)

func setupReporters(ctx context.Context) (*pubsubreport.Reporter, error) {
    ps, err := pubsubreport.New(ctx)
    if err != nil {
        return nil, err
    }
    services.TestServiceServer.Reporter = report.MultiReporter{
        logreport.Reporter{},
        ps, // publishes RunPublishedEvent to Pub/Sub via go.alis.build/events
    }
    return ps, nil // Close() at server drain
}
```

Swap `pubsubreport` for `bqreport` (see
[`report/bigquery`](/packages/report-bigquery.md)) to stream Runs
into BigQuery instead, or fan out to both.

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
