# Packages

One page per Go subpackage in `go.alis.build/evals`.

# Public authoring surface

* [`evals`](/packages/evals.md) - the root package. `NewIntegrationSuite`, `NewAgentEvalSuite`, `NewLoadSuite`, `T`, `Call`, `Rouge1F1`, SLO constructors, registration functions.

# Runtime subpackages

* [`evals/adk`](/packages/adk.md) - transport-agnostic ADK evaluation-launcher client and lazy `AgentEvalProvider`.
* [`evals/env`](/packages/env.md) - shared environment registration and activation.
* [`evals/errors`](/packages/errors.md) - `EvalError` interface and gRPC translation helpers.
* [`evals/execution`](/packages/execution.md) - proto-free in-process result types.
* [`evals/loadgen`](/packages/loadgen.md) - embedded load generator.
* [`evals/mapper`](/packages/mapper.md) - `execution` → `evalspb.Run` translation.
* [`evals/registry`](/packages/registry.md) - registered suites, filter grammar, selection validation.
* [`evals/report`](/packages/report.md) - `Reporter` interface + `NoOpReporter`, `MultiReporter`.
* [`evals/report/log`](/packages/report-log.md) - default log reporter (`log.Reporter`).
* [`evals/report/bqschema`](/packages/report-bqschema.md) - shared BigQuery schema + `EnsureTable` provisioning.
* [`evals/report/bigquery`](/packages/report-bigquery.md) - BigQuery streaming reporter.
* [`evals/report/pubsub`](/packages/report-pubsub.md) - JSON Pub/Sub reporter for bare `evalspb.Run`.
* [`evals/runner`](/packages/runner.md) - environment activation, suite execution, panic recovery.
* [`evals/suite`](/packages/suite.md) - internal `TestSuite`, `EvalSuite`, `LoadSuite` primitives.
