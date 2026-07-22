---
type: Concept
title: Suite
description: A named group of related cases plus optional environment dependencies and lifecycle hooks.
tags: [suite, core]
timestamp: 2026-07-08T00:00:00Z
---

# Definition

A **suite** is the unit of registration and shared lifecycle in the
framework. Every case belongs to exactly one suite. Four suite kinds
exist, one per RPC on the deployed `TestService`:

| Kind | Constructor | Register with | Case type |
| ---- | ----------- | ------------- | --------- |
| Integration test | `evals.NewIntegrationSuite` | `evals.RegisterIntegration` | `func(ctx, *T)` |
| Agent eval       | `evals.NewAgentEvalSuite` | `evals.RegisterEval` / `evals.RegisterAgent` | `func(ctx, *T)` |
| Load             | `evals.NewLoadSuite` | `evals.RegisterLoad` | `ResultTarget` + `[]SLO` |
| Infra observation | `evals.NewInfraObserveSuite` | `evals.RegisterInfraObserve` | lookback + infra targets |

# Properties

- **Name.** A non-empty string that must not contain `.`. Case ids are
  `{suite-name}.{case-name}`; the dot separator is why names cannot
  themselves contain a dot.
- **Kind.** Reported by `Suite.Kind()` as `KindTest` or `KindEval`.
  Kinds cannot be mixed — passing a `KindEval` suite to
  `RegisterIntegration` returns `evals.ErrWrongSuiteKind`.
- **Environments.** Declared via `WithEnv(...)` / `WithLoadEnv(...)`.
  [`evals.Freeze`](/concepts/registry.md) validates every name after all
  environments and suites have been registered.
- **Lifecycle hooks.** `WithSetup` / `WithTeardown` run once per LRO.
  Setup failure fails every case with an `_evals.setup` marker and skips
  teardown. Teardown failure marks every completed case failed with an
  `_evals.teardown` diagnostic.
- **Context decoration.** `WithContext(fn)` installs a
  `func(ctx) ctx` applied to the suite's setup, teardown, and every
  case body. The framework's only auth-adjacent surface: use it to
  stamp caller identity, auth headers, tokens, tracing state, or any
  request-scoped values. The framework itself never attaches auth.
- **Failure propagation.** `StopOnFailure`, `WithLoadStopOnFailure`, and
  `WithInfraObserveStopOnFailure` mark subsequent cases `NOT_EVALUATED` once
  any case fails. Infra observation becomes sequential when enabled. See
  [StopOnFailure](/operations/stop-on-failure.md).

# Uniqueness

Case names must be unique within a suite; error-returning registration methods
return `suite.ErrDuplicateCase`, while their `Must*` variants panic. Suite names
must be unique within a kind; duplicate registry registration returns
`registry.ErrDuplicateSuite`.

# Related

* [Suite constructors](/api/suite-constructors.md)
* [Shared suite options](/api/suite-options.md)
* [Load-suite options](/api/load-suite-options.md)
* [Registry](/concepts/registry.md)

# Citations

[1] [suite.go — NewSuite / NewEvalSuite / NewLoadSuite](https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite.go)
