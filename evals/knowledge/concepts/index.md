---
title: concepts
description: Core concepts in the typed evals API.
tags: [concepts]
---

# Concepts

## Suite

A suite is a named collection of cases for one result branch. The constructor
selects the branch:

- `NewIntegrationSuite`
- `NewAgentEvalSuite`
- `NewLoadSuite`
- `NewInfraObservationSuite`

## Case

A case is a named function added with `AddCase("name", fn)`. Case IDs are
qualified on the wire as `{suite}.{case}`.

## Execution

`Run` executes cases and returns the materialized protobuf run without
publishing. `RunAndPublish` executes the same path and then reports the run.

Default concurrency is one active case. `WithMaxConcurrency(n)` allows bounded
parallelism while preserving result order.

The first execution seals the case definition. A sealed suite may be run again,
including concurrently. Calling `AddCase` after sealing records
`ErrSuiteSealed`; the next run returns aggregated `ConfigErrors` without
executing cases.

Run options apply to one invocation:

| Option | Contract |
| --- | --- |
| `WithMaxConcurrency(n)` | Bounds active cases; `n <= 0` is invalid. |
| `WithReporter(r)` | Replaces publication reporter; nil is invalid. |
| `WithBatchID(id)` | Copies a non-empty value to `Run.batch_id`. |
| `WithOperation(name)` | Copies a non-empty value to `Run.operation`. |
| `WithGoogleProjectID(id)` | Overrides `ALIS_OS_PROJECT` for `Run.google_project_id`. |

## Lifecycle

There is no framework-managed environment registry. Use normal Go:

- call setup before `Run`;
- use `defer` for cleanup;
- construct clients in the case or pass them through closures;
- use `context.WithoutCancel` when cleanup must outlive a cancelled run context.

## Result contract

The emitted `evalspb.Run` shape remains compatible with the P0 parity fixtures.
Specialized cases add `validations`; integration cases continue using `checks`.

A suite containing no cases is a valid no-op run with status `PASSED`. A
registered case that emits no branch data is `NOT_EVALUATED`.
