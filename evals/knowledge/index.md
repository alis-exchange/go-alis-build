---
title: evals knowledge
description: Current knowledge bundle for go.alis.build/evals after typed-suite simplification.
tags: [evals, typed-suites, runs]
---

# evals knowledge

`go.alis.build/evals` now exposes four explicit typed suite APIs:

- `evals.NewIntegrationSuite`
- `evals.NewAgentEvalSuite`
- `evals.NewLoadSuite`
- `evals.NewInfraObservationSuite`

There is no registry, no `evals.T`, no framework-managed environment lifecycle,
and no runner/mapper/execution layer for developers to configure. Developers use
ordinary Go for setup, teardown, clients, credentials, and orchestration.

Start here:

- [Overview](overview.md)
- [Concepts](concepts/index.md)
- [Suites](suites/index.md)
- [Packages](packages/index.md)
- [Operations](operations/lifecycle.md)
- [ADK provider operation](operations/adk.md)
- [Wire contract](wire-types/run.md)
