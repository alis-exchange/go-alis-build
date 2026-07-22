---
type: API Reference
title: Environment API
description: The `env` package — `Register`, `WithSetup`, `WithTeardown`, `Get`.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/env/env.go
tags: [api, env, environment]
timestamp: 2026-07-08T00:00:00Z
---

# Package `env`

```go
package env

func Register(name string, opts ...Option) error       // returns ErrDuplicateRegistration
func MustRegister(name string, opts ...Option)         // panics on duplicate
func WithSetup(hook Hook) Option
func WithTeardown(hook Hook) Option
func Get(name string) *Environment

type Hook func(context.Context) error

func New() *Registry
func (r *Registry) Register(name string, opts ...Option) error
func (r *Registry) Get(name string) *Environment
```

# Functions

| Function | Effect |
| -------- | ------ |
| `env.Register(name, opts...) error` | Register a globally-named environment. Returns `env.ErrDuplicateRegistration` if `name` was already registered. |
| `env.MustRegister(name, opts...)` | Like `Register` but panics on error. Use at package init when a duplicate should halt the process. |
| `env.WithSetup(hook)` | Optional setup, invoked once per LRO if any selected suite depends on this env. |
| `env.WithTeardown(hook)` | Optional teardown, invoked in reverse-registration order after all suites finish. |
| `env.Get(name)` | Look up a registered environment. Returns nil for unknown names. |

# Default and isolated registries

The package-level functions use one process-global default registry. Tests and
embedders can create an isolated registry with `env.New()`, register directly
on it, and attach it to a suite registry with `SetEnvRegistry`.

# Failure surface

- Setup failure: `env.ErrSetupFailed` is surfaced on every selected case as a
  `FAILED` `_evals.setup` diagnostic.
- Missing env: `env.ErrNotRegistered` when the runner is asked to
  activate an environment absent from the run's registry. Calling
  `evals.Freeze()` before serving normally catches this at startup.

# Related

* [Environment concept](/concepts/environment.md)
* [`env` package](/packages/env.md)

# Citations

[1] [env/env.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/env/env.go)
[2] [env/errors.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/env/errors.go)
