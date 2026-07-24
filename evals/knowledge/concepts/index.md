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

## Lifecycle

There is no framework-managed environment registry. Use normal Go:

- call setup before `Run`;
- use `defer` for cleanup;
- construct clients in the case or pass them through closures;
- use `context.WithoutCancel` when cleanup must outlive a cancelled run context.

## Result contract

The emitted `evalspb.Run` shape remains compatible with the P0 parity fixtures.
Specialized cases add `validations`; integration cases continue using `checks`.
