---
type: Suite Kind
title: ADK agent eval (dynamic)
description: Lazy provider that discovers eval sets from a deployed ADK agent at runtime and adapts them to AgentEvalResults.
tags: [adk, agent, dynamic, provider]
timestamp: 2026-07-08T00:00:00Z
---

# Purpose

Rather than writing eval cases in Go, point the framework at a deployed
ADK agent that already publishes its eval sets. The `adk` provider
discovers eval sets over HTTP at run time, filters them against the
incoming `case_ids`, and adapts responses into the same
`AgentEvalResults` shape as in-process eval suites.

Configuration lives entirely on the `adk.Agent` struct that
`adk.NewProvider` wraps — there is no separate registration table.

# Wiring

```go
import (
    _ "go.alis.build/adk/launchers/evals"                // mounts /api/... on the deployed agent
    "go.alis.build/adk/launchers/evals/evaluation/models"

    "go.alis.build/evals"
    "go.alis.build/evals/adk"
)

func Register() {
    provider := adk.NewProvider(adk.Agent{
        BaseURL:    "https://example-agent-...run.app",
        PathPrefix: "/api", // default; override only if the launcher was mounted elsewhere
        AppName:    "example.agent.v1",
        DefaultMetrics: []models.EvalMetric{
            adk.ResponseMatchScore(0.7),
        },
        // Optional: override metrics per eval set.
        // MetricOverrides: map[string][]models.EvalMetric{"regressions": { ... }},
        // Optional: skip eval sets you don't want the runner to touch.
        // IncludeEvalSet: func(id string) bool { return id != "experimental" },
    })
    if err := evals.RegisterAgent(provider); err != nil {
        panic(err)
    }
}
```

# Requirements on the target agent

The **deployed agent** must itself embed
`go.alis.build/adk/launchers/evals` so its `/api/list_eval_sets` and
`/api/run_eval` endpoints are reachable. This is where the launcher
requirement bites: the eval-consuming neuron only needs the launcher
import when it *also* serves ADK agent evals of its own.

# Configuration surface

| Field | Purpose |
| ----- | ------- |
| `BaseURL` | Root URL of the deployed agent. |
| `PathPrefix` | HTTP prefix the launcher was mounted under. Defaults to `/api`. |
| `AppName` | ADK application id. |
| `DefaultMetrics` | Metrics applied to every eval set unless overridden. |
| `MetricOverrides` | Per-eval-set metric list. Fully replaces `DefaultMetrics` for that set. |
| `IncludeEvalSet` | Predicate that filters eval sets before they are exposed to the runner. |

# Context and authentication

The provider's default HTTP client is unauthenticated — suitable for
local or already-authenticated endpoints. When the target sublauncher
requires auth, plug in a custom factory that constructs the client
with your own transport:

```go
factory := func(_ context.Context, baseURL, pathPrefix string) (adk.Client, error) {
    return adk.NewHTTPClient(baseURL,
        adk.WithTransport(myAuthTransport),
        adk.WithPathPrefix(pathPrefix),
    ), nil
}

provider := adk.NewProvider(agent, adk.WithClientFactory(factory))
```

See [`adk` package](/packages/adk.md) for the full client surface.

# Related

* [Agent-eval suite (in-process)](/suites/agent-eval-suite.md)
* [`adk` package](/packages/adk.md)
* [Registration functions](/api/registration.md)

# Citations

[1] [adk/provider.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/adk/provider.go)
[2] [adk/agent.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/adk/agent.go)
[3] [README — Dynamic agent evals via ADK](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#dynamic-agent-evals-via-adk)
