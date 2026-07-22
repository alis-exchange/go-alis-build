---
type: API Reference
title: Errors
description: The `EvalError` interface and helpers for translating framework errors into gRPC statuses.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/errors/errors.go
tags: [api, errors, grpc]
timestamp: 2026-07-08T00:00:00Z
---

# Interface

The framework surfaces validation errors (unknown environment,
invalid suite name, unknown case_ids, etc.) as typed values that
also implement `GRPCStatus()` so the RPC boundary translates them
cleanly:

```go
type EvalError interface {
    error
    GRPCStatus() *status.Status
}
```

The interface lives in `go.alis.build/evals/errors`. Callers rarely
construct these directly.

# Helpers

Four helpers convert to and inspect gRPC statuses at the RPC boundary:

| Helper | Purpose |
| ------ | ------- |
| `errors.ToGRPC(err error) error` | Preserve the underlying status code when `err` is an `EvalError`; otherwise return `codes.InvalidArgument`. Returns `nil` on `nil`. |
| `errors.ToGRPCf(field string, err error)` | Same, prefixing the message with `field:` for RPC validation errors. |
| `errors.IsEval(err error) bool` | Reports whether `err` (or any wrapped cause) implements `EvalError`. |
| `errors.Code(err error) codes.Code` | Returns the `codes.Code` for an `EvalError`, or `codes.Unknown`. |

# Concrete error types

All implement `EvalError`.

| Error | Package | Triggered by |
| ----- | ------- | ------------ |
| `evals.ErrNilCaseFunc` | `evals` | `Suite.Case` called with a nil case function. |
| `evals.ErrNilTarget` | `evals` | `LoadSuite.LoadCase` called with a nil target. |
| `evals.ErrNilProvider` | `evals` | `RegisterAgent` called with a nil provider. |
| `evals.ErrWrongSuiteKind` | `evals` | Suite passed to the wrong `Register*` (e.g. eval suite to `RegisterIntegration`). |
| `evals.ErrUnknownSuiteKind` | `evals` | Internal invariant: `Suite.Case` invoked on a `Suite` whose `kind` field is neither `KindTest` nor `KindEval`. Maps to `codes.Internal`. |
| `suite.ErrNilSuite` | `evals/suite` | Registration or case-add on a nil suite. |
| `suite.ErrInvalidSuiteName` | `evals/suite` | Empty name or name containing `.`. |
| `suite.ErrDuplicateCase` | `evals/suite` | Two cases with the same short name inside one suite. |
| `suite.ErrInvalidCaseName` | `evals/suite` | Case name containing `.`. |
| `registry.ErrUnknownEnvironments` | `evals/registry` | `Freeze` found one or more suite environment names absent from its environment registry. |
| `suite.ErrInvalidFilterPath` | `evals/suite` | `case_ids` entry that is not `suite` or `suite.case`. |
| `suite.ErrLoadProfileUnspecifiedMode` | `evals/suite` | `WithLoadProfile` targeting `MODE_UNSPECIFIED`. |
| `env.ErrDuplicateRegistration` | `evals/env` | `env.Register` called twice for the same name. |
| `env.ErrNotRegistered` | `evals/env` | Runner asked for an env that wasn't `env.Register`ed. |
| `env.ErrSetupFailed` | `evals/env` | Env setup hook returned an error; every case in dependent suites is marked with a setup-error result. |
| `registry.ErrNoSuites` / `ErrNoEvalSuites` / `ErrNoLoadSuites` / `ErrNoInfraObserveSuites` | `evals/registry` | No suites are registered for the requested run type. |
| `registry.ErrUnknownCase` | `evals/registry` | A non-empty `case_ids` filter matched no registered case. |

# Construction-time vs runtime

- **Startup errors** (name violations, duplicate cases, unknown environments
  at Freeze, wrong-kind registration) are returned by
  `evals.NewIntegrationSuite` / `evals.NewAgentEvalSuite` /
  `evals.NewLoadSuite`, `Suite.Case` / `LoadSuite.LoadCase`, and the
  `Register*` functions. Callers decide whether to `log.Fatal`,
  propagate, or ignore. The `MustNew*` / `MustCase` / `MustLoadCase`
  and `env.MustRegister` variants panic for init-time code that would
  otherwise `log.Fatal` on an error.
- **Runtime-discovered errors** (unknown filter path, env setup
  failure) surface via `EvalError` and are translated to a gRPC
  status by the RPC handlers.

# Related

* [`errors` package](/packages/errors.md)
* [Registry](/concepts/registry.md)
* [Environment](/concepts/environment.md)

# Citations

[1] [errors/errors.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/errors/errors.go)
[2] [suite/errors.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite/errors.go)
[3] [env/errors.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/env/errors.go)
[4] [registry/errors.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/registry/errors.go)
