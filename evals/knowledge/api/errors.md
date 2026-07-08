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
| `suite.ErrInvalidSuiteName` | `evals/suite` | Empty name or name containing `.`. |
| `suite.ErrDuplicateCase` | `evals/suite` | Two cases with the same short name inside one suite. |
| `suite.ErrInvalidCaseName` | `evals/suite` | Case name containing `.`. |
| `suite.ErrUnknownEnvironment` | `evals/suite` | `WithEnv` naming an env that hasn't been registered. |
| `suite.ErrInvalidFilterPath` | `evals/suite` | `case_ids` entry that is not `suite` or `suite.case`. |
| `env.ErrNotRegistered` | `evals/env` | Runner asked for an env that wasn't `env.Register`ed. |
| `env.ErrSetupFailed` | `evals/env` | Env setup hook returned an error; every case in dependent suites is marked with a setup-error result. |
| `registry.ErrNoTestSuites` / `ErrNoEvalSuites` / `ErrNoLoadSuites` | `evals/registry` | Filter matches nothing. |

# Construction-time vs runtime

- **Construction-time errors** (name violations, duplicate cases,
  unknown envs) surface at `evals.NewSuite` / `evals.NewEvalSuite` /
  `evals.NewLoadSuite` as **panics** wrapping the typed error — the
  intent is to fail loudly at process init so misconfigured neurons
  never start.
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
