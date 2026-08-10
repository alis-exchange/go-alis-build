# evals

`go.alis.build/evals` defines and runs explicit evaluation suites from Go. It emits the shared `alis.evals.v1.Run` protobuf contract used by Pub/Sub, BigQuery, and downstream warehouse consumers.

The package intentionally has a small runtime surface:

- no registry;
- no `evals.T` recorder;
- no framework-managed environments, setup, teardown, clients, or credentials;
- no separate mapper/execution layer in user code.

Developers define named suites directly in Go, add named cases fluently, run them synchronously, and choose whether to publish the resulting `*evalspb.Run`.

## Suite types

| Suite             | Constructor                            | Case function                                          | Result branch             |
| ----------------- | -------------------------------------- | ------------------------------------------------------ | ------------------------- |
| Integration       | `evals.NewIntegrationSuite(name)`      | `func(context.Context, *validation.Validator)`         | `IntegrationTestResults`  |
| Agent eval        | `evals.NewAgentEvalSuite(name)`        | `func(context.Context, *evals.AgentEvalResult)`        | `AgentEvalResults`        |
| Load              | `evals.NewLoadSuite(name)`             | `func(context.Context, *evals.LoadResult)`             | `LoadTestResults`         |
| Infra observation | `evals.NewInfraObservationSuite(name)` | `func(context.Context, *evals.InfraObservationResult)` | `InfraObservationResults` |

Every suite supports:

```go
AddCase(name string, fn CaseFunc) *Suite
Run(ctx context.Context, opts ...evals.RunOption) (*evalspb.Run, error)
RunAndPublish(ctx context.Context, opts ...evals.RunOption) (*evalspb.Run, error)
```

`AddCase` returns the same suite for fluent definitions. Cases are identified on the wire as `{suite}.{case}`.

## Quickstart

```go
package checkout_evals

import (
    "context"
    "time"

    evalspb "go.alis.build/common/alis/evals"
    "go.alis.build/evals"
    "go.alis.build/validation"
)

func RunCheckoutRegression(ctx context.Context, client CheckoutClient) (*evalspb.Run, error) {
    suite := evals.NewIntegrationSuite("checkout-regression").
        AddCase("creates-order", func(ctx context.Context, v *validation.Validator) {
            start := time.Now()
            order, err := client.CreateOrder(ctx, validCreateOrderRequest())
            v.Custom("grpc.status_ok", err == nil)
            v.Custom("order.id_present", order.GetId() != "")
            v.Custom("latency.under_500ms", time.Since(start) < 500*time.Millisecond)
        }).
        AddCase("rejects-invalid-card", func(ctx context.Context, v *validation.Validator) {
            _, err := client.CreateOrder(ctx, invalidCardRequest())
            v.Custom("card.declined", isCardDeclined(err))
        })

    return suite.RunAndPublish(ctx,
        evals.WithOperation("operations/manual-checkout-regression"),
    )
}
```

Use normal Go for lifecycle:

```go
func RunWithFixtures(ctx context.Context, client CheckoutClient) (*evalspb.Run, error) {
    cleanup, err := seedCheckoutFixtures(ctx, client)
    if err != nil {
        return nil, err
    }
    defer cleanup(context.WithoutCancel(ctx))

    return evals.NewIntegrationSuite("checkout-fixtures").
        AddCase("reads-seeded-order", func(ctx context.Context, v *validation.Validator) {
            got, err := client.GetOrder(ctx, seededOrderName())
            v.Custom("read.seeded_order", err == nil && got.GetName() != "")
        }).
        Run(ctx)
}
```

## Running and publishing

`Run` executes cases and returns the materialized protobuf run. It does not report.

`RunAndPublish` executes the same path and then calls a reporter. If no reporter is supplied, the suite lazily creates the standard evals reporter and closes it after publication. Use `WithReporter` to replace that reporter for one run; custom reporters are borrowed and are not closed by the suite. To publish to more than one sink, pass `report.MultiReporter`, `report.All`, or another reporter implementation.

