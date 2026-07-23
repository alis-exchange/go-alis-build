---
title: evals overview
description: High-level model for the typed evals runtime.
tags: [overview, typed-suites]
---

# evals overview

The package builds `alis.evals.v1.Run` protobufs from named Go suites.

Each suite:

1. is constructed with an explicit name;
2. registers named cases with fluent `AddCase`;
3. runs synchronously with default one active case;
4. returns a deterministic branch-specific `*evalspb.Run`;
5. optionally publishes through `RunAndPublish`.

The public runtime is intentionally small. Cases are normal functions and may
call normal Go setup/cleanup, clients, fixtures, and helper packages. The evals
package owns only the evaluation envelope: case ordering, panic recovery,
bounded concurrency, cancellation accounting, status rollup, timestamps, and
publication.

Specialized branches use result builders that accept protobuf-native values.
That keeps the existing result contract stable while removing older abstraction
layers.
