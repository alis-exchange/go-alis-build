# Wire types

Proto messages that consumers see. The framework produces these; the
proto module `go.alis.build/common/alis/evals/v1` defines them.

* [Run](/wire-types/run.md) - top-level envelope, common to every kind.
* [IntegrationTestResults](/wire-types/integration-results.md) - one `Case` per test, each with `Check` leaves.
* [LoadTestResults](/wire-types/load-results.md) - `Summary` + `SloCheck` leaves per case.
* [AgentEvalResults](/wire-types/agent-eval-results.md) - `Metric` leaves per case, plus optional `JudgeInfo`.

Consumers that ingest runs from Pub/Sub or BigQuery pin the proto
module directly. The Go framework imports it as `evalspb`.
