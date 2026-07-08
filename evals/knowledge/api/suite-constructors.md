---
type: API Reference
title: Suite constructors
description: The three exported suite constructors — one per suite kind.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite.go
tags: [api, constructor, suite]
timestamp: 2026-07-08T00:00:00Z
---

# Constructors

| Function | Returns | Notes |
| -------- | ------- | ----- |
| `evals.NewSuite(name string, opts ...SuiteOption) *Suite` | `*Suite` (`KindTest`) | Integration suite. Panics on invalid name (empty or containing `.`) or invalid option. |
| `evals.NewEvalSuite(name string, opts ...SuiteOption) *Suite` | `*Suite` (`KindEval`) | Agent-eval suite. Same panic rules. |
| `evals.NewLoadSuite(name string, opts ...LoadSuiteOption) *LoadSuite` | `*LoadSuite` | Load suite. Same panic rules. |

# Names

Suite names:

- must be non-empty,
- must not contain `.` (which separates suite from case in
  `case_ids`).

Violations panic with `suite.ErrInvalidSuiteName`.

# Kinds

`Suite.Kind()` reports `KindTest` or `KindEval`. Kinds cannot be
mixed:

- Passing a `KindEval` suite to `RegisterIntegration` panics.
- Passing a `KindTest` suite to `RegisterEval` panics.

Load suites do not have a kind field — they're a separate `*LoadSuite`
type registered exclusively via `RegisterLoad`.

# Panics at construction

Construction-time errors surface as **panics** wrapping the typed
error — the intent is to fail loudly at process init so misconfigured
neurons never start:

- Invalid suite name → `suite.ErrInvalidSuiteName`.
- Duplicate case → `suite.ErrDuplicateCase` (raised by `Case`/`LoadCase`).
- Unknown env → `suite.ErrUnknownEnvironment`.

# Related

* [Shared suite options](/api/suite-options.md)
* [Load-suite options](/api/load-suite-options.md)
* [Suite](/concepts/suite.md)
* [Errors](/api/errors.md)

# Citations

[1] [suite.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite.go)
