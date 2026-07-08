---
type: API Reference
title: Shared suite options (test + eval)
description: Options accepted by both `NewSuite` and `NewEvalSuite`. Load suites use a separate option set.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite.go
tags: [api, options, suite]
timestamp: 2026-07-08T00:00:00Z
---

# `SuiteOption`

All apply to both `NewSuite` (integration) and `NewEvalSuite` (agent
eval).

| Option | Effect |
| ------ | ------ |
| `evals.WithEnv(names ...string)` | Declare shared environments. Every name must have been passed to [`env.Register`](/api/environment.md) before the suite is constructed, or the constructor panics with `suite.ErrUnknownEnvironment`. |
| `evals.WithSetup(hook suite.SuiteHook)` | Runs once per LRO before the suite's cases. Signature: `func(ctx context.Context) error`. Failure fails every case in the suite with a `setup` marker and skips teardown. |
| `evals.WithTeardown(hook suite.SuiteHook)` | Runs once after the suite's cases (or before propagating cancellation). Errors are logged but ignored. |
| `evals.WithContext(fn evals.ContextDecorator)` | Install a `func(ctx) ctx` applied to the suite's setup, teardown, and every case body. The framework's only auth-adjacent surface: use it to stamp caller identity, auth headers, tokens, tracing state, or any request-scoped values. The framework itself attaches no auth. |
| `evals.StopOnFailure()` | Once any case in the suite ends non-`PASSED`, remaining cases are recorded `NOT_EVALUATED` with a "preceding case … failed" reason. Use for stateful flows. |

# Hook signatures

```go
type SuiteHook func(ctx context.Context) error
```

The hook receives the LRO context. Its error is treated as follows:

- **Setup**: return non-nil → suite's cases are all marked with a
  `setup` failure marker; teardown is skipped.
- **Teardown**: return non-nil → logged as `alog.Warn`; case
  outcomes are not affected.

# Composability

Options apply in the order supplied. `WithSetup` / `WithTeardown` set
the corresponding hook; later options overwrite earlier ones for the
same slot.

# Related

* [Load-suite options](/api/load-suite-options.md)
* [Environment API](/api/environment.md)
* [StopOnFailure](/operations/stop-on-failure.md)

# Citations

[1] [suite.go — Options](https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite.go)
