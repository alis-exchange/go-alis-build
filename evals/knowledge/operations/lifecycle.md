---
type: Reference
title: End-to-end lifecycle
description: The timeline of one integration, agent-eval, load, or infra-observation RPC.
tags: [lifecycle, runtime, lro]
timestamp: 2026-07-08T00:00:00Z
---

# Timeline

1. **Register.** Each case package calls `env.Register(...)` and one
   of `RegisterIntegration` / `RegisterEval` / `RegisterLoad` /
   `RegisterInfraObserve` / `RegisterAgent` during startup.
2. **Freeze.** After registration completes, `evals.Freeze()` validates
   environment references, profiles, and duplicate suite names, then seals
   the registry.
3. **Wire.** The neuron's `TestServiceServer` is constructed with
   `evals.DefaultRegistry()`, `runner.New()`, and a reporter (default
   `logreport.Reporter{}` from `go.alis.build/evals/report/log`).
4. **RPC arrives.** `RunIntegrationTest` / `RunLoadTest` /
   `RunAgentEval` / `RunInfraObservation`. `Registry.ValidateSelection` rejects unknown
   `case_ids` synchronously with `InvalidArgument`.
5. **LRO starts.** A long-running operation is created with initial
   metadata (`case_count`, `suite_count`). A resume task is scheduled;
   locally the `lro` library runs it in a goroutine via
   `httptest.NewRecorder`, in production it's dispatched via Cloud
   Tasks.
6. **Runner executes.** Environment setups fire once, then suites
   run sequentially. Each case runs under panic recovery — one bad
   case cannot take the batch down. LRO metadata is updated after
   each case (`completed_case_count`) and each suite
   (`completed_suite_count`).
7. **Map & report.** Each completed suite is mapped to `evalspb.Run`
   via [`mapper`](/packages/mapper.md) and passed to the configured
   `Reporter`. Reporter errors are logged; they do not fail the LRO.
8. **Complete.** The LRO completes with `RunXxxResponse.runs`
   listing the resource names of every emitted run
   (`runs/{run_id}`). Consumers fetch or subscribe to those.

Every selected case appears in the result — passing, failing, or
`NOT_EVALUATED` — so dashboards can compute pass rate, headroom, and
trend without reconstructing what was intended to run.

# Panic recovery

The `runner` wraps every case in a defer/recover pair. A panic is
recorded as one leaf on the case with id `_evals.case` and status
`FAILED`, and the case's status rolls up to `FAILED`. Subsequent
cases still execute.

# Environment fan-out

Env setup runs **once per LRO**, not once per suite. If ten suites
share `example-v1` and the RPC selects all of them, `seedExample`
runs once at the start and `cleanupExample` runs once at the end.

If setup returns an error, every case in every dependent suite is
marked `FAILED` with an `_evals.setup` result and teardown for that env
is skipped.

# Cancellation

- Client-side cancellation propagates through the LRO context.
- The `RequestTimeout` in a load `Profile` is always further capped
  by the remaining measurement window so a straggler cannot pollute
  the next case.
- Teardown hooks are still invoked on cancellation.

# Related

* [Overview](/overview.md)
* [Registry](/concepts/registry.md)
* [Environment](/concepts/environment.md)
* [`runner` package](/packages/runner.md)

# Citations

[1] [README — End-to-end lifecycle](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#end-to-end-lifecycle)
[2] [runner/runner.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/runner/runner.go)
