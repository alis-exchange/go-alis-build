---
type: Wire Type
title: Run envelope
description: The top-level `Run` proto — common to integration, load, agent-eval, and infra-observation results.
resource: https://buf.build/googleapis/api-common-protos
tags: [wire, proto, run]
timestamp: 2026-07-08T00:00:00Z
---

# Definition

```protobuf
enum Status {
  STATUS_UNSPECIFIED = 0;
  PASSED             = 1;   // executed and every check passed
  FAILED             = 2;   // executed and one or more checks failed
  NOT_EVALUATED      = 3;   // selected but skipped (StopOnFailure or cancellation)
}

message Run {
  string      name        = 2;   // runs/{run_id}
  optional    string batch_id = 3;
  Run.Type    type        = 4;   // INTEGRATION_TEST | LOAD_TEST | AGENT_EVAL | INFRA_OBSERVATION
  Status      status      = 5;
  oneof data {
    IntegrationTestResults integration_test = 6;
    LoadTestResults        load_test        = 7;
    AgentEvalResults       agent_eval       = 8;
    InfraObservationResults infra_observation = 9;
  }
  Timestamp   start_time  = 21;
  Timestamp   end_time    = 22;
  string      operation   = 23;   // operations/{op_id}
  rpc.Status  error       = 24;
  Timestamp   create_time = 25;
  string      google_project_id = 26;
}
```

# Field notes

- `name` — resource name `runs/{run_id}`. Consumers store runs under
  this key.
- `batch_id` — shared across runs produced by one integration, load, or
  infrastructure-observation RPC. Agent-eval requests currently do not supply
  a batch identifier, so their runs omit this field.
- `type` — matches the RPC that produced this run.
- `oneof data` — exactly one of the four result messages is set.
  Switch on `type` rather than guessing from the `oneof` presence.
- `operation` — resource name of the LRO that produced this run.
- `error` — part of the shared wire envelope but currently left unset by the
  evals mappers. Setup and case failures are represented in the selected case
  results; operation-level failures are returned by the RPC/LRO layer.

# Every case appears

Whether a selected case passed, failed, or was skipped, it appears in the
appropriate `results` message. Dashboards can compute pass rate,
headroom, and trend without reconstructing what was intended to run.

# Related

* [Run concept](/concepts/run.md)
* [Status enum](/concepts/status.md)
* [IntegrationTestResults](/wire-types/integration-results.md)
* [LoadTestResults](/wire-types/load-results.md)
* [AgentEvalResults](/wire-types/agent-eval-results.md)

# Citations

[1] [README — Wire types](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#wire-types)
