---
type: Concept
title: Registry
description: Process-global publish point for suites and agent-eval providers. Mirrors `http.DefaultServeMux`.
tags: [registry, publish, filter]
timestamp: 2026-07-08T00:00:00Z
---

# Definition

The **registry** is a process-global map of suites and lazy providers.
It mirrors `http.DefaultServeMux` — anywhere in the binary can publish
into it at `init()`, and the RPC server reads from it at request time.

Registration happens through four functions:

- `evals.RegisterIntegration(*Suite)`
- `evals.RegisterEval(*Suite)`
- `evals.RegisterLoad(*LoadSuite)`
- `evals.RegisterAgent(registry.AgentEvalProvider)`

All four target `evals.DefaultRegistry()`.

# Selection

`Registry.ValidateSelection(caseIDs, kind)` synchronously rejects
unknown suite or case ids with `InvalidArgument` before an LRO is
created. Selection follows the [filter grammar](/operations/filter-grammar.md):

| Filter entry | Selects |
| ------------ | ------- |
| _empty list_ | Every registered suite of the requested kind. |
| `"my-suite"` | Every case in `my-suite`. |
| `"my-suite.foo"` | Just the `foo` case in `my-suite`. |

Filters union; mixing suite-scoped and case-scoped entries against the
same suite promotes to whole-suite selection.

# Lazy providers

`RegisterAgent(provider)` publishes a lazy provider — a
`registry.AgentEvalProvider` — instead of a static `Suite`. The
canonical implementation is `adk.NewProvider`, which discovers eval
sets over HTTP against a deployed ADK agent and produces cases at
runtime. See [ADK agent eval](/suites/adk-agent-eval.md).

# Errors

- `registry.ErrNoTestSuites`, `ErrNoEvalSuites`, `ErrNoLoadSuites` —
  filter matches nothing.
- `registry.ErrUnknownSuite`, `ErrUnknownCase` — unknown ids in the
  filter.

All implement [`EvalError`](/concepts/reporter.md#errors) and translate
cleanly to gRPC statuses at the RPC boundary.

# Related

* [Filter grammar](/operations/filter-grammar.md)
* [`registry` package](/packages/registry.md)
* [Registration functions](/api/registration.md)

# Citations

[1] [registry/registry.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/registry/registry.go)
[2] [register.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/register.go)
