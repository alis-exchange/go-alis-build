---
title: packages
description: Current package map.
tags: [packages]
---

# Packages

| Package | Purpose |
| --- | --- |
| `go.alis.build/evals` | Typed suites, builders, options, call/stream helpers, scoring helpers. |
| `go.alis.build/evals/adk` | ADK helper and protobuf-native agent eval conversion. |
| `go.alis.build/evals/loadgen` | Focused load generation and `Summary` conversion. |
| `go.alis.build/evals/loadinfra` | Focused Cloud Monitoring collection and protobuf snapshots. |
| `go.alis.build/evals/report` | Reporter interface and fan-out combinators. |
| `go.alis.build/evals/report/log` | alog summary reporter. |
| `go.alis.build/evals/report/pubsub` | Pub/Sub reporter for protojson `Run` payloads. |
| `go.alis.build/evals/report/bigquery` | BigQuery streaming insert reporter. |
| `go.alis.build/evals/report/bqschema` | Canonical BigQuery schema helpers. |
| `go.alis.build/evals/errors` | gRPC status bridging for typed errors. |

Deleted registry-era packages are intentionally absent: `env`, `suite`,
`registry`, `runner`, `mapper`, `execution`, `harness`, and `verdict`.
