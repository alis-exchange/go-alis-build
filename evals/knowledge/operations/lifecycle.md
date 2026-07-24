---
title: lifecycle
description: Runtime lifecycle for typed eval suites.
tags: [lifecycle, run, publish]
---

# Lifecycle

1. Construct a named typed suite.
2. Add named cases with fluent `AddCase`.
3. Run the suite with `Run` or `RunAndPublish`.
4. Receive a materialized `*evalspb.Run`.
5. If using `RunAndPublish`, the selected reporter receives that run.

There is no startup registration or freeze step.

The first run seals the suite definition. Repeated and concurrent runs use
snapshots of that sealed definition. `AddCase` after sealing does not mutate an
active run; it records `ErrSuiteSealed` for the next run.

Configuration errors are deferred so fluent definitions can be checked once.
Invalid suite/case names, duplicate cases, nil functions/options/reporters, late
`AddCase`, and invalid concurrency are returned together as `*evals.ConfigErrors`
before execution.

Cases execute synchronously under a bounded scheduler. The scheduler:

- defaults to one active case for every suite type;
- preserves `AddCase` order even when parallel cases finish out of order;
- recovers each panic and fails only that case;
- stops scheduling replacement cases after cancellation;
- propagates cancellation to active cases and waits for them to return;
- emits unstarted cases as `NOT_EVALUATED` with `_evals.skipped`; and
- returns the materialized partial run alongside the cancellation error.

Go cannot stop an arbitrary case goroutine. Case functions should observe their
context when they need prompt cancellation.

For setup and cleanup, write normal Go around the suite call:

```go
cleanup, err := seed(ctx)
if err != nil {
    return nil, err
}
defer cleanup(context.WithoutCancel(ctx))

return suite.RunAndPublish(ctx)
```
