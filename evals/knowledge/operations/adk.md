---
title: ADK provider operation
description: Running ADK eval cases inside AgentEvalSuite, and the deprecated envelope path.
tags: [adk, provider, agent-eval, reporting, suite]
---

# ADK provider operation

`adk.NewProvider` discovers and executes eval sets exposed by an ADK
sublauncher. It is not a suite registry and does not publish.

## Suite bridge (recommended)

Run each ADK case as an `AgentEvalSuite` case, so durations are measured per
case and `WithMaxConcurrency`, cancellation, and `RunAndPublish` apply:

```go
ids, err := provider.ListCases(ctx, set)
if err != nil {
    return err
}
suite := evals.NewAgentEvalSuite(set)
for _, id := range ids {
    suite.AddCase(adk.SuiteCaseName(id), func(ctx context.Context, r *evals.AgentEvalResult) {
        res, err := provider.RunCase(ctx, set, id)
        if err != nil {
            r.Fail(err)
            return
        }
        res.RecordTo(r)
    })
}
run, err := suite.Run(ctx, evals.WithMaxConcurrency(4))
```

The case closure captures the loop variable `id`. That relies on Go 1.22+
per-iteration loop variables; the module declares `go 1.26`. Code built with
an older `go` directive must copy `id` inside the loop.

- **Case names.** `SuiteCaseName` replaces `.` with `_`, because suite case
  names cannot contain `.`. Suite case ids become
  `{suite}.{SuiteCaseName(id)}`.
- **Listing.** `Provider.ListCases` needs a client that implements
  `adk.CaseLister`. `HTTPClient` does; a custom `adk.Client` without it makes
  `ListCases` return `adk.ErrCaseListingUnsupported`.
- **Order.** Case ids come back in the order the client returns them; the
  HTTP launcher sorts them by id.
- **Status.** ADK `FAILED` fails the suite case with an `_evals.case`
  validation `adk: final eval status FAILED`. ADK `NOT_EVALUATED` gives a
  `NOT_EVALUATED` suite case that keeps its session, metrics and judge,
  recorded through `AgentEvalResult.SetNotEvaluated`. It is never `PASSED`,
  and a failure (failed metric, failed validation, `Fail`) still wins. The run
  is then `NOT_EVALUATED` unless some case failed.
- **Judge.** Each case declares its own judge. Mixing judge models or model
  versions across cases in one suite fails the conflicting cases with an
  `_evals.judge` validation.

## Envelope path (deprecated)

`ProviderResult.Run` is deprecated. `Provider.Run` returns `[]adk.ProviderResult`. Each entry contains:

- the ADK eval-set name in `SuiteName`;
- measured `StartTime` and `EndTime`; and
- protobuf-native `*evalspb.AgentEvalResults` in `Results`.

Call `result.Run()` to construct the branch-correct `*evalspb.Run` with its
identity, measured timestamps, and case-status rollup. The caller sets optional
metadata and sends it through the chosen `report.Reporter`.

ADK exposes total elapsed time for an eval-set execution, not an independent
duration for each returned case. The provider divides the total evenly across
cases. Treat case durations as approximations; `ProviderResult.StartTime` and
`EndTime` preserve the measured set-level interval.

Normal provider failures remain normal Go errors. The caller decides whether to
stop orchestration or represent the failure as evaluation data elsewhere.