```go
r := report.MultiReporter{
    logreport.Reporter{},
    pubsubReporter,
}
run, err := suite.RunAndPublish(ctx, evals.WithReporter(r))
```

Reporter replacement is deliberate: `WithReporter(a)` followed by `WithReporter(b)` leaves `b` as the reporter. Developers who want fan-out should specify a multi-reporter explicitly.

`report.MultiReporter` is the fail-fast `report.FailFast` combinator. `report.All` invokes every reporter serially and joins all errors.

`RunAndPublish` publishes partial cancelled runs. If the run context is cancelled, active case functions may return on their own after observing cancellation; otherwise the evals runtime waits for active cases to return because Go cannot safely stop arbitrary goroutines. Cases that never started are emitted as `NOT_EVALUATED` with the framework `_evals.skipped` marker for compatibility.

Publication gets its own 10-second timeout derived with `context.WithoutCancel` from the execution context. This preserves context values while removing the execution context's cancellation and deadline before applying the fresh publication deadline. `RunAndPublish` can therefore deliver a partial run after execution cancellation or expiry. Reporter failures and timeout errors are returned alongside the materialized run.

## Run options

| Option                    | Behavior                                                                                                         |
| ------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| `WithMaxConcurrency(n)`   | Sets the maximum number of active cases. Default is `1` for every suite type. `n <= 0` is a configuration error. |
| `WithReporter(r)`         | Replaces the reporter used by `RunAndPublish`. Nil is a configuration error.                                     |
| `WithBatchID(id)`         | Sets `Run.batch_id` when non-empty.                                                                              |
| `WithOperation(name)`     | Sets `Run.operation` when non-empty.                                                                             |
| `WithGoogleProjectID(id)` | Sets `Run.google_project_id`. If omitted, `ALIS_OS_PROJECT` is used.                                             |

Suite and case configuration errors are deferred until `Run` / `RunAndPublish` so fluent definitions can be built linearly and checked once.

## Execution semantics

- A suite seals its case list on the first run.
- Re-running a sealed suite is allowed, including concurrent runs.
- Adding a case after sealing records a configuration error on the next run.
- Cases run sequentially by default.
- `WithMaxConcurrency` bounds parallel cases while preserving result order in the original `AddCase` order.
- Case panics are recovered at the case boundary and mark that case failed.
- Case failures are result data, not Go errors. Operational errors such as cancellation or reporter failure are returned as Go errors and still preserve the partial `*evalspb.Run` when available.
- A suite with no cases is a valid no-op run and has status `PASSED`. A registered case that emits no evaluation data is `NOT_EVALUATED`.

## Specialized result builders

The three specialized builders share these rules:

- `Validator()` returns a case-local validator; broken rules populate `validations` and fail the case.
- `Fail(nil)` is a no-op. `Fail(err)` appends an ordered `_evals.case` validation and retains partial data.
- Added protobuf messages are cloned, so later caller mutation cannot change the run.
- Nil protobuf inputs fail the case without discarding previously added values.
- Singleton setters keep their first value; repeated setters fail the case without replacing it.
- An empty builder is `NOT_EVALUATED`.

Agent session ID and judge info, load summary, and infra observation window are singleton values. Metrics, SLO checks, tags, validations, and snapshots preserve insertion order.

## Integration suites

Integration cases receive a fresh `*validation.Validator`.

```go
suite := evals.NewIntegrationSuite("checkout-integration").
    AddCase("get-order", func(ctx context.Context, v *validation.Validator) {
        got, err := client.GetOrder(ctx, req)
        v.Custom("grpc.status_ok", err == nil)
        v.Custom("order.present", got.GetName() != "")
    })
```

At the end of the case the evals runtime reads `validator.Rules()` and maps them to integration checks. Broken rules fail the case. Cases with no rules are `NOT_EVALUATED`.

Normal Go errors stay normal inside the case. Convert them to validation rules explicitly when they are part of the evaluation result.

## Agent eval suites

