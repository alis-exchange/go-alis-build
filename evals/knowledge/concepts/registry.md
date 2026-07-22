---
type: Concept
title: Registry
description: Default or isolated publish point for suites, environments, and agent-eval providers.
tags: [registry, publish, filter]
timestamp: 2026-07-08T00:00:00Z
---

# Definition

The root package exposes one process-global default registry, mirroring
`http.DefaultServeMux`: case packages can publish during startup and the RPC
server reads it at request time. `registry.New()` creates an isolated registry
for tests or embedders.

Registration into the default registry happens through five functions:

- `evals.RegisterIntegration(*Suite)`
- `evals.RegisterEval(*Suite)`
- `evals.RegisterLoad(*LoadSuite)`
- `evals.RegisterInfraObserve(*InfraObserveSuite)`
- `evals.RegisterAgent(registry.AgentEvalProvider)`

All five target `evals.DefaultRegistry()` and return errors that callers must
check.

# Freeze

After registration completes, call `evals.Freeze()` (or `Registry.Freeze()`
for an isolated registry) before serving requests. Freeze validates duplicate
suite names, environment references, and load profile overrides, then rejects
later registration with `ErrRegistryFrozen`.

`Registry.SetEnvRegistry` selects the isolated environment registry used both
for Freeze validation and for runs selected from that registry. Call it before
Freeze; changing it afterward returns `ErrRegistryFrozen`.

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

- `registry.ErrNoSuites`, `ErrNoEvalSuites`, `ErrNoLoadSuites`, and
  `ErrNoInfraObserveSuites` — no suites exist for the requested run type.
- `registry.ErrUnknownCase` — a filter does not match a registered case.
- `registry.ErrDuplicateSuite`, `ErrUnknownEnvironments`, and
  `ErrRegistryFrozen` — startup registration or Freeze validation failed.

All implement [`EvalError`](/concepts/reporter.md#errors) and translate
cleanly to gRPC statuses at the RPC boundary.

# Related

* [Filter grammar](/operations/filter-grammar.md)
* [`registry` package](/packages/registry.md)
* [Registration functions](/api/registration.md)

# Citations

[1] [registry/registry.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/registry/registry.go)
[2] [register.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/register.go)
