---
title: ADK provider operation
description: Running ADK eval sets and assembling Run envelopes.
tags: [adk, provider, agent-eval, reporting]
---

# ADK provider operation

`adk.NewProvider` discovers and executes eval sets exposed by an ADK
sublauncher. It is not a suite registry and does not publish.

```go
provider := adk.NewProvider(agent)
results, err := provider.Run(ctx, filters)
```

`Provider.Run` returns `[]adk.ProviderResult`. Each entry contains:

- the ADK eval-set name in `SuiteName`;
- measured `StartTime` and `EndTime`; and
- protobuf-native `*evalspb.AgentEvalResults` in `Results`.

The caller owns the outer `*evalspb.Run`: generate its identity, set metadata
and status, attach `Results` through `Run_AgentEval`, and send it through the
chosen `report.Reporter`.

ADK exposes total elapsed time for an eval-set execution, not an independent
duration for each returned case. The provider divides the total evenly across
cases. Treat case durations as approximations; `ProviderResult.StartTime` and
`EndTime` preserve the measured set-level interval.

Normal provider failures remain normal Go errors. The caller decides whether to
stop orchestration or represent the failure as evaluation data elsewhere.
