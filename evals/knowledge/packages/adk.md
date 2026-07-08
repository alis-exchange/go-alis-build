---
type: Go Package
title: package evals/adk
description: ADK evaluation-launcher client and lazy AgentEvalProvider.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/adk
tags: [package, adk, agent, provider]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/adk` implements a lazy `registry.AgentEvalProvider` that
discovers eval sets from a deployed ADK agent at run time and adapts
responses into the same `AgentEvalResults` shape as in-process eval
suites.

# Public surface

- `adk.Agent` — configuration struct: `BaseURL`, `PathPrefix`,
  `AppName`, `DefaultMetrics`, `MetricOverrides`, `IncludeEvalSet`.
- `adk.NewProvider(agent Agent) *Provider` — constructor.
- `adk.ResponseMatchScore(threshold float64)` — helper that produces
  a common ADK metric.

# Files

| File | Purpose |
| ---- | ------- |
| `agent.go` | `Agent` struct — configuration surface. |
| `provider.go` | `Provider` — implements `registry.AgentEvalProvider`. |
| `client.go` | HTTP client for `/api/list_eval_sets` and `/api/run_eval`. |
| `adapter.go` | Converts ADK responses into `execution` cases. |
| `filter.go` | Filters eval sets by `case_ids` and `IncludeEvalSet`. |
| `auth.go` | Sets `X-Serverless-Authorization`, `X-Alis-Identity`, `X-Alis-Forwarded-Authorization`. |
| `errors.go` | Typed errors for missing agent, HTTP failures. |
| `doc.go` | Package documentation. |
| `*_test.go` | Tests. |

# Requirement on the target agent

The **deployed agent** must embed
`go.alis.build/adk/launchers/evals` so its `/api/list_eval_sets` and
`/api/run_eval` endpoints are reachable.

# Related

* [ADK agent eval (suite guide)](/suites/adk-agent-eval.md)
* [Agent-eval suite (in-process)](/suites/agent-eval-suite.md)

# Citations

[1] [evals/adk tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/adk)
