---
type: Playbook
title: StopOnFailure — stateful test flows
description: Mark a suite so remaining cases skip once any case fails. Useful for create → get → update → delete.
tags: [stop-on-failure, stateful, integration]
timestamp: 2026-07-08T00:00:00Z
---

# When to use

Some test flows only make sense as a linear sequence:

- `create → get → update → delete`
- `enable-feature → verify-side-effect → disable-feature`

If `create` fails, running `get` measures nothing meaningful. Mark the
suite with `evals.StopOnFailure()`; every case after the first failure
is recorded `NOT_EVALUATED` with a "preceding case … failed" reason.

# Example

```go
s := evals.NewSuite("orders-lifecycle-v1",
    evals.WithEnv("example-v1"),
    evals.StopOnFailure(),
)

s.Case("create", createOrder)
s.Case("get",    getOrder)
s.Case("update", updateOrder)
s.Case("delete", deleteOrder)

evals.RegisterIntegration(s)
```

If `get` fails, both `update` and `delete` skip. Their `Case` messages
still appear on the wire with status `NOT_EVALUATED`.

# When not to use

- **Independent cases.** If your suite is a set of unrelated checks
  against the same env, don't skip on the first failure — you want
  the whole failure surface, not one leaf.
- **Load suites.** `StopOnFailure` is not available for load suites;
  load cases run sequentially anyway but one case failing does not
  invalidate the next case's measurement.
- **Eval suites.** Available in principle (eval suites use the same
  `SuiteOption`) but usually not what you want: eval cases are meant
  to be graded independently.

# Wire behavior

- The failing case surfaces with `FAILED` and its full check list.
- Every subsequent case surfaces as one `Case` with status
  `NOT_EVALUATED` and no `Check`/`Metric` leaves.
- Suite/run status rolls up to `FAILED` — the run is not "half-run",
  it's a definitively failed sequence.

# Related

* [Suite](/concepts/suite.md)
* [Shared suite options](/api/suite-options.md)
* [Status](/concepts/status.md)

# Citations

[1] [suite.go — StopOnFailure](https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite.go)
[2] [runner/runner.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/runner/runner.go)
