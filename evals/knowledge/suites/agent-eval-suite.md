---
title: agent eval suites
description: Agent eval builder usage.
tags: [agent-eval, builders]
---

# Agent eval suites

Agent eval cases receive `*evals.AgentEvalResult`.

Use the builder to add protobuf-native metrics, session IDs, judge info, and
general validation rules:

```go
suite := evals.NewAgentEvalSuite("assistant-quality").
    AddCase("answers-correctly", func(ctx context.Context, r *evals.AgentEvalResult) {
        r.SetSessionID(sessionID)
        r.AddMetric(metricProto)
        r.SetJudgeInfo(judgeProto)
        r.Validator().Custom("session.persisted", sessionID != "")
    })
```

`r.Fail(err)` marks the case failed without discarding already recorded data.
Validation rules are emitted under `AgentEvalResults.Case.validations`.

Builder values are cloned when added. `SetSessionID` and `SetJudgeInfo` are
singletons: the first value wins and a repeated setter fails the case while
retaining the first value. Nil metrics/judge values also fail the case.
`Fail(nil)` is a no-op.

An empty builder is `NOT_EVALUATED`. Failed metrics, broken validation rules,
or `Fail(err)` fail the case while retaining partial data.

ADK eval cases can run as cases of this suite. Use `adk.Provider.ListCases`,
`Provider.RunCase`, and `ProviderCase.RecordTo`, which records into
`*evals.AgentEvalResult` through `adk.CaseRecorder`. An ADK
`NOT_EVALUATED` case calls `AgentEvalResult.SetNotEvaluated`, which keeps
the case `NOT_EVALUATED` with its data unless something failed. See
[ADK provider operation](../operations/adk.md).
