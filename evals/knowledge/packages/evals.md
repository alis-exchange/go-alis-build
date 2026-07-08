---
type: Go Package
title: package evals
description: The public authoring surface — suite constructors, T recorder, SLO constructors, registration.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals
tags: [package, evals, public]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals` is the root package and the only one most consumers import
directly. It exposes:

- Suite constructors: `NewIntegrationSuite`, `NewAgentEvalSuite`, `NewLoadSuite`.
- The `T` recorder and its methods.
- SLO constructors for load suites.
- The `Call`/`Result[T]` RPC wrapper.
- The `Rouge1F1` deterministic scorer.
- Registration functions: `RegisterIntegration`, `RegisterEval`,
  `RegisterLoad`, `RegisterAgent`, `DefaultRegistry`.

# Files

| File | Purpose |
| ---- | ------- |
| `doc.go` | Package-level documentation covering everything below. |
| `suite.go` | `Suite`, `NewIntegrationSuite`, `NewAgentEvalSuite`, `Case`, `SuiteOption`, `StopOnFailure`, `WithEnv`, `WithIdentity`, `WithSetup`, `WithTeardown`. |
| `load.go` | `LoadSuite`, `NewLoadSuite`, `LoadCase`, `LoadSuiteOption`, `WithLoadEnv`, `WithLoadProfile`. |
| `load_profile.go` | `Profile` (re-export), `DefaultLoadProfile`, `ResolveLoadProfile`. |
| `slo.go` | `SLO` type and `SLOLatencyP50/P95/P99`, `SLOErrorRate`, `SLOMinQPS`. |
| `t.go` | `T` and its methods (`Check`, `Checkf`, `NoErr`, `Max`, `Score`). `DuplicateCheckIDName`. |
| `call.go` | `Call`, `Result[T]`. |
| `score.go` | `Rouge1F1`. |
| `register.go` | `RegisterIntegration`, `RegisterEval`, `RegisterLoad`, `RegisterAgent`, `DefaultRegistry`. |
| `suite_test.go`, `load_test.go`, `slo_test.go`, `t_test.go`, `call_test.go`, `example_test.go` | Tests and doc examples. |

# Related

* [API reference](/api/index.md)
* [Concepts](/concepts/index.md)
* [`doc.go`](https://github.com/alis-exchange/go-alis-build/blob/main/evals/doc.go)

# Citations

[1] [evals package tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals)