Agent eval cases receive `*evals.AgentEvalResult`, a branch-specific builder. It accepts protobuf-native values and exposes a case-local validator for general validation rules.

```go
suite := evals.NewAgentEvalSuite("assistant-quality").
    AddCase("answers-correctly", func(ctx context.Context, r *evals.AgentEvalResult) {
        result, err := runAgentScenario(ctx)
        if err != nil {
            r.Fail(err)
            return
        }
        r.SetSessionID(result.SessionID)
        r.AddMetric(&evalspb.AgentEvalResults_Case_Metric{
            Id:        "rubric_based_final_response_quality_v1",
            Status:    evalspb.Status_PASSED,
            Score:     proto.Float64(0.91),
            Threshold: proto.Float64(0.70),
        })
        r.SetJudgeInfo(&evalspb.AgentEvalResults_JudgeInfo{
            Model:          "gemini-2.5-pro",
            JudgeCallCount: 1,
        })
        r.Validator().Custom("session.persisted", result.SessionID != "")
    })
```

`Fail(err)` marks the case failed while preserving already-added result data. Builder validation failures are emitted under the additive `validations` field on specialized result branches.

`evals/adk` remains available for ADK-specific helpers. `Provider.Run` returns `[]adk.ProviderResult` containing suite names, measured set-level timestamps, and protobuf-native `*evalspb.AgentEvalResults`. Calling `ProviderResult.Run()` constructs a complete agent-eval envelope with status rollup but does not publish it. The caller owns metadata and reporter invocation.

ADK exposes only total eval-set duration, so the provider divides it evenly across returned cases. Treat those case durations as approximations; `ProviderResult.StartTime` and `EndTime` preserve the measured set-level interval.

## Load suites

Load cases receive `*evals.LoadResult`, a protobuf-native builder. Use `evals/loadgen` for optional traffic generation and summary conversion, or use your own load tool and add the resulting protobuf values explicitly.

```go
suite := evals.NewLoadSuite("checkout-capacity").
    AddCase("steady-traffic", func(ctx context.Context, r *evals.LoadResult) {
        profile := loadgen.Profile{
            QPS:         100,
            Concurrency: 25,
            Duration:    time.Minute,
        }
        metrics, err := loadgen.Run(ctx, profile, target)
        if err != nil {
            r.Fail(err)
            return
        }
        r.SetSummary(loadgen.Summary(loadgen.Moderate, profile, metrics))
        r.AddSLOCheck(&evalspb.LoadTestResults_SloCheck{
            Id:       "latency.p99_ms",
            Status:   evalspb.Status_PASSED,
            Observed: metrics.Latency.P99Ms,
            Limit:    500,
            Unit:     "ms",
        })
        r.AddTag("rpc", "CreateOrder")
    })
```

Default concurrency is one active case. `WithMaxConcurrency` can run load cases in parallel, but parallel load cases combine their traffic and can distort measurements. Use it only when the combined traffic is the scenario being evaluated.

For load-integrated infrastructure diagnostics, observe the exact measurement
window and add the returned snapshots explicitly:

```go
observed, err := loadinfra.ObserveLoad(ctx, metricClient, targets, metrics)
if err != nil {
    r.Fail(err)
    return
}
for _, snapshot := range observed.CloudRun {
    r.AddCloudRunSnapshot(snapshot)
}
for _, snapshot := range observed.Spanner {
    r.AddSpannerSnapshot(snapshot)
}
```

`ObserveLoad` requires non-nil metrics. Use `ObserveLookback` for standalone
settled windows or `Observe` with a named `loadinfra.Request` for a custom
window and target concurrency.

For client-streaming load targets, copy the timing returned by `evals.CallClientStream` into the load generator's protobuf-independent stream sample:

