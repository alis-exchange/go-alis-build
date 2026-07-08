---
type: Go Package
title: package evals/runner
description: Environment activation, suite execution, panic recovery, status rollups.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/runner
tags: [package, runner, execution]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/runner` is the workhorse. Given a selection from the registry,
it activates the required environments once, runs each selected suite
sequentially, and produces `execution.RunResult`s for the mapper.

# Responsibilities

- **Environment activation.** Setups fire once, teardowns in
  reverse-registration order.
- **Suite sequencing.** Suites within a run execute in registration
  order.
- **Case execution.** Each case runs under `defer/recover`; panics
  become `panic` leaves with `FAILED` status.
- **Load orchestration.** For load cases, delegates to `loadgen` and
  evaluates every declared `SLO` against the aggregate `Metrics`.
- **Status rollup.** Every leaf, case, suite, and run gets a
  correctly rolled-up `Status`.
- **LRO metadata.** Updates `completed_case_count` and
  `completed_suite_count` on the enclosing LRO as work progresses.

# Public surface

- `runner.Runner` — the concrete type. Its `New()` constructor is
  called by the neuron's wiring code.
- Options for `Runner` configuring env failure behavior and hooks.

# Files

| File | Purpose |
| ---- | ------- |
| `runner.go` | Main orchestration. |
| `env.go`, `env_failure.go` | Environment activation and setup-failure fan-out. |
| `errors.go` | Runner-scoped errors. |
| `doc.go` | Package documentation. |
| `runner_test.go`, `load_test.go`, `features_test.go` | Tests. |

# Related

* [End-to-end lifecycle](/operations/lifecycle.md)
* [Environment concept](/concepts/environment.md)
* [`loadgen` package](/packages/loadgen.md)

# Citations

[1] [evals/runner tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/runner)
