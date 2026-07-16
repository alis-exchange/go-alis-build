---
type: API Reference
title: Suite constructors
description: The exported suite constructors — one per suite kind.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite.go
tags: [api, constructor, suite]
timestamp: 2026-07-08T00:00:00Z
---

# Constructors

| Function | Returns | Notes |
| -------- | ------- | ----- |
| `evals.NewIntegrationSuite(name string, opts ...SuiteOption) (*Suite, error)` | `(*Suite (KindTest), error)` | Integration suite. Returns a typed error on invalid name (empty or containing `.`) or invalid option. |
| `evals.NewAgentEvalSuite(name string, opts ...SuiteOption) (*Suite, error)` | `(*Suite (KindEval), error)` | Agent-eval suite. Same error rules. |
| `evals.NewLoadSuite(name string, opts ...LoadSuiteOption) (*LoadSuite, error)` | `(*LoadSuite, error)` | Load suite. Same error rules. |
| `evals.NewInfraObserveSuite(name string, opts ...InfraObserveSuiteOption) (*InfraObserveSuite, error)` | `(*InfraObserveSuite, error)` | Infra observation suite. Same error rules. |
| `evals.MustNewIntegrationSuite(name string, opts ...SuiteOption) *Suite` | `*Suite (KindTest)` | Like `NewIntegrationSuite` but panics on error. Use in init-time code that would `log.Fatal` on error. |
| `evals.MustNewAgentEvalSuite(name string, opts ...SuiteOption) *Suite` | `*Suite (KindEval)` | Panicking variant of `NewAgentEvalSuite`. |
| `evals.MustNewLoadSuite(name string, opts ...LoadSuiteOption) *LoadSuite` | `*LoadSuite` | Panicking variant of `NewLoadSuite`. |
| `evals.MustNewInfraObserveSuite(name string, opts ...InfraObserveSuiteOption) *InfraObserveSuite` | `*InfraObserveSuite` | Panicking variant of `NewInfraObserveSuite`. |

# Names

Suite names:

- must be non-empty,
- must not contain `.` (which separates suite from case in
  `case_ids`).

Violations return `suite.ErrInvalidSuiteName`.

# Kinds

`Suite.Kind()` reports `KindTest` or `KindEval`. Kinds cannot be
mixed:

- Passing a `KindEval` suite to `RegisterIntegration` returns
  `evals.ErrWrongSuiteKind`.
- Passing a `KindTest` suite to `RegisterEval` returns
  `evals.ErrWrongSuiteKind`.

Load suites do not have a kind field — they're a separate `*LoadSuite`
type registered exclusively via `RegisterLoad`.

# Construction-time errors

Construction-time errors surface as typed values returned from the
constructor — callers decide how to handle them:

- Invalid suite name → `suite.ErrInvalidSuiteName`.
- Duplicate case → `suite.ErrDuplicateCase` (returned from `Case`/`LoadCase`).
- Unknown env → `suite.ErrUnknownEnvironment`.
- Nil case function → `evals.ErrNilCaseFunc`.
- Nil load target → `evals.ErrNilTarget`.

Use the `Must*` variants at package-init when a bad config should
halt the process:

```go
var suite = evals.MustNewIntegrationSuite("example-v1",
    evals.WithEnv("example-v1"),
)
```

# Related

* [Shared suite options](/api/suite-options.md)
* [Load-suite options](/api/load-suite-options.md)
* [Suite](/concepts/suite.md)
* [Errors](/api/errors.md)

# Citations

[1] [suite.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite.go)
