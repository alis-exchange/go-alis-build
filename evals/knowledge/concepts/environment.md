---
type: Concept
title: Environment
description: Shared setup and teardown identified by name, activated once per LRO across all suites that reference it.
tags: [environment, setup, teardown]
timestamp: 2026-07-08T00:00:00Z
---

# Definition

An **environment** is a named pair of `Setup` and `Teardown` hooks that
one or more suites can depend on. Environments are registered once
(usually in `init()`) via `env.Register` and referenced by name via
`WithEnv` / `WithLoadEnv` on suites.

```go
env.Register("example-v1",
    env.WithSetup(seedExample),
    env.WithTeardown(cleanupExample),
)
```

# Activation semantics

- **Once per LRO.** An environment activates exactly once per
  `RunIntegrationTest` / `RunAgentEval` / `RunLoadTest` invocation,
  regardless of how many selected suites reference it. This lets
  several suites share expensive setup (seeding a database, warming a
  cache) without paying the cost repeatedly.
- **Teardown order.** Environments tear down in reverse registration
  order after all suites finish, whether the run succeeded or was
  cancelled.
- **Failure fans out.** If setup for an environment returns an error,
  every case in every dependent suite is recorded with an
  `env-setup-failed` marker; teardown for that environment is skipped.
- **Teardown errors are logged.** They do not affect case outcomes.

# Process-globality

Environments are stored in a **process-wide map**. Re-entrant libraries
must gate registration with `sync.Once` or check `env.Get(name)`
first; duplicate registration panics with `env.ErrDuplicateName`.

# Related

* [Environment API](/api/environment.md)
* [`env` package](/packages/env.md)
* [Suite](/concepts/suite.md)

# Citations

[1] [env/env.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/env/env.go)
