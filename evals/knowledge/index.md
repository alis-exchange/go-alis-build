---
okf_version: "0.1"
title: go.alis.build/evals — Knowledge Bundle
description: OKF-formatted knowledge base for the evals framework — integration tests, agent evaluations, and load tests against live services.
tags: [evals, testing, go, alis-build]
timestamp: 2026-07-08T00:00:00Z
---

# go.alis.build/evals

`evals` is a Go framework for writing four kinds of post-deploy test
against live services:

- **Integration tests** — behavioural contracts on the gRPC surface.
- **Load tests** — traffic generation at declared intensities with SLO
  evaluation.
- **Agent evaluations** — deterministic scorers, LLM-as-judge scores,
  and rubric dimensions against agent transcripts.
- **Infra observation** — Cloud Run and Spanner Monitoring snapshots
  over a settled lookback window (standalone) or attached to load cases.

Suites are authored in Go, registered once at `init()`, and exposed via
four LRO-backed RPCs on the deployed `TestService`. Each completed
suite becomes a `Run` proto that fans out to whichever reporters
(Pub/Sub, BigQuery, log) the neuron wires up.

# Getting oriented

Start with the [Overview](/overview.md) for the mental model, then
follow the section indexes below for depth.

# Contents

* [Overview](/overview.md) — mental model, four suite kinds, and how a run flows through the framework.
* [Concepts](/concepts/) — the reusable abstractions: suite, case, T recorder, environment, registry, reporter, run, status.
* [Suites](/suites/) — the four suite kinds and how to author each one.
* [API reference](/api/) — every exported type, function, and option.
* [Wire types](/wire-types/) — proto messages consumers receive from reporters.
* [Operations](/operations/) — quickstart, filter grammar, authentication, lifecycle, load-mode presets.
* [Packages](/packages/) — one page per Go subpackage in the module.

# Bundle conventions

- The bundle targets **OKF v0.1**. See [SPEC.md][spec].
- Every non-reserved `.md` file has parseable YAML frontmatter with a
  `type` field.
- Cross-links use **bundle-relative** absolute paths (starting with `/`).
- `resource:` on API documents points to a GitHub blob URL for the
  underlying Go source when applicable.

[spec]: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
