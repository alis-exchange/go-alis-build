---
type: Go Package
title: package evals/env
description: Shared environment registration and activation.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/env
tags: [package, env, environment]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/env` stores and activates shared setup/teardown steps that any
suite can depend on. Environments activate **once per LRO** and share
their state across every suite that references them.

# Public surface

- `env.Register(name string, opts ...Option) error` — returns
  `env.ErrDuplicateRegistration` on duplicate name.
- `env.MustRegister(name string, opts ...Option)` — panicking variant.
- `env.WithSetup(hook Hook) Option`.
- `env.WithTeardown(hook Hook) Option`.
- `env.Get(name string) *Environment`.
- `env.Hook` — `func(context.Context) error`.
- `env.ErrNotRegistered`, `env.ErrSetupFailed` — typed
  `EvalError`s.

# Files

| File | Purpose |
| ---- | ------- |
| `env.go` | Registration and lookup, `Environment` struct. |
| `errors.go` | Typed errors implementing `EvalError`. |
| `doc.go` | Package documentation. |
| `env_test.go` | Tests. |

# Related

* [Environment concept](/concepts/environment.md)
* [Environment API](/api/environment.md)

# Citations

[1] [evals/env tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/env)
