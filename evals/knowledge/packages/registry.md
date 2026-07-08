---
type: Go Package
title: package evals/registry
description: Registered suites and providers, filter grammar, selection validation.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/registry
tags: [package, registry, filter]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/registry` implements the process-wide registry that
`TestServiceServer` consumes. It stores suites, agent-eval
providers, and knows how to select subsets against the filter
grammar.

# Public surface

- `registry.Registry` — the concrete type. `DefaultRegistry()` in
  the root `evals` package returns the process-wide one.
- `registry.AgentEvalProvider` — the interface `adk.NewProvider`
  implements.
- `registry.ValidateSelection(caseIDs, kind) error` — the check that
  RPC handlers call synchronously before creating an LRO.
- `registry.ErrNoTestSuites` / `ErrNoEvalSuites` / `ErrNoLoadSuites`,
  `ErrUnknownSuite`, `ErrUnknownCase` — typed `EvalError`s.

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
