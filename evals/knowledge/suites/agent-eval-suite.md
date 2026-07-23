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

The `evals/adk` package provides ADK-specific conversion helpers that return
protobuf-native `AgentEvalResults` data.
