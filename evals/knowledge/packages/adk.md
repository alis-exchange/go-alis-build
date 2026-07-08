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
- `adk.NewProvider(agent Agent, opts ...ProviderOption) *Provider` —
  provider constructor.
- `adk.WithClientFactory(fn)` — override the default HTTP client
  factory (typically to install authentication on the client).
- `adk.NewHTTPClient(baseURL string, opts ...HTTPClientOption) *HTTPClient`
  — transport-agnostic HTTP client for the sublauncher.
- `adk.WithTransport(rt http.RoundTripper)` — install any auth
  transport (bearer, oauth2, Cloud Run ID token, mTLS, etc.).
- `adk.WithTimeout(d time.Duration)` — override the default
  10-minute request timeout.
- `adk.WithPathPrefix(prefix string)` — override the default `/api`
  path prefix.
- `adk.AudienceFromBaseURL(baseURL string)` — URL helper for callers
  minting Cloud Run ID tokens.
- `adk.ResponseMatchScore(threshold float64)` — helper that produces
  a common ADK metric.

# Files

| File | Purpose |
| ---- | ------- |
| `agent.go` | `Agent` struct — configuration surface. |
| `provider.go` | `Provider` — implements `registry.AgentEvalProvider`. |
| `client.go` | Transport-agnostic HTTP client for `/api/list_eval_sets` and `/api/run_eval`. |
| `adapter.go` | Converts ADK responses into `execution` cases. |
| `filter.go` | Filters eval sets by `case_ids` and `IncludeEvalSet`. |
| `url.go` | URL parsing helpers (`AudienceFromBaseURL`). |
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
