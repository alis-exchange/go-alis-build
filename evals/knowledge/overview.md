---
type: Overview
title: Framework overview
description: The mental model for evals — three suite kinds, one authoring surface, one wire envelope.
tags: [overview, mental-model]
timestamp: 2026-07-08T00:00:00Z
---

# What the framework is

`go.alis.build/evals` is a **single authoring surface** for three kinds
of post-deploy test:

| Kind | Constructor | Registration | Wire result |
| ---- | ----------- | ------------ | ----------- |
| Integration | `evals.NewIntegrationSuite` | `evals.RegisterIntegration` | `IntegrationTestResults` |
| Agent eval  | `evals.NewAgentEvalSuite` | `evals.RegisterEval` / `evals.RegisterAgent` | `AgentEvalResults` |
| Load        | `evals.NewLoadSuite` | `evals.RegisterLoad` | `LoadTestResults` |

Each suite kind maps to one RPC on the deployed `TestService`:
`RunIntegrationTest`, `RunAgentEval`, `RunLoadTest`. Every RPC returns
a long-running operation; every completed suite becomes a `Run` proto
published to whichever [reporters](/concepts/reporter.md) the neuron
wires up.

# How a run flows

1. **Register.** Suites and shared [environments](/concepts/environment.md)
   publish themselves at `init()` to the process-wide
   [registry](/concepts/registry.md).
2. **Wire.** The neuron constructs `TestServiceServer` with the
   registry, a `runner.Runner`, and a [reporter](/concepts/reporter.md).
3. **RPC arrives.** `Registry.ValidateSelection` synchronously rejects
   unknown `case_ids` with `InvalidArgument`. A long-running operation
   is created with `case_count` and `suite_count` metadata.
4. **Runner executes.** Environments set up once, then suites run
   sequentially. Each case runs under panic recovery — one bad case
   cannot take the batch down. LRO metadata updates after each case.
5. **Map & report.** Each completed suite is mapped to `evalspb.Run`
   and passed to the configured `Reporter`. Reporter errors are logged;
   they do not fail the LRO.
6. **Complete.** The LRO completes with `runs/{run_id}` names. Every
   case appears in the result — passing, failing, or `NOT_EVALUATED`.

# The `T` recorder

Integration and eval cases receive a per-case `*T` recorder. Every
method (`Check`, `NoErr`, `Max`, `Score`, `Checkf`) records one leaf
and **returns whether it passed**, so authors control flow with plain
`if !… { return }`:

```go
r := evals.Call(ctx, func(ctx context.Context) (*examplepb.Item, error) {
    return client.GetItem(ctx, &examplepb.GetItemRequest{Name: rootItem})
})
if !t.NoErr("grpc", r.Err) { return }
if !t.Max("latency", r.Latency, 300*time.Millisecond) { return }
t.Check("has-name", r.Resp.GetName() != "")
```

Load cases do not use `T`. Their assertions are declared as
[SLOs](/api/slo-constructors.md) alongside the `Target` function.

For gRPC streaming RPCs, use [`CallServerStream`](/api/helpers.md#streaming-helpers)
and [`CallClientStream`](/api/helpers.md#streaming-helpers) instead of
unary `Call`.

# Why three kinds under one framework

The three kinds share far more than they differ:

- Same [registry](/concepts/registry.md) and filter grammar.
- Same [environment](/concepts/environment.md) activation model.
- Same [reporter](/concepts/reporter.md) plane and `Run` envelope.
- Same [status enum](/concepts/status.md) rollup semantics.

Consolidating them lets one neuron carry all its testing artefacts
alongside production code, and lets one downstream sink ingest every
kind of run.

# Where to go next

* [Quickstart](/operations/quickstart.md) — the shortest possible
  wiring path from zero to a `Run`.
* [Integration suite](/suites/integration-suite.md) — behavioural
  contracts.
* [Agent-eval suite](/suites/agent-eval-suite.md) — LLM output grading.
* [Load suite](/suites/load-suite.md) — traffic generation and SLOs.
* [End-to-end lifecycle](/operations/lifecycle.md) — the detailed
  timeline of a single RPC.

# Citations

[1] [Package doc.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/doc.go)
[2] [README.md](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md)
