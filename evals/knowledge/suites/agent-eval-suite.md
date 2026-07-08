---
type: Suite Kind
title: Agent-eval suite
description: Grade agent transcripts with deterministic checks, LLM-as-judge scores, and rubric dimensions.
tags: [agent, eval, llm, judge]
timestamp: 2026-07-08T00:00:00Z
---

# Purpose

An **agent-eval suite** grades outputs from an agent — deterministic
scorers, LLM-as-judge scores, or arbitrary in-process Go logic. Cases
share the `T` recorder with integration suites; the wire mapping
turns leaves into `Metric`s instead of `Check`s.

# Anatomy

```go
package v1

import (
    "context"

    "go.alis.build/evals"

    "example.com/internal/clients"
    "example.com/internal/judge"
    agentpb "example.com/pb/example/agent/v1"
)

func Register() {
    s := evals.MustNewAgentEvalSuite("example-agent-v1",
        evals.WithEnv("agent-runtime"),
    )

    s.MustCase("golden-short-summary", func(ctx context.Context, t *evals.T) {
        r := evals.Call(ctx, func(ctx context.Context) (*agentpb.Reply, error) {
            return clients.Agent.Chat(ctx, prompt)
        })
        if !t.NoErr("transport", r.Err) {
            return
        }

        // Deterministic scorer bundled with the framework.
        t.Score("rouge-1", evals.Rouge1F1(r.Resp.GetText(), golden), 0.5, "vs golden")

        // LLM-as-judge is just a plain Go call whose output you feed in.
        grade, err := judge.Grade(ctx, r.Resp.GetText())
        if !t.NoErr("judge", err) {
            return
        }
        t.Score("judge.coherence", grade.Coherence, 0.7, grade.Rationale)
        t.Score("judge.factuality", grade.Factuality, 0.9, grade.FactRationale)

        // A binary check (no score) surfaces as a metric with no `score` field.
        t.Check("no-refusal", !grade.Refused)
    })

    if err := evals.RegisterEval(s); err != nil {
        panic(err)
    }
}
```

# Primary primitive

`T.Score(id, score, threshold, rationale)` is the distinguishing
method for eval cases. It carries both the observed score and the
pass threshold onto the wire so consumers can compute headroom
without reconstructing intent.

# Deterministic scorers

The framework bundles a minimal set:

- `evals.Rouge1F1(hypothesis, reference)` — unigram ROUGE F1.
  Empty-empty returns 1; one-empty returns 0.

Everything else is authored in your neuron: bring any scorer you like
(BLEU, embedding similarity, tool-call correctness), compute a
`float64`, and feed it to `t.Score`.

# LLM-as-judge

LLM judges are ordinary Go calls that return numeric scores plus
rationale strings. Their output feeds `t.Score`; the rationale
becomes the metric message on the wire.

Consider capturing judge model info in a wrapper so the eventual
`AgentEvalResults.judge` field can be populated by the reporter or
downstream sink.

# Wire shape

Results appear as [`AgentEvalResults`](/wire-types/agent-eval-results.md):
one `Case` per registered case, each with a list of `Metric` leaves.
Every `Metric` carries `threshold`, optional `score`, `status`, and
`message`. `RubricScore` sub-scores can be attached per metric.

# Related

* [T recorder](/concepts/t-recorder.md)
* [T methods](/api/t-methods.md)
* [Helpers — Rouge1F1](/api/helpers.md)
* [ADK agent eval (dynamic)](/suites/adk-agent-eval.md)
* [AgentEvalResults](/wire-types/agent-eval-results.md)

# Citations

[1] [README — Agent evaluations](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#agent-evaluations)
[2] [score.go — Rouge1F1](https://github.com/alis-exchange/go-alis-build/blob/main/evals/score.go)
