---
type: Go Package
title: package evals/registry
description: Registered suites and providers, filter grammar, selection validation.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/registry
tags: [package, registry, filter]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/registry` implements the default or isolated registry consumed by
`TestServiceServer`. It stores suites and agent-eval providers, validates
startup configuration, and selects subsets against the filter grammar.

# Public surface

- `registry.Registry` — the concrete type. `DefaultRegistry()` in
  the root `evals` package returns the process-wide one; `registry.New()`
  creates an isolated instance.
- `registry.AgentEvalProvider` — the interface `adk.NewProvider`
  implements.
- `(*Registry).Freeze() error` — validates and seals startup registration.
- `(*Registry).SetEnvRegistry(*env.Registry) error` — supplies the environment
  registry used by Freeze and selected runs.
- `(*Registry).ValidateSelection(kind, caseIDs) error` — the check that
  RPC handlers call synchronously before creating an LRO.
- `registry.ErrNoSuites`, `ErrNoEvalSuites`, `ErrNoLoadSuites`,
  `ErrNoInfraObserveSuites`, `ErrUnknownCase`, `ErrDuplicateSuite`,
  `ErrUnknownEnvironments`, and `ErrRegistryFrozen` — typed errors.

# Files

| File | Purpose |
| ---- | ------- |
| `registry.go` | `Registry` type, selection validation. |
| `errors.go` | Typed errors implementing `EvalError`. |
| `doc.go` | Package documentation. |
| `registry_test.go`, `load_test.go` | Tests. |

# Related

* [Registry concept](/concepts/registry.md)
* [Filter grammar](/operations/filter-grammar.md)
* [Registration functions](/api/registration.md)

# Citations

[1] [evals/registry tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/registry)
