---
type: Wire Type
title: IntegrationTestResults
description: One `Case` per test, each with a list of `Check` leaves.
tags: [wire, proto, integration]
timestamp: 2026-07-08T00:00:00Z
---

# Definition

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

# Field notes

- `Case.id` — `{suite}.{case}`, matching the filter grammar.
- `Case.status` — rolled up from every `Check`. `PASSED` iff every
  check passed; `FAILED` if any failed; `NOT_EVALUATED` if the case
  was skipped.
- `Case.duration` — wall-clock time for the case function.
- `Check.id` — the id passed to `t.Check`, `t.NoErr`, etc.
- `Check.message` — empty on pass. Failure formatting depends on the
  primitive:
  - `NoErr` failure → `err.Error()`.
  - `Max` failure → `id 350ms exceeds limit 300ms` (values inline).
  - `Checkf` failure → the format string with args applied.

# Sentinel leaves

- `duplicate-check-id` (id `evals.DuplicateCheckIDName`) — recorded
  once per case when the same id is reused.
- `panic` — recorded when the case function panics; the message
  contains the panic value and a truncated stack.
- `setup` — recorded on every case in a suite whose `WithSetup`
  hook returned an error.

# Related

* [T methods](/api/t-methods.md)
* [Case](/concepts/case.md)
* [Status](/concepts/status.md)

# Citations

[1] [README — Integration wire](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#integration)
[2] [mapper/mapper.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/mapper/mapper.go)
