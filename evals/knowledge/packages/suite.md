---
type: Go Package
title: package evals/suite
description: Internal TestSuite, EvalSuite, LoadSuite primitives that back the public constructors.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/suite
tags: [package, suite, internal]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/suite` holds the concrete `TestSuite`, `EvalSuite`, and
`LoadSuite` types that the root `evals` package wraps. It also
defines the typed errors that suite construction can produce.

The split exists so the public `evals` package stays small and
`registry`, `runner`, and other subpackages can share the same
underlying suite primitives without importing the public surface.

# Public surface

- `suite.TestSuite`, `suite.EvalSuite`, `suite.LoadSuite`.
- `suite.SuiteHook` — `func(context.Context) error`.
- `suite.SuiteKind` enum (`KindTest`, `KindEval`).
- Typed errors: `ErrInvalidSuiteName`, `ErrInvalidCaseName`,
  `ErrDuplicateCase`, `ErrUnknownEnvironment`, `ErrInvalidFilterPath`.

# Files

| File | Purpose |
| ---- | ------- |
| `suite.go` | Suite structs, add-case methods. |
| `types.go` | Shared types and enums. |
| `errors.go` | Typed errors implementing `EvalError`. |
| `doc.go` | Package documentation. |
| `suite_test.go`, `errors_test.go`, `load_test.go` | Tests. |

# Related

* [Suite concept](/concepts/suite.md)
* [Errors API](/api/errors.md)

# Citations

[1] [evals/suite tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/suite)