```go
target := func(ctx context.Context, _ loadgen.CallData) loadgen.TargetResult {
    got := evals.CallClientStream(ctx, openStream, sendRequests)
    return loadgen.TargetResult{
        TransportErr: got.Err,
        Stream: &loadgen.StreamSample{
            SendDuration:    got.SendDuration,
            ResponseLatency: got.ResponseLatency,
            TotalDuration:   got.TotalDuration,
            MessagesSent:    got.MessagesSent,
        },
    }
}
```

This explicit bridge keeps `loadgen` independent of the root evals package while preserving stream aggregation in `loadgen.Metrics.Stream`.

## Infra observation suites

Infra observation cases receive `*evals.InfraObservationResult`. Use `evals/loadinfra` to collect Cloud Run and Spanner Monitoring snapshots, or add protobuf snapshots explicitly.

```go
suite := evals.NewInfraObservationSuite("checkout-runtime").
    AddCase("peak-window", func(ctx context.Context, r *evals.InfraObservationResult) {
        client, err := loadinfra.NewMetricClient(ctx)
        if err != nil {
            r.Fail(err)
            return
        }
        defer client.Close()

        obs, err := loadinfra.ObserveLookback(ctx, client, loadinfra.Targets{
            CloudRun: cloudTargets,
            Spanner:  spannerTargets,
        }, 30*time.Minute)
        if err != nil {
            r.Fail(err)
            return
        }
        r.SetWindow(30*time.Minute, obs.Window.Start, obs.Window.End)
        for _, snapshot := range obs.CloudRun {
            r.AddCloudRunSnapshot(snapshot)
        }
        for _, snapshot := range obs.Spanner {
            r.AddSpannerSnapshot(snapshot)
        }
    })
```

Standalone infra observation cases fail when an added Cloud Run or Spanner snapshot has `FetchStatus == INFRA_FETCH_STATUS_UNAVAILABLE`. That preserves the existing result semantics without requiring an extra validation row.

## Result contract

Existing `evalspb.Run` fields keep their branch-native placement and types. The result contract adds only `repeated Validation validations` to specialized cases:

- `AgentEvalResults.Case.validations`
- `LoadTestResults.Case.validations`
- `InfraObservationResults.Case.validations`

Integration results continue to use `checks`.

`validation.Validator` exposes a rule description and satisfied state but not a separate legacy failed-check message. Integration check-message parity is therefore the approved limitation: failed check messages use the rule description.

Maintainer details for the normalized four-branch parity gate are documented
in [knowledge/wire-types/run.md](knowledge/wire-types/run.md).

## Reporters

`report.Reporter` is the only reporting interface:

```go
type Reporter interface {
    ReportRun(context.Context, *evalspb.Run) error
}
```

Bundled reporters include:

- `report/log` for alog summaries;
- `report/pubsub` for protojson `Run` payloads on topic `alis.evals.v1.Run`;
- `report/bigquery` for streaming inserts;
- `report/bqschema` for canonical BigQuery schema generation/provisioning.

For Pub/Sub, the reporter project is the product project (`ALIS_OS_PRODUCT_PROJECT`) because the topic and BigQuery subscription live there. For the `Run.google_project_id` field, evals uses `ALIS_OS_PROJECT` by default and `WithGoogleProjectID` overrides it.

## Package layout

| Package                          | Purpose                                                                          |
| -------------------------------- | -------------------------------------------------------------------------------- |
| `go.alis.build/evals`            | Four typed suites, builders, run options, call/stream helpers, scoring helpers.  |
| `go.alis.build/evals/adk`        | ADK launcher helpers and protobuf-native agent eval conversion.                  |
| `go.alis.build/evals/loadgen`    | Focused load generation algorithms and `Summary` conversion. No suite lifecycle. |
| `go.alis.build/evals/loadinfra`  | Focused Cloud Monitoring collection and protobuf snapshots. No suite lifecycle.  |
| `go.alis.build/evals/report`     | Reporter interface and fan-out combinators.                                      |
| `go.alis.build/evals/report/...` | Log, Pub/Sub, BigQuery, and schema helpers.                                      |
| `go.alis.build/evals/errors`     | gRPC status bridging for typed evals errors.                                     |
