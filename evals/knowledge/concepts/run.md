---
type: Concept
title: Run
description: The top-level wire envelope covering every suite kind, published to reporters.
tags: [run, wire, proto]
timestamp: 2026-07-08T00:00:00Z
---

# Definition

A `Run` is the wire envelope emitted for every completed suite. It is
what reporters receive and what downstream sinks (Pub/Sub, BigQuery,
Spanner) ingest.

# Envelope

```protobuf
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

# Every case appears

Whether a case passed, failed, or was skipped, it appears in the
result. This lets dashboards compute pass rate, headroom, and trend
without reconstructing what was intended to run.

# Batch id

A single RPC can complete multiple suites. Every `Run` from that RPC
shares the same `batch_id`, so consumers can group them for aggregate
dashboards.

# The oneof

Exactly one of `integration_test`, `load_test`, `agent_eval` is set,
matching `Run.Type`. Consumers should switch on `Type` rather than
guessing from the `oneof` presence.

# Related

* [Wire types index](/wire-types/index.md)
* [Status](/concepts/status.md)
* [Reporter](/concepts/reporter.md)

# Citations

[1] [README — Wire types](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#wire-types)
