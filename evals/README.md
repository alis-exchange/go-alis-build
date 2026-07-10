# evals

A single Go framework for writing three kinds of post-deploy test against your live services:

- **Integration tests** — assert behavioural contracts on your gRPC surface.
- **Load tests** — generate traffic at a chosen intensity and evaluate Service Level Objectives(SLOs) on the aggregate performance.
- **Agent evaluations** — grade agent transcripts with deterministic checks, LLM-as-judge scores, and rubric dimensions.

You author suites in Go, register them once, and the deployed service exposes them via three RPCs on a `TestService`. Each RPC returns a long-running operation; each completed suite becomes a `Run`
published to whichever reporters (Pub/Sub, BigQuery, Spanner, log) you wire up.

The wire types this framework produces live in a separate proto module, `go.alis.build/common/alis/evals/v1` — imported here as `evalspb`. Consumers that ingest runs from
Pub/Sub or BigQuery pin that module directly.

---

## Table of contents

1. [Quickstart](#quickstart)
2. [Concepts](#concepts)
3. [Wire types](#wire-types)
4. [Integration tests](#integration-tests)
5. [Agent evaluations](#agent-evaluations)
6. [Load tests](#load-tests)
7. [Options reference](#options-reference)

- [Suite constructors](#suite-constructors)
- [Shared suite options (test + eval)](#shared-suite-options-test--eval)
- [Load-suite options](#load-suite-options)
- [Case registration](#case-registration)
- [Assertion primitives (](#assertion-primitives-t)`T`[)](#assertion-primitives-t)
- [Load profile fields](#load-profile-fields)
- [SLO constructors](#slo-constructors)
- [Environment API](#environment-api)
- [Registration functions](#registration-functions)
- [Reporters](#reporters)
- [Errors](#errors)
- [Helpers](#helpers)

8. [Filter grammar](#filter-grammar)
9. [Context and authentication](#context-and-authentication)
10. [Package layout](#package-layout)
11. [End-to-end lifecycle](#end-to-end-lifecycle)

---

## Quickstart

```go
import (
    _ "go.alis.build/adk/launchers/evals"    // mounts /api/... handlers
    "go.alis.build/evals"
    "go.alis.build/evals/env"
    "go.alis.build/evals/report"
    logreport "go.alis.build/evals/report/log"
    pubsubreport "go.alis.build/evals/report/pubsub"
)

// 1. Register any shared environment in package init.
func init() {
    // MustRegister panics on duplicate names; use env.Register when you'd
    // rather propagate the error.
    env.MustRegister("example-v1",
        env.WithSetup(seedExample),
        env.WithTeardown(cleanupExample),
    )
}

// 2. Author a suite and publish it once.
func Register() error {
    s, err := evals.NewIntegrationSuite("example-v1",
        evals.WithEnv("example-v1"),
    )
    if err != nil {
        return err
    }
    if err := s.Case("get-item", func(ctx context.Context, t *evals.T) {
        r := evals.Call(ctx, func(ctx context.Context) (*examplepb.Item, error) {
            return clients.Example.GetItem(ctx, &examplepb.GetItemRequest{Name: rootItem})
        })
        if !t.NoErr("grpc", r.Err) { return }
        t.Max("latency", r.Latency, 300*time.Millisecond)
        t.Check("has-name", r.Resp.GetName() != "")
    }); err != nil {
        return err
    }
    return evals.RegisterIntegration(s)
}

// 3. Wire the service and (optionally) fan out to more reporters.
func setupReporters(ctx context.Context) (*pubsubreport.Reporter, error) {
    ps, err := pubsubreport.New(ctx)
    if err != nil {
        return nil, err
    }
    services.TestServiceServer.Reporter = report.MultiReporter{
        logreport.Reporter{},
        ps, // publishes bare evalspb.Run JSON to alis.evals.v1.Run via pubsub/v2
    }
    return ps, nil // Close() at server drain
}
```

Once the binary starts, `RunIntegrationTest` / `RunAgentEval` / `RunLoadTest` on the `TestService` see the registered suites; the `/api/...` paths used by ADK-backed agent evals are installed by the
launcher import above.

---

## Concepts

**Suite.** A named group of related cases plus optional environment dependencies and lifecycle hooks. Three kinds exist (integration, eval, load) matching the three RPCs.

**Case.** The unit the runner executes. For integration and eval suites a case is a `func(ctx, *T)` that measures the SUT and records assertions on the per-case `T` recorder. For load suites a case is a `Target = func(ctx) error` invoked many times by the built-in load generator, plus declared SLOs against the aggregate result.

`T` **recorder.** A per-case handle passed to integration and eval cases. Every recording method (`Check`, `NoErr`, `Max`, `Score`, …) returns whether the assertion passed, so authors control flow
with plain `if !… { return }`. No panics, no `runtime.Goexit`.

**Environment.** Shared setup/teardown identified by name. Registered once (`env.Register`) and referenced by any suite via `WithEnv` — activated once per LRO, not once per suite.

**Reporter.** A `report.Reporter` sink that receives each completed `Run` proto. The default is `log.Reporter` (one alog line per run); use `MultiReporter{…}` to fan out to BigQuery,
Pub/Sub, Spanner, etc. Concrete sinks live in subpackages — see [Reporters](#reporters).

**Registry.** Process-global (mirrors `http.DefaultServeMux`) — suites publish themselves at `init()` via `RegisterIntegration`, `RegisterEval`, `RegisterLoad`, or `RegisterAgent`.
The registered suites are what the RPCs can see.

**Case ids.** Every case is qualified as `{suite}.{case}` at registration. The RPC's `case_ids` field is a filter:

- empty → run everything of the requested kind
- `"example-v1"` → run every case in that suite
- `"example-v1.get-item"` → run just that case

---

## Wire types

Consumers see three top-level messages, one per RPC, all sharing the `Run` envelope and `Status` enum. Every case appears in the results — passed and failed alike — so downstream dashboards can
compute headroom, not just breaches.

### Common

```protobuf
enum Status {
  STATUS_UNSPECIFIED = 0;
  PASSED             = 1;   // executed and every check passed
  FAILED             = 2;   // executed and one or more checks failed
  NOT_EVALUATED      = 3;   // skipped (StopOnFailure, setup fail, filter)
}

message Run {
  string      name        = 2;   // runs/{run_id}
  optional    string batch_id = 3;
  Run.Type    type        = 4;   // INTEGRATION_TEST | LOAD_TEST | AGENT_EVAL
  Status      status      = 5;
  oneof data {
    IntegrationTestResults integration_test = 6;
    LoadTestResults        load_test        = 7;
    AgentEvalResults       agent_eval       = 8;
  }
  Timestamp   start_time  = 21;
  Timestamp   end_time    = 22;
  string      operation   = 23;   // operations/{op_id}
  rpc.Status  error       = 24;
  Timestamp   create_time = 25;
  string      google_project_id = 26;
}
```

### Integration

```protobuf
message IntegrationTestResults {
  repeated Case cases = 1;
  message Case {
    string   id       = 1;   // example-v1.get-item
    Status   status   = 2;
    repeated Check checks = 3;
    Duration duration = 4;
    message Check {
      string id      = 1;   // "grpc", "latency", "has-name", …
      Status status  = 2;
      string message = 3;   // failure detail; empty on pass
    }
  }
}
```

### Load

```protobuf
message LoadTestResults {
  repeated Case cases = 1;

  message Case {
    string  id      = 1;
    Status  status  = 2;
    Summary summary = 3;
    repeated SloCheck checks = 4;
  }

  message Summary {
    RunLoadTestRequest.Mode mode            = 1;
    double                  target_qps      = 2;
    int32                   concurrency     = 3;
    Duration                duration        = 4;
    int64                   request_count   = 5;
    int64                   error_count     = 6;
    double                  actual_qps      = 7;
    LatencyPercentiles      latency         = 8;
    map<string,int64>       errors_by_code  = 9;   // "UNAVAILABLE" → n
  }

  message LatencyPercentiles {
    double p50_ms  = 1;
    double p95_ms  = 2;
    double p99_ms  = 3;
    double min_ms  = 4;
    double mean_ms = 5;
    double max_ms  = 6;
  }

  message SloCheck {
    string id       = 1;   // "latency.p99_ms", "error_rate", …
    Status status   = 2;
    string message  = 3;
    double observed = 4;
    double limit    = 5;
    string unit     = 6;   // "ms", "%", "rps"
  }
}
```

### Agent eval

```protobuf
message AgentEvalResults {
  repeated Case cases = 1;
  JudgeInfo     judge = 2;

  message Case {
    string   id         = 1;
    Status   status     = 2;
    Duration duration   = 3;
    string   session_id = 4;
    repeated Metric metrics = 5;

    message Metric {
      string   id        = 1;
      Status   status    = 2;
      optional double score = 3;
      double   threshold = 4;
      string   message   = 5;
      repeated RubricScore rubric = 6;

      message RubricScore {
        string id     = 1;
        Status status = 2;
        optional double score = 3;
      }
    }
  }

  message JudgeInfo {
    string model             = 1;
    string model_version     = 2;
    int64  judge_call_count  = 3;
    int64  judge_error_count = 4;
  }
}
```

---

## Integration tests

### Example

```go
package v1

import (
    "context"
    "time"

    "go.alis.build/evals"
    "go.alis.build/evals/env"

    "example.com/internal/clients"
    examplepb "example.com/pb/example/v1"
)

const exampleEnv = "example-v1"

// This example uses the panicking Must* variants for brevity. Prefer the
// error-returning constructors (NewIntegrationSuite, Case, Register*, env.Register)
// when you want to propagate configuration failures explicitly.
func Register() {
    env.MustRegister(exampleEnv,
        env.WithSetup(seedExample),
        env.WithTeardown(cleanupExample),
    )

    s := evals.MustNewIntegrationSuite("example-v1",
        evals.WithEnv(exampleEnv),
        evals.WithSetup(sanityCheck),
    )

    s.MustCase("get-item", func(ctx context.Context, t *evals.T) {
        r := evals.Call(ctx, func(ctx context.Context) (*examplepb.Item, error) {
            return clients.Example.GetItem(ctx, &examplepb.GetItemRequest{Name: rootItem})
        })
        // Guarded chain: bail on the first failure, but keep reporting
        // every check up to that point.
        if !t.NoErr("grpc", r.Err) {
            return
        }
        if !t.Max("latency", r.Latency, 300*time.Millisecond) {
            return
        }
        t.Check("has-name", r.Resp.GetName() != "")
        t.Checkf("size-positive", r.Resp.GetSize() > 0, "got size=%d, want > 0", r.Resp.GetSize())
    }).MustCase("list-empty-parent", func(ctx context.Context, t *evals.T) {
        r := evals.Call(ctx, func(ctx context.Context) (*examplepb.ListItemsResponse, error) {
            return clients.Example.ListItems(ctx, &examplepb.ListItemsRequest{Parent: emptyParent})
        })
        if !t.NoErr("grpc", r.Err) {
            return
        }
        t.Check("empty", len(r.Resp.GetItems()) == 0)
    })

    if err := evals.RegisterIntegration(s); err != nil {
        panic(err)
    }
}
```

### What each case can assert

Every method on `*T` records exactly one check leaf (which becomes one `IntegrationTestResults.Case.Check` on the wire) and returns whether that leaf passed. See
[Assertion primitives](#assertion-primitives-t) for the full list of methods.

**Guarded chain pattern.** Each method returns a pass boolean so authors can short-circuit without ceremony:

```go
if !t.NoErr("grpc", err)                { return }
if !t.Max("latency", r.Latency, budget) { return }
t.Check("shape", r.Resp.GetName() != "")
```

**Duplicate check ids.** Using the same id twice inside one case records a single `duplicate-check-id` failure leaf so downstream tooling stays deterministic.

**Setup / teardown.** If `WithSetup` returns an error, every case in the suite is recorded with a `setup` failure marker and teardown is skipped. Teardown errors are logged but don't affect case
outcomes.

**StopOnFailure.** For stateful flows (create → get → update → delete), `evals.StopOnFailure()` on the suite marks all subsequent cases `NOT_EVALUATED` once any case fails.

---

## Agent evaluations

### Example

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

        // A binary check (no score) works too — surfaces as a metric with
        // no `score` field on the wire.
        t.Check("no-refusal", !grade.Refused)
    })

    if err := evals.RegisterEval(s); err != nil {
        panic(err)
    }
}
```

Eval cases share `T` with integration cases; the adapter emits the recorded leaves as `Metric`s instead of `Check`s. `T.Score` is the distinguishing primitive — it carries both the observed score
and the pass threshold onto the wire so consumers see how much headroom each metric had.

### Dynamic agent evals via ADK

Rather than writing eval cases in Go, you can point the framework at a deployed ADK agent that already publishes its eval sets. Configuration lives entirely on the `adk.Agent` struct that
`adk.NewProvider` wraps — there is no separate registration table:

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
        // Optional: declare judge provenance so AgentEvalResults.JudgeInfo
        // is populated on the wire. Authoritative when non-empty; else the
        // provider probes the metric criteria for a judgeModelOptions.judgeModel.
        JudgeModel:        "gemini-2.5-pro",
        JudgeModelVersion: "2025-06-05",
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

The provider discovers eval sets over HTTP at run time, filters them against the incoming `case_ids`, and adapts responses into the same `AgentEvalResults` shape as in-process eval suites.

The **deployed agent** must itself embed `go.alis.build/adk/launchers/evals` so its `/api/list_eval_sets` and `/api/run_eval` endpoints are reachable — this is where the launcher
requirement bites; the eval-consuming neuron only needs the launcher import when it _also_ serves ADK agent evals of its own.

**Judge provenance and call counts.** When any suite in the run uses an LLM-as-judge metric (`final_response_match_v2`, `rubric_based_*_v1`, `hallucinations_v1`, `per_turn_user_simulator_quality_v1`), the mapper emits a non-nil `AgentEvalResults.JudgeInfo` sidecar carrying `model`, `model_version`, and a per-suite `judge_call_count` synthesised from the number of judge-classified metric results across cases. `Agent.JudgeModel` is authoritative; if empty, the provider probes `Agent.DefaultMetrics` (or `Agent.MetricOverrides[setID]`) in slice order for the first non-empty `judgeModelOptions.judgeModel`. Non-judge runs emit a nil sidecar. Note the asymmetry with adk-python, whose `JudgeModelOptions.judge_model` defaults to `"gemini-2.5-flash"` when unset: set `Agent.JudgeModel` explicitly here to avoid an empty `model` on the wire even though real judge calls happened. `judge_call_count` counts result entries, not per-invocation samples — treat it as a lower bound. The default `log.Reporter` emits a WARN when judge metrics are configured but the count is zero (see the reporter's godoc for the diagnostic).

---

## Load tests

### Example

```go
package v1

import (
    "context"
    "time"

    "go.alis.build/evals"
    evalspb "example.com/pb/alis/ge/evals/v1"
    examplepb "example.com/pb/example/v1"

    "example.com/internal/clients"
)

func Register() {
    s := evals.MustNewLoadSuite("example-v1-load",
        evals.WithLoadEnv(exampleEnv),
        // Override MODERATE for this suite: heavier than the framework default.
        evals.WithLoadProfile(evalspb.RunLoadTestRequest_MODERATE, evals.Profile{
            QPS:            250,
            Concurrency:    40,
            Duration:       90 * time.Second,
            Warmup:         15 * time.Second,
            RequestTimeout: 5 * time.Second,
        }),
    )

    s.MustLoadCase("list-items",
        func(ctx context.Context) error {
            _, err := clients.Example.ListItems(ctx, &examplepb.ListItemsRequest{PageSize: 5})
            return err
        },
        evals.SLOLatencyP99(500*time.Millisecond),
        evals.SLOLatencyP50(50*time.Millisecond),
        evals.SLOErrorRate(0.01),
        evals.SLOMinQPS(20),
    ).MustLoadCase("get-item",
        func(ctx context.Context) error {
            _, err := clients.Example.GetItem(ctx, &examplepb.GetItemRequest{Name: rootItem})
            return err
        },
        evals.SLOLatencyP99(300*time.Millisecond),
        evals.SLOErrorRate(0.005),
    )

    if err := evals.RegisterLoad(s); err != nil {
        panic(err)
    }
}
```

### How a load case is executed

For each case the runner:

1. Resolves the effective `Profile` (framework default for the requested `Mode`, overridden by any suite-level `WithLoadProfile` for that mode).
2. Spawns `Concurrency` worker goroutines and a fixed-rate pacer.
3. Runs `Warmup + Duration` of traffic; samples produced during `Warmup` are discarded so autoscalers / JITs have time to settle.
4. Aggregates latency into an HDR histogram, counts errors, groups them by canonical gRPC status code.
5. Evaluates every declared SLO against the aggregate metrics; each produces one `SloCheck` — passed or failed — with the observed value, the limit, and the unit.
6. Rolls the case up: PASSED only when every SLO passed.

Load cases run **sequentially within a suite** by design — concurrent load windows against different targets would contaminate each other's measurements.

### Mode presets

`RunLoadTest.mode` picks a framework preset:

| Mode           | QPS  | Concurrency | Duration | Warmup |
| -------------- | ---- | ----------- | -------- | ------ |
| `MINIMAL`      | 5    | 2           | 15 s     | 2 s    |
| `CONSERVATIVE` | 25   | 10          | 30 s     | 5 s    |
| `MODERATE`     | 100  | 25          | 60 s     | 10 s   |
| `HIGH`         | 400  | 100         | 120 s    | 15 s   |
| `LUDICROUS`    | 1000 | 250         | 180 s    | 20 s   |

Overrides fully replace the default for that mode — there is no field-level merging (a partial override at high intensity is easy to get wrong).

### Saturation warning

When the worker pool cannot keep up with the target rate, the observed `Summary.actual_qps` falls below `target_qps`. The generator emits an `alog.Warnf` when `actual_qps < 0.9 × target_qps` so you
notice you're measuring the generator, not the SUT — bump `Concurrency` and rerun.

---

## Options reference

Everything in the public API of the framework, in one place.

### Suite constructors

| Function                                                                     | Returns              | Notes                                                                                                            |
| ---------------------------------------------------------------------------- | -------------------- | ---------------------------------------------------------------------------------------------------------------- |
| `evals.NewIntegrationSuite(name string, opts ...SuiteOption)`                | `(*Suite, error)`    | Integration suite. Returns a typed error on invalid name (empty or containing `.`) or invalid option.            |
| `evals.NewAgentEvalSuite(name string, opts ...SuiteOption)`                  | `(*Suite, error)`    | Agent-eval suite. Same error rules.                                                                              |
| `evals.NewLoadSuite(name string, opts ...LoadSuiteOption)`                   | `(*LoadSuite, error)`| Load suite. Same error rules.                                                                                    |
| `evals.MustNewIntegrationSuite` / `MustNewAgentEvalSuite` / `MustNewLoadSuite` | `*Suite` / `*LoadSuite` | Panicking variants for init-time code that would `log.Fatal` on a config error.                              |

`Suite.Kind()` reports `KindTest` or `KindEval`. Kinds cannot be mixed — passing a `KindEval` suite
to `RegisterIntegration` returns `evals.ErrWrongSuiteKind`.

### Shared suite options (test + eval)

All apply to both `NewIntegrationSuite` and `NewAgentEvalSuite`.

| Option                                     | Effect                                                                                                                                                                           |
| ------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `evals.WithEnv(names ...string)`           | Declare shared environments. Every name must have been passed to `env.Register` before the suite is constructed.                                                                 |
| `evals.WithSetup(hook suite.SuiteHook)`    | Runs once per LRO before the suite's cases. Signature: `func(ctx context.Context) error`. Failure fails every case in the suite with a `setup` marker and skips teardown.        |
| `evals.WithTeardown(hook suite.SuiteHook)` | Runs once after the suite's cases (or before propagating cancellation). Errors are logged but ignored.                                                                           |
| `evals.WithContext(fn evals.ContextDecorator)` | Install a `func(ctx) ctx` applied to the suite's setup, teardown, and every case body. The framework's only auth-adjacent surface: use it to stamp caller identity, auth headers, tokens, tracing, or any request-scoped context values. See [Context and authentication](#context-and-authentication). |
| `evals.StopOnFailure()`                    | Once any case in the suite ends non-`PASSED`, remaining cases are recorded `NOT_EVALUATED` with a "preceding case … failed" reason. Use for stateful flows.                      |

### Load-suite options

Applied to `NewLoadSuite`. Kept separate from `SuiteOption` because several test/eval options
(`StopOnFailure`, `WithContext`) do not have sensible load-test semantics — load suites always
run under whatever context the runner installs so measurements match production traffic.

| Option                                                                   | Effect                                                                                                                                                                 |
| ------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `evals.WithLoadEnv(names ...string)`                                     | Declare shared environments. Same semantics as `WithEnv` on test/eval suites.                                                                                          |
| `evals.WithLoadSetup(hook suite.SuiteHook)`                              | Suite-level pre-cases hook. Failure fails every case with a `setup` marker.                                                                                            |
| `evals.WithLoadTeardown(hook suite.SuiteHook)`                           | Suite-level post-cases hook. Errors logged, ignored.                                                                                                                   |
| `evals.WithLoadProfile(mode evalspb.RunLoadTestRequest_Mode, p Profile)` | Override the framework default profile for that specific mode. The override fully replaces the default; other modes keep theirs. Returns `suite.ErrLoadProfileUnspecifiedMode` if `mode == MODE_UNSPECIFIED`. |

### Case registration

| Method                                                                          | Effect                                                                                                                        |
| ------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `(*Suite).Case(name string, fn CaseFunc) error`                                 | Register a test or eval case. Returns a typed error (`evals.ErrNilCaseFunc`, `suite.ErrInvalidCaseName`, `suite.ErrDuplicateCase`). |
| `(*Suite).MustCase(name string, fn CaseFunc) *Suite`                            | Panicking variant returning the receiver for fluent chaining.                                                                 |
| `(*LoadSuite).LoadCase(name string, target Target, slos ...SLO) error`          | Register a load case. `target` is `func(ctx context.Context) error`. Returns a typed error (`evals.ErrNilTarget`, etc.).      |
| `(*LoadSuite).MustLoadCase(name string, target Target, slos ...SLO) *LoadSuite` | Panicking variant returning the receiver for fluent chaining.                                                                 |

Types:

```go
type CaseFunc func(ctx context.Context, t *T)
type Target   = loadgen.Target    // func(ctx context.Context) error
type Profile  = loadgen.Profile
type SLO      struct { … opaque … }
```

### Assertion primitives (`T`)

`T` is the per-case recorder for test and eval cases. Each method records one leaf and returns
whether it passed.

| Method                                                                | Semantics                                                                                                                                           | Wire on integration                            | Wire on eval                                         |
| --------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------- | ---------------------------------------------------- |
| `t.Check(id string, pass bool) bool`                                  | Records `id` with `PASSED`/`FAILED`. Message empty.                                                                                                 | `Check{id, status, ""}`                        | `Metric{id, status, "", 0, nil, nil}`                |
| `t.Checkf(id string, pass bool, format string, args ...any) bool`     | Same as `Check` but formats a failure message.                                                                                                      | `Check{id, status, msg}`                       | `Metric{id, status, msg, 0, nil, nil}`               |
| `t.NoErr(id string, err error) bool`                                  | Records `PASSED` when `err == nil`, otherwise `FAILED` with `err.Error()`.                                                                          | `Check{id, status, err.Error()}`               | `Metric{id, status, err.Error(), …}`                 |
| `t.Max(id string, got, limit time.Duration) bool`                     | Records `PASSED` when `got <= limit`. Failure message includes both values.                                                                         | `Check{id, …, "id 350ms exceeds limit 300ms"}` | `Metric{id, …, message}`                             |
| `t.Score(id string, score, threshold float64, rationale string) bool` | Records `PASSED` when `score >= threshold`. `rationale` becomes the metric message; a default is generated when `pass == false && rationale == ""`. | Not used (score dropped on integration).       | `Metric{id, status, message, threshold, score, nil}` |

`T` is not safe for concurrent use inside one case. If your case fans out to goroutines, gather
their results back on the case goroutine and record from there.

**Duplicate ids.** Recording the same id twice inside one case produces a single sentinel leaf with
id `duplicate-check-id` (see the exported constant `evals.DuplicateCheckIDName`) and status
`FAILED`. All further attempts to reuse the id are ignored so results remain parseable.

### Load profile fields

`evals.Profile` (re-exports `loadgen.Profile`):

| Field            | Type            | Meaning                                                                                                                                                |
| ---------------- | --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `QPS`            | `float64`       | Target requests per second. Must be > 0.                                                                                                               |
| `Concurrency`    | `int`           | Number of worker goroutines. Must be ≥ 1. Sized to keep enough requests in flight for the target rate at the target's expected latency (Little's law). |
| `Duration`       | `time.Duration` | The measurement window. Must be > 0.                                                                                                                   |
| `Warmup`         | `time.Duration` | Traffic runs at target rate for this leading period but the samples are dropped. Zero disables warmup.                                                 |
| `RequestTimeout` | `time.Duration` | Per-request `context.WithTimeout` cap. Zero → 30 s default. Always further capped by the remaining window so a straggler cannot pollute the next case. |

### SLO constructors

Each constructor produces one `SLO` value; each SLO produces one `SloCheck` per case run (passed or
failed).

| Constructor                               | Check id         | Unit  | Passes when         | Notes                                                                                                                            |
| ----------------------------------------- | ---------------- | ----- | ------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `evals.SLOLatencyP50(max time.Duration)`  | `latency.p50_ms` | `ms`  | `observed <= limit` | Median latency.                                                                                                                  |
| `evals.SLOLatencyP95(max time.Duration)`  | `latency.p95_ms` | `ms`  | `observed <= limit` | 95th percentile.                                                                                                                 |
| `evals.SLOLatencyP99(max time.Duration)`  | `latency.p99_ms` | `ms`  | `observed <= limit` | 99th percentile — the usual tail-latency guardrail.                                                                              |
| `evals.SLOErrorRate(maxFraction float64)` | `error_rate`     | `%`   | `observed <= limit` | Constructor accepts a fraction (`0.01` for 1%). Observed and limit are recorded as percent so wire values match human intuition. |
| `evals.SLOMinQPS(min float64)`            | `actual_qps`     | `rps` | `observed >= limit` | Throughput floor — useful for detecting silent capacity regressions.                                                             |

### Environment API

```go
package env

func Register(name string, opts ...Option) error       // returns env.ErrDuplicateRegistration
func MustRegister(name string, opts ...Option)         // panics on duplicate
func WithSetup(hook Hook) Option
func WithTeardown(hook Hook) Option
func Get(name string) *Environment

type Hook func(context.Context) error
```

| Function                              | Effect                                                                                                  |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| `env.Register(name, opts...) error`   | Register a globally-named environment. Returns `env.ErrDuplicateRegistration` on duplicate.             |
| `env.MustRegister(name, opts...)`     | Panicking variant. Use at package init when a duplicate should halt the process.                        |
| `env.WithSetup(hook)`                 | Optional setup, invoked once per LRO if any selected suite depends on this env.                         |
| `env.WithTeardown(hook)`              | Optional teardown, invoked in reverse-registration order after all suites finish.                       |
| `env.Get(name)`                       | Look up a registered environment. Returns nil for unknown names.                                        |

Environments are process-global. If you're building a library that wants to be re-entrant, avoid
re-registering the same name — call `env.Get(name)` first, or gate registration behind `sync.Once`.

### Registration functions

| Function                                                    | Effect                                                                                                                              |
| ----------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `evals.RegisterIntegration(s *Suite) error`                 | Publish an integration suite. Returns `suite.ErrNilSuite` if `s == nil` or `evals.ErrWrongSuiteKind` if `s.Kind() != KindTest`.     |
| `evals.RegisterEval(s *Suite) error`                        | Publish an eval suite. Returns `suite.ErrNilSuite` if `s == nil` or `evals.ErrWrongSuiteKind` if `s.Kind() != KindEval`.            |
| `evals.RegisterLoad(s *LoadSuite) error`                    | Publish a load suite. Returns `suite.ErrNilSuite` if `s == nil`.                                                                    |
| `evals.RegisterAgent(p registry.AgentEvalProvider) error`   | Publish a lazy agent-eval provider (for example an ADK-backed one). Returns `evals.ErrNilProvider` if `p == nil`.                   |
| `evals.DefaultRegistry() *registry.Registry`                | Return the process-wide registry that `TestServiceServer` consumes. Useful for tests.                                               |

Call these once at `init()` time. All four target `evals.DefaultRegistry()`. Callers must handle
the returned error (or `log.Fatal`); the framework does not panic on registration errors.

### Reporters

Reporters receive one `evalspb.Run` per completed suite. Wire your sink onto
`TestServiceServer.Reporter` at process start. `nil` disables reporting entirely.

```go
package report

type Reporter interface {
    ReportRun(ctx context.Context, run *evalspb.Run) error
}
```

Bundled reporter implementations:

| Type | Package | Purpose |
| ---- | ------- | ------- |
| `log.Reporter{}` | `go.alis.build/evals/report/log` | Default. Emits a one-line summary via `alog` — `Info` for `PASSED` runs, `Warn` for anything else. Nil-safe on nil runs. |
| `bigquery.Reporter` | `go.alis.build/evals/report/bigquery` | Streams each Run to BigQuery via protobq (Duration as STRING). |
| `pubsub.Reporter` | `go.alis.build/evals/report/pubsub` | Publishes bare `Run` JSON via `protojson` + `pubsub/v2`. |
| `report.NoOpReporter{}` | `go.alis.build/evals/report` | Discard. Useful for local tests where you want the LRO to complete without emitting anything. |
| `report.MultiReporter{…}` | `go.alis.build/evals/report` | Fan out to multiple reporters, in order. First error aborts the fan-out. |

Companion packages (not themselves Reporters):

| Symbol | Package | Purpose |
| ------ | ------- | ------- |
| `bqschema.Schema()` / `bqschema.SchemaJSON()` / `bqschema.EnsureTable()` | `go.alis.build/evals/report/bqschema` | Canonical BigQuery schema for `evalspb.Run` rows and a table-provisioning helper. Both `bigquery.Reporter` and Pub/Sub → BigQuery subscriptions target this schema. |

Wiring:

```go
import (
    "context"

    "go.alis.build/evals/report"
    logreport "go.alis.build/evals/report/log"
    bqreport "go.alis.build/evals/report/bigquery"
    pubsubreport "go.alis.build/evals/report/pubsub"
)

func setupReporters(ctx context.Context, projectID string) error {
    bq, err := bqreport.New(ctx, projectID, "evals", "runs")
    if err != nil {
        return err
    }
    ps, err := pubsubreport.New(ctx)
    if err != nil {
        _ = bq.Close()
        return err
    }
    services.TestServiceServer.Reporter = report.MultiReporter{
        logreport.Reporter{},
        bq,
        ps,
    }
    return nil // Close bq and ps at server drain
}
```

#### BigQuery reporter

The BigQuery reporter writes each completed `Run` to a pre-existing table whose schema matches `bqschema.Schema()` (equivalently `bqreport.InferSchema()`, which delegates to `bqschema`). Schema derivation is now owned by the companion `bqschema` package so that both this reporter and Pub/Sub → BigQuery subscriptions land rows in the same layout. Type mapping:

- `google.protobuf.Timestamp` → `TIMESTAMP`
- `google.protobuf.Duration` → `STRING` (protojson-native, e.g. `"1.500s"`)
- `google.rpc.Status` → `RECORD{code, message}` (`details` is intentionally omitted)
- `oneof data` → sibling nullable `RECORD` columns (`integration_test`, `load_test`, `agent_eval`)

Provision the table at deploy time (the reporter is insert-only by default):

```go
schemaJSON, err := bqschema.SchemaJSON()
// write schemaJSON to schema.json, then:
// bq mk --table PROJECT:evals.runs schema.json
```

`bqreport.WithSchemaOptions(...)` still exists but now controls **row marshaling only**, not schema inference. Provision from `bqschema.Schema()` regardless of the marshal options you pass to the reporter.

Alternatively, let the reporter manage the table itself with `bqreport.WithAutoCreateTable(...)`, which delegates to `bqschema.EnsureTable`:

```go
r, err := bqreport.New(ctx, projectID, "evals", "runs",
    bqreport.WithAutoCreateTable(bigquery.TableMetadata{
        TimePartitioning: &bigquery.TimePartitioning{
            Field: "start_time",
            Type:  bigquery.DayPartitioningType,
        },
        Clustering: &bigquery.Clustering{Fields: []string{"type", "status"}},
    }),
)
```

At construction:

- The **dataset must exist** — a missing dataset returns an error immediately (create it via Terraform or `bq mk`).
- If the table is missing it is created with the reporter's schema plus any `TableMetadata` you supplied.
- If the table exists an additive schema update is applied. BigQuery enforces additive-only server-side: renames, drops, and type changes return an error. Do not hand-edit tables managed by this option; use plain Terraform provisioning instead when you need custom columns.

Each row uses `run.name` (`runs/{run_id}`) as the BigQuery insert ID for best-effort deduplication within the streaming insert window (absorbs Cloud Tasks retries). Inserts are bounded by a 10s timeout (override with `bqreport.WithInsertTimeout`). `bqreport.New` owns the underlying BigQuery client and closes it on `Close`; `bqreport.NewWithClient` borrows a client and Close is a no-op.

#### Pub/Sub reporter

The Pub/Sub reporter publishes each completed `Run` to Pub/Sub as JSON, using `google.golang.org/protobuf/encoding/protojson` on top of `cloud.google.com/go/pubsub/v2`. The payload is a **bare `evalspb.Run`** — no `RunPublishedEvent` envelope. Marshaling is locked to `UseProtoNames=true` and `EmitUnpopulated=true` so downstream consumers see stable snake_case keys and can distinguish unset fields.

The default topic is `"alis.evals.v1.Run"` (the payload's proto full name). Unlike `*Event`-suffixed messages, **this topic is not auto-provisioned by the Alis Build platform's define step** — callers must provision the topic (and any Pub/Sub → BigQuery subscription) via Terraform. Override with `WithTopic(...)` when your platform provisions under a different name.

```go
import pubsubreport "go.alis.build/evals/report/pubsub"

func setupPubsub(ctx context.Context) (*pubsubreport.Reporter, error) {
    ps, err := pubsubreport.New(ctx)
    if err != nil {
        return nil, err
    }
    services.TestServiceServer.Reporter = ps
    return ps, nil // Close() at server drain
}
```

By default every `ReportRun` blocks until the Pub/Sub broker acks the message — the safer choice for short-lived eval processes that may exit right after completing a run. Options:

| Option | Effect |
| ------ | ------ |
| `pubsubreport.WithProject(id)` | Override the Google Cloud project (defaults to `ALIS_OS_PROJECT`). Only valid with `New`. |
| `pubsubreport.WithTopic(name)` | Override the default topic. Accepts a bare topic ID or a fully-qualified `projects/<p>/topics/<t>` resource string. |
| `pubsubreport.WithOrderingKey(k)` | Apply the Pub/Sub ordering key on every message and enable message ordering on the publisher. |
| `pubsubreport.WithBackground()` | Fire-and-forget publishing (does not wait for broker ack). Best-effort delivery only; call `Close` before process exit to flush pending messages. |
| `pubsubreport.WithPublishTimeout(d)` | Bound each publish via `context.WithTimeout`. Defaults to 10s. |

`pubsubreport.New(ctx, opts...)` owns the underlying `*pubsub.Client` and the `*pubsub.Publisher` for the configured topic; `Close` stops the publisher and closes the client. `pubsubreport.NewWithClient(client, opts...)` borrows a client (Close stops only the publisher); passing `WithProject` there returns an error since the borrowed client already has a project bound.

**Pub/Sub → BigQuery.** Because this reporter emits protojson-formatted JSON, a Pub/Sub "Use table schema" subscription can land each published message directly into a table provisioned from `bqschema.SchemaJSON()` — no glue subscriber required. The two constraints of the older proto-schema path (unmappable `oneof`, no WKT unwrapping) do not apply.

If you want a table fed by both the streaming-insert path (`bqreport.Reporter`) and the Pub/Sub → BigQuery path, use `bqschema.Schema()` for both — the two writers produce rows that match the same layout. If you only need a raw archive of published events, attach a "Don't use a schema" subscription with a single `data BYTES` column instead.

**Contract for custom reporters:**

1. Handle `run == nil` as a no-op.
2. Do not block the LRO goroutine for long — persist async or with a short timeout.
3. Errors are best-effort. Returning one is logged by the caller; subsequent reporters in a
   `MultiReporter` are skipped.

Minimal implementation skeleton (webhook example — use the bundled `pubsubreport` / `bqreport` for Pub/Sub and BigQuery):

```go
type WebhookReporter struct {
    URL    string
    Client *http.Client
}

func (r *WebhookReporter) ReportRun(ctx context.Context, run *evalspb.Run) error {
    if run == nil {
        return nil
    }
    body, err := protojson.Marshal(run)
    if err != nil {
        return err
    }
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, r.URL, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := r.Client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        return fmt.Errorf("webhook: %s", resp.Status)
    }
    return nil
}
```

### Errors

The framework surfaces validation errors (unknown environment, invalid suite name, unknown case_ids,
etc.) as typed values that also implement `GRPCStatus()` so the RPC boundary translates them
cleanly. The interface lives in `go.alis.build/evals/errors`:

```go
type EvalError interface {
    error
    GRPCStatus() *status.Status
}
```

Callers rarely construct these directly. Four helpers are available for converting to and inspecting
gRPC statuses at the RPC boundary:

| Helper                                    | Purpose                                                                                                                             |
| ----------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `errors.ToGRPC(err error) error`          | Preserve the underlying status code when `err` is an `EvalError`; otherwise return `codes.InvalidArgument`. Returns `nil` on `nil`. |
| `errors.ToGRPCf(field string, err error)` | Same, prefixing the message with `field:` for RPC validation errors.                                                                |
| `errors.IsEval(err error) bool`           | Reports whether `err` (or any wrapped cause) implements `EvalError`.                                                                |
| `errors.Code(err error) codes.Code`       | Returns the `codes.Code` for an `EvalError`, or `codes.Unknown`.                                                                    |

Concrete error types worth knowing about (all implement `EvalError`):

| Error                                                              | Package          | Triggered by                                                                                          |
| ------------------------------------------------------------------ | ---------------- | ----------------------------------------------------------------------------------------------------- |
| `evals.ErrNilCaseFunc`                                             | `evals`          | `Suite.Case` called with a nil case function.                                                          |
| `evals.ErrNilTarget`                                               | `evals`          | `LoadSuite.LoadCase` called with a nil target.                                                         |
| `evals.ErrNilProvider`                                             | `evals`          | `RegisterAgent` called with a nil provider.                                                            |
| `evals.ErrWrongSuiteKind`                                          | `evals`          | Suite passed to the wrong `Register*` (e.g. eval suite to `RegisterIntegration`).                      |
| `evals.ErrUnknownSuiteKind`                                        | `evals`          | Internal invariant: `Suite.Case` invoked on a suite whose `kind` is neither `KindTest` nor `KindEval`. |
| `suite.ErrNilSuite`                                                | `evals/suite`    | Registration or case-add on a nil suite.                                                              |
| `suite.ErrInvalidSuiteName`                                        | `evals/suite`    | Empty name, name containing `.`.                                                                      |
| `suite.ErrDuplicateCase`                                           | `evals/suite`    | Two cases with the same short name inside one suite.                                                  |
| `suite.ErrInvalidCaseName`                                         | `evals/suite`    | Case name containing `.`.                                                                             |
| `suite.ErrUnknownEnvironment`                                      | `evals/suite`    | `WithEnv` naming an env that hasn't been registered.                                                  |
| `suite.ErrInvalidFilterPath`                                       | `evals/suite`    | `case_ids` entry that is not `suite` or `suite.case`.                                                 |
| `suite.ErrLoadProfileUnspecifiedMode`                              | `evals/suite`    | `WithLoadProfile` targeting `MODE_UNSPECIFIED`.                                                        |
| `env.ErrDuplicateRegistration`                                     | `evals/env`      | `env.Register` called twice for the same name.                                                         |
| `env.ErrNotRegistered`                                             | `evals/env`      | Runner asked for an env that wasn't `env.Register`ed.                                                 |
| `env.ErrSetupFailed`                                               | `evals/env`      | Env setup hook returned an error; every case in dependent suites is marked with a setup-error result. |
| `registry.ErrNoTestSuites` / `ErrNoEvalSuites` / `ErrNoLoadSuites` | `evals/registry` | Filter matches nothing.                                                                               |

Construction-time errors (name violations, duplicate cases, unknown envs, wrong-kind
registration) are returned as typed `EvalError` values from `evals.NewIntegrationSuite` /
`evals.NewAgentEvalSuite` / `evals.NewLoadSuite`, from `Suite.Case` / `LoadSuite.LoadCase`, and
from the `Register*` functions. Callers decide whether to `log.Fatal`, propagate, or ignore.
The `MustNew*` / `MustCase` / `MustLoadCase` / `env.MustRegister` variants panic wrapping the
same typed error for init-time code that would otherwise `log.Fatal`. Everything discovered by
the runtime (unknown filter path, env setup failure) surfaces via `EvalError` and is translated
to a gRPC status by the RPC handlers.

### Helpers

| Function                                                                            | Purpose                                                                                                                 |
| ----------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `evals.Call[T](ctx context.Context, fn func(context.Context) (T, error)) Result[T]` | Invoke an RPC, capture typed response, error, and wall-clock latency. Records nothing on `T` — assertions are explicit. |
| `evals.Result[T]`                                                                   | `{Resp T, Err error, Latency time.Duration}`.                                                                           |
| `evals.Rouge1F1(hypothesis, reference string) float64`                              | Deterministic ROUGE-1 unigram F1 scorer. Empty-empty → 1; one-empty → 0. Feed into `t.Score`.                           |
| `evals.DefaultLoadProfile(mode evalspb.RunLoadTestRequest_Mode) (Profile, bool)`    | Look up the framework default profile for `mode`. Returns `(zero, false)` for `MODE_UNSPECIFIED`.                       |
| `evals.ResolveLoadProfile(mode, overrides map[Mode]Profile) (Profile, bool)`        | Merge suite overrides with defaults. Override wins; other modes keep defaults.                                          |

---

## Filter grammar

Every RPC accepts a `case_ids` list. The grammar:

| Filter entry     | Selects                                                                      |
| ---------------- | ---------------------------------------------------------------------------- |
| _empty list_     | Every registered suite of the requested kind.                                |
| `"my-suite"`     | Every case in `my-suite`. Its `WithSetup` / `WithTeardown` run exactly once. |
| `"my-suite.foo"` | Just the `foo` case in `my-suite`.                                           |

Multiple entries union: `["example-v1.get-item", "example-v1.list-empty-parent"]` selects two cases.
Mixing suite-scoped and case-scoped filters against the same suite promotes to whole-suite
selection.

Unknown suite/case ids are rejected synchronously at the RPC boundary with `InvalidArgument` (see
`evalspb.ValidateSelection` — no LRO is created for invalid inputs).

---

## Context and authentication

The framework does **not** attach any authentication to outgoing calls. It propagates whatever
context the caller supplies, so consumers wire auth (bearer tokens, oauth2, IAM identity headers,
mTLS, service-account impersonation, …) via a single seam: a `ContextDecorator`.

```go
type ContextDecorator = func(context.Context) context.Context
```

Two layers apply the decorator:

- `evals.WithContext(fn)` — suite level. Applied to the suite's setup, teardown, and every case body.
- Runner level (via the runner's own `WithContext`) — applied to environment hooks and to any suite
  that doesn't declare its own decorator. A suite-level decorator fully replaces the runner-level
  one for that suite; there is no chaining.

**Context propagation contract.** Every `ctx` handed to user code (env hooks, suite hooks, case
bodies, load workers) is a descendant, via zero or more `ContextDecorator` calls, of the ctx the
caller passed to the framework. Nothing in the framework calls `context.Background()`,
`context.TODO()`, or `context.WithoutCancel()`. Deadlines, cancellation, tracing state, and custom
values propagate through to every outbound call the case body issues.

Case authors always retain the escape hatch of further decorating `ctx` inside the case body.

### Example: stamping IAM identity headers

Consumers of the Alis IAM stack write a tiny adapter in their own repo (this package deliberately
has no `iam` dependency):

```go
func WithAlisIdentity(id *iam.Identity) evals.SuiteOption {
    if id == nil {
        id = iam.SystemIdentity
    }
    return evals.WithContext(func(ctx context.Context) context.Context {
        ctx = id.Context(ctx)
        return id.LegacyOutgoingMetadata(ctx)
    })
}

s := evals.MustNewIntegrationSuite("example-v1", WithAlisIdentity(iam.SystemIdentity))
```

### ADK agent evals

The ADK HTTP client is likewise transport-agnostic. Consumers plug in whatever auth they use via
`adk.WithTransport`:

```go
client := adk.NewHTTPClient(baseURL,
    adk.WithTransport(myAuthTransport),   // any http.RoundTripper
    adk.WithTimeout(30*time.Minute),      // override default 10 minutes
)
```

For Cloud Run–hosted sublaunchers, `adk.AudienceFromBaseURL(baseURL)` returns the audience string
callers pass into their own token source when building the transport.

---

## Package layout

| Path                    | Role                                                                                                                                     |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `evals`                 | Public authoring surface: `NewIntegrationSuite`, `NewAgentEvalSuite`, `NewLoadSuite`, `T`, `Call`, `Rouge1F1`, SLO constructors, registration functions. |
| `evals/adk`             | ADK evaluation-launcher client (transport-agnostic) and lazy `AgentEvalProvider`.                                                        |
| `evals/env`             | Shared environment registration and activation.                                                                                          |
| `evals/errors`          | `EvalError` interface + `ToGRPC` / `ToGRPCf` for RPC boundary translation.                                                               |
| `evals/execution`       | Proto-free in-process result types (boundary between case-facing and wire-facing).                                                       |
| `evals/loadgen`         | Embedded load generator (`Profile`, `Pacer`, `Generator`, `Metrics`).                                                                    |
| `evals/mapper`          | `execution` → `evalspb.Run` translation.                                                                                                 |
| `evals/registry`        | Registered suites, filter grammar, selection validation.                                                                                 |
| `evals/report`          | `Reporter` interface + `NoOpReporter`, `MultiReporter`.                                                                                  |
| `evals/report/log`      | Default log reporter (`log.Reporter`).                                                                                                   |
| `evals/report/bqschema` | Shared BigQuery schema + `EnsureTable` for Run rows. |
| `evals/report/bigquery` | BigQuery streaming reporter (`bigquery.Reporter`). |
| `evals/report/pubsub`   | JSON Pub/Sub reporter (`pubsub.Reporter`) on `pubsub/v2`. |
| `evals/runner`          | Environment activation, suite execution, panic recovery, status rollups.                                                                 |
| `evals/suite`           | Internal `TestSuite`, `EvalSuite`, `LoadSuite` primitives.                                                                               |

---

## End-to-end lifecycle

1. **Register.** Each case package calls `env.Register(...)` and one of `RegisterIntegration` /
   `RegisterEval` / `RegisterLoad` / `RegisterAgent` in its `init()` function.
2. **Wire.** The neuron's `TestServiceServer` is constructed with `evals.DefaultRegistry()`,
   `runner.New()`, and a reporter (default `logreport.Reporter{}`).
3. **RPC arrives.** `RunIntegrationTest` / `RunLoadTest` / `RunAgentEval`.
   `Registry.ValidateSelection` rejects unknown `case_ids` synchronously with `InvalidArgument`.
4. **LRO starts.** A long-running operation is created with initial metadata (`case_count`,
   `suite_count`). A resume task is scheduled; locally the `lro` library runs it in a goroutine via
   `httptest.NewRecorder`, in production it's dispatched via Cloud Tasks.
5. **Runner executes.** Environment setups fire once, then suites run sequentially. Each case runs
   under panic recovery — one bad case cannot take the batch down. LRO metadata is updated after
   each case (`completed_case_count`) and each suite (`completed_suite_count`).
6. **Map & report.** Each completed suite is mapped to `evalspb.Run` via `mapper` and passed to the
   configured `Reporter`. Reporter errors are logged; they do not fail the LRO.
7. **Complete.** The LRO completes with `RunXxxResponse.runs` listing the resource names of every
   emitted run (`runs/{run_id}`). Consumers fetch or subscribe to those.

Every case appears in the result — passing, failing, or `NOT_EVALUATED` — so dashboards can compute
pass rate, headroom, and trend without reconstructing what was intended to run.
